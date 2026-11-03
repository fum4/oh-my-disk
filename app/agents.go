package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/creack/pty"
	"github.com/fum4/oh-my-disk/internal/agents"
	"github.com/fum4/oh-my-disk/internal/report"
	wr "github.com/wailsapp/wails/v2/pkg/runtime"
)

type agentSession struct {
	mu     sync.Mutex
	file   *os.File
	cmd    *exec.Cmd
	dir    string
	output chan []byte
	stop   chan struct{}
	exited bool
	waited bool
}
type TerminalOutput struct {
	Data   string `json:"data"`
	Exited bool   `json:"exited"`
}

func (a *App) Agents() []agents.CLI { return agents.Detect() }
func (a *App) StartAgent(name, prompt string, cols, rows uint16) error {
	return a.StartAgentWith(name, "", "", prompt, cols, rows)
}

// StartAgentWith launches with the Agent page model and reasoning choices.
func (a *App) StartAgentWith(name, model, effort, prompt string, cols, rows uint16) error {
	a.agentMu.Lock()
	defer a.agentMu.Unlock()
	if a.terminal != nil {
		return fmt.Errorf("close the current agent first")
	}
	var cli string
	for _, c := range agents.Detect() {
		if c.Name == name {
			cli = c.Path
		}
	}
	if cli == "" {
		return fmt.Errorf("agent is not installed; check again after installation")
	}
	a.mu.RLock()
	s := a.snapshot
	a.mu.RUnlock()
	dir, err := os.MkdirTemp("", "oh-my-disk-agent-")
	if err != nil {
		return err
	}
	ok := false
	defer func() {
		if !ok {
			os.RemoveAll(dir)
		}
	}()
	if s != nil {
		if err = report.SaveCache(filepath.Join(dir, "scan.bin"), s); err != nil {
			return err
		}
	} else if prompt == "" {
		prompt = "No scan has been loaded yet. Help me understand disk cleanup; suggest scanning in oh-my-disk for context before making specific recommendations."
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	binary := filepath.Join(filepath.Dir(exe), "oh-my-disk-cli")
	if _, err = os.Stat(binary); err != nil {
		return fmt.Errorf("bundled MCP helper missing; rebuild with make app")
	}
	config, err := agents.ClaudeConfig(binary, filepath.Join(dir, "scan.bin"))
	if err != nil {
		return err
	}
	for file, data := range map[string][]byte{"mcp.json": config, "AGENTS.md": []byte(agents.Context), "CLAUDE.md": []byte(agents.Context)} {
		if err = os.WriteFile(filepath.Join(dir, file), data, 0600); err != nil {
			return err
		}
	}
	args, err := agents.ArgsWith(name, binary, dir, prompt, agents.Options{Model: model, Effort: effort})
	if err != nil {
		return err
	}
	cmd := exec.Command(cli, args...)
	// A private workspace avoids loading instructions/hooks from arbitrary scanned folders.
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "TERM=xterm-256color", "COLORTERM=truecolor", "PATH="+filepath.Dir(cli)+string(os.PathListSeparator)+os.Getenv("PATH")+":/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin")
	if cols < 2 {
		cols = 100
	}
	if rows < 2 {
		rows = 24
	}

	t, err := startPTY(cmd, dir, cols, rows)
	if err != nil {
		return err
	}
	a.terminal = t
	ok = true
	return nil
}

func startPTY(cmd *exec.Cmd, dir string, cols, rows uint16) (*agentSession, error) {
	f, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: cols, Rows: rows})
	if err != nil {
		return nil, err
	}
	t := &agentSession{file: f, cmd: cmd, dir: dir, output: make(chan []byte, 128), stop: make(chan struct{})}
	go func() {
		cmd.Wait()
		t.mu.Lock()
		// Terminate lingering MCP children that inherited the terminal.
		syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		t.waited = true
		t.mu.Unlock()
	}()
	go func() {
		defer func() { f.Close(); t.mu.Lock(); t.exited = true; t.mu.Unlock() }()
		buf := make([]byte, 16384)
		for {
			n, e := f.Read(buf)
			if n > 0 {
				chunk := append([]byte(nil), buf[:n]...)
				select {
				case t.output <- chunk:
				case <-t.stop:
					return
				}
			}
			if e != nil {
				return
			}
		}
	}()
	return t, nil
}

func (a *App) ReadAgent() TerminalOutput {
	a.agentMu.Lock()
	defer a.agentMu.Unlock()
	t := a.terminal
	if t == nil {
		return TerminalOutput{Exited: true}
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	var data []byte
	for len(data) < 256<<10 {
		select {
		case chunk := <-t.output:
			data = append(data, chunk...)
		default:
			return TerminalOutput{base64.StdEncoding.EncodeToString(data), t.exited}
		}
	}
	return TerminalOutput{base64.StdEncoding.EncodeToString(data), t.exited && len(t.output) == 0}
}
func (a *App) WriteAgent(data string) error {
	if len(data) > 1<<20 {
		return fmt.Errorf("input too large")
	}
	a.agentMu.Lock()
	t := a.terminal
	a.agentMu.Unlock()
	if t == nil {
		return fmt.Errorf("no agent")
	}
	_, err := t.file.Write([]byte(data))
	return err
}
func (a *App) ResizeAgent(cols, rows uint16) error {
	a.agentMu.Lock()
	defer a.agentMu.Unlock()
	if a.terminal == nil {
		return nil
	}
	if cols < 2 || rows < 2 {
		return nil
	}
	return pty.Setsize(a.terminal.file, &pty.Winsize{Cols: cols, Rows: rows})
}
func (a *App) CloseAgent() {
	a.agentMu.Lock()
	defer a.agentMu.Unlock()
	t := a.terminal
	if t == nil {
		return
	}
	a.terminal = nil
	// The PTY creates a new session/process group. Kill the entire group, including MCP.
	t.mu.Lock()
	if !t.waited {
		syscall.Kill(-t.cmd.Process.Pid, syscall.SIGKILL)
	}
	t.mu.Unlock()
	close(t.stop)
	t.file.Close()
	os.RemoveAll(t.dir)
}

func (a *App) Paste() (string, error) { return wr.ClipboardGetText(a.ctx) }
