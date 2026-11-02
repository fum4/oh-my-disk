package main

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/fum4/oh-my-disk/internal/report"
	"github.com/fum4/oh-my-disk/internal/rules"
	"github.com/fum4/oh-my-disk/internal/scanner"
)

func TestNativeSnapshotAPI(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "sample"), make([]byte, 8192), 0600); err != nil {
		t.Fatal(err)
	}
	scan, err := scanner.Scan(context.Background(), root, scanner.Options{})
	if err != nil {
		t.Fatal(err)
	}
	eng, err := rules.Load("", root)
	if err != nil {
		t.Fatal(err)
	}
	a := &App{}
	request := func(path string, code int) *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		a.serveHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != code {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
		}
		return w
	}
	request("/api/scan", 404)
	a.publish(report.New(scan, eng, 0))
	w := request("/api/scan", 200)
	var data struct {
		Volume *report.Volume `json:"volume"`
	}
	if err = json.Unmarshal(w.Body.Bytes(), &data); err != nil || data.Volume == nil {
		t.Fatalf("missing volume: %s %v", w.Body.String(), err)
	}
	request("/api/node?id=0&depth=1", 200)
	request("/api/node?id=-1", 400)
	w = httptest.NewRecorder()
	a.serveHTTP(w, httptest.NewRequest("POST", "/api/scan", nil))
	if w.Code != 405 {
		t.Fatal("native handler accepted mutation")
	}
	if !a.State().HasScan || a.State().Revision != 1 {
		t.Fatal("snapshot not published")
	}
	// Cancellation leaves the immutable displayed snapshot available.
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	a.Cancel()
	if ctx.Err() == nil || !a.State().HasScan {
		t.Fatal("cancellation lost snapshot")
	}
}
func TestScanRejectsInvalidAndConcurrentRequests(t *testing.T) {
	a := &App{ctx: context.Background()}
	if a.Scan("relative") == nil {
		t.Fatal("accepted relative path")
	}
	if a.Scan(filepath.Join(t.TempDir(), "missing")) == nil {
		t.Fatal("accepted missing directory")
	}
	a.status.Busy = true
	if a.Scan(t.TempDir()) == nil {
		t.Fatal("accepted overlapping scan")
	}
}
