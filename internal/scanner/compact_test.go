package scanner

import (
	"context"
	"encoding/json"
	"fmt"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"testing"
	"time"
	"unsafe"
)

func normalized(n *Node) []byte {
	c := *n
	c.ID = 0
	c.ChildCount = 0
	c.ChildOffset = 0
	c.Children = nil
	for _, child := range n.Children {
		var v Node
		json.Unmarshal(normalized(child), &v)
		c.Children = append(c.Children, &v)
	}
	b, _ := json.Marshal(c)
	return b
}
func TestCompactScannerEquivalence(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 15; i++ {
		d := filepath.Join(root, fmt.Sprint(i))
		os.Mkdir(d, 0700)
		write(t, d+"/payload", 4097)
		os.Mkdir(d+"/nested", 0700)
	}
	os.Link(root+"/0/payload", root+"/14/hardlink")
	os.Symlink(root, root+"/loop")
	old := time.Date(2000, 1, 2, 3, 4, 5, 123456789, time.Local)
	filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
		if err == nil && fi.Mode()&os.ModeSymlink == 0 {
			os.Chtimes(p, old, old)
		}
		return err
	})
	// Directory atimes change during enumeration; preserve raw atimes by resetting
	// the fixture between runs. Source file contents/modes/mtime remain untouched.
	reset := func() {
		filepath.WalkDir(root, func(p string, d os.DirEntry, e error) error {
			if e == nil && d.IsDir() {
				os.Chtimes(p, old, old)
			}
			return e
		})
	}
	var want []byte
	for _, legacy := range []bool{true, false} {
		for _, fallback := range []bool{true, false} {
			reset()
			opts := Options{Workers: 4, Fallback: fallback}
			var r *Result
			var e error
			if legacy {
				r, e = scanLegacy(context.Background(), root, opts)
			} else {
				r, e = Scan(context.Background(), root, opts)
			}
			if e != nil {
				t.Fatal(e)
			}
			n := r.Root
			if r.Tree != nil {
				if e = r.Tree.Validate(); e != nil {
					t.Fatal(e)
				}
				n = r.Tree.View(0, -1, 0)
			}
			got := normalized(n)
			if want == nil {
				want = got
			} else if string(got) != string(want) {
				t.Fatalf("legacy=%v fallback=%v differs\nwant %s\ngot %s", legacy, fallback, want, got)
			}
		}
	}
}
func TestCompactChunksAndDeepAggregate(t *testing.T) {
	if unsafe.Sizeof(Entry{}) > 64 {
		t.Fatalf("entry grew to %d bytes", unsafe.Sizeof(Entry{}))
	}
	tr := &Tree{Names: []string{"n"}, RootPath: "/n"}
	for i := int32(0); i < 40000; i++ {
		e := Entry{Parent: i - 1, First: -1, Next: -1, NameLen: 1, OwnBytes: 1, Kind: Directory}
		e.SetTime(0, time.Date(1600, 1, 1, 0, 0, 0, 123, time.UTC))
		if i > 0 {
			tr.At(i - 1).First = i
		}
		tr.Add(e)
	}
	if e := tr.Validate(); e != nil {
		t.Fatal(e)
	}
	tr.Aggregate()
	if tr.Bytes(0) != 40000 || tr.Time(0, 2).Year() != 1600 || tr.Time(0, 2).Nanosecond() != 123 {
		t.Fatal("bad aggregate or time loss")
	}
	tr.At(1).Next = 1
	if tr.Validate() == nil {
		t.Fatal("cyclic cache accepted")
	}
}
func TestPinnedDirectoryRejectsAncestorSymlink(t *testing.T) {
	root := t.TempDir()
	os.Mkdir(root+"/parent", 0700)
	os.Mkdir(root+"/parent/child", 0700)
	anchor, e := os.Open(root)
	if e != nil {
		t.Fatal(e)
	}
	defer anchor.Close()
	os.Rename(root+"/parent", root+"/moved")
	os.Symlink("moved", root+"/parent")
	fd, e := openDirectoryAt(int(anchor.Fd()), "parent/child")
	if e == nil {
		unix.Close(fd)
		t.Fatal("followed a replaced ancestor symlink")
	}
	fd, e = openDirectoryAt(int(anchor.Fd()), "moved/child")
	if e != nil {
		t.Fatal(e)
	}
	unix.Close(fd)
}
func BenchmarkCompactAggregate(b *testing.B) {
	root := &Node{Kind: "directory"}
	for i := 0; i < 1000; i++ {
		d := &Node{Kind: "directory"}
		root.Children = append(root.Children, d)
		for j := 0; j < 100; j++ {
			d.Children = append(d.Children, &Node{Kind: "file", OwnBytes: 4096})
		}
	}
	tr := Compact(root)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		tr.Aggregate()
	}
}

func TestLargeFixtureBulkFallbackEquivalence(t *testing.T) {
	root := os.Getenv("OMD_FIXTURE")
	if root == "" {
		t.Skip("set OMD_FIXTURE to compare bulk and fallback on a large fixture")
	}
	a, e := Scan(context.Background(), root, Options{})
	if e != nil {
		t.Fatal(e)
	}
	b, e := Scan(context.Background(), root, Options{Fallback: true})
	if e != nil {
		t.Fatal(e)
	}
	if a.Entries != b.Entries || len(a.Skipped) != len(b.Skipped) {
		t.Fatal("different coverage")
	}
	type pair struct{ a, b int32 }
	pending := []pair{{0, 0}}
	for len(pending) > 0 {
		p := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		x, y := a.Tree.At(p.a), b.Tree.At(p.b)
		if a.Tree.Name(p.a) != b.Tree.Name(p.b) || x.Kind != y.Kind || x.Flags != y.Flags || x.OwnBytes != y.OwnBytes || a.Tree.Bytes(p.a) != b.Tree.Bytes(p.b) || a.Tree.Files(p.a) != b.Tree.Files(p.b) || !x.Time(0).Equal(y.Time(0)) || x.Kind == File && !x.Time(1).Equal(y.Time(1)) || !a.Tree.Time(p.a, 2).Equal(b.Tree.Time(p.b, 2)) || !a.Tree.Time(p.a, 3).Equal(b.Tree.Time(p.b, 3)) {
			t.Fatalf("metadata differs at %s", a.Tree.Path(p.a))
		}
		ca, cb := x.First, y.First
		for ca >= 0 && cb >= 0 {
			pending = append(pending, pair{ca, cb})
			ca = a.Tree.At(ca).Next
			cb = b.Tree.At(cb).Next
		}
		if ca != cb {
			t.Fatal("child count differs")
		}
	}
}

func TestExclusionBasenamePrefilterIsExact(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"a", "b"} {
		os.Mkdir(root+"/"+d, 0700)
		write(t, root+"/"+d+"/same", 4096)
	}
	r, e := Scan(context.Background(), root, Options{Exclude: []string{root + "/a/same"}})
	if e != nil {
		t.Fatal(e)
	}
	if Find(r.Root, root+"/a/same") != nil || Find(r.Root, root+"/b/same") == nil {
		t.Fatal("basename prefilter changed exclusion semantics")
	}
}
