// Package scanner only reads filesystem metadata. It never opens file contents,
// follows symbolic links, or performs cleanup operations.
package scanner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"syscall"
	"time"
)

type Node struct {
	Name              string    `json:"name"`
	Path              string    `json:"path"`
	Kind              string    `json:"kind"`
	Category          string    `json:"category,omitempty"`
	Bytes             int64     `json:"bytes"`
	OwnBytes          int64     `json:"own_bytes"`
	Files             int64     `json:"files"`
	Modified          time.Time `json:"modified"`
	Accessed          time.Time `json:"accessed"`
	LatestModified    time.Time `json:"latest_modified"`
	LatestAccessed    time.Time `json:"latest_accessed"`
	Children          []*Node   `json:"children,omitempty"`
	HardlinkDuplicate bool      `json:"hardlink_duplicate,omitempty"`
	Incomplete        bool      `json:"incomplete,omitempty"`
	id                identity
	links             uint64
}
type Skipped struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}
type Result struct {
	Root        *Node     `json:"root"`
	Started     time.Time `json:"started"`
	Finished    time.Time `json:"finished"`
	Skipped     []Skipped `json:"skipped"`
	Entries     int64     `json:"entries"`
	CrossMounts bool      `json:"cross_mounts"`
}
type Progress struct {
	Entries     int64
	Directories int64
	Pending     int
}
type Options struct {
	Workers     int
	CrossMounts bool
	Exclude     []string
	Progress    func(Progress)
}
type batch struct {
	parent   *Node
	children []*Node
	skipped  []Skipped
	err      error
}

func fromInfo(path string, info os.FileInfo) *Node {
	b, id, l := diskInfo(info)
	kind := "file"
	if info.IsDir() {
		kind = "directory"
	} else if info.Mode()&os.ModeSymlink != 0 {
		kind = "symlink"
	} else if !info.Mode().IsRegular() {
		kind = "special"
	}
	return &Node{Name: info.Name(), Path: path, Kind: kind, OwnBytes: b, Modified: info.ModTime(), Accessed: accessTime(info), id: id, links: l}
}

func Scan(ctx context.Context, root string, opts Options) (*Result, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("scan root must be a real directory (symlinks are not followed): %s", root)
	}
	n := fromInfo(root, info)
	result := &Result{Root: n, Started: time.Now(), CrossMounts: opts.CrossMounts, Skipped: []Skipped{}}
	excludes := map[string]bool{}
	for _, p := range opts.Exclude {
		p, e := filepath.Abs(p)
		if e != nil {
			return nil, e
		}
		excludes[filepath.Clean(p)] = true
	}
	if excludes[root] {
		return nil, fmt.Errorf("scan root is excluded: %s", root)
	}

	allowedDevices, skipData := devicePolicy(runtime.GOOS, root, n.id.dev, func() (uint64, error) {
		data, e := os.Lstat("/System/Volumes/Data")
		if e != nil {
			return 0, e
		}
		_, id, _ := diskInfo(data)
		return id.dev, nil
	})
	if skipData {
		excludes["/System/Volumes/Data"] = true
	}

	workers := opts.Workers
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	if workers > 64 {
		workers = 64
	}
	jobs := make(chan *Node)
	results := make(chan batch, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for dir := range jobs {
				b := batch{parent: dir}
				// O_NOFOLLOW rejects a directory replaced with a symlink since Lstat.
				var entries []os.DirEntry
				f, e := os.OpenFile(dir.Path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_DIRECTORY, 0)
				if e == nil {
					fi, se := f.Stat()
					if se != nil {
						e = se
					} else {
						_, id, _ := diskInfo(fi)
						if id != dir.id {
							e = fmt.Errorf("directory changed during scan")
						} else {
							entries, e = f.ReadDir(-1)
						}
					}
					f.Close()
				}
				if e != nil {
					b.err = e
				}
				for _, entry := range entries {
					if ctx.Err() != nil {
						break
					}
					p := filepath.Join(dir.Path, entry.Name())
					if excludes[p] {
						b.skipped = append(b.skipped, Skipped{p, "excluded application state/output or macOS Data firmlink alias"})
						continue
					}
					fi, e := os.Lstat(p)
					if e != nil {
						b.skipped = append(b.skipped, Skipped{p, e.Error()})
						continue
					}
					child := fromInfo(p, fi)
					if !opts.CrossMounts && !allowedDevices[child.id.dev] {
						b.skipped = append(b.skipped, Skipped{p, "different filesystem device"})
						continue
					}
					b.children = append(b.children, child)
				}
				results <- b
			}
		}()
	}
	queue := []*Node{n}
	active := 0
	dirs := int64(0)
	seenDirs := map[identity]bool{n.id: true}
	// A coordinator owns the unbounded directory queue. Workers never enqueue into
	// their own bounded work channel (which can deadlock on broad trees).
	for len(queue) > 0 || active > 0 {
		if ctx.Err() != nil {
			close(jobs)
			for active > 0 {
				<-results
				active--
			}
			wg.Wait()
			return nil, ctx.Err()
		}
		var send chan *Node
		var next *Node
		if len(queue) > 0 {
			send = jobs
			next = queue[0]
		}
		select {
		case send <- next:
			queue = queue[1:]
			active++
		case b := <-results:
			active--
			dirs++
			b.parent.Children = b.children
			if b.err != nil {
				b.parent.Incomplete = true
				result.Skipped = append(result.Skipped, Skipped{b.parent.Path, b.err.Error()})
			}
			if len(b.skipped) > 0 {
				b.parent.Incomplete = true
			}
			result.Skipped = append(result.Skipped, b.skipped...)
			result.Entries += int64(len(b.children))
			for _, c := range b.children {
				if c.Kind == "directory" {
					if seenDirs[c.id] {
						c.Incomplete = true
						c.OwnBytes = 0
						result.Skipped = append(result.Skipped, Skipped{c.Path, "directory inode already visited (mount/firmlink alias)"})
					} else {
						seenDirs[c.id] = true
						queue = append(queue, c)
					}
				}
			}
			if opts.Progress != nil {
				opts.Progress(Progress{result.Entries, dirs, len(queue) + active})
			}
		case <-ctx.Done():
		}
	}
	close(jobs)
	wg.Wait()
	aggregate(n, map[identity]bool{})
	sort.Slice(result.Skipped, func(i, j int) bool { return result.Skipped[i].Path < result.Skipped[j].Path })
	result.Finished = time.Now()
	return result, nil
}

func aggregate(n *Node, seen map[identity]bool) {
	sort.Slice(n.Children, func(i, j int) bool { return n.Children[i].Name < n.Children[j].Name })
	// Deterministic path order assigns a shared inode to its first lexical path.
	if n.Kind != "directory" && n.links > 1 {
		if seen[n.id] {
			n.OwnBytes = 0
			n.HardlinkDuplicate = true
		}
		seen[n.id] = true
	}
	n.Bytes = n.OwnBytes
	n.LatestModified = n.Modified
	// Reading a directory may update its atime. Keep its raw Accessed field,
	// but derive activity from regular-file atimes so scans do not rejuvenate caches.
	n.LatestAccessed = time.Time{}
	if n.Kind == "file" {
		n.LatestAccessed = n.Accessed
	}
	if n.Kind == "file" {
		n.Files = 1
	}
	for _, c := range n.Children {
		aggregate(c, seen)
		n.Bytes += c.Bytes
		n.Files += c.Files
		n.Incomplete = n.Incomplete || c.Incomplete
		if c.LatestModified.After(n.LatestModified) {
			n.LatestModified = c.LatestModified
		}
		if c.LatestAccessed.After(n.LatestAccessed) {
			n.LatestAccessed = c.LatestAccessed
		}
	}
}
func Walk(n *Node, fn func(*Node)) {
	fn(n)
	for _, c := range n.Children {
		Walk(c, fn)
	}
}
func Find(n *Node, path string) *Node {
	if n.Path == path {
		return n
	}
	for _, c := range n.Children {
		if found := Find(c, path); found != nil {
			return found
		}
	}
	return nil
}

// On macOS / spans a sealed System volume and firmlinked Data directories.
// Permit Data only through the canonical namespace; omit its duplicate subtree.
func devicePolicy(platform, root string, rootDevice uint64, dataDevice func() (uint64, error)) (map[uint64]bool, bool) {
	allowed := map[uint64]bool{rootDevice: true}
	if platform == "darwin" && root == "/" {
		if dev, e := dataDevice(); e == nil {
			allowed[dev] = true
		}
		return allowed, true
	}
	return allowed, false
}
