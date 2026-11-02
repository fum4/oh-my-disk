package web

import (
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
