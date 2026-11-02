package agents

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestResolve(t *testing.T) {
	home := t.TempDir()
	bin := filepath.Join(home, ".local/bin/claude")
	os.MkdirAll(filepath.Dir(bin), 0700)
	os.WriteFile(bin, []byte("#!/bin/sh\n"), 0700)
	missing := func(string) (string, error) { return "", errors.New("missing") }
	login := func(string, string) string { t.Fatal("unexpected shell lookup"); return "" }
	if got := Resolve("claude", home, "/bin/zsh", missing, login); got != bin {
		t.Fatal(got)
	}
	if got := Resolve("claude", home, "/bin/zsh", func(string) (string, error) { return bin, nil }, login); got != bin {
		t.Fatal(got)
	}
	if got := Resolve("invalid; touch /tmp/no", home, "/bin/zsh", missing, login); got != "" {
		t.Fatal(got)
	}
	for _, tc := range []struct {
		name, want  string
		files       []string
		shellResult string
	}{
		{"claude", "/login/claude", nil, "/login/claude"},
		{"claude", "", nil, "claude: shell function"},
		{"claude", "/h/.npm-global/bin/claude", []string{"/h/.npm-global/bin/claude"}, ""},
		{"codex", BundledCodex, []string{BundledCodex}, ""},
		{"codex", "/h/.local/share/mise/shims/codex", []string{"/h/.local/share/mise/shims/codex"}, ""},
	} {
		valid := func(p string) bool {
			if p == "/login/claude" {
				return true
			}
			for _, f := range tc.files {
				if p == f {
					return true
				}
			}
			return false
		}
		got := resolve(tc.name, "/h", "/bin/zsh", missing, func(sh, cmd string) string {
			if sh != "/bin/zsh" || cmd != "command -v "+tc.name {
				t.Fatal(sh, cmd)
			}
			return tc.shellResult
		}, valid)
		if got != tc.want {
			t.Fatalf("%s: got %q want %q", tc.name, got, tc.want)
		}
	}

}
func TestConfigAndArgs(t *testing.T) {
	binary := `/Applications/My "Disk" App/oh-my-disk-cli`
	dir := `/tmp/session space`
	scan := filepath.Join(dir, "scan.bin")
	b, err := ClaudeConfig(binary, scan)
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		Servers map[string]struct {
			Command string
			Args    []string
			Env     map[string]string
		} `json:"mcpServers"`
	}
	if err = json.Unmarshal(b, &config); err != nil {
		t.Fatal(err)
	}
	server := config.Servers["oh-my-disk"]
	if server.Command != binary || !reflect.DeepEqual(server.Args, []string{"mcp"}) || server.Env["OMD_SESSION_SCAN"] != scan {
		t.Fatalf("%s", b)
	}
	for _, name := range []string{"claude", "codex"} {
		args, e := Args(name, binary, dir, "")
		if e != nil {
			t.Fatal(e)
		}
		if args[len(args)-1] != InitialPrompt {
			t.Fatal(args)
		}
		joined := strings.Join(args, "\n")
		if name == "claude" {
			if !strings.Contains(joined, "--strict-mcp-config") || !strings.Contains(joined, Context) {
				t.Fatal(args)
			}
		} else {
			if !strings.Contains(joined, "read-only") || !strings.Contains(joined, "--no-daemon") {
				t.Fatal(args)
			}
			for _, arg := range args {
				if strings.HasPrefix(arg, "mcp_servers.oh-my-disk.command=") {
					var got string
					if e = json.Unmarshal([]byte(strings.SplitN(arg, "=", 2)[1]), &got); e != nil || got != binary {
						t.Fatal(arg, e)
					}
				}
			}
		}
		custom := `Explain /tmp/a; $(touch nope)`
		args, _ = Args(name, binary, dir, custom)
		if args[len(args)-1] != custom {
			t.Fatal("prompt changed")
		}
	}
	if _, e := Args("bogus", binary, dir, ""); e == nil {
		t.Fatal("unknown accepted")
	}
}
