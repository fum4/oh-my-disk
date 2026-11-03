package main

import (
	"bufio"
	"fmt"
	"github.com/fum4/oh-my-disk/internal/report"
	"github.com/fum4/oh-my-disk/internal/rules"
	"github.com/fum4/oh-my-disk/internal/trash"
	"os"
	"strings"
)

func runTrash(args []string) error {
	yes := false
	paths := []string{}
	for _, a := range args {
		if a == "--yes" {
			yes = true
		} else if strings.HasPrefix(a, "-") {
			return fmt.Errorf("unknown trash option: %s", a)
		} else {
			paths = append(paths, a)
		}
	}
	if len(paths) == 0 || len(paths) > 100 {
		return fmt.Errorf("trash requires 1–100 absolute paths from latest scan")
	}
	s, err := report.LoadCache(report.CachePath())
	if err != nil {
		return err
	}
	home, _ := os.UserHomeDir()
	eng, err := rules.Load("", home)
	if err != nil {
		return err
	}
	var total int64
	for _, p := range paths {
		n, e := trash.Validate(s, p, home, eng)
		if e != nil {
			return fmt.Errorf("%s: %w", p, e)
		}
		total += n.Bytes
		fmt.Fprintln(os.Stderr, p)
	}
	fmt.Fprintf(os.Stderr, "Move %d items (%s) to Trash? ", len(paths), report.HumanBytes(total))
	if !yes {
		fmt.Fprint(os.Stderr, "Type yes: ")
		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		if strings.TrimSpace(line) != "yes" {
			return fmt.Errorf("cancelled")
		}
	}
	failed := false
	results := trash.Execute(s, paths, home, eng)
	for _, r := range results {
		if r.Error != "" {
			failed = true
			fmt.Fprintf(os.Stderr, "%s: %s\n", r.Path, r.Error)
		} else {
			fmt.Fprintln(os.Stdout, "Moved to Trash:", r.Path)
		}
	}
	// Invalidate persisted totals; a subsequent command scans again.
	for _, r := range results {
		if r.Trashed {
			if e := os.Remove(report.CachePath()); e != nil && !os.IsNotExist(e) {
				return e
			}
			break
		}
	}
	if failed {
		return fmt.Errorf("one or more items failed; see per-item results")
	}
	return nil
}
