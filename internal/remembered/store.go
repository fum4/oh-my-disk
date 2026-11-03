// Package remembered keeps explicit cleanup choices in the local app config.
package remembered

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/fum4/oh-my-disk/internal/report"
	"github.com/fum4/oh-my-disk/internal/scanner"
)

type Entry struct {
	Path      string    `json:"path"`
	Bytes     int64     `json:"bytes"`
	Kind      string    `json:"kind"`
	Rule      string    `json:"rule"`
	TrashedAt time.Time `json:"trashed_at"`
}
type Store struct {
	Version int     `json:"version"`
	Items   []Entry `json:"items"`
}
type Match struct {
	Entry
	CurrentBytes int64  `json:"current_bytes"`
	CurrentKind  string `json:"current_kind"`
	Growth       int64  `json:"growth"`
	Warning      bool   `json:"warning"`
}

var mu sync.Mutex

func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "oh-my-disk", "remembered.json"), nil
}
func Load(path string) (Store, error) {
	s := Store{Version: 1, Items: []Entry{}}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	if err = json.Unmarshal(b, &s); err != nil {
		return s, err
	}
	if s.Version != 1 {
		return s, fmt.Errorf("unsupported remembered choices version %d", s.Version)
	}
	return s, nil
}

// Update serializes changes and replaces the private JSON file atomically.
func Update(path string, change func(*Store)) error {
	mu.Lock()
	defer mu.Unlock()
	s, err := Load(path)
	if err != nil {
		return err
	}
	change(&s)
	sort.Slice(s.Items, func(i, j int) bool { return s.Items[i].Path < s.Items[j].Path })
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err = os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".remembered-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(b); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), path)
}
func (s *Store) Remember(e Entry) {
	for i := range s.Items {
		if s.Items[i].Path == e.Path {
			s.Items[i] = e
			return
		}
	}
	s.Items = append(s.Items, e)
}
func (s *Store) Forget(path string) {
	kept := s.Items[:0]
	for _, e := range s.Items {
		if e.Path != path {
			kept = append(kept, e)
		}
	}
	s.Items = kept
}
func Node(s *report.Snapshot, path string) *scanner.Node {
	if s.Scan.Tree != nil {
		id := s.Scan.Tree.Find(path)
		if id < 0 {
			return nil
		}
		return s.Scan.Tree.View(id, 0, 0)
	}
	return scanner.Find(s.Scan.Root, path)
}
func Capture(s *report.Snapshot, path string, now time.Time) (Entry, bool) {
	n := Node(s, path)
	if n == nil {
		return Entry{}, false
	}
	e := Entry{Path: path, Bytes: n.Bytes, Kind: n.Kind, TrashedAt: now.UTC()}
	for _, suggestion := range s.Suggestions {
		if suggestion.Path == path {
			e.Rule = suggestion.Rule
			break
		}
	}
	return e, true
}

// MatchSnapshot uses exact paths from this scan, never the truncated UI tree.
func (s Store) MatchSnapshot(snapshot *report.Snapshot) []Match {
	out := []Match{}
	for _, e := range s.Items {
		// A cached scan from before cleanup cannot establish that a path returned.
		if !snapshot.Scan.Finished.IsZero() && !snapshot.Scan.Finished.After(e.TrashedAt) {
			continue
		}
		n := Node(snapshot, e.Path)
		if n == nil || n.Path != e.Path || n.Path == snapshot.Scan.Root.Path || n.Kind == "symlink" || n.Kind == "special" {
			continue
		}
		growth := n.Bytes - e.Bytes
		out = append(out, Match{e, n.Bytes, n.Kind, growth, n.Bytes > e.Bytes*2 || growth > 1_000_000_000})
	}
	return out
}
