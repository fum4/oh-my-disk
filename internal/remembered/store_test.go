package remembered

import (
	"github.com/fum4/oh-my-disk/internal/report"
	"github.com/fum4/oh-my-disk/internal/scanner"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStoreLifecycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config", "remembered.json")
	s, err := Load(path)
	if err != nil || s.Version != 1 || len(s.Items) != 0 {
		t.Fatal(s, err)
	}
	now := time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)
	entry := Entry{Path: "/home/cache", Bytes: 100, Kind: "directory", Rule: "pnpm", TrashedAt: now}
	if err = Update(path, func(s *Store) { s.Remember(entry); entry.Bytes = 200; s.Remember(entry) }); err != nil {
		t.Fatal(err)
	}
	s, err = Load(path)
	if err != nil || len(s.Items) != 1 || s.Items[0].Bytes != 200 || !s.Items[0].TrashedAt.Equal(now) || s.Items[0].Rule != "pnpm" {
		t.Fatal(s, err)
	}
	fi, _ := os.Stat(path)
	if fi.Mode().Perm() != 0600 {
		t.Fatal("choices must be private", fi.Mode())
	}
	if err = Update(path, func(s *Store) { s.Forget(entry.Path) }); err != nil {
		t.Fatal(err)
	}
	s, _ = Load(path)
	if len(s.Items) != 0 {
		t.Fatal(s)
	}
	os.WriteFile(path, []byte(`{"version":2,"items":[]}`), 0600)
	if _, err = Load(path); err == nil {
		t.Fatal("future version accepted")
	}
	if err = Update(path, func(s *Store) { s.Remember(entry) }); err == nil {
		t.Fatal("future store overwritten")
	}
	os.WriteFile(path, []byte(`broken`), 0600)
	if _, err = Load(path); err == nil {
		t.Fatal("corrupt store accepted")
	}
}
func TestExactMatchingAndGrowth(t *testing.T) {
	root := &scanner.Node{Path: "/home", Name: "home", Kind: "directory", Children: []*scanner.Node{
		{Path: "/home/cache", Name: "cache", Kind: "directory", Bytes: 2_500_000_000},
		{Path: "/home/cache-copy", Name: "cache-copy", Kind: "directory", Bytes: 100},
		{Path: "/home/link", Name: "link", Kind: "symlink", Bytes: 50},
	}}
	snapshot := &report.Snapshot{Scan: &scanner.Result{Root: root, Tree: scanner.Compact(root)}}
	store := Store{Version: 1, Items: []Entry{
		{Path: "/home/cache", Bytes: 1_400_000_000}, // >1 GB growth, less than 2x
		{Path: "/home/cache-copy", Bytes: 40},       // >2x
		{Path: "/home/missing", Bytes: 1}, {Path: "/home/link", Bytes: 1}, {Path: "/home", Bytes: 1},
	}}
	matches := store.MatchSnapshot(snapshot)
	if len(matches) != 2 || !matches[0].Warning || !matches[1].Warning || matches[0].Growth != 1_100_000_000 {
		t.Fatal(matches)
	}
	store.Items[0].Bytes = 2_000_000_000
	if store.MatchSnapshot(snapshot)[0].Warning {
		t.Fatal("normal growth warned")
	}
	entry, ok := Capture(snapshot, "/home/cache", time.Now())
	if !ok || entry.Bytes != 2_500_000_000 || entry.Kind != "directory" {
		t.Fatal(entry, ok)
	}
	now := time.Now()
	store.Items = []Entry{{Path: "/home/cache", Bytes: 1, TrashedAt: now}}
	snapshot.Scan.Finished = now.Add(-time.Second)
	if len(store.MatchSnapshot(snapshot)) != 0 {
		t.Fatal("pre-cleanup cached scan treated as returned")
	}
	snapshot.Scan.Finished = now.Add(time.Second)
	if len(store.MatchSnapshot(snapshot)) != 1 {
		t.Fatal("later scan did not match")
	}
	store.Items = []Entry{{Path: "/home/cach", Bytes: 1}}
	if len(store.MatchSnapshot(snapshot)) != 0 {
		t.Fatal("prefix matched")
	}
}
