package cleanup

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

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
	if _, err := Run(context.Background(), "rm -rf /"); err == nil {
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

func TestTail(t *testing.T) {
	if Tail([]byte("  ok\n"), 10) != "ok" {
		t.Fatal("trim")
	}
	got := Tail([]byte(strings.Repeat("x", 20)+"\nlast line"), 12)
	if got != "…last line" {
		t.Fatalf("Tail = %q", got)
	}
}
