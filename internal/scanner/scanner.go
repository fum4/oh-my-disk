// Package scanner only reads filesystem metadata. It never opens file contents,
// follows symbolic links, or performs cleanup operations.
package scanner

import (
	"os"
	"sort"
	"strings"
	"time"
)

type Node struct {
	ID                int32     `json:"id"`
	ChildCount        int       `json:"child_count,omitempty"`
	ChildOffset       int       `json:"child_offset,omitempty"`
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
	Dataless          bool      `json:"dataless,omitempty"`
	tree              *Tree
	index             int32
	id                identity
	links             uint64
}
type Skipped struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}
type Result struct {
	WalkDuration      time.Duration `json:"-"`
	AggregateDuration time.Duration `json:"-"`
	Tree              *Tree         `json:"-"`
	Root              *Node         `json:"root"`
	Started           time.Time     `json:"started"`
	Finished          time.Time     `json:"finished"`
	Skipped           []Skipped     `json:"skipped"`
	Entries           int64         `json:"entries"`
	CrossMounts       bool          `json:"cross_mounts"`
}
type Progress struct {
	Entries     int64
	Directories int64
	Pending     int
	Phase       string
	Path        string
}
type Options struct {
	Workers     int
	CrossMounts bool
	Exclude     []string
	Progress    func(Progress)
	Fallback    bool // force portable metadata reader (equivalence/debugging)
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
	return &Node{Dataless: isDataless(info), Name: info.Name(), Path: path, Kind: kind, OwnBytes: b, Modified: info.ModTime(), Accessed: accessTime(info), id: id, links: l}
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
	if n.tree != nil {
		stack := []int32{n.ID}
		for len(stack) > 0 {
			id := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			fn(n.tree.View(id, 1, 0))
			var children []int32
			for c := n.tree.At(id).First; c >= 0; c = n.tree.At(c).Next {
				children = append(children, c)
			}
			for i := len(children) - 1; i >= 0; i-- {
				stack = append(stack, children[i])
			}
		}
		return
	}

	fn(n)
	for _, c := range n.Children {
		Walk(c, fn)
	}
}
func Find(n *Node, path string) *Node {
	if n.tree != nil {
		if path != n.Path && n.Path != "/" && !strings.HasPrefix(path, n.Path+"/") {
			return nil
		}
		id := n.tree.Find(path)
		if id < 0 {
			return nil
		}
		return n.tree.View(id, 1, 0)
	}
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
