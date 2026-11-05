package cleanup

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestArgvExactAllowlist(t *testing.T) {
	argv, ok := Argv("pnpm store prune")
	if !ok || !reflect.DeepEqual(argv, []string{"pnpm", "store", "prune"}) {
		t.Fatalf("pnpm store prune -> %v %v", argv, ok)
	}
	argv[0] = "rm" // callers get a copy
	if again, _ := Argv("pnpm store prune"); again[0] != "pnpm" {
		t.Fatal("Argv leaked the allowlist slice")
	}
	for _, c := range []string{
		"", "pnpm", " pnpm store prune", "pnpm store prune ", "pnpm  store prune",
		"pnpm store prune; rm -rf ~", "pnpm store prune && echo", "pnpm store prune # x",
		"brew cleanup --prune=all", "rm -rf -- '/Users/x/Library/Caches'",
		"rm -rf -- {path}  # review first; SwiftPM will rebuild", "du -sh {path}/*",
		"pip cache purge  # or: uv cache clean", "open -a Xcode", "docker system df -v",
	} {
		if _, ok := Argv(c); ok {
			t.Errorf("%q must not be runnable", c)
		}
	}
}

// Every runnable command is one of our own rule commands, verbatim, and its argv is
// plain words: no path templates, removals, shell syntax or home expansion.
func TestAllowlistComesFromRules(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "rules", "rules.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Rules []struct {
			Command string `yaml:"command"`
		} `yaml:"rules"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	inRules := map[string]bool{}
	for _, r := range doc.Rules {
		inRules[r.Command] = true
	}
	for command, argv := range allowed {
		if !inRules[command] {
			t.Errorf("%q is not a rules.yaml command", command)
		}
		if strings.Join(argv, " ") != command {
			t.Errorf("%q argv %q drifted from the command text", command, argv)
		}
		if argv[0] == "rm" || argv[0] == "sh" || argv[0] == "bash" || argv[0] == "zsh" {
			t.Errorf("%q runs %s", command, argv[0])
		}
		for _, a := range argv {
			if strings.ContainsAny(a, "{}$`;&|<>*~#'\"\\") {
				t.Errorf("%q has unsafe argv word %q", command, a)
			}
		}
	}
	if len(Commands()) != len(allowed) {
		t.Fatal("Commands() incomplete")
	}
}

func TestRunRejectsUnlisted(t *testing.T) {
	if _, err := Run(context.Background(), "rm -rf /", 80, 24, io.Discard); err == nil {
		t.Fatal("unlisted command ran")
	}
}

func TestLookupUsesOnlyAbsoluteDirs(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "pnpm")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got, err := Lookup("pnpm", "relative:"+dir); err != nil || got != bin {
		t.Fatalf("Lookup = %q %v", got, err)
	}
	if _, err := Lookup("pnpm", "relative"); err == nil {
		t.Fatal("relative PATH entries must be ignored")
	}
}

func script(t *testing.T, body string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "tool")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin
}

// syncBuffer is safe to read while a run is still writing to it.
type syncBuffer struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestExecuteStreamsThroughATerminal(t *testing.T) {
	bin := script(t, `[ -t 1 ] && echo "stdout is a terminal"; stty size <&1; echo oops >&2`)
	var out syncBuffer
	code, err := execute(context.Background(), bin, nil, os.Getenv("PATH"), 91, 17, &out)
	if err != nil || code != 0 {
		t.Fatalf("execute = %d %v, output %q", code, err, out.String())
	}
	for _, want := range []string{"stdout is a terminal", "17 91", "oops"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output %q lacks %q", out.String(), want)
		}
	}
}

func TestExecuteReportsFailure(t *testing.T) {
	var out syncBuffer
	code, err := execute(context.Background(), script(t, "echo nope; exit 3"), nil, os.Getenv("PATH"), 80, 24, &out)
	if err == nil || code != 3 || !strings.Contains(out.String(), "nope") {
		t.Fatalf("execute = %d %v, output %q", code, err, out.String())
	}
}

// A command that asks a question gets an empty answer rather than hanging the run.
func TestExecuteNeverWaitsForInput(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var out syncBuffer
	if _, err := execute(ctx, script(t, "read answer; echo answered"), nil, os.Getenv("PATH"), 80, 24, &out); ctx.Err() != nil {
		t.Fatalf("hung on input: %v", err)
	}
}

func TestExecuteStopsTheWholeProcessGroup(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "survived")
	bin := script(t, "(sleep 2; touch '"+marker+"') &\necho started\nwait")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var out syncBuffer
	result := make(chan error, 1)
	go func() {
		_, err := execute(ctx, bin, nil, os.Getenv("PATH"), 80, 24, &out)
		result <- err
	}()
	for deadline := time.Now().Add(20 * time.Second); !strings.Contains(out.String(), "started"); {
		if time.Now().After(deadline) {
			t.Fatal("never started")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("a stopped run must report an error")
		}
	case <-time.After(20 * time.Second):
		t.Fatal("did not stop")
	}
	time.Sleep(2500 * time.Millisecond) // past the child's own sleep: its marker must not appear
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("the child survived stopping the command")
	}
}
