package main

import (
	"context"
	"encoding/json"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestMCPStdioHelper(t *testing.T) {
	if os.Getenv("OMD_MCP_HELPER") == "1" {
		if err := runMCP(); err != nil {
			os.Exit(2)
		}
		os.Exit(0)
	}
}
func TestMCPOverStdio(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.Command(os.Args[0], "-test.run=^TestMCPStdioHelper$")
	cmd.Env = append(os.Environ(), "OMD_MCP_HELPER=1")
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	listed, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Tools) != 6 {
		t.Fatalf("tools: %v", listed.Tools)
	}
	for _, tool := range listed.Tools {
		if tool.Name == "trash" || tool.Annotations == nil || !tool.Annotations.ReadOnlyHint {
			t.Fatal("non-read-only tool", tool.Name)
		}
	}
	root := t.TempDir()
	p := filepath.Join(root, "blob")
	if err = os.WriteFile(p, make([]byte, 2<<20), 0600); err != nil {
		t.Fatal(err)
	}
	for _, call := range []struct {
		name string
		args map[string]any
	}{
		{"scan", map[string]any{"root": root, "refresh": true}},
		{"summary", map[string]any{"limit": 1}},
		{"top", map[string]any{"path": root, "depth": 1, "limit": 1}},
		{"suggest", map[string]any{"sort": "size", "limit": 1}},
		{"explain", map[string]any{"path": p}},
		{"whole_disk", map[string]any{}},
	} {
		r, e := session.CallTool(ctx, &mcp.CallToolParams{Name: call.name, Arguments: call.args})
		if e != nil {
			t.Fatal(e)
		}
		if r.IsError || len(r.Content) != 1 {
			t.Fatalf("%s: %+v", call.name, r)
		}
		data := r.Content[0].(*mcp.TextContent).Text
		if len(data) > 64<<10 || !json.Valid([]byte(data)) {
			t.Fatalf("unbounded/invalid JSON: %s", call.name)
		}
	}
	for _, a := range []map[string]any{{"sort": "bogus"}, {"limit": 10000}} {
		r, e := session.CallTool(ctx, &mcp.CallToolParams{Name: "suggest", Arguments: a})
		if e == nil && !r.IsError {
			t.Fatal("accepted invalid input")
		}
	}
}
