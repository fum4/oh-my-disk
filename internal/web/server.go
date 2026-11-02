// Package web exposes a read-only, loopback-only view of an immutable snapshot.
package web

import (
	"compress/gzip"
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
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
	mux.HandleFunc("/api/scan", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("Accept-Encoding") != "" { // gzip is negotiated explicitly below
			for _, v := range []string{"gzip", "gzip, deflate", "gzip, deflate, br"} {
				if r.Header.Get("Accept-Encoding") == v {
					w.Header().Set("Content-Encoding", "gzip")
					w.Header().Set("Vary", "Accept-Encoding")
					z := gzip.NewWriter(w)
					defer z.Close()
					json.NewEncoder(z).Encode(s)
					return
				}
			}
		}
		json.NewEncoder(w).Encode(s)
	})
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
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
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
		if err = report.WriteJSON(o.JSON, map[string]any{"url": url, "root": s.Scan.Root.Path, "read_only": true}); err != nil {
			ln.Close()
			return err
		}
	} else {
		fmt.Fprintln(os.Stdout, "Read-only disk explorer:", url, "\nPress Ctrl-C to stop.")
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
