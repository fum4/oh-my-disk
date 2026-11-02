package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCLIExportsAndCacheReuse(t *testing.T) {
	base := t.TempDir()
	home := filepath.Join(base, "home")
	os.MkdirAll(filepath.Join(home, ".npm"), 0700)
	os.WriteFile(filepath.Join(home, ".npm", "blob"), make([]byte, 8192), 0600)
	cacheDir := filepath.Join(base, "state")
	os.Mkdir(cacheDir, 0700)
	cache := filepath.Join(cacheDir, "latest.json")
	scan := filepath.Join(base, "scan.json")
	summary := filepath.Join(base, "summary.json")
	if e := run([]string{"scan", home, "--home", home, "--cache", cache, "--min-size", "0", "--json", scan, "--summary", summary}); e != nil {
		t.Fatal(e)
	}
	for _, cmd := range []string{"top", "suggest", "explain"} {
		args := []string{cmd}
		if cmd == "explain" {
			args = append(args, filepath.Join(home, ".npm", "blob"))
		}
		args = append(args, "--home", home, "--cache", cache, "--min-size", "0", "--json", filepath.Join(base, cmd+".json"))
		if e := run(args); e != nil {
			t.Fatal(e)
		}
	}
	before, _ := os.Stat(cache)
	if e := run([]string{"suggest", "--cache", cache, "--home", home, "--json", filepath.Join(base, "again.json")}); e != nil {
		t.Fatal(e)
	}
	after, _ := os.Stat(cache)
	if !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("cache reuse triggered a scan")
	}
	if e := run([]string{"scan", home, "--cache", cache, "--json", scan}); e == nil {
		t.Fatal("overwrote export")
	}
}
func TestSizeParser(t *testing.T) {
	for s, want := range map[string]int64{"1MiB": 1048576, "1.5GB": 1500000000, "0": 0, "8KiB": 8192} {
		v, e := parseBytes(s)
		if e != nil || v != want {
			t.Fatalf("%s => %d %v", s, v, e)
		}
	}
	for _, s := range []string{"-1", "NaN", "1e40", "abc"} {
		if _, e := parseBytes(s); e == nil {
			t.Fatalf("accepted %s", s)
		}
	}
}

func TestOptionalJSONFlag(t *testing.T) {
	for _, args := range [][]string{{"--json", "--limit", "2"}, {"--json", "-"}, {"--json"}} {
		got := normalize(args)
		if args[0] == "--json" && len(args) == 2 && args[1] == "-" {
			if got[0] != "--json" {
				t.Fatal("explicit dash rewritten")
			}
		} else if got[0] != "--json=-" {
			t.Fatal("bare JSON flag not normalized")
		}
	}
}
