package scanner

import (
	"bytes"
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"time"
)

const chunkBits = 14
const chunkSize = 1 << chunkBits
const (
	File uint8 = iota
	Directory
	Symlink
	Special
)
const (
	Duplicate uint8 = 1 << iota
	Incomplete
	Dataless
	Compressed
)

var kinds = [...]string{"file", "directory", "symlink", "special"}

// Entry retains device/inode for Trash revalidation (16 bytes per scanned entry).
// Entry has no pointers. Chunks never move or copy during growth. Timestamps
// retain seconds AND nanoseconds, including dates outside UnixNano's range.
type Entry struct {
	Device, Inode                 uint64
	OwnBytes                      int64
	Seconds                       [2]int64
	Nanos                         [2]uint32
	Parent, First, Next, Slot     int32
	NameBlock, NameStart, NameLen uint32
	Kind, Flags, Category         uint8
}

// Tree stores sibling lists as int32 indices, and names in immutable per-batch
// byte arenas. Paths are constructed only for directory jobs or visible output.
type Tree struct {
	Chunks     [][]Entry
	DataChunks [][]DirectoryData
	DataCount  int32
	Names      []string
	Count      int32
	RootPath   string
	Categories []string
	Paths      []string // only legacy trees with explicit paths
}

func (t *Tree) At(id int32) *Entry { return &t.Chunks[int(id)>>chunkBits][int(id)&(chunkSize-1)] }
func (t *Tree) Add(e Entry) int32 {
	if t.Count == math.MaxInt32 {
		panic("scan exceeds int32 node limit")
	}
	if e.Kind == Directory {
		e.Slot = t.addData(DirectoryData{})
	} else {
		e.Slot = -1
	}
	id := t.Count
	if int(id)&(chunkSize-1) == 0 {
		t.Chunks = append(t.Chunks, make([]Entry, chunkSize))
	}
	*t.At(id) = e
	t.Count++
	return id
}
func (t *Tree) Name(id int32) string {
	e := t.At(id)
	return t.Names[e.NameBlock][e.NameStart : e.NameStart+e.NameLen]
}
func (t *Tree) Path(id int32) string {
	if int(id) < len(t.Paths) {
		return t.Paths[id]
	}
	var buf [1024]byte
	return string(t.appendPath(buf[:0], id))
}

func (e *Entry) Time(which int) time.Time {
	if e.Seconds[which] == math.MinInt64 {
		return time.Time{}
	}
	return time.Unix(e.Seconds[which], int64(e.Nanos[which]))
}
func (e *Entry) SetTime(which int, v time.Time) {
	if v.IsZero() {
		e.Seconds[which] = math.MinInt64
		e.Nanos[which] = 0
	} else {
		e.Seconds[which] = v.Unix()
		e.Nanos[which] = uint32(v.Nanosecond())
	}
}
func entryFromNode(n *Node, parent int32) Entry {
	e := Entry{Device: n.Device, Inode: n.Inode, OwnBytes: n.OwnBytes, Parent: parent, First: -1, Next: -1}
	for k, s := range kinds {
		if n.Kind == s {
			e.Kind = uint8(k)
		}
	}
	if n.Incomplete {
		e.Flags |= Incomplete
	}
	if n.HardlinkDuplicate {
		e.Flags |= Duplicate
	}
	if n.Dataless {
		e.Flags |= Dataless
	}
	for i, v := range []time.Time{n.Modified, n.Accessed} {
		e.SetTime(i, v)
	}
	return e
}

// Compact is also used for semantic equivalence tests and legacy JSON migration.
func Compact(root *Node) *Tree {
	t := &Tree{RootPath: root.Path, Categories: []string{""}}
	var add func(*Node, int32) int32
	add = func(n *Node, p int32) int32 {
		e := entryFromNode(n, p)
		found := false
		for i, c := range t.Categories {
			if c == n.Category {
				e.Category = uint8(i)
				found = true
				break
			}
		}
		if !found {
			e.Category = uint8(len(t.Categories))
			t.Categories = append(t.Categories, n.Category)
		}
		e.NameBlock = uint32(len(t.Names))
		e.NameLen = uint32(len(n.Name))
		t.Names = append(t.Names, n.Name)
		id := t.Add(e)
		derivedFiles := int64(0)
		if e.Kind == File {
			derivedFiles = 1
		}
		if e.Kind == Directory || n.Bytes != n.OwnBytes || n.Files != derivedFiles || !n.LatestModified.Equal(n.Modified) || e.Kind == File && !n.LatestAccessed.Equal(n.Accessed) {
			if t.At(id).Slot < 0 {
				t.At(id).Slot = t.addData(DirectoryData{})
			}
			d := t.Data(t.At(id).Slot)
			d.Bytes = n.Bytes
			d.Files = n.Files
			d.SetTime(0, n.LatestModified)
			d.SetTime(1, n.LatestAccessed)
		}
		t.Paths = append(t.Paths, n.Path)
		prev := int32(-1)
		for _, c := range n.Children {
			cid := add(c, id)
			if prev < 0 {
				t.At(id).First = cid
			} else {
				t.At(prev).Next = cid
			}
			prev = cid
		}
		return id
	}
	add(root, -1)
	return t
}

// View materializes only requested levels. Aggregate totals are never pruned.
func (t *Tree) View(id int32, depth int, min int64) *Node {
	e := t.At(id)
	n := &Node{Device: e.Device, Inode: e.Inode, ID: id, Name: t.Name(id), Path: t.Path(id), Kind: kinds[e.Kind], OwnBytes: e.OwnBytes, Bytes: t.Bytes(id), Files: t.Files(id), Modified: e.Time(0), Accessed: e.Time(1), LatestModified: t.Time(id, 2), LatestAccessed: t.Time(id, 3), HardlinkDuplicate: e.Flags&Duplicate != 0, Incomplete: e.Flags&Incomplete != 0, Dataless: e.Flags&Dataless != 0, tree: t, index: id}
	if int(e.Category) < len(t.Categories) {
		n.Category = t.Categories[e.Category]
	}
	if depth != 0 {
		next := depth
		if next > 0 {
			next--
		}
		for c := e.First; c >= 0; c = t.At(c).Next {
			if t.Bytes(c) >= min {
				n.Children = append(n.Children, t.View(c, next, min))
			}
		}
	}
	return n
}
func (t *Tree) Find(path string) int32 {
	if path == t.RootPath {
		return 0
	}
	rel, err := filepath.Rel(t.RootPath, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return -1
	}
	id := int32(0)
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		found := int32(-1)
		for c := t.At(id).First; c >= 0; c = t.At(c).Next {
			if t.Name(c) == part {
				found = c
				break
			}
		}
		if found < 0 {
			return -1
		}
		id = found
	}
	return id
}

// Aggregate is one reverse index pass: parents are always inserted before children.
func (t *Tree) Aggregate() {
	for id := int32(0); id < t.Count; id++ {
		e := t.At(id)
		if e.Slot >= 0 {
			d := t.Data(e.Slot)
			d.Bytes = e.OwnBytes
			d.Files = 0
			d.SetTime(0, e.Time(0))
			d.SetTime(1, time.Time{})
		}
	}
	for id := t.Count - 1; id > 0; id-- {
		e := t.At(id)
		p := t.At(e.Parent)
		d := t.Data(p.Slot)
		d.Bytes += t.Bytes(id)
		d.Files += t.Files(id)
		p.Flags |= e.Flags & Incomplete
		d.Later(0, t.Time(id, 2))
		d.Later(1, t.Time(id, 3))
	}
}

// Validate rejects invalid indices before a cache can expose an unsafe traversal.
func (t *Tree) Validate() error {
	if t.Count <= 0 || len(t.Chunks) != (int(t.Count)+chunkSize-1)/chunkSize {
		return fmt.Errorf("invalid tree count")
	}
	refs := make([]bool, t.Count)
	for id := int32(0); id < t.Count; id++ {
		e := t.At(id)
		if e.Slot < -1 || e.Slot >= t.DataCount {
			return fmt.Errorf("invalid aggregate slot")
		}
		if e.Kind == Directory && e.Slot < 0 {
			return fmt.Errorf("directory without aggregate")
		}
		if int(e.NameBlock) >= len(t.Names) || uint64(e.NameStart)+uint64(e.NameLen) > uint64(len(t.Names[e.NameBlock])) || e.Kind > Special {
			return fmt.Errorf("invalid node %d", id)
		}
		if id == 0 {
			if e.Parent != -1 {
				return fmt.Errorf("invalid root parent")
			}
		} else if e.Parent < 0 || e.Parent >= id {
			return fmt.Errorf("invalid parent")
		}
		if e.Next != -1 && (e.Next <= id || e.Next >= t.Count || t.At(e.Next).Parent != e.Parent) {
			return fmt.Errorf("invalid sibling")
		}
		if e.First != -1 && (e.First <= id || e.First >= t.Count) {
			return fmt.Errorf("invalid child")
		}
		for c := e.First; c >= 0; c = t.At(c).Next {
			if c >= t.Count || t.At(c).Parent != id || refs[c] {
				return fmt.Errorf("invalid child chain")
			}
			refs[c] = true
			if t.At(c).Next != -1 && t.At(c).Next <= c {
				return fmt.Errorf("cyclic sibling")
			}
		}
	}
	for id := int32(1); id < t.Count; id++ {
		if !refs[id] {
			return fmt.Errorf("orphan node")
		}
	}
	return nil
}

func (n *Node) CompactTree() *Tree { return n.tree }

// Only directories need aggregate sizes, counts and activity timestamps. Files
// derive these from their own metadata, saving 40 bytes per ordinary file.
type DirectoryData struct {
	Bytes, Files int64
	Seconds      [2]int64
	Nanos        [2]uint32
}

func (t *Tree) Data(slot int32) *DirectoryData {
	return &t.DataChunks[int(slot)>>chunkBits][int(slot)&(chunkSize-1)]
}
func (t *Tree) addData(d DirectoryData) int32 {
	slot := t.DataCount
	if int(slot)&(chunkSize-1) == 0 {
		t.DataChunks = append(t.DataChunks, make([]DirectoryData, chunkSize))
	}
	*t.Data(slot) = d
	t.DataCount++
	return slot
}
func (t *Tree) Bytes(id int32) int64 {
	e := t.At(id)
	if e.Slot >= 0 {
		return t.Data(e.Slot).Bytes
	}
	return e.OwnBytes
}
func (t *Tree) Files(id int32) int64 {
	e := t.At(id)
	if e.Slot >= 0 {
		return t.Data(e.Slot).Files
	}
	if e.Kind == File {
		return 1
	}
	return 0
}
func (t *Tree) Time(id int32, which int) time.Time {
	e := t.At(id)
	if which < 2 {
		return e.Time(which)
	}
	if e.Slot >= 0 {
		return t.Data(e.Slot).Time(which - 2)
	}
	if which == 2 {
		return e.Time(0)
	}
	if e.Kind == File {
		return e.Time(1)
	}
	return time.Time{}
}
func (d *DirectoryData) Time(i int) time.Time {
	if d.Seconds[i] == math.MinInt64 {
		return time.Time{}
	}
	return time.Unix(d.Seconds[i], int64(d.Nanos[i]))
}
func (d *DirectoryData) SetTime(i int, v time.Time) {
	if v.IsZero() {
		d.Seconds[i] = math.MinInt64
		d.Nanos[i] = 0
	} else {
		d.Seconds[i] = v.Unix()
		d.Nanos[i] = uint32(v.Nanosecond())
	}
}
func (d *DirectoryData) Later(i int, v time.Time) {
	if v.After(d.Time(i)) {
		d.SetTime(i, v)
	}
}
func (t *Tree) appendPath(dst []byte, id int32) []byte {
	if int(id) < len(t.Paths) {
		return append(dst, t.Paths[id]...)
	}
	var local [64]int32
	chain := local[:0]
	for id > 0 {
		chain = append(chain, id)
		id = t.At(id).Parent
	}
	dst = append(dst, t.RootPath...)
	for i := len(chain) - 1; i >= 0; i-- {
		if len(dst) > 1 || t.RootPath != "/" {
			dst = append(dst, '/')
		}
		dst = append(dst, t.Name(chain[i])...)
	}
	return dst
}
func (t *Tree) LessPath(a, b int32) bool {
	var ab, bb [1024]byte
	return bytes.Compare(t.appendPath(ab[:0], a), t.appendPath(bb[:0], b)) < 0
}
