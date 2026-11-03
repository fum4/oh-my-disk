package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/fum4/oh-my-disk/internal/memlimit"
	"github.com/fum4/oh-my-disk/internal/report"
	"github.com/fum4/oh-my-disk/internal/rules"
	"github.com/fum4/oh-my-disk/internal/scanner"
	"github.com/fum4/oh-my-disk/internal/web"
)

const usage = `oh-my-disk — disk space analysis and explicit Move to Trash

Usage: oh-my-disk <scan|top|suggest|explain|ui|mcp|trash> [path] [options]

  scan [root]     Scan root (default: home), cache results and show totals
  top             Largest entries in the latest scan (sizes may overlap)
  suggest         Ranked, disjoint cleanup suggestions; never executes commands
  explain <path>  Explain a path from the latest scan
  mcp             Read-only MCP server over stdio
  trash <paths>   Move scanned items to Trash (--yes or interactive confirmation)
  ui              Serve the latest scan in a local browser

Common options:
  --root PATH       Scan this root; scan and ui accept / for whole disk
  --refresh         Rescan before top/suggest/explain/ui
  --json [FILE]     JSON to stdout (-) or a NEW file; never overwrite
  --summary FILE    Compact LLM-friendly JSON (- for stdout); --limit controls size
  --sort KEY        Suggestion order: score (default), size, stale
  --category NAME   Filter cleanup suggestions by category
  --safety NAME     Filter cleanup suggestions by safety
  --limit N         Top results (default 20)
  --depth N         Export tree depth (-1 = full, 0 = root only)
  --min-size SIZE   Export/suggestion threshold (e.g. 16MiB; default 1MiB)
  --config FILE     Additional YAML rules (custom rules take precedence)
  --home PATH       Home path used for ~/ rule patterns (default: current home)
  --workers N       Scanner concurrency (default 2× CPU count, max 64)
  --cross-mounts    Include other filesystem devices
  --cache FILE      Application scan cache (default: private OS temp directory)
  --no-cache        Do not read/write a scan cache
  --port N          UI port (default 0 = random localhost port)
  --binary          Display binary KiB/MiB/GiB instead of decimal units
  --no-open         Do not open a browser
  --help            Show this help

Scanning is read-only. Trash requires explicit user confirmation; no permanent
deletion. MCP tools are read-only. Cache/output paths are excluded from scans.
`

func main() {
	// Bound default Go heap growth on multi-million-entry homes; honor user tuning.
	memlimit.Apply()
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "oh-my-disk:", err)
		os.Exit(1)
	}
}
func parseBytes(s string) (int64, error) {
	s = strings.TrimSpace(strings.ToUpper(s))
	units := []struct {
		suffix string
		factor float64
	}{{"TIB", 1 << 40}, {"GIB", 1 << 30}, {"MIB", 1 << 20}, {"KIB", 1 << 10}, {"TB", 1e12}, {"GB", 1e9}, {"MB", 1e6}, {"KB", 1e3}, {"B", 1}}
	factor := float64(1)
	for _, u := range units {
		if strings.HasSuffix(s, u.suffix) {
			s = strings.TrimSpace(strings.TrimSuffix(s, u.suffix))
			factor = u.factor
			break
		}
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil || n < 0 || n*factor >= float64(1<<63) || n != n {
		return 0, fmt.Errorf("invalid size %q", s)
	}
	return int64(n * factor), nil
}
func normalize(args []string) []string {
	out := []string{}
	for i, a := range args {
		if a == "--json" && (i+1 == len(args) || args[i+1] != "-" && strings.HasPrefix(args[i+1], "-")) {
			a = "--json=-"
		}
		out = append(out, a)
	}
	return out
}
func run(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "help" {
		fmt.Print(usage)
		return nil
	}
	cmd := args[0]
	if cmd == "trash" {
		return runTrash(args[1:])
	}
	if cmd == "mcp" {
		return runMCP()
	}
	switch cmd {
	case "scan", "top", "suggest", "explain", "ui":
	default:
		return fmt.Errorf("unknown command %q; use --help", cmd)
	}
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	home, _ := os.UserHomeDir()
	root := fs.String("root", "", "scan root")
	homeFlag := fs.String("home", home, "rule home")
	refresh := fs.Bool("refresh", false, "rescan")
	jsonPath := fs.String("json", "", "JSON output")
	summary := fs.String("summary", "", "summary output")
	sortBy := fs.String("sort", "score", "suggestion sort: score|size|stale")
	category := fs.String("category", "", "filter suggestion category")
	safety := fs.String("safety", "", "filter suggestion safety")
	limit := fs.Int("limit", 20, "limit")
	depth := fs.Int("depth", -1, "export depth")
	minSize := fs.String("min-size", "1MiB", "minimum bytes")
	config := fs.String("config", "", "rules YAML")
	workers := fs.Int("workers", 0, "workers")
	cross := fs.Bool("cross-mounts", false, "cross devices")
	cache := fs.String("cache", report.CachePath(), "scan cache")
	noCache := fs.Bool("no-cache", false, "no cache")
	port := fs.Int("port", 0, "UI port")
	binary := fs.Bool("binary", false, "binary display units")
	noOpen := fs.Bool("no-open", false, "don't open browser")
	rest := args[1:]
	pos := ""
	if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
		pos = rest[0]
		rest = rest[1:]
	}
	if err := fs.Parse(normalize(rest)); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() > 0 {
		if pos != "" || fs.NArg() > 1 {
			return errors.New("too many positional arguments")
		}
		pos = fs.Arg(0)
	}
	if *limit < 1 || *limit > 10000 || *depth < -1 || *workers < 0 || *port < 0 || *port > 65535 {
		return errors.New("invalid limit, depth, workers or port")
	}
	if pos != "" && cmd != "explain" {
		if cmd != "scan" && cmd != "ui" {
			return errors.New("use --root PATH --refresh to scan a different root")
		}
		if *root != "" {
			return errors.New("use either positional root or --root")
		}
		*root = pos
	}
	if cmd == "explain" && pos == "" {
		return errors.New("explain requires a path")
	}
	if *jsonPath != "" && *jsonPath == *summary {
		return errors.New("JSON and summary destinations must differ")
	}
	min, err := parseBytes(*minSize)
	if err != nil {
		return err
	}
	*homeFlag, err = filepath.Abs(*homeFlag)
	if err != nil {
		return err
	}
	eng, err := rules.Load(*config, *homeFlag)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	display := startProgress(os.Stderr)
	defer display.close()
	var s *report.Snapshot
	doScan := cmd == "scan" || *refresh || *noCache
	if !doScan {
		display.phase("reading cache")
		s, err = report.LoadCache(*cache)
		if err != nil {
			if os.IsNotExist(err) {
				doScan = true
			} else {
				return fmt.Errorf("read cache (use --refresh to replace it): %w", err)
			}
		}
	}
	if !doScan && *root != "" {
		abs, e := filepath.Abs(*root)
		if e != nil {
			return e
		}
		doScan = abs != s.Scan.Root.Path
	}
	if doScan {
		if *root == "" {
			*root = home
		}
		excludes := []string{filepath.Dir(*cache)}
		for _, p := range []string{*jsonPath, *summary} {
			if p != "" && p != "-" {
				excludes = append(excludes, p)
			}
		}
		// A custom cache may sit beside source files; exclude only its file there.
		if *cache != report.CachePath() {
			excludes[0] = *cache
		}
		display.phase("walking")
		scan, e := scanner.Scan(ctx, *root, scanner.Options{Workers: *workers, CrossMounts: *cross, Exclude: excludes, Progress: display.update})
		if e != nil {
			return e
		}
		display.phase("classifying")
		s = report.New(scan, eng, min)
		if !*noCache {
			display.phase("writing cache")
			if err = report.SaveCache(*cache, s); err != nil {
				return fmt.Errorf("save application cache: %w", err)
			}
		}
	} else {
		display.phase("classifying")
		s = report.New(s.Scan, eng, min)
	}
	display.phase("preparing output")
	if *summary != "" {
		if err = report.WriteJSON(*summary, report.Summary(s, *limit)); err != nil {
			return err
		}
	}
	var output any
	switch cmd {
	case "scan":
		output = report.Export(s, *depth, min)
	case "top":
		output = report.Top(s.Scan.Root, *limit)
	case "suggest":
		v, e := rules.Select(s.Suggestions, *sortBy, *category, *safety, *limit)
		if e != nil {
			return e
		}
		if len(v) > *limit {
			v = v[:*limit]
		}
		output = v
	case "explain":
		p, e := filepath.Abs(pos)
		if e != nil {
			return e
		}
		if s.Scan.Tree != nil {
			output, err = eng.ExplainTree(s.Scan.Tree, p, time.Now())
		} else {
			output, err = eng.Explain(s.Scan.Root, p, time.Now())
		}
		if err != nil {
			return err
		}
	case "ui":
		display.phase("starting UI")
		display.close()
		return web.Serve(ctx, s, web.Options{Port: *port, Open: !*noOpen, JSON: *jsonPath})
	}
	if *jsonPath != "" {
		display.phase("exporting JSON")
		return report.WriteJSON(*jsonPath, output)
	}
	if *summary == "-" {
		return nil
	}
	humanUnits(os.Stdout, cmd, s, output, *limit, *binary)
	return nil
}
func human(w io.Writer, cmd string, s *report.Snapshot, output any, limit int) {
	humanUnits(w, cmd, s, output, limit, false)
}
func humanUnits(w io.Writer, cmd string, s *report.Snapshot, output any, limit int, binary bool) {
	size := func(b int64) string { return report.FormatBytes(b, binary) }
	if v, err := report.VolumeFor(s.Scan.Root.Path, s.Scan.Root.Bytes); err == nil {
		fmt.Fprintf(w, "Volume: %s capacity · %s used · %s available · %s outside scanned folder\n", size(v.Capacity), size(v.Used), size(v.Available), size(v.Outside))
	}
	fmt.Fprintln(w, report.VolumeNote)
	fmt.Fprintf(w, "%s  %s allocated · %d files · scanned %s\n", s.Scan.Root.Path, size(s.Scan.Root.Bytes), s.Scan.Root.Files, s.Scan.Finished.Format(time.RFC3339))
	switch cmd {
	case "scan":
		for _, n := range s.Scan.Root.Children {
			fmt.Fprintf(w, "%10s  %8d files  %s\n", size(n.Bytes), n.Files, n.Name)
		}
	case "top":
		fmt.Fprintln(w, "Largest entries (parent/child sizes overlap):")
		for _, n := range output.([]*scanner.Node) {
			fmt.Fprintf(w, "%10s  %s\n", size(n.Bytes), n.Path)
		}
	case "suggest":
		for i, v := range output.([]rules.Suggestion) {
			fmt.Fprintf(w, "%2d. %10s  %-14s  %s\n    %s · stale %dd · cleanup score %.0f/100\n    %s\n", i+1, size(v.Bytes), v.Safety, v.Path, v.Category, v.StaleDays, rules.CleanupScore(v.Score), v.Explanation)
			if v.Command != "" {
				fmt.Fprintf(w, "    Human-only suggestion: %s\n", v.Command)
			}
		}
	case "explain":
		v := output.(*rules.Suggestion)
		fmt.Fprintf(w, "%s\n%s · %s · %s\n%s\nLast activity: %s (%d days); cleanup score %.0f/100\n", v.Path, size(v.Bytes), v.Category, v.Safety, v.Explanation, v.LastActivity.Format(time.RFC3339), v.StaleDays, rules.CleanupScore(v.Score))
		if v.ProjectPath != "" {
			fmt.Fprintln(w, "Project activity:", v.ProjectPath)
		}
		if v.Command != "" {
			fmt.Fprintln(w, "Human-only suggestion:", v.Command)
		}
	}
	if len(s.Scan.Skipped) > 0 {
		fmt.Fprintf(w, "\nSkipped %d paths (totals are partial):\n", len(s.Scan.Skipped))
		for i, p := range s.Scan.Skipped {
			if i == 20 {
				fmt.Fprintln(w, "… full list in scan --json")
				break
			}
			fmt.Fprintf(w, "  %s: %s\n", p.Path, p.Reason)
		}
	}
	fmt.Fprintln(w, "\nRead-only. No cleanup commands are executed. Sizes are allocated bytes, not guaranteed reclaimable space.")
}
