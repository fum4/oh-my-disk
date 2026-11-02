package report

import (
	"encoding/binary"
	"github.com/fum4/oh-my-disk/internal/scanner"
	"hash/crc32"
	"os"
	"path/filepath"
	"testing"
)

func TestPruningPreservesTotalsAndSource(t *testing.T) {
	root := &scanner.Node{Path: "/fixture", Bytes: 100, Children: []*scanner.Node{{Path: "/fixture/a", Bytes: 90, Children: []*scanner.Node{{Path: "/fixture/a/b", Bytes: 50}}}, {Path: "/fixture/small", Bytes: 10}}}
	pruned := Prune(root, 1, 20)
	if pruned.Bytes != 100 || len(pruned.Children) != 1 || len(pruned.Children[0].Children) != 0 {
		t.Fatal("bad pruning")
	}
	if len(root.Children) != 2 || len(root.Children[0].Children) != 1 {
		t.Fatal("source changed")
	}
}
func TestCacheAndExclusiveReports(t *testing.T) {
	dir := t.TempDir()
	os.Chmod(dir, 0700)
	path := filepath.Join(dir, "latest.json")
	s := &Snapshot{Version: 1, Scan: &scanner.Result{Root: &scanner.Node{Path: "/fixture", Bytes: 123}}}
	if e := SaveCache(path, s); e != nil {
		t.Fatal(e)
	}
	got, e := LoadCache(path)
	if e != nil || got.Scan.Root.Bytes != 123 {
		t.Fatalf("load %v %v", got, e)
	}
	if e = SaveCache(path, s); e != nil {
		t.Fatal(e)
	}
	out := filepath.Join(dir, "report.json")
	if e = WriteJSON(out, s); e != nil {
		t.Fatal(e)
	}
	if e = WriteJSON(out, s); e == nil {
		t.Fatal("overwrote report")
	}
	user := filepath.Join(dir, "valuable.txt")
	os.WriteFile(user, []byte("keep me"), 0600)
	if e = SaveCache(user, s); e == nil {
		t.Fatal("overwrote unrelated file")
	}
	link := filepath.Join(dir, "link.json")
	os.Symlink(path, link)
	if e = SaveCache(link, s); e == nil {
		t.Fatal("overwrote symlink cache")
	}
}

// Previous identity-free binary caches can be replaced after a fresh scan,
// but must never authorize Trash operations by being loaded as current caches.
func TestPreviousCacheUpgrade(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "latest.bin")
	root := &scanner.Node{Path: dir, Name: "root", Kind: "directory"}
	s := &Snapshot{Version: 1, Scan: &scanner.Result{Root: root}}
	if err := SaveCache(path, s); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	copy(data[:8], "OMDCACH4")
	binary.LittleEndian.PutUint32(data[len(data)-4:], crc32.ChecksumIEEE(data[:len(data)-4]))
	if err = os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = LoadCache(path); err == nil {
		t.Fatal("accepted previous cache")
	}
	if err = SaveCache(path, s); err != nil {
		t.Fatal("could not replace valid previous cache", err)
	}
}
