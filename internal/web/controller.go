package web

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/fum4/oh-my-disk/internal/report"
	"github.com/fum4/oh-my-disk/internal/rules"
	"github.com/fum4/oh-my-disk/internal/scanner"
	"github.com/fum4/oh-my-disk/internal/trash"
)

// controller serializes reads, rescans and Trash to keep every response consistent.
func controller(s *report.Snapshot, updated func(*report.Snapshot)) http.Handler {
	var mu sync.Mutex
	api := SnapshotHandlerRead(s)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Method == "GET" || r.Method == "HEAD" {
			api.ServeHTTP(w, r)
			return
		}
		if r.Method != "POST" || (r.URL.Path != "/api/rescan" && r.URL.Path != "/api/trash") {
			http.Error(w, "method not allowed", 405)
			return
		}
		if r.Header.Get("X-OMD-Action") != "1" || r.Header.Get("Content-Type") != "application/json" {
			http.Error(w, "explicit same-origin JSON action required", 403)
			return
		}
		var in struct {
			Paths []string `json:"paths"`
			Root  string   `json:"root"`
		}
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&in); err != nil {
			http.Error(w, "invalid request", 400)
			return
		}
		home, _ := os.UserHomeDir()
		eng, err := rules.Load("", home)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		var results []trash.Result
		root := s.Scan.Root.Path
		if r.URL.Path == "/api/trash" {
			results = trash.Execute(s, in.Paths, home, eng)
		} else if in.Root != "" {
			root = in.Root
		}
		// Rescanning also removes successfully trashed nodes and recomputes all totals.
		needsScan := r.URL.Path == "/api/rescan"
		for _, v := range results {
			needsScan = needsScan || v.Trashed
		}
		var scanError string
		if needsScan {
			scan, e := scanner.Scan(r.Context(), root, scanner.Options{Exclude: []string{filepath.Dir(report.CachePath())}})
			if e != nil {
				scanError = e.Error()
			} else {
				s = report.New(scan, eng, 1<<20)
				api = SnapshotHandlerRead(s)
				if updated != nil {
					updated(s)
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"results": results, "scan_error": scanError})
	})
}
