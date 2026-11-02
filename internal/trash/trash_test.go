package trash

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/fum4/oh-my-disk/internal/report"
	"github.com/fum4/oh-my-disk/internal/rules"
	"github.com/fum4/oh-my-disk/internal/scanner"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBlocklist(t *testing.T) {
	home := "/Users/example"
	for _, p := range []string{"/", "/System/x", "/Library/x", "/Applications/x", "/usr/x", "/bin/x", "/sbin/x", "/private/x", home, home + "/Library", home + "/Documents/a", home + "/Desktop/a", home + "/Pictures", home + "/Pictures/Photos Library.photoslibrary/data", home + "/Library/Mobile Documents/a", home + "/Library/CloudStorage/a"} {
		if Blocked(p, "/", home, false) == nil {
			t.Fatal("allowed", p)
		}
	}
	if Blocked("/outside/a", "/scan", home, false) == nil {
		t.Fatal("outside root")
	}
	if Blocked("/private/var/folders/a/cache", "/", home, false) == nil {
		t.Fatal("unmatched private cache")
	}
	if Blocked("/private/var/folders/a/cache", "/", home, true) != nil {
		t.Fatal("matched private cache")
	}
	if Blocked(home+"/.npm", "/", home, false) != nil {
		t.Fatal("blocked ordinary cache")
	}
}
func fixture(t *testing.T) (string, *report.Snapshot, *rules.Engine) {
	t.Helper()
	base, err := os.MkdirTemp(".", "trash-test-")
	if err != nil {
		t.Fatal(err)
	}
	base, err = filepath.Abs(base)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err != nil {
			os.RemoveAll(base)
		}
	}()
	t.Cleanup(func() { os.RemoveAll(base) })
	for _, p := range []string{"a", "b"} {
		if err = os.WriteFile(filepath.Join(base, p), []byte("test"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	scan, err := scanner.Scan(context.Background(), base, scanner.Options{})
	if err != nil {
		t.Fatal(err)
	}
	eng, err := rules.Load("", base)
	if err != nil {
		t.Fatal(err)
	}
	return base, report.New(scan, eng, 0), eng
}
func TestRevalidationAndAudit(t *testing.T) {
	root, s, e := fixture(t)
	home := filepath.Join(root, "fake-home")
	a := filepath.Join(root, "a")
	b := filepath.Join(root, "b")
	// Cache round trip must retain the scanned identity.
	cache := filepath.Join(root, "state", "latest.bin")
	if err := report.SaveCache(cache, s); err != nil {
		t.Fatal(err)
	}
	s, err := report.LoadCache(cache)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Validate(s, a, home, e); err != nil {
		t.Fatal(err)
	}
	if err = os.Rename(a, a+".old"); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(a, []byte("replacement"), 0600)
	if _, err = Validate(s, a, home, e); err == nil {
		t.Fatal("replacement accepted")
	}
	calls := 0
	results := execute(s, []string{a, b}, home, e, func(p string, dev, ino uint64) error {
		calls++
		if dev == 0 || ino == 0 {
			t.Fatal("no identity")
		}
		return os.Rename(p, p+".test-trash")
	})
	if calls != 1 || results[0].Error == "" || !results[1].Trashed {
		t.Fatalf("bad results: %+v", results)
	}
	data, err := os.ReadFile(filepath.Join(home, "Library/Logs/oh-my-disk/trash.log"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if !json.Valid([]byte(line)) {
			t.Fatal("invalid audit", string(data))
		}
	}
	if !strings.Contains(string(data), a) || !strings.Contains(string(data), b) {
		t.Fatal("missing failed/successful action")
	}
}
func TestSymlinkMissingAndMoverFailure(t *testing.T) {
	root, s, e := fixture(t)
	p := filepath.Join(root, "a")
	home := filepath.Join(root, "fake-home")
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	if _, err := Validate(s, p, home, e); err == nil {
		t.Fatal("missing accepted")
	}
	if err := os.Symlink(filepath.Join(root, "b"), p); err != nil {
		t.Fatal(err)
	}
	if _, err := Validate(s, p, home, e); err == nil {
		t.Fatal("symlink accepted")
	}
	r := execute(s, []string{filepath.Join(root, "b")}, home, e, func(string, uint64, uint64) error { return fmt.Errorf("simulated failure") })
	if len(r) != 1 || r[0].Trashed || r[0].Error != "simulated failure" {
		t.Fatal(r)
	}
}
