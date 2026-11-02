package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fum4/oh-my-disk/internal/report"
	"github.com/fum4/oh-my-disk/internal/scanner"
)

func TestLocalReadOnlyServer(t *testing.T) {
	h := handler(&report.Snapshot{Scan: &scanner.Result{Root: &scanner.Node{Path: "/fixture"}}}, "127.0.0.1:1234", "secret")
	for _, tc := range []struct {
		method, path, host, cookie string
		code                       int
	}{
		{"GET", "/?token=secret", "127.0.0.1:1234", "", 200},
		{"GET", "/api/scan", "127.0.0.1:1234", "secret", 200},
		{"GET", "/api/scan", "127.0.0.1:1234", "", 403},
		{"GET", "/api/scan", "evil.example", "secret", 403},
		{"POST", "/api/scan", "127.0.0.1:1234", "secret", 405},
	} {
		r := httptest.NewRequest(tc.method, "http://"+tc.host+tc.path, nil)
		if tc.cookie != "" {
			r.AddCookie(&http.Cookie{Name: "omd", Value: tc.cookie})
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.code {
			t.Fatalf("%+v => %d", tc, w.Code)
		}
		if tc.path == "/api/scan" && tc.code == 200 && !strings.Contains(w.Body.String(), "/fixture") {
			t.Fatal("missing scan data")
		}
	}
}

func TestLazyPagesAndBudget(t *testing.T) {
	root := &scanner.Node{Name: "root", Path: "/root", Kind: "directory"}
	for i := 0; i < 350; i++ {
		d := &scanner.Node{Name: fmt.Sprintf("d%03d", i), Path: fmt.Sprintf("/root/d%03d", i), Kind: "directory", Bytes: int64(1000 + i)}
		for j := 0; j < 50; j++ {
			d.Children = append(d.Children, &scanner.Node{Name: fmt.Sprint(j), Path: fmt.Sprintf("%s/%d", d.Path, j), Kind: "file", OwnBytes: 1, Bytes: 1, Category: "unknown"})
		}
		root.Children = append(root.Children, d)
	}
	tr := scanner.Compact(root)
	s := &report.Snapshot{Version: 1, Scan: &scanner.Result{Tree: tr, Root: root}, Categories: map[string]int64{"unknown": 17500}}
	h := handler(s, "127.0.0.1:1234", "secret")
	get := func(path string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "http://127.0.0.1:1234"+path, nil)
		r.AddCookie(&http.Cookie{Name: "omd", Value: "secret"})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	w := get("/api/node?id=0&depth=3&limit=200")
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	var first scanner.Node
	if e := json.Unmarshal(w.Body.Bytes(), &first); e != nil {
		t.Fatal(e)
	}
	if len(first.Children) != 200 || first.ChildCount != 350 || first.Children[0].Name != "d349" {
		t.Fatal("incorrect ordering/page")
	}
	count := 0
	scanner.Walk(&first, func(*scanner.Node) { count++ })
	if count > 1200 {
		t.Fatalf("response exceeded budget: %d", count)
	}
	w = get("/api/node?id=0&depth=1&offset=200")
	var next scanner.Node
	json.Unmarshal(w.Body.Bytes(), &next)
	if len(next.Children) != 150 || next.Children[0].Name != "d149" {
		t.Fatal("second page missing/duplicated")
	}
	for _, path := range []string{"/api/node?id=-1", "/api/node?depth=100", "/api/node?limit=50000", "/api/category?name=invalid"} {
		if get(path).Code != 400 {
			t.Fatalf("accepted %s", path)
		}
	}
	if get("/api/category?name=unknown").Code != 200 {
		t.Fatal("category route failed")
	}
}
