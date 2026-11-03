// Package cleanup runs the few tool-owned cleanup commands our rules suggest.
//
// Only exact command strings from the built-in rules are runnable, each mapped to a
// fixed argv. Nothing is passed through a shell and no scan- or user-derived text is
// ever interpolated, so path-templated commands (du, rm) can never be executed here.
package cleanup

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
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
	Output   string `json:"output"`
	ExitCode int    `json:"exitCode"`
}

const outputTail = 1200

// Tail keeps the last n bytes of output, trimmed to whole lines where possible.
func Tail(b []byte, n int) string {
	s := strings.TrimSpace(string(b))
	if len(s) <= n {
		return s
	}
	s = s[len(s)-n:]
	if i := strings.IndexByte(s, '\n'); i >= 0 && i < len(s)-1 {
		s = s[i+1:]
	}
	return "…" + s
}

// Run executes an allowlisted command with the login PATH. The error carries the
// output tail so the UI can show why it failed.
func Run(ctx context.Context, command string) (Result, error) {
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
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, argv[1:]...)
	env := []string{"PATH=" + path}
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "PATH=") {
			env = append(env, kv)
		}
	}
	cmd.Env = env
	if home, e := os.UserHomeDir(); e == nil {
		cmd.Dir = home
	}
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err = cmd.Run()
	res.Output = Tail(out.Bytes(), outputTail)
	if cmd.ProcessState != nil {
		res.ExitCode = cmd.ProcessState.ExitCode()
	}
	if err != nil {
		if res.Output != "" {
			return res, fmt.Errorf("%s failed: %s", command, res.Output)
		}
		return res, fmt.Errorf("%s failed: %v", command, err)
	}
	return res, nil
}
