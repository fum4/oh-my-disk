package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/fum4/oh-my-disk/internal/rules"
	"github.com/fum4/oh-my-disk/internal/scanner"
)

const Note = "Read-only analysis. Commands are suggestions for a human; never executed. Dataless cloud placeholders contribute logical bytes without opening file contents; other entries use allocated blocks. Allocated blocks are not guaranteed reclaimable space (APFS clones, snapshots, compression, hardlinks, sparse images). Access times may be disabled or affected by reading directories. Skipped paths make totals incomplete."

type Snapshot struct {
	Version     int                `json:"version"`
	Scan        *scanner.Result    `json:"scan"`
	Suggestions []rules.Suggestion `json:"suggestions"`
	Categories  map[string]int64   `json:"categories"`
	Note        string             `json:"note"`
}

func New(scan *scanner.Result, e *rules.Engine, min int64) *Snapshot {
	var s []rules.Suggestion
	var c map[string]int64
	if scan.Tree != nil {
		s, c = e.AnalyzeTree(scan.Tree, time.Now(), min)
		scan.Root = scan.Tree.View(0, 1, 0)
	} else {
		s, c = e.Analyze(scan.Root, time.Now(), min)
	}
	return &Snapshot{1, scan, s, c, Note}
}
func CachePath() string {
	return filepath.Join(os.TempDir(), fmt.Sprintf("oh-my-disk-%d", os.Getuid()), "latest.bin")
}
func SaveCache(path string, s *Snapshot) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	fi, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if !fi.IsDir() || fi.Mode()&0077 != 0 {
		return fmt.Errorf("cache directory must be private (0700), not a symlink: %s", dir)
	}
	if existing, e := os.Lstat(path); e == nil {
		if !existing.Mode().IsRegular() {
			return fmt.Errorf("refusing to replace non-regular cache: %s", path)
		}
		if e = validateExisting(path); e != nil {
			return fmt.Errorf("refusing to overwrite a file that is not an oh-my-disk cache: %w", e)
		}
	} else if !os.IsNotExist(e) {
		return e
	}
	f, err := os.CreateTemp(dir, ".scan-*.bin")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name) // Only removes the application's own temporary report.
	if err = writeCache(f, s); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
func LoadCache(path string) (*Snapshot, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return readCache(f)
}

// Prune copies the tree, preserving aggregated totals even when children are omitted.
func Prune(n *scanner.Node, depth int, min int64) *scanner.Node {
	if t := n.CompactTree(); t != nil {
		return t.View(n.ID, depth, min)
	}
	c := *n
	c.Children = nil
	if depth != 0 {
		next := depth
		if next > 0 {
			next--
		}
		for _, child := range n.Children {
			if child.Bytes >= min {
				c.Children = append(c.Children, Prune(child, next, min))
			}
		}
	}
	return &c
}

type ExportView struct {
	Snapshot *Snapshot
	Depth    int
	Min      int64
}

func Export(s *Snapshot, depth int, min int64) ExportView { return ExportView{s, depth, min} }

func Top(root *scanner.Node, limit int) []*scanner.Node {
	if t := root.CompactTree(); t != nil {
		return topCompact(t, limit)
	}
	nodes := []*scanner.Node{}
	scanner.Walk(root, func(n *scanner.Node) {
		if n != root && n.Bytes > 0 {
			nodes = append(nodes, n)
		}
	})
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Bytes == nodes[j].Bytes {
			return nodes[i].Path < nodes[j].Path
		}
		return nodes[i].Bytes > nodes[j].Bytes
	})
	if limit >= 0 && len(nodes) > limit {
		nodes = nodes[:limit]
	}
	return nodes
}
func Summary(s *Snapshot, limit int) any {
	type item struct {
		Path  string `json:"path"`
		Bytes int64  `json:"bytes"`
		Files int64  `json:"files"`
	}
	top := []item{}
	for _, n := range Top(s.Scan.Root, limit) {
		top = append(top, item{n.Path, n.Bytes, n.Files})
	}
	sug := s.Suggestions
	if len(sug) > limit {
		sug = sug[:limit]
	}
	return struct {
		Root        string             `json:"root"`
		Scanned     time.Time          `json:"scanned"`
		Bytes       int64              `json:"allocated_bytes"`
		Skipped     int                `json:"skipped_count"`
		Categories  map[string]int64   `json:"categories"`
		Top         []item             `json:"top_overlapping_items"`
		Suggestions []rules.Suggestion `json:"disjoint_suggestions"`
		Note        string             `json:"note"`
	}{s.Scan.Root.Path, s.Scan.Finished, s.Scan.Root.Bytes, len(s.Scan.Skipped), s.Categories, top, sug, Note}
}
func HumanBytes(b int64) string {
	if b < 1024 {
		return fmt.Sprintf("%d B", b)
	}
	v := float64(b)
	u := []string{"KiB", "MiB", "GiB", "TiB", "PiB"}
	for _, unit := range u {
		v /= 1024
		if v < 1024 || unit == "PiB" {
			return fmt.Sprintf("%.1f %s", v, unit)
		}
	}
	return ""
}

// WriteJSON creates only new report files. Existing files are never overwritten.
func WriteJSON(path string, v any) error {
	if path == "-" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return encodeOutput(os.Stdout, enc, v)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return encodeOutput(f, enc, v)
}
