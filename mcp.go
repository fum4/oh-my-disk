package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/fum4/oh-my-disk/internal/report"
	"github.com/fum4/oh-my-disk/internal/rules"
	"github.com/fum4/oh-my-disk/internal/scanner"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type mcpArgs struct {
	Root     string `json:"root,omitempty"`
	Refresh  bool   `json:"refresh,omitempty"`
	Path     string `json:"path,omitempty"`
	Depth    int    `json:"depth,omitempty"`
	Limit    int    `json:"limit,omitempty"`
	Sort     string `json:"sort,omitempty"`
	Category string `json:"category,omitempty"`
	Safety   string `json:"safety,omitempty"`
}

func runMCP() error { return newMCPServer().Run(context.Background(), &mcp.StdioTransport{}) }
func newMCPServer() *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "oh-my-disk", Version: "v1"}, nil)
	var mu sync.Mutex
	var snapshot *report.Snapshot
	for name, description := range map[string]string{
		"scan":       "Read-only metadata scan. root defaults to home; refresh forces rescan. No file contents read.",
		"summary":    "Compact latest scan summary. limit 1–100 (default 20).",
		"top":        "Largest entries under path, within depth 1–8 (default 1). limit 1–100. Sizes can overlap.",
		"suggest":    "Cleanup suggestions. sort score|size|stale, optional category/safety, limit 1–100.",
		"explain":    "Explain one absolute path in the latest scan.",
		"whole_disk": "Live containing-volume accounting including space outside scan. APFS purgeable/accounting caveats included.",
	} {
		mcp.AddTool(server, &mcp.Tool{Name: name, Description: description, Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true}}, func(ctx context.Context, _ *mcp.CallToolRequest, a mcpArgs) (*mcp.CallToolResult, any, error) {
			mu.Lock()
			defer mu.Unlock()
			limit := a.Limit
			if limit == 0 {
				limit = 20
			}
			if limit < 1 || limit > 100 {
				return nil, nil, fmt.Errorf("limit must be 1–100")
			}
			home, _ := os.UserHomeDir()
			eng, err := rules.Load("", home)
			if err != nil {
				return nil, nil, err
			}
			if snapshot == nil {
				cache := os.Getenv("OMD_SESSION_SCAN")
				if cache == "" {
					cache = report.CachePath()
				}
				snapshot, _ = report.LoadCache(cache)
			}
			if name == "scan" {
				root := a.Root
				if root == "" {
					root = home
				}
				root, err = filepath.Abs(root)
				if err != nil {
					return nil, nil, err
				}
				if snapshot == nil || a.Refresh || snapshot.Scan.Root.Path != root {
					scan, e := scanner.Scan(ctx, root, scanner.Options{Exclude: []string{filepath.Dir(report.CachePath())}})
					if e != nil {
						return nil, nil, e
					}
					snapshot = report.New(scan, eng, 1<<20)
				}
			}
			if snapshot == nil {
				return nil, nil, fmt.Errorf("no scan available; call scan first")
			}
			var out any
			switch name {
			case "scan", "summary":
				out = report.Summary(snapshot, limit)
			case "suggest":
				out, err = rules.Select(snapshot.Suggestions, a.Sort, a.Category, a.Safety, limit)
			case "explain":
				out, err = eng.Explain(snapshot.Scan.Root, a.Path, time.Now())
			case "whole_disk":
				var v report.Volume
				v, err = report.VolumeFor(snapshot.Scan.Root.Path, snapshot.Scan.Root.Bytes)
				out = map[string]any{"volume": v, "scanned_bytes": snapshot.Scan.Root.Bytes, "note": report.VolumeNote, "units": "decimal GB = 1,000,000,000 bytes"}
			case "top":
				path := a.Path
				if path == "" {
					path = snapshot.Scan.Root.Path
				}
				n := scanner.Find(snapshot.Scan.Root, path)
				if n == nil {
					return nil, nil, fmt.Errorf("path absent from scan")
				}
				depth := a.Depth
				if depth == 0 {
					depth = 1
				}
				if depth < 1 || depth > 8 {
					return nil, nil, fmt.Errorf("depth must be 1–8")
				}
				t := snapshot.Scan.Tree
				if t == nil {
					t = scanner.Compact(snapshot.Scan.Root)
				}
				id := t.Find(path)
				ids := []int32{}
				var visit func(int32, int)
				visit = func(parent int32, d int) {
					if d == 0 {
						return
					}
					for c := t.At(parent).First; c >= 0; c = t.At(c).Next {
						ids = append(ids, c)
						visit(c, d-1)
					}
				}
				visit(id, depth)
				sort.Slice(ids, func(i, j int) bool {
					if t.Bytes(ids[i]) == t.Bytes(ids[j]) {
						return t.LessPath(ids[i], ids[j])
					}
					return t.Bytes(ids[i]) > t.Bytes(ids[j])
				})
				if len(ids) > limit {
					ids = ids[:limit]
				}
				items := []*scanner.Node{}
				for _, id := range ids {
					items = append(items, t.View(id, 0, 0))
				}
				out = items
			}
			if err != nil {
				return nil, nil, err
			}
			data, err := json.Marshal(out)
			if err != nil {
				return nil, nil, err
			}
			if len(data) > 64<<10 {
				return nil, nil, fmt.Errorf("response exceeds 64 KiB; lower limit or narrow path/category")
			}
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(data)}}}, nil, nil
		})
	}
	return server
}
