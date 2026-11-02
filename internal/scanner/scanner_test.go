package scanner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func write(t *testing.T, p string, n int) {
	t.Helper()
	if err := os.WriteFile(p, []byte(strings.Repeat("x", n)), 0600); err != nil {
		t.Fatal(err)
	}
}
func TestAllocatedHardlinksSymlinksAndReadOnly(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	os.Mkdir(sub, 0700)
	original := filepath.Join(root, "a")
	write(t, original, 8192)
	if err := os.Link(original, filepath.Join(sub, "linked")); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	write(t, filepath.Join(outside, "not-scanned"), 10000)
	os.Symlink(outside, filepath.Join(root, "outside"))
	os.Symlink(root, filepath.Join(sub, "loop"))
	sparse := filepath.Join(root, "sparse")
	f, e := os.Create(sparse)
	if e != nil {
		t.Fatal(e)
	}
	f.Truncate(1 << 30)
	f.Close()
	before, _ := os.Lstat(original)
	// Compare to filesystem allocated blocks, not logical length (APFS can compress).
	var expected int64
	filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			t.Fatal(err)
		}
		if p != filepath.Join(sub, "linked") {
			b, _, _ := diskInfo(fi)
			expected += b
		}
		return nil
	})
	for _, workers := range []int{1, 8} {
		res, err := Scan(context.Background(), root, Options{Workers: workers})
		if err != nil {
			t.Fatal(err)
		}
		if res.Root.Bytes != expected {
			t.Fatalf("allocated=%d expected=%d", res.Root.Bytes, expected)
		}
		if res.Root.Files != 3 {
			t.Fatalf("file entries=%d", res.Root.Files)
		}
		dup := Find(res.Root, filepath.Join(sub, "linked"))
		if !dup.HardlinkDuplicate || dup.OwnBytes != 0 {
			t.Fatal("hardlink counted twice")
		}
		if Find(res.Root, filepath.Join(root, "outside", "not-scanned")) != nil {
			t.Fatal("followed symlink")
		}
		if Find(res.Root, sparse).Bytes >= 1<<30 {
			t.Fatal("used logical size for sparse file")
		}
		if res.Root.LatestModified.IsZero() || res.Root.LatestAccessed.IsZero() {
			t.Fatal("missing timestamps")
		}
	}
	after, _ := os.Lstat(original)
	if after.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) || after.Mode() != before.Mode() {
		t.Fatal("source modified")
	}
}
func TestExclusionsAndPermissions(t *testing.T) {
	root := t.TempDir()
	private := filepath.Join(root, "private")
	os.Mkdir(private, 0700)
	write(t, filepath.Join(private, "hidden"), 20)
	excluded := filepath.Join(root, "state")
	os.Mkdir(excluded, 0700)
	write(t, filepath.Join(excluded, "cache"), 20)
	os.Chmod(private, 0000)
	defer os.Chmod(private, 0700)
	res, err := Scan(context.Background(), root, Options{Exclude: []string{excluded}})
	if err != nil {
		t.Fatal(err)
	}
	if Find(res.Root, excluded) != nil {
		t.Fatal("exclusion scanned")
	}
	if !res.Root.Incomplete || len(res.Skipped) == 0 {
		t.Fatal("skipped paths unreported")
	}
	if os.Geteuid() != 0 {
		found := false
		for _, s := range res.Skipped {
			if s.Path == private {
				found = true
			}
		}
		if !found {
			t.Fatal("permission denied unreported")
		}
	}
}
func TestCancellationAndBroadTree(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 200; i++ {
		os.MkdirTemp(root, "branch-")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := Scan(ctx, root, Options{Workers: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Root.Children) != 200 {
		t.Fatal("missing directories")
	}
	ctx2, cancel2 := context.WithCancel(context.Background())
	cancel2()
	if _, err = Scan(ctx2, root, Options{}); err == nil {
		t.Fatal("ignored cancellation")
	}
	link := filepath.Join(root, "link")
	os.Symlink(root, link)
	if _, err = Scan(context.Background(), link, Options{}); err == nil {
		t.Fatal("symlink root accepted")
	}
}
func TestDevicePolicy(t *testing.T) {
	for _, tc := range []struct {
		platform, root string
		wantData       bool
	}{
		{"darwin", "/", true}, {"darwin", "/home", false}, {"linux", "/", false},
	} {
		devices, exclude := devicePolicy(tc.platform, tc.root, 1, func() (uint64, error) { return 2, nil })
		if !devices[1] || devices[2] != tc.wantData || exclude != tc.wantData {
			t.Fatalf("bad mount/firmlink policy: %+v %v", devices, exclude)
		}
		if devices[3] {
			t.Fatal("unrelated volume allowed")
		}
	}
}

func TestDirectoryReadsDoNotRefreshActivity(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "old")
	write(t, p, 4096)
	old := time.Now().Add(-200 * 24 * time.Hour)
	os.Chtimes(p, old, old)
	os.Chtimes(root, time.Now(), old)
	res, e := Scan(context.Background(), root, Options{})
	if e != nil {
		t.Fatal(e)
	}
	if time.Since(res.Root.LatestAccessed) < 199*24*time.Hour {
		t.Fatal("directory atime polluted subtree activity")
	}
	if !res.Root.Accessed.After(old) {
		t.Fatal("raw directory atime not recorded")
	}
}
