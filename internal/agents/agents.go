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
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
)

//go:embed context.md
var Context string

const InitialPrompt = "Analyze my latest oh-my-disk scan. Reply with a small overview of 2 or 3 sentences on what's taking space and how much I could get back, then a list of what to clean, biggest wins first. For each item give its name and size, what it is, and the command to clean it up, as your instructions describe. Mark anything I should check before removing. End your reply with the list: no notes or closing remarks after it."
const DefaultCodexModel = "gpt-6.1-sol"
const DefaultClaudeModel = "opus"

const DefaultEffort = "medium"

// Model is one choice in the Agent page model picker.
type Model struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// Options are the per-launch choices from the Agent page. Empty values use the defaults.
type Options struct {
	Model  string `json:"model"`
	Effort string `json:"effort"`
}

var modelID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,63}$`)

// pick returns the env override first (developer escape hatch), then a well-formed chosen value, then the default.
func pick(env, chosen, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(env)); value != "" {
		return value
	}
	if chosen = strings.TrimSpace(chosen); modelID.MatchString(chosen) {
		return chosen
	}
	return fallback
}

var efforts = []string{"low", "medium", "high"}

func effort(chosen string) string {
	if slices.Contains(efforts, chosen) {
		return chosen
	}
	return DefaultEffort
}

var codexModels = []Model{{"gpt-6.1-sol", "GPT-6.1-Sol"}, {"gpt-6-sol", "GPT-6-Sol"}, {"gpt-6.1-terra", "GPT-6.1 Terra"}, {"gpt-6-luna", "GPT-6-Luna"}}
var claudeModels = []Model{{"opus", "Opus"}, {"sonnet", "Sonnet"}, {"fable", "Fable"}}

// ParseCodexModels reads `codex debug models` JSON, keeping models listed for the signed-in account.
func ParseCodexModels(data []byte) []Model {
	var catalog struct {
		Models []struct {
			Slug        string `json:"slug"`
			DisplayName string `json:"display_name"`
			Visibility  string `json:"visibility"`
		} `json:"models"`
	}
	if json.Unmarshal(data, &catalog) != nil {
		return nil
	}
	var out []Model
	for _, m := range catalog.Models {
		if m.Visibility != "list" || !modelID.MatchString(m.Slug) {
			continue
		}
		label := m.DisplayName
		if label == "" {
			label = m.Slug
		}
		out = append(out, Model{m.Slug, label})
	}
	if len(out) == 0 {
		return nil
	}
	// The explicit default always stays first and available.
	i := slices.IndexFunc(out, func(m Model) bool { return m.ID == DefaultCodexModel })
	if i < 0 {
		return append([]Model{codexModels[0]}, out...)
	}
	return append(append([]Model{out[i]}, out[:i]...), out[i+1:]...)
}

func discoverCodexModels(binary string) []Model {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	b, err := exec.CommandContext(ctx, binary, "debug", "models").Output()
	if err != nil {
		return nil
	}
	return ParseCodexModels(b)
}

const BundledCodex = "/Applications/ChatGPT.app/Contents/Resources/codex-cli/CodexCLI.app/Contents/MacOS/codex"

// Definition is the single registration point for detection, presentation and launch.
// Commands are trusted registry literals; user input is passed only as argv.
type Definition struct {
	Name          string                                               `json:"name"`
	Label         string                                               `json:"label"`
	Icon          string                                               `json:"icon"`
	InstallURL    string                                               `json:"installURL"`
	Command       string                                               `json:"-"`
	Fallback      string                                               `json:"-"`
	Models        []Model                                              `json:"models"`
	DefaultModel  string                                               `json:"defaultModel"`
	Efforts       []string                                             `json:"efforts,omitempty"`
	DefaultEffort string                                               `json:"defaultEffort,omitempty"`
	ModelEnv      string                                               `json:"modelEnv"`
	Launch        func(binary, dir, prompt string, o Options) []string `json:"-"`
	discover      func(binary string) []Model
}

var registry = []Definition{
	{Name: "claude", Label: "Claude", Icon: "agent-claude", InstallURL: "https://code.claude.com/docs/en/setup", Command: "claude", Models: claudeModels, DefaultModel: DefaultClaudeModel, ModelEnv: "OMD_CLAUDE_MODEL", Launch: func(binary, dir, prompt string, o Options) []string {
		return []string{"--model", pick("OMD_CLAUDE_MODEL", o.Model, DefaultClaudeModel), "--mcp-config", filepath.Join(dir, "mcp.json"), "--strict-mcp-config", "--tools", "", "--append-system-prompt", Context, "--", prompt}
	}},
	{Name: "codex", Label: "Codex", Icon: "agent-codex", InstallURL: "https://developers.openai.com/codex/cli", Command: "codex", Fallback: BundledCodex, Models: codexModels, DefaultModel: DefaultCodexModel, Efforts: efforts, DefaultEffort: DefaultEffort, ModelEnv: "OMD_CODEX_MODEL", discover: discoverCodexModels, Launch: func(binary, dir, prompt string, o Options) []string {
		q := func(s string) string { b, _ := json.Marshal(s); return string(b) }
		return []string{"-m", pick("OMD_CODEX_MODEL", o.Model, DefaultCodexModel), "-c", "model_reasoning_effort=" + effort(o.Effort), "--no-daemon", "--sandbox", "read-only", "-c", "mcp_servers={}", "-c", "mcp_servers.oh-my-disk.command=" + q(binary), "-c", `mcp_servers.oh-my-disk.args=["mcp"]`, "-c", "mcp_servers.oh-my-disk.env.OMD_SESSION_SCAN=" + q(filepath.Join(dir, "scan.bin")), "-c", "developer_instructions=" + q(Context), "--", prompt}
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
	// ModelOverride is the env model, which wins over the picker.
	ModelOverride string `json:"modelOverride,omitempty"`
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
		ctx, cancel := context.WithTimeout(context.Background(), 750*time.Millisecond)
		defer cancel()
		b, _ := exec.CommandContext(ctx, shell, "-lc", cmd).Output()
		lines := strings.Split(strings.TrimSpace(string(b)), "\n")
		return lines[len(lines)-1]
	}
	out := make([]CLI, len(registry))
	var workers sync.WaitGroup
	for i, d := range registry {
		workers.Add(1)
		go func() {
			defer workers.Done()
			c := CLI{Definition: d, Path: Resolve(d.Name, home, shell, exec.LookPath, login), ModelOverride: strings.TrimSpace(os.Getenv(d.ModelEnv))}
			if c.Path != "" && d.discover != nil {
				if models := d.discover(c.Path); len(models) > 0 {
					c.Models = models
				}
			}
			out[i] = c
		}()
	}
	workers.Wait()
	return out
}
func ClaudeConfig(binary, scan string) ([]byte, error) {
	return json.Marshal(map[string]any{"mcpServers": map[string]any{"oh-my-disk": map[string]any{"command": binary, "args": []string{"mcp"}, "env": map[string]string{"OMD_SESSION_SCAN": scan}}}})
}

// Args uses argv directly, never shell interpolation. JSON strings are valid TOML basic strings.
func Args(name, binary, dir, prompt string) ([]string, error) {
	return ArgsWith(name, binary, dir, prompt, Options{})
}

// ArgsWith applies the Agent page model and effort choices.
func ArgsWith(name, binary, dir, prompt string, o Options) ([]string, error) {
	if prompt == "" {
		prompt = InitialPrompt
	}
	if d, ok := definition(name); ok {
		return d.Launch(binary, dir, prompt, o), nil
	}
	return nil, fmt.Errorf("unknown agent")
}
