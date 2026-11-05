package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/fum4/oh-my-disk/internal/cleanup"
	"github.com/fum4/oh-my-disk/internal/report"
	"github.com/fum4/oh-my-disk/internal/scanner"
)

// maxCleanupBacklog bounds output the UI hasn't read yet; the oldest bytes go first.
const maxCleanupBacklog = 1 << 20

// cleanupLive is the running cleanup as the UI sees it: unread output and a way to stop it.
type cleanupLive struct {
	mu   sync.Mutex
	data []byte
	stop context.CancelFunc
}

func (l *cleanupLive) begin(stop context.CancelFunc) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.data, l.stop = nil, stop
}

func (l *cleanupLive) end() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.stop = nil
}

func (l *cleanupLive) cancel() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.stop != nil {
		l.stop()
	}
}

func (l *cleanupLive) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.data = append(l.data, p...)
	if extra := len(l.data) - maxCleanupBacklog; extra > 0 {
		l.data = append([]byte(nil), l.data[extra:]...)
	}
	return len(p), nil
}

func (l *cleanupLive) drain() []byte {
	l.mu.Lock()
	defer l.mu.Unlock()
	data := l.data
	l.data = nil
	return data
}

// CleanupCommands lists the suggested commands the UI may offer to run.
func (a *App) CleanupCommands() []string { return cleanup.Commands() }

// CleanupRun reports one cleanup and the item's size before and after it.
type CleanupRun struct {
	cleanup.Result
	Path     string `json:"path"`
	Before   int64  `json:"before"`
	After    int64  `json:"after"`
	Measured bool   `json:"measured"`
}

// RunCleanup runs an allowlisted cleanup command (fixed argv, no shell) and rescans
// just the item's folder to report what it freed. Output is read with ReadCleanup while
// it runs, in a terminal of the given size.
func (a *App) RunCleanup(command, path string, cols, rows uint16) (CleanupRun, error) {
	run := CleanupRun{Path: path}
	if _, ok := cleanup.Argv(command); !ok {
		return run, fmt.Errorf("this command can't be run from oh-my-disk; copy it instead")
	}
	if !a.cleanMu.TryLock() {
		return run, fmt.Errorf("another cleanup is still running")
	}
	defer a.cleanMu.Unlock()
	parent := a.ctx
	if parent == nil {
		parent = context.Background()
	}
	ctx, stop := context.WithCancel(parent)
	defer stop()
	a.cleanLive.begin(stop)
	defer a.cleanLive.end()
	// Measuring a large folder can outlast the command, so it has to show as activity too.
	note := func(text string) { fmt.Fprintf(&a.cleanLive, "\x1b[2m%s\x1b[0m\r\n", text) }
	note("Measuring the size first…")
	before, measured := measure(ctx, path)
	res, err := cleanup.Run(ctx, command, cols, rows, &a.cleanLive)
	run.Result = res
	if err != nil {
		return run, err
	}
	note("Measuring what's left…")
	if after, ok := measure(ctx, path); ok && measured {
		run.Before, run.After, run.Measured = before, after, true
	}
	return run, nil
}

// ReadCleanup returns the running cleanup's output since the last read, base64-encoded.
func (a *App) ReadCleanup() string {
	return base64.StdEncoding.EncodeToString(a.cleanLive.drain())
}

// CancelCleanup stops the running cleanup, if there is one.
func (a *App) CancelCleanup() { a.cleanLive.cancel() }

// measure rescans one absolute folder; a folder the command removed counts as empty.
func measure(ctx context.Context, path string) (int64, bool) {
	if ctx == nil {
		ctx = context.Background()
	}
	if !filepath.IsAbs(path) {
		return 0, false
	}
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		return 0, true
	}
	scan, err := scanner.Scan(ctx, path, scanner.Options{Exclude: []string{filepath.Dir(report.CachePath())}})
	if err != nil || scan.Root == nil {
		return 0, false
	}
	return scan.Root.Bytes, true
}
