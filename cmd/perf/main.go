// perf measures the real pipeline without UI or export noise.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/fum4/oh-my-disk/internal/report"
	"github.com/fum4/oh-my-disk/internal/rules"
	"github.com/fum4/oh-my-disk/internal/scanner"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"runtime/pprof"
	"strings"
	"time"
)

func main() {
	export := flag.Bool("export", false, "time full streaming JSON to io.Discard")
	load := flag.String("load", "", "time loading an existing cache instead of scanning")
	min := flag.Int64("min", 1, "suggestion threshold in bytes (CLI default 1048576)")
	exclude := flag.String("exclude", "", "comma-separated excluded application/fixture paths")
	root := flag.String("root", ".fixtures/large", "fixture root")
	workers := flag.Int("workers", 0, "workers")
	cpu := flag.String("cpu", "", "CPU profile")
	heap := flag.String("heap", "", "heap profile")
	cache := flag.String("cache", "", "cache output")
	flag.Parse()
	if os.Getenv("GOMEMLIMIT") == "" {
		debug.SetMemoryLimit(704 << 20)
	}
	*root, _ = filepath.Abs(*root)
	if *cpu != "" {
		f, e := os.Create(*cpu)
		must(e)
		must(pprof.StartCPUProfile(f))
		defer f.Close()
		defer pprof.StopCPUProfile()
	}
	e, err := rules.Load("", *root)
	must(err)
	t := time.Now()
	var s *scanner.Result
	if *load != "" {
		cached, err := report.LoadCache(*load)
		must(err)
		s = cached.Scan
		fmt.Printf("load=%s entries=%d\n", time.Since(t), s.Entries)
	} else {
		s, err = scanner.Scan(context.Background(), *root, scanner.Options{Workers: *workers, Exclude: func() []string {
			if *exclude == "" {
				return nil
			}
			return strings.Split(*exclude, ",")
		}()})
		must(err)
		fmt.Printf("walk_and_aggregate=%s walk=%s aggregate=%s entries=%d\n", time.Since(t), s.WalkDuration, s.AggregateDuration, s.Entries)

	}
	t = time.Now()
	snap := report.New(s, e, *min)
	fmt.Printf("classify=%s suggestions=%d bytes=%d\n", time.Since(t), len(snap.Suggestions), s.Root.Bytes)
	if s.Tree != nil {
		count := int64(0)
		logical := int64(0)
		for id := int32(0); id < s.Tree.Count; id++ {
			n := s.Tree.At(id)
			if n.Flags&scanner.Dataless != 0 {
				count++
				logical += n.OwnBytes
			}
		}
		fmt.Printf("dataless_entries=%d logical_placeholder_bytes=%d\n", count, logical)
	}

	if *cache != "" {
		t = time.Now()
		must(report.SaveCache(*cache, snap))
		fmt.Printf("cache=%s\n", time.Since(t))
	}
	if *export {
		t = time.Now()
		must(report.StreamSnapshot(io.Discard, snap, -1, 0))
		fmt.Printf("export=%s\n", time.Since(t))
	}
	if *heap != "" {
		runtime.GC()
		f, err := os.Create(*heap)
		must(err)
		must(pprof.WriteHeapProfile(f))
		must(f.Close())
		runtime.KeepAlive(snap)
	}
	json.NewEncoder(os.Stdout).Encode(report.Summary(snap, 10))
}
func must(e error) {
	if e != nil {
		panic(e)
	}
}
