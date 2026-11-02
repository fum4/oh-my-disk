package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"sync"
	"time"

	"github.com/fum4/oh-my-disk/internal/report"
	"github.com/fum4/oh-my-disk/internal/rules"
	"github.com/fum4/oh-my-disk/internal/scanner"
	"github.com/fum4/oh-my-disk/internal/web"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	wr "github.com/wailsapp/wails/v2/pkg/runtime"
)

type Status struct {
	Busy        bool     `json:"busy"`
	Phase       string   `json:"phase"`
	Entries     int64    `json:"entries"`
	Directories int64    `json:"directories"`
	Rate        float64  `json:"rate"`
	Path        string   `json:"path"`
	Error       string   `json:"error"`
	HasScan     bool     `json:"hasScan"`
	Revision    int      `json:"revision"`
	Home        string   `json:"home"`
	Protected   []string `json:"protected"`
}

type App struct {
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
	snapshot *report.Snapshot
	handler  http.Handler
	agentMu  sync.Mutex
	terminal *agentSession
	status   Status
}

func main() {
	if os.Getenv("GOMEMLIMIT") == "" {
		debug.SetMemoryLimit(704 << 20)
	}
	a := &App{}
	err := wails.Run(&options.App{
		Title: "oh-my-disk", Width: 1280, Height: 900, MinWidth: 900, MinHeight: 650,
		BackgroundColour: options.NewRGBA(0, 0, 0, 0),
		Mac:              &mac.Options{TitleBar: mac.TitleBarHiddenInset(), WebviewIsTransparent: true, WindowIsTranslucent: true},
		AssetServer:      &assetserver.Options{Assets: web.Assets(), Handler: http.HandlerFunc(a.serveHTTP)},
		OnDomReady:       func(context.Context) { configureWindowAppearance() },
		OnStartup:        a.startup, OnShutdown: func(context.Context) { a.Cancel(); a.CloseAgent() }, Bind: []interface{}{a},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	home, _ := os.UserHomeDir()
	a.mu.Lock()
	a.status.Home = home
	a.status.Busy = true
	a.status.Phase = "Opening last cached scan"
	a.mu.Unlock()
	// Loading a compact cache needs no scan or classification. Keep the welcome UI responsive.
	go func() {
		defer func() { a.mu.Lock(); a.status.Busy = false; a.status.Phase = ""; a.mu.Unlock() }()
		a.checkAccess()
		s, err := report.LoadCurrentCache(report.CachePath())
		if err == nil {
			a.publish(s)
		} else if !os.IsNotExist(err) {
			a.mu.Lock()
			a.status.Error = "Could not open last cache: " + err.Error()
			a.mu.Unlock()
		}
	}()
}
func (a *App) checkAccess() {
	home, _ := os.UserHomeDir()
	var denied []string
	for _, rel := range []string{"Library/Mail", "Library/Messages", "Library/Safari", "Library/Calendars", "Library/Cookies", "Library/HomeKit", "Library/Application Support/MobileSync", "Library/Application Support/com.apple.TCC"} {
		p := filepath.Join(home, rel)
		f, err := os.Open(p)
		if err == nil {
			_, err = f.Readdirnames(1)
			f.Close()
		}
		if os.IsPermission(err) {
			denied = append(denied, p)
		}
	}
	a.mu.Lock()
	a.status.Protected = denied
	a.mu.Unlock()
}
func (a *App) publish(s *report.Snapshot) {
	h := web.ManagedHandler(s, func(next *report.Snapshot) { a.mu.Lock(); a.snapshot = next; a.mu.Unlock() })
	a.mu.Lock()
	defer a.mu.Unlock()
	a.snapshot = s
	a.handler = h
	a.status.HasScan = true
	a.status.Revision++
}
func (a *App) State() Status {
	a.mu.RLock()
	defer a.mu.RUnlock()
	s := a.status
	s.Protected = append([]string{}, s.Protected...)
	return s
}
func (a *App) serveHTTP(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	h := a.handler
	a.mu.RUnlock()
	if h == nil {
		http.Error(w, "No scan yet", 404)
		return
	}
	h.ServeHTTP(w, r)
}
func (a *App) PickFolder() (string, error) {
	home, _ := os.UserHomeDir()
	return wr.OpenDirectoryDialog(a.ctx, wr.OpenDialogOptions{Title: "Choose a folder to scan", DefaultDirectory: home})
}
func (a *App) Scan(root string) error {
	if !filepath.IsAbs(root) {
		return fmt.Errorf("choose an absolute folder path")
	}
	fi, err := os.Stat(root)
	if err != nil {
		return err
	}
	if !fi.IsDir() {
		return fmt.Errorf("choose a folder")
	}
	a.mu.Lock()
	if a.status.Busy {
		a.mu.Unlock()
		return fmt.Errorf("a scan is already running")
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.cancel = cancel
	a.status.Busy = true
	a.status.Phase = "walking"
	a.status.Error = ""
	a.status.Entries = 0
	a.status.Directories = 0
	a.status.Rate = 0
	a.status.Path = root
	a.mu.Unlock()
	go func() {
		defer cancel()
		a.scan(ctx, root)
	}()
	return nil
}
func (a *App) scan(ctx context.Context, root string) {
	started := time.Now()
	defer func() { a.checkAccess(); a.mu.Lock(); a.status.Busy = false; a.cancel = nil; a.mu.Unlock() }()
	fail := func(err error) {
		a.mu.Lock()
		defer a.mu.Unlock()
		if ctx.Err() != nil {
			a.status.Phase = "Cancelled · displayed scan kept"
		} else {
			a.status.Error = err.Error()
			a.status.Phase = "Scan failed"
		}
	}
	scan, err := scanner.Scan(ctx, root, scanner.Options{Exclude: []string{filepath.Dir(report.CachePath())}, Progress: func(p scanner.Progress) {
		a.mu.Lock()
		defer a.mu.Unlock()
		a.status.Phase = p.Phase
		a.status.Entries = p.Entries
		a.status.Directories = p.Directories
		a.status.Path = p.Path
		a.status.Rate = float64(p.Entries) / time.Since(started).Seconds()
	}})
	if err != nil {
		fail(err)
		return
	}
	a.mu.Lock()
	a.status.Phase = "classifying"
	a.mu.Unlock()
	home, _ := os.UserHomeDir()
	eng, err := rules.Load("", home)
	if err != nil {
		fail(err)
		return
	}
	s := report.New(scan, eng, 1<<20)
	if ctx.Err() != nil {
		fail(ctx.Err())
		return
	}
	a.mu.Lock()
	a.status.Phase = "writing cache"
	a.mu.Unlock()
	if err = report.SaveCache(report.CachePath(), s); err != nil {
		a.mu.Lock()
		a.status.Error = "Scan complete, but cache could not be saved: " + err.Error()
		a.mu.Unlock()
	}
	if ctx.Err() != nil {
		fail(ctx.Err())
		return
	}
	a.publish(s)
	a.mu.Lock()
	a.status.Phase = "Complete"
	a.mu.Unlock()
}
func (a *App) Cancel() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cancel != nil {
		a.cancel()
	}
}
func (a *App) Reveal(path string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("invalid path")
	}
	return exec.Command("/usr/bin/open", "-R", path).Run()
}
func (a *App) Copy(text string) error { return wr.ClipboardSetText(a.ctx, text) }
func (a *App) FullDiskAccess() error {
	return exec.Command("/usr/bin/open", "x-apple.systempreferences:com.apple.preference.security?Privacy_AllFiles").Run()
}
func (a *App) Export(summary bool) (string, error) {
	a.mu.RLock()
	s := a.snapshot
	a.mu.RUnlock()
	if s == nil {
		return "", fmt.Errorf("scan a folder first")
	}
	name := "oh-my-disk.json"
	if summary {
		name = "oh-my-disk-summary.json"
	}
	path, err := wr.SaveFileDialog(a.ctx, wr.SaveDialogOptions{Title: "Export scan (choose a new filename)", DefaultFilename: name, Filters: []wr.FileFilter{{DisplayName: "JSON", Pattern: "*.json"}}})
	if err != nil || path == "" {
		return "", err
	}
	var output any = report.Export(s, -1, 0)
	if summary {
		output = report.Summary(s, 50)
	}
	if err = report.WriteJSON(path, output); err != nil {
		return "", err
	}
	return path, nil
}
