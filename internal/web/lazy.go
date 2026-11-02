package web

import (
	"encoding/json"
	"github.com/fum4/oh-my-disk/internal/report"
	"github.com/fum4/oh-my-disk/internal/scanner"
	"net/http"
	"sort"
	"strconv"
)

func param(r *http.Request, name string, def, min, max int) (int, bool) {
	s := r.URL.Query().Get(name)
	if s == "" {
		return def, true
	}
	v, e := strconv.Atoi(s)
	return v, e == nil && v >= min && v <= max
}
func installAPI(mux *http.ServeMux, s *report.Snapshot) {
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
		if len(out.Suggestions) > 100 {
			out.Suggestions = out.Suggestions[:100]
		}
		if len(scan.Skipped) > 500 {
			scan.Skipped = scan.Skipped[:500]
		}
		respond(w, struct {
			*report.Snapshot
			SkippedCount int `json:"skipped_count"`
		}{&out, len(s.Scan.Skipped)})
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
		ids := []int32{}
		for id := int32(0); id < t.Count; id++ {
			e := t.At(id)
			if e.OwnBytes > 0 && int(e.Category) < len(t.Categories) && t.Categories[e.Category] == name {
				ids = append(ids, id)
			}
		}
		key, asc, ok := sortParams(r)
		if !ok {
			http.Error(w, "invalid sort", 400)
			return
		}
		orderIds(t, ids, key, asc, true)
		n := &scanner.Node{Name: name, Path: name, Kind: "directory", Bytes: s.Categories[name], ChildCount: len(ids), ChildOffset: offset}
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
	sort.Slice(ids, func(i, j int) bool {
		a, b := ids[i], ids[j]
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
			return t.Name(a) < t.Name(b)
		}
		if asc {
			return cmp < 0
		}
		return cmp > 0
	})
}
