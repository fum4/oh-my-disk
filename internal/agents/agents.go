// Package agents contains session-only agent configuration and reusable advisor context.
package agents

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

//go:embed context.md
var Context string

const InitialPrompt = "Analyze my latest oh-my-disk scan and tell me what's taking space and what's safe to clean, biggest wins first."
const BundledCodex = "/Applications/ChatGPT.app/Contents/Resources/codex-cli/CodexCLI.app/Contents/MacOS/codex"

// Definition is the single registration point for detection, presentation and launch.
// Commands are trusted registry literals; user input is passed only as argv.
type Definition struct {
	Name       string                                    `json:"name"`
	Label      string                                    `json:"label"`
	Icon       string                                    `json:"icon"`
	InstallURL string                                    `json:"installURL"`
	Command    string                                    `json:"-"`
	Fallback   string                                    `json:"-"`
	Launch     func(binary, dir, prompt string) []string `json:"-"`
}

var registry = []Definition{
	{Name: "claude", Label: "Claude", Icon: "regular-magnifying-glass", InstallURL: "https://code.claude.com/docs/en/setup", Command: "claude", Launch: func(binary, dir, prompt string) []string {
		return []string{"--mcp-config", filepath.Join(dir, "mcp.json"), "--strict-mcp-config", "--tools", "", "--append-system-prompt", Context, "--", prompt}
	}},
	{Name: "codex", Label: "Codex", Icon: "regular-magnifying-glass", InstallURL: "https://developers.openai.com/codex/cli", Command: "codex", Fallback: BundledCodex, Launch: func(binary, dir, prompt string) []string {
		q := func(s string) string { b, _ := json.Marshal(s); return string(b) }
		return []string{"--no-daemon", "--sandbox", "read-only", "-c", "mcp_servers={}", "-c", "mcp_servers.oh-my-disk.command=" + q(binary), "-c", `mcp_servers.oh-my-disk.args=["mcp"]`, "-c", "mcp_servers.oh-my-disk.env.OMD_SESSION_SCAN=" + q(filepath.Join(dir, "scan.bin")), "-c", "developer_instructions=" + q(Context), "--", prompt}
	}},
}

func Registry() []Definition { return append([]Definition(nil), registry...) }
func definition(name string) (Definition, bool) {
	for _, d := range registry {
		if d.Name == name {
			return d, true
		}
	}
	return Definition{}, false
}

type CLI struct {
	Definition
	Path string `json:"path"`
}

func executable(p string) bool {
	f, e := os.Stat(p)
	return e == nil && f.Mode().IsRegular() && f.Mode()&0111 != 0 && filepath.IsAbs(p)
}

// Resolve checks PATH, common native/npm/mise installs, then a bounded login-shell lookup.
func Resolve(name, home, shell string, lookup func(string) (string, error), login func(string, string) string) string {
	return resolve(name, home, shell, lookup, login, executable)
}

func resolve(name, home, shell string, lookup func(string) (string, error), login func(string, string) string, valid func(string) bool) string {
	d, ok := definition(name)
	if !ok {
		return ""
	}
	name = d.Command
	if p, e := lookup(name); e == nil && valid(p) {
		return p
	}
	dirs := []string{filepath.Join(home, ".local/bin"), "/opt/homebrew/bin", "/usr/local/bin", filepath.Join(home, ".npm-global/bin"), filepath.Join(home, ".npm/bin"), filepath.Join(home, ".local/share/mise/shims"), filepath.Join(home, ".mise/shims")}
	for _, d := range dirs {
		p := filepath.Join(d, name)
		if valid(p) {
			return p
		}
	}
	if d.Fallback != "" && valid(d.Fallback) {
		return d.Fallback
	}
	if filepath.IsAbs(shell) {
		p := strings.TrimSpace(login(shell, "command -v "+name))
		if valid(p) {
			return p
		}
	}
	return ""
}
func Detect() []CLI {
	home, _ := os.UserHomeDir()
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/zsh"
	}
	login := func(shell, cmd string) string {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		b, _ := exec.CommandContext(ctx, shell, "-lc", cmd).Output()
		lines := strings.Split(strings.TrimSpace(string(b)), "\n")
		return lines[len(lines)-1]
	}
	out := []CLI{}
	for _, d := range registry {
		out = append(out, CLI{d, Resolve(d.Name, home, shell, exec.LookPath, login)})
	}
	return out
}
func ClaudeConfig(binary, scan string) ([]byte, error) {
	return json.Marshal(map[string]any{"mcpServers": map[string]any{"oh-my-disk": map[string]any{"command": binary, "args": []string{"mcp"}, "env": map[string]string{"OMD_SESSION_SCAN": scan}}}})
}

// Args uses argv directly, never shell interpolation. JSON strings are valid TOML basic strings.
func Args(name, binary, dir, prompt string) ([]string, error) {
	if prompt == "" {
		prompt = InitialPrompt
	}
	if d, ok := definition(name); ok {
		return d.Launch(binary, dir, prompt), nil
	}
	return nil, fmt.Errorf("unknown agent")
}
