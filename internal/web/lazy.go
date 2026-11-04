package web

import (
	"encoding/json"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"sync"

	"github.com/fum4/oh-my-disk/internal/agents"
	"github.com/fum4/oh-my-disk/internal/report"
	"github.com/fum4/oh-my-disk/internal/rules"
	"github.com/fum4/oh-my-disk/internal/scanner"
)

// categoryPrefix is how many ids a category order selects up front: several pages.
const categoryPrefix = 1000

func param(r *http.Request, name string, def, min, max int) (int, bool) {
	s := r.URL.Query().Get(name)
	if s == "" {
		return def, true
	}
	v, e := strconv.Atoi(s)
	return v, e == nil && v >= min && v <= max
}
func installAPI(mux *http.ServeMux, s *report.Snapshot) {
	mux.HandleFunc("/api/agents", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(agents.Registry())
	})
	t := s.Scan.Tree
	if t == nil {
		t = scanner.Compact(s.Scan.Root)
	}
	respond := func(w http.ResponseWriter, v any) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(v)
	}
	mux.HandleFunc("/api/scan", func(w http.ResponseWriter, r *http.Request) {
		budget := 1200
		scan := *s.Scan
		scan.Root = visible(t, 0, 2, 0, 200, &budget, "bytes", false)
		out := *s
		out.Scan = &scan
		ranked, _ := rules.Select(s.Suggestions, "score", "", "", 0)
		out.Suggestions = ranked[:min(len(ranked), 100)]
		reclaimable, other := rules.Totals(s.Suggestions)
		if len(scan.Skipped) > 500 {
			scan.Skipped = scan.Skipped[:500]
		}
		respond(w, struct {
			*report.Snapshot
			SkippedCount     int            `json:"skipped_count"`
			SuggestionCount  int            `json:"suggestion_count"`
			ReclaimableBytes int64          `json:"reclaimable_bytes"`
			NotCountedBytes  int64          `json:"not_counted_bytes"`
			Volume           *report.Volume `json:"volume,omitempty"`
			VolumeNote       string         `json:"volume_note"`
		}{&out, len(s.Scan.Skipped), len(ranked), reclaimable, other, volume(s), report.VolumeNote})
	})
	mux.HandleFunc("/api/suggest", func(w http.ResponseWriter, r *http.Request) {
		limit, ok := param(r, "limit", 100, 1, 500)
		if !ok {
			http.Error(w, "invalid limit", 400)
			return
		}
		v, err := rules.Select(s.Suggestions, r.URL.Query().Get("sort"), r.URL.Query().Get("category"), r.URL.Query().Get("safety"), 0)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if r.URL.Query().Get("asc") == "1" {
			for i, j := 0, len(v)-1; i < j; i, j = i+1, j-1 {
				v[i], v[j] = v[j], v[i]
			}
		}
		if len(v) > limit {
			v = v[:limit]
		}
		respond(w, v)
	})
	mux.HandleFunc("/api/node", func(w http.ResponseWriter, r *http.Request) {
		id, a := param(r, "id", 0, 0, int(t.Count)-1)
		depth, b := param(r, "depth", 2, 0, 3)
		offset, c := param(r, "offset", 0, 0, int(t.Count))
		limit, d := param(r, "limit", 200, 1, 500)
		if !a || !b || !c || !d {
			http.Error(w, "invalid node/depth/offset/limit", 400)
			return
		}
		key, asc, ok := sortParams(r)
		if !ok {
			http.Error(w, "invalid sort", 400)
			return
		}
		budget := 1200
		respond(w, visible(t, int32(id), depth, offset, limit, &budget, key, asc))
	})
	// The tree is immutable for this API instance. Keep one index and a
	// bounded set of immutable sorted orders for pagination and revisits.
	var indexOnce sync.Once
	index := make(map[string][]int32)
	type categorySort struct {
		name, key string
		asc       bool
	}
	var categoryMu sync.Mutex
	orders := make(map[categorySort][]int32)
	var recent []categorySort
	// categoryIDs returns at least the first `need` ids in order, and the category's
	// total. A category can hold millions of entries and a page shows 200, so only
	// the requested prefix is ever ordered.
	categoryIDs := func(name, key string, asc bool, need int) ([]int32, int) {
		indexOnce.Do(func() {
			for id := int32(0); id < t.Count; id++ {
				e := t.At(id)
				if e.OwnBytes > 0 && int(e.Category) < len(t.Categories) {
					category := t.Categories[e.Category]
					index[category] = append(index[category], id)
				}
			}
		})
		all := index[name]
		query := categorySort{name, key, asc}
		categoryMu.Lock()
		defer categoryMu.Unlock()
		ids, cached := orders[query]
		if cached {
			recent = slices.DeleteFunc(recent, func(entry categorySort) bool { return entry == query })
		} else if len(recent) == 4 {
			delete(orders, recent[0])
			recent = recent[1:]
		}
		recent = append(recent, query)
		if !cached || (len(ids) < need && len(ids) < len(all)) {
			// Select ahead so paging through a category does not reselect every page.
			ids = firstIds(all, max(2*need, categoryPrefix), idLess(t, key, asc, true))
			orders[query] = ids
		}
		return ids, len(all)
	}
	mux.HandleFunc("/api/category", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		if _, ok := s.Categories[name]; !ok {
			http.Error(w, "unknown category", 400)
			return
		}
		offset, a := param(r, "offset", 0, 0, int(t.Count))
		limit, b := param(r, "limit", 200, 1, 500)
		if !a || !b {
			http.Error(w, "invalid offset/limit", 400)
			return
		}
		key, asc, ok := sortParams(r)
		if !ok {
			http.Error(w, "invalid sort", 400)
			return
		}
		ids, total := categoryIDs(name, key, asc, offset+limit)
		n := &scanner.Node{Name: name, Path: name, Kind: "directory", Bytes: s.Categories[name], ChildCount: total, ChildOffset: offset}
		if offset > len(ids) {
			offset = len(ids)
		}
		end := offset + limit
		if end > len(ids) {
			end = len(ids)
		}
		for _, id := range ids[offset:end] {
			c := t.View(id, 0, 0)
			c.Bytes = c.OwnBytes
			n.Children = append(n.Children, c)
		}
		respond(w, n)
	})
}

// A response has both a depth and a total node budget; a million-entry single
// directory cannot accidentally become a million-entry API response.
func visible(t *scanner.Tree, id int32, depth, offset, limit int, budget *int, key string, asc bool) *scanner.Node {
	n := t.View(id, 0, 0)
	*budget--
	ids := []int32{}
	for c := t.At(id).First; c >= 0; c = t.At(c).Next {
		ids = append(ids, c)
	}
	n.ChildCount = len(ids)
	n.ChildOffset = offset
	if depth == 0 || *budget <= 0 {
		return n
	}
	orderIds(t, ids, key, asc, false)
	if offset > len(ids) {
		offset = len(ids)
	}
	end := offset + limit
	if end > len(ids) {
		end = len(ids)
	}
	if end-offset > *budget {
		end = offset + *budget
	}
	// Reserve siblings before expanding grandchildren so the visible table always
	// gets a full page, even when the first branch is exceptionally broad.
	chosen := ids[offset:end]
	*budget -= len(chosen)
	for _, c := range chosen {
		n.Children = append(n.Children, t.View(c, 0, 0))
	}
	for i, c := range chosen {
		*budget++
		n.Children[i] = visible(t, c, depth-1, 0, min(limit, 40), budget, "bytes", false)
	}
	return n
}

func sortParams(r *http.Request) (string, bool, bool) {
	key := r.URL.Query().Get("sort")
	if key == "" {
		key = "bytes"
	}
	return key, r.URL.Query().Get("asc") == "1", key == "bytes" || key == "name" || key == "files" || key == "modified"
}
func orderIds(t *scanner.Tree, ids []int32, key string, asc, own bool) {
	less := idLess(t, key, asc, own)
	sort.Slice(ids, func(i, j int) bool { return less(ids[i], ids[j]) })
}

// firstIds returns the k first ids under less, leaving ids untouched. Each compaction
// raises the bar, so nearly every later id is rejected by a single comparison.
func firstIds(ids []int32, k int, less func(a, b int32) bool) []int32 {
	if k >= len(ids) {
		out := slices.Clone(ids)
		sort.Slice(out, func(i, j int) bool { return less(out[i], out[j]) })
		return out
	}
	kept := make([]int32, 0, 2*k)
	compact := func() {
		sort.Slice(kept, func(i, j int) bool { return less(kept[i], kept[j]) })
		kept = kept[:min(k, len(kept))]
	}
	full := false
	for _, id := range ids {
		if full && !less(id, kept[k-1]) {
			continue
		}
		kept = append(kept, id)
		if len(kept) == 2*k {
			compact()
			full = true
		}
	}
	compact()
	return kept
}

func idLess(t *scanner.Tree, key string, asc, own bool) func(a, b int32) bool {
	return func(a, b int32) bool {
		cmp := 0
		switch key {
		case "name":
			if t.Name(a) < t.Name(b) {
				cmp = -1
			} else if t.Name(a) > t.Name(b) {
				cmp = 1
			}
		case "modified":
			x, y := t.At(a).Time(0), t.At(b).Time(0)
			if x.Before(y) {
				cmp = -1
			} else if x.After(y) {
				cmp = 1
			}
		default:
			var x, y int64
			if key == "files" {
				x, y = t.Files(a), t.Files(b)
			} else if own {
				x, y = t.At(a).OwnBytes, t.At(b).OwnBytes
			} else {
				x, y = t.Bytes(a), t.Bytes(b)
			}
			if x < y {
				cmp = -1
			} else if x > y {
				cmp = 1
			}
		}
		if cmp == 0 {
			if own {
				return t.LessPath(a, b)
			}
			return t.Name(a) < t.Name(b)
		}
		if asc {
			return cmp < 0
		}
		return cmp > 0
	}
}

func volume(s *report.Snapshot) *report.Volume {
	v, err := report.VolumeFor(s.Scan.Root.Path, s.Scan.Root.Bytes)
	if err != nil {
		return nil
	}
	return &v
}
