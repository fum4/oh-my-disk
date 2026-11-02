// Package web exposes loopback-only disk analysis with explicit, authenticated actions.
package web

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"time"

	"github.com/fum4/oh-my-disk/internal/report"
)

//go:embed assets/*
var assets embed.FS

type Options struct {
	Port int
	Open bool
	JSON string
}

func handler(s *report.Snapshot, host, token string) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/api/", controller(s, nil))
	sub, _ := fs.Sub(assets, "assets")
	mux.Handle("/", http.FileServer(http.FS(sub)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; connect-src 'self'; img-src 'self' data:; frame-ancestors 'none'; base-uri 'none'")
		if r.Host != host {
			http.Error(w, "invalid host", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead && (r.Method != http.MethodPost || (r.URL.Path != "/api/trash" && r.URL.Path != "/api/rescan")) {
			http.Error(w, "read-only", http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path == "/" && r.URL.Query().Get("token") == token {
			http.SetCookie(w, &http.Cookie{Name: "omd", Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
		} else {
			c, e := r.Cookie("omd")
			if e != nil || c.Value != token {
				http.Error(w, "Open the private URL printed by oh-my-disk", http.StatusForbidden)
				return
			}
		}
		if r.Method == "POST" && r.Header.Get("Origin") != "" && r.Header.Get("Origin") != "http://"+host {
			http.Error(w, "invalid origin", 403)
			return
		}
		mux.ServeHTTP(w, r)
	})
}
func Serve(ctx context.Context, s *report.Snapshot, o Options) error {
	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(o.Port)))
	if err != nil {
		return err
	}
	tokenBytes := make([]byte, 24)
	if _, err = rand.Read(tokenBytes); err != nil {
		ln.Close()
		return err
	}
	token := hex.EncodeToString(tokenBytes)
	url := "http://" + ln.Addr().String() + "/?token=" + token
	if o.JSON != "" {
		if err = report.WriteJSON(o.JSON, map[string]any{"url": url, "root": s.Scan.Root.Path, "read_only_scanning": true, "explicit_trash": true}); err != nil {
			ln.Close()
			return err
		}
	} else {
		fmt.Fprintln(os.Stdout, "Disk explorer (read-only scans; explicit Move to Trash):", url, "\nPress Ctrl-C to stop.")
	}
	server := &http.Server{Handler: handler(s, ln.Addr().String(), token), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			c, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			server.Shutdown(c)
		case <-done:
		}
	}()
	if o.Open {
		command := "open"
		if runtime.GOOS != "darwin" {
			command = "xdg-open"
		}
		if err := exec.Command(command, url).Run(); err != nil {
			fmt.Fprintln(os.Stderr, "Could not open browser; use", url)
		}
	}
	err = server.Serve(ln)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// Assets and SnapshotHandler are shared by the native WebView. No TCP listener,
// browser cookies or authentication token are needed within that process.
func Assets() fs.FS                                   { sub, _ := fs.Sub(assets, "assets"); return sub }
func SnapshotHandler(s *report.Snapshot) http.Handler { return controller(s, nil) }
func SnapshotHandlerRead(s *report.Snapshot) http.Handler {
	mux := http.NewServeMux()
	installAPI(mux, s)
	return mux
}

func ManagedHandler(s *report.Snapshot, updated func(*report.Snapshot)) http.Handler {
	return controller(s, updated)
}
