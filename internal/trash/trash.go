// Package trash implements explicit, restorable cleanup. It never permanently deletes.
package trash

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/fum4/oh-my-disk/internal/report"
	"github.com/fum4/oh-my-disk/internal/rules"
	"github.com/fum4/oh-my-disk/internal/scanner"
)

type Result struct {
	Path    string `json:"path"`
	Error   string `json:"error,omitempty"`
	Trashed bool   `json:"trashed"`
}

func within(root, p string) bool {
	return p == root || strings.HasPrefix(p, strings.TrimSuffix(root, "/")+"/")
}

// Blocked also protects ancestors of protected user data: selecting ~/Pictures
// must not bypass the Photos Library protection.
func Blocked(p, root, home string, privateCache bool) error {
	p = filepath.Clean(p)
	root = filepath.Clean(root)
	if !filepath.IsAbs(p) || !within(root, p) || p == root {
		return fmt.Errorf("outside scan or scan root itself")
	}
	for _, b := range []string{"/System", "/Library", "/Applications", "/usr", "/bin", "/sbin", "/private"} {
		if within(b, p) {
			if b == "/private" && strings.HasPrefix(p, "/private/var/folders/") && privateCache {
				continue
			}
			return fmt.Errorf("protected system path")
		}
	}
	if p == "/" || p == home || p == filepath.Join(home, "Library") {
		return fmt.Errorf("protected home or Library")
	}
	for _, rel := range []string{"Documents", "Desktop", "Library/Mobile Documents", "Library/CloudStorage", "Library/Logs/oh-my-disk"} {
		b := filepath.Join(home, rel)
		if within(b, p) || within(p, b) {
			return fmt.Errorf("protected user data")
		}
	}
	photos := filepath.Join(home, "Pictures")
	if within(p, photos) || strings.HasPrefix(p, photos+"/Photos Library") {
		return fmt.Errorf("protected Photos Library")
	}
	return nil
}
func Validate(s *report.Snapshot, p, home string, e *rules.Engine) (*scanner.Node, error) {
	if p != filepath.Clean(p) || !filepath.IsAbs(p) {
		return nil, fmt.Errorf("absolute clean path required")
	}
	n := scanner.Find(s.Scan.Root, p)
	if n == nil {
		return nil, fmt.Errorf("path absent from scan")
	}
	privateCache := false
	if e != nil {
		if r := e.Match(p, false); r != nil {
			privateCache = r.Category == "caches" && r.Safety == "cache"
		}
	}
	if err := Blocked(p, s.Scan.Root.Path, home, privateCache); err != nil {
		return nil, err
	}
	// Resolve each component using Lstat. No symlink, including an ancestor, is eligible.
	for cur := p; ; cur = filepath.Dir(cur) {
		fi, err := os.Lstat(cur)
		if err != nil {
			return nil, err
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("symlink path blocked")
		}
		if cur == "/" {
			break
		}
	}
	fi, err := os.Lstat(p)
	if err != nil {
		return nil, err
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok || n.Inode == 0 || uint64(st.Ino) != n.Inode || uint64(st.Dev) != n.Device {
		return nil, fmt.Errorf("identity changed or unavailable; rescan required")
	}
	if strings.HasPrefix(p, "/private/var/folders/") && st.Uid != uint32(os.Getuid()) {
		return nil, fmt.Errorf("private cache is not owned by current user")
	}
	if n.Kind == "symlink" || n.Kind == "special" {
		return nil, fmt.Errorf("unsupported entry kind")
	}
	// A directory selection must not conceal protected descendants.
	if fi.IsDir() {
		if err := filepath.WalkDir(p, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if path == p {
				return nil
			}
			return Blocked(path, s.Scan.Root.Path, home, privateCache)
		}); err != nil {
			return nil, err
		}
	}
	return n, nil
}
func audit(f *os.File, v any) error {
	if err := json.NewEncoder(f).Encode(v); err != nil {
		return err
	}
	return f.Sync()
}
func Execute(s *report.Snapshot, paths []string, home string, e *rules.Engine) []Result {
	return execute(s, paths, home, e, move)
}
func execute(s *report.Snapshot, paths []string, home string, e *rules.Engine, mover func(string, uint64, uint64) error) []Result {
	out := []Result{}
	if len(paths) == 0 || len(paths) > 100 {
		return []Result{{Error: "select 1–100 items"}}
	}
	logPath := filepath.Join(home, "Library/Logs/oh-my-disk/trash.log")
	// Refuse redirected audit destinations.
	dir := filepath.Dir(logPath)
	err := os.MkdirAll(dir, 0700)
	if err == nil {
		for cur := dir; cur != filepath.Dir(cur); cur = filepath.Dir(cur) {
			fi, x := os.Lstat(cur)
			if x != nil {
				err = x
				break
			}
			if fi.Mode()&os.ModeSymlink != 0 {
				err = fmt.Errorf("symlink audit directory")
				break
			}
		}
	}
	var f *os.File
	if err == nil {
		var fd int
		fd, err = syscall.Open(logPath, syscall.O_WRONLY|syscall.O_APPEND|syscall.O_CREAT|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0600)
		if err == nil {
			f = os.NewFile(uintptr(fd), logPath)
			defer f.Close()
			if fi, x := f.Stat(); x != nil {
				err = x
			} else if !fi.Mode().IsRegular() {
				err = fmt.Errorf("audit destination is not a regular file")
			}
		}
	}
	seen := map[string]bool{}
	for _, p := range paths {
		r := Result{Path: p}
		actionErr := err
		if actionErr == nil && seen[p] {
			actionErr = fmt.Errorf("duplicate selection")
		}
		seen[p] = true
		if actionErr == nil {
			_, actionErr = Validate(s, p, home, e)
		}
		if actionErr == nil {
			actionErr = audit(f, map[string]any{"time": time.Now().UTC(), "path": p, "status": "requested"})
		}
		if actionErr == nil { // Revalidate immediately before the platform Trash call.
			var n *scanner.Node
			n, actionErr = Validate(s, p, home, e)
			if actionErr == nil {
				actionErr = mover(p, n.Device, n.Inode)
				r.Trashed = actionErr == nil
			}
		}
		if actionErr != nil {
			r.Error = actionErr.Error()
		}
		if f != nil {
			if x := audit(f, map[string]any{"time": time.Now().UTC(), "result": r}); x != nil {
				r.Error += " audit result: " + x.Error()
			}
		}
		out = append(out, r)
	}
	return out
}
