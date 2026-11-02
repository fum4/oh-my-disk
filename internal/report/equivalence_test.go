package report

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/fum4/oh-my-disk/internal/rules"
	"github.com/fum4/oh-my-disk/internal/scanner"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

// Compare the complete original JSON cache without loading its pointer tree.
// Directory raw atime changes when directories are enumerated; all derived
// activity timestamps, file atimes, totals and ranking must still agree.
func TestOriginalLargeCacheEquivalent(t *testing.T) {
	path, root := os.Getenv("OMD_BASELINE"), os.Getenv("OMD_FIXTURE")
	if path == "" || root == "" {
		t.Skip("set OMD_BASELINE and OMD_FIXTURE for full baseline comparison")
	}
	scan, e := scanner.Scan(context.Background(), root, scanner.Options{})
	if e != nil {
		t.Fatal(e)
	}
	eng, e := rules.Load("", scan.Root.Path)
	if e != nil {
		t.Fatal(e)
	}
	snap := New(scan, eng, 1)
	f, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	d := json.NewDecoder(f)
	var compare func(int32) error
	compare = func(id int32) error {
		if _, e := d.Token(); e != nil {
			return e
		}
		n := scanner.Node{}
		for d.More() {
			k, e := d.Token()
			if e != nil {
				return e
			}
			var dst any
			switch k {
			case "name":
				dst = &n.Name
			case "path":
				dst = &n.Path
			case "kind":
				dst = &n.Kind
			case "category":
				dst = &n.Category
			case "bytes":
				dst = &n.Bytes
			case "own_bytes":
				dst = &n.OwnBytes
			case "files":
				dst = &n.Files
			case "modified":
				dst = &n.Modified
			case "accessed":
				dst = &n.Accessed
			case "latest_modified":
				dst = &n.LatestModified
			case "latest_accessed":
				dst = &n.LatestAccessed
			case "hardlink_duplicate":
				dst = &n.HardlinkDuplicate
			case "incomplete":
				dst = &n.Incomplete
			case "children":
				if _, e = d.Token(); e != nil {
					return e
				}
				child := scan.Tree.At(id).First
				for d.More() {
					if child < 0 {
						return fmt.Errorf("extra baseline child")
					}
					if e = compare(child); e != nil {
						return e
					}
					child = scan.Tree.At(child).Next
				}
				if _, e = d.Token(); e != nil {
					return e
				}
				if child >= 0 {
					return fmt.Errorf("missing baseline child")
				}
				continue
			default:
				var discard json.RawMessage
				dst = &discard
			}
			if e = d.Decode(dst); e != nil {
				return e
			}
		}
		if _, e := d.Token(); e != nil {
			return e
		}
		want := scan.Tree.View(id, 0, 0)
		if n.Name != want.Name || n.Path != want.Path || n.Kind != want.Kind || n.Category != want.Category || n.Bytes != want.Bytes || n.OwnBytes != want.OwnBytes || n.Files != want.Files || n.Incomplete != want.Incomplete || n.HardlinkDuplicate != want.HardlinkDuplicate || !n.Modified.Equal(want.Modified) || !n.LatestModified.Equal(want.LatestModified) || !n.LatestAccessed.Equal(want.LatestAccessed) || n.Kind == "file" && !n.Accessed.Equal(want.Accessed) {
			return fmt.Errorf("node differs: %s", n.Path)
		}
		return nil
	}
	d.Token()
	var baselineSuggestions []rules.Suggestion
	var baselineCategories map[string]int64
	for d.More() {
		key, _ := d.Token()
		switch key {
		case "scan":
			d.Token()
			for d.More() {
				k, _ := d.Token()
				if k == "root" {
					if e = compare(0); e != nil {
						t.Fatal(e)
					}
				} else {
					var discard json.RawMessage
					if e = d.Decode(&discard); e != nil {
						t.Fatal(e)
					}
				}
			}
			d.Token()
		case "suggestions":
			if e = d.Decode(&baselineSuggestions); e != nil {
				t.Fatal(e)
			}
		case "categories":
			if e = d.Decode(&baselineCategories); e != nil {
				t.Fatal(e)
			}
		default:
			var discard json.RawMessage
			d.Decode(&discard)
		}
	}
	d.Token()
	for i := range baselineSuggestions {
		baselineSuggestions[i].LastActivity = baselineSuggestions[i].LastActivity.UTC()
	}
	for i := range snap.Suggestions {
		snap.Suggestions[i].LastActivity = snap.Suggestions[i].LastActivity.UTC()
	}
	if !reflect.DeepEqual(baselineSuggestions, snap.Suggestions) || !reflect.DeepEqual(baselineCategories, snap.Categories) {
		t.Fatal("suggestions/ranking/categories differ from original cache")
	}
	t.Logf("all %d nodes and %d ranked suggestions identical", scan.Tree.Count, len(snap.Suggestions))
}
func TestStreamingFullExportAndBinaryCache(t *testing.T) {
	root := &scanner.Node{Name: "root", Path: "/root", Kind: "directory", Bytes: 10, Children: []*scanner.Node{{Name: "a", Path: "/root/a", Kind: "file", OwnBytes: 10, Bytes: 10, Files: 1, Modified: time.Now()}}}
	s := &Snapshot{Version: 1, Scan: &scanner.Result{Tree: scanner.Compact(root), Root: root}}
	path := t.TempDir() + "/cache.bin"
	os.Chmod(filepath.Dir(path), 0700)
	if e := SaveCache(path, s); e != nil {
		t.Fatal(e)
	}
	got, e := LoadCache(path)
	if e != nil {
		t.Fatal(e)
	}
	if got.Scan.Tree.Find("/root/a") < 0 {
		t.Fatal("lost child")
	}
	out := t.TempDir() + "/out.json"
	if e = WriteJSON(out, Export(got, -1, 0)); e != nil {
		t.Fatal(e)
	}
	f, e := os.Open(out)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	var decoded Snapshot
	if e = json.NewDecoder(f).Decode(&decoded); e != nil {
		t.Fatal(e)
	}
	if len(decoded.Scan.Root.Children) != 1 {
		t.Fatal("stream lost child")
	}
	f, e = os.OpenFile(path, os.O_RDWR, 0)
	if e != nil {
		t.Fatal(e)
	}
	f.Seek(-1, io.SeekEnd)
	f.Write([]byte{0xff})
	f.Close()
	if _, e = LoadCache(path); e == nil {
		t.Fatal("corrupted checksum accepted")
	}
	if e = SaveCache(path, s); e == nil {
		t.Fatal("corrupted cache overwritten")
	}
}
