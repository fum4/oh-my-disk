// Package cleanup runs the few tool-owned cleanup commands our rules suggest.
//
// Only exact command strings from the built-in rules are runnable, each mapped to a
// fixed argv. Nothing is passed through a shell and no scan- or user-derived text is
// ever interpolated, so path-templated commands (du, rm) can never be executed here.
package cleanup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
)

// allowed maps a rule's exact suggested command to the argv we execute. Every entry is
// the tool's own prune/clean operation; inspection commands (du, list) and anything
// that removes arbitrary paths stay copy-only.
var allowed = map[string][]string{
	"npm cache clean --force":         {"npm", "cache", "clean", "--force"},
	"yarn cache clean":                {"yarn", "cache", "clean"},
	"pnpm store prune":                {"pnpm", "store", "prune"},
	"bun pm cache rm":                 {"bun", "pm", "cache", "rm"},
	"go clean -modcache":              {"go", "clean", "-modcache"},
	"go clean -cache":                 {"go", "clean", "-cache"},
	"xcrun simctl delete unavailable": {"xcrun", "simctl", "delete", "unavailable"},
	"pod cache clean --all":           {"pod", "cache", "clean", "--all"},
	"brew cleanup":                    {"brew", "cleanup"},
}

// Commands lists the runnable command strings, for the UI to decide where a Run button belongs.
func Commands() []string {
	out := make([]string, 0, len(allowed))
	for c := range allowed {
		out = append(out, c)
	}
	return out
}

// Argv returns the fixed argv for an exactly allowlisted command.
func Argv(command string) ([]string, bool) {
	argv, ok := allowed[command]
	if !ok {
		return nil, false
	}
	return append([]string(nil), argv...), true
}

var (
	pathOnce  sync.Once
	loginPath string
)

// LoginPath is the PATH of the user's login shell, so Homebrew, pnpm and friends are
// found from a GUI app launched by Finder. The shell runs a fixed script with no input.
func LoginPath() string {
	pathOnce.Do(func() {
		fallback := strings.Join([]string{os.Getenv("PATH"), "/opt/homebrew/bin", "/usr/local/bin", "/usr/bin", "/bin", "/usr/sbin", "/sbin"}, ":")
		loginPath = fallback
		shell := os.Getenv("SHELL")
		switch shell {
		case "/bin/zsh", "/bin/bash", "/bin/sh", "/opt/homebrew/bin/bash", "/opt/homebrew/bin/zsh", "/usr/local/bin/bash", "/usr/local/bin/zsh":
		default:
			shell = "/bin/zsh"
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		out, err := exec.CommandContext(ctx, shell, "-l", "-c", `printf %s "$PATH"`).Output()
		if p := strings.TrimSpace(string(out)); err == nil && p != "" {
			loginPath = p + ":" + fallback
		}
	})
	return loginPath
}

// Lookup finds an executable in a PATH list without consulting the process PATH.
func Lookup(name, path string) (string, error) {
	for _, dir := range filepath.SplitList(path) {
		if dir == "" || !filepath.IsAbs(dir) {
			continue
		}
		candidate := filepath.Join(dir, name)
		if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() && info.Mode()&0o111 != 0 {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("%s is not installed (not found on your login PATH)", name)
}

// Result is what one run reports back to the UI.
type Result struct {
	Command  string `json:"command"`
	ExitCode int    `json:"exitCode"`
}

const (
	runTimeout = 15 * time.Minute
	// stopGrace is how long a stopped command gets to exit before it is killed.
	stopGrace = 3 * time.Second
	// outputGrace bounds the wait for the last output: a background child that inherited
	// the terminal must not keep a finished run open.
	outputGrace = time.Second
)

// Run executes an allowlisted command with the login PATH, streaming what it prints to
// output as it happens. Cancelling ctx stops it.
func Run(ctx context.Context, command string, cols, rows uint16, output io.Writer) (Result, error) {
	res := Result{Command: command}
	argv, ok := Argv(command)
	if !ok {
		return res, errors.New("this command can't be run from oh-my-disk; copy it instead")
	}
	path := LoginPath()
	bin, err := Lookup(argv[0], path)
	if err != nil {
		return res, err
	}
	res.ExitCode, err = execute(ctx, bin, argv[1:], path, cols, rows, output)
	if err != nil {
		return res, fmt.Errorf("%s failed: %w", command, err)
	}
	return res, nil
}

// execute gives the command a real terminal for its output, so tools stream lines and
// progress instead of buffering them for a pipe. Stdin stays empty and the terminal isn't
// a controlling one, so nothing can sit waiting on a prompt no one can answer.
func execute(ctx context.Context, bin string, args []string, path string, cols, rows uint16, output io.Writer) (int, error) {
	ptmx, tty, err := pty.Open()
	if err != nil {
		return -1, err
	}
	defer ptmx.Close()
	if err = pty.Setsize(ptmx, &pty.Winsize{Cols: cols, Rows: rows}); err != nil {
		tty.Close()
		return -1, err
	}
	ctx, cancel := context.WithTimeout(ctx, runTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = []string{"PATH=" + path, "TERM=xterm-256color"}
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "PATH=") && !strings.HasPrefix(kv, "TERM=") {
			cmd.Env = append(cmd.Env, kv)
		}
	}
	if home, e := os.UserHomeDir(); e == nil {
		cmd.Dir = home
	}
	cmd.Stdout, cmd.Stderr = tty, tty
	// Its own process group, so stopping it also stops whatever it started.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Cancel = func() error {
		if e := syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM); e != nil {
			return os.ErrProcessDone
		}
		return nil
	}
	cmd.WaitDelay = stopGrace
	err = cmd.Start()
	tty.Close()
	if err != nil {
		return -1, err
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		io.Copy(output, ptmx) // ends with an error once the terminal closes; that's normal
	}()
	err = cmd.Wait()
	select {
	case <-done:
	case <-time.After(outputGrace):
	}
	code := -1
	if cmd.ProcessState != nil {
		code = cmd.ProcessState.ExitCode()
	}
	return code, err
}
