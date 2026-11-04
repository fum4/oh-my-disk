package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/fum4/oh-my-disk/internal/remembered"
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
	for _, tc := range []struct {
		query, firstPath string
	}{
		{"", "/root/d000/0"},
		{"&offset=2", "/root/d000/10"},
		{"&sort=name&asc=1&offset=2", "/root/d002/0"},
		{"&offset=2", "/root/d000/10"},
	} {
		response := get("/api/category?name=unknown&limit=2" + tc.query)
		var page scanner.Node
		if err := json.Unmarshal(response.Body.Bytes(), &page); response.Code != 200 || err != nil {
			t.Fatalf("category request %q: status %d, decode error %v", tc.query, response.Code, err)
		}
		if page.ChildCount != 17500 || len(page.Children) != 2 || page.Children[0].Path != tc.firstPath {
			t.Fatalf("incorrect category page for %q: %+v", tc.query, page)
		}
	}
	// Pages crossing the selected prefix stay in one global order, with no repeats.
	seen := map[string]bool{}
	previous := int64(1 << 62)
	for offset := 0; offset < 3000; offset += 500 {
		var page scanner.Node
		json.Unmarshal(get(fmt.Sprintf("/api/category?name=unknown&limit=500&offset=%d", offset)).Body.Bytes(), &page)
		if len(page.Children) != 500 {
			t.Fatalf("offset %d: %d children", offset, len(page.Children))
		}
		for _, c := range page.Children {
			if seen[c.Path] || c.Bytes > previous {
				t.Fatalf("offset %d: %s repeated or out of order", offset, c.Path)
			}
			seen[c.Path] = true
			previous = c.Bytes
		}
	}
}

func TestFirstIdsMatchesFullSortPrefix(t *testing.T) {
	ids := make([]int32, 5000)
	for i := range ids {
		ids[i] = int32((i * 2654435761) % 5000)
	}
	// Heavy primary-key ties, like millions of one-block files, with an id tie-break.
	less := func(a, b int32) bool {
		if a%7 != b%7 {
			return a%7 < b%7
		}
		return a < b
	}
	input := slices.Clone(ids)
	want := slices.Clone(ids)
	sort.Slice(want, func(i, j int) bool { return less(want[i], want[j]) })
	for _, k := range []int{1, 2, 199, 200, 1000, 2500, 4999, 5000, 9000} {
		got := firstIds(ids, k, less)
		if !slices.Equal(got, want[:min(k, len(want))]) {
			t.Fatalf("k=%d: wrong prefix", k)
		}
	}
	if !slices.Equal(ids, input) {
		t.Fatal("firstIds reordered the shared category index")
	}
}

func TestLocateReturnsChainFromRoot(t *testing.T) {
	root := &scanner.Node{Name: "root", Path: "/root", Kind: "directory", Children: []*scanner.Node{
		{Name: "cache", Path: "/root/cache", Kind: "directory", Children: []*scanner.Node{
			{Name: "big file", Path: "/root/cache/big file", Kind: "file", OwnBytes: 8},
		}},
	}}
	h := SnapshotHandlerRead(&report.Snapshot{Scan: &scanner.Result{Root: root, Tree: scanner.Compact(root)}})
	get := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/api/locate?path="+url.QueryEscape(path), nil))
		return w
	}
	var chain []scanner.Node
	if err := json.Unmarshal(get("/root/cache/big file").Body.Bytes(), &chain); err != nil {
		t.Fatal(err)
	}
	if len(chain) != 3 || chain[0].Path != "/root" || chain[1].Path != "/root/cache" || chain[2].Kind != "file" {
		t.Fatalf("wrong chain: %+v", chain)
	}
	for _, path := range []string{"/root/gone", "/elsewhere", ""} {
		if code := get(path).Code; code != 404 {
			t.Fatalf("%q: status %d", path, code)
		}
	}
}

func TestLargeCachedResponseBounded(t *testing.T) {
	path := os.Getenv("OMD_CACHE")
	if path == "" {
		t.Skip("set OMD_CACHE for an actual large-snapshot API measurement")
	}
	s, e := report.LoadCache(path)
	if e != nil {
		t.Fatal(e)
	}
	h := handler(s, "127.0.0.1:1234", "secret")
	r := httptest.NewRequest("GET", "http://127.0.0.1:1234/api/scan", nil)
	r.AddCookie(&http.Cookie{Name: "omd", Value: "secret"})
	w := httptest.NewRecorder()
	start := time.Now()
	h.ServeHTTP(w, r)
	elapsed := time.Since(start)
	var got report.Snapshot
	if e = json.Unmarshal(w.Body.Bytes(), &got); e != nil {
		t.Fatal(e)
	}
	count := 0
	scanner.Walk(got.Scan.Root, func(*scanner.Node) { count++ })
	if count > 1200 || len(got.Suggestions) > 100 {
		t.Fatal("unbounded response")
	}
	t.Logf("%d source entries -> %d response nodes, %d bytes in %s", s.Scan.Entries, count, w.Body.Len(), elapsed)
}

func TestMutationRequiresTokenAndCSRFHeader(t *testing.T) {
	s := &report.Snapshot{Scan: &scanner.Result{Root: &scanner.Node{Path: "/fixture"}}}
	h := handler(s, "127.0.0.1:1234", "secret")
	for _, tc := range []struct {
		cookie, header, origin string
		code                   int
	}{
		{"", "1", "", 403}, {"secret", "", "", 403}, {"secret", "1", "http://evil.example", 403}, {"secret", "1", "http://127.0.0.1:1234", 200},
	} {
		r := httptest.NewRequest("POST", "http://127.0.0.1:1234/api/trash", strings.NewReader(`{"paths":[]}`))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-OMD-Action", tc.header)
		r.Header.Set("Origin", tc.origin)
		if tc.cookie != "" {
			r.AddCookie(&http.Cookie{Name: "omd", Value: tc.cookie})
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.code {
			t.Fatalf("%+v => %d %s", tc, w.Code, w.Body.String())
		}
	}
}

func TestRescanPublishesNewTree(t *testing.T) {
	root := t.TempDir()
	s := &report.Snapshot{Scan: &scanner.Result{Root: &scanner.Node{Path: "/old", Kind: "directory"}}}
	h := SnapshotHandler(s)
	p := root + "/new-file"
	if err := os.WriteFile(p, []byte("new scan"), 0600); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]string{"root": root})
	r := httptest.NewRequest("POST", "/api/rescan", strings.NewReader(string(body)))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-OMD-Action", "1")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 || strings.Contains(w.Body.String(), `"scan_error":"`) == false {
		t.Fatal(w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/scan", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "new-file") || strings.Contains(w.Body.String(), `"path":"/old"`) {
		t.Fatal("rescan did not replace tree", w.Code, w.Body.String())
	}
}

func TestRememberedAPI(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)
	path, err := remembered.Path()
	if err != nil {
		t.Fatal(err)
	}
	if err = remembered.Update(path, func(store *remembered.Store) {
		store.Remember(remembered.Entry{Path: "/fixture/cache", Bytes: 10, Kind: "directory", Rule: "cache", TrashedAt: time.Now()})
	}); err != nil {
		t.Fatal(err)
	}
	root := &scanner.Node{Path: "/fixture", Kind: "directory", Children: []*scanner.Node{{Path: "/fixture/cache", Name: "cache", Kind: "directory", Bytes: 30}}}
	h := handler(&report.Snapshot{Scan: &scanner.Result{Root: root}}, "127.0.0.1:1234", "secret")
	call := func(method, body, action string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://127.0.0.1:1234/api/remembered", strings.NewReader(body))
		r.AddCookie(&http.Cookie{Name: "omd", Value: "secret"})
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-OMD-Action", action)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	w := call("GET", "", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"current_bytes":30`) || !strings.Contains(w.Body.String(), `"warning":true`) {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = call("POST", `{"forget_all":true}`, ""); w.Code != 403 {
		t.Fatal("unauthenticated mutation", w.Code)
	}
	if w = call("POST", `{"paths":["/fixture/cache"]}`, "1"); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	store, _ := remembered.Load(path)
	if len(store.Items) != 0 {
		t.Fatal("forget did not persist")
	}
	remembered.Update(path, func(store *remembered.Store) {
		store.Remember(remembered.Entry{Path: "/fixture/a"})
		store.Remember(remembered.Entry{Path: "/fixture/b"})
	})
	if w = call("POST", `{"forget_all":true}`, "1"); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	store, _ = remembered.Load(path)
	if len(store.Items) != 0 {
		t.Fatal("forget all did not persist")
	}
}
