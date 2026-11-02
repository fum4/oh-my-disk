package scanner

import (
	"context"
	"fmt"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

func Scan(ctx context.Context, root string, opts Options) (*Result, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	fi, err := os.Lstat(root)
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("scan root must be a real directory (symlinks are not followed): %s", root)
	}
	rn := fromInfo(root, fi)
	t := &Tree{RootPath: root, Names: []string{rn.Name}}
	re := entryFromNode(rn, -1)
	re.NameLen = uint32(len(rn.Name))
	t.Add(re)
	res := &Result{Tree: t, Started: time.Now(), CrossMounts: opts.CrossMounts, Skipped: []Skipped{}}
	excludes := map[string]bool{}
	excludeNames := map[string]bool{}
	for _, p := range opts.Exclude {
		p, err = filepath.Abs(p)
		if err != nil {
			return nil, err
		}
		excludes[p] = true
		excludeNames[filepath.Base(p)] = true
	}
	if excludes[root] {
		return nil, fmt.Errorf("scan root is excluded: %s", root)
	}
	if rn.Dataless {
		t.At(0).Flags |= Incomplete
		t.Aggregate()
		res.Skipped = append(res.Skipped, Skipped{root, "dataless directory: not enumerated to avoid cloud downloads"})
		res.Root = t.View(0, 1, 0)
		res.Finished = time.Now()
		return res, nil
	}
	anchor, err := os.OpenFile(root, os.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	defer anchor.Close()
	afi, err := anchor.Stat()
	if err != nil {
		return nil, err
	}
	_, aid, _ := diskInfo(afi)
	if aid != rn.id {
		return nil, unix.ESTALE
	}

	devices, skipData := devicePolicy(runtime.GOOS, root, rn.id.dev, func() (uint64, error) {
		fi, e := os.Lstat("/System/Volumes/Data")
		if e != nil {
			return 0, e
		}
		_, id, _ := diskInfo(fi)
		return id.dev, nil
	})
	if skipData {
		excludes["/System/Volumes/Data"] = true
		excludeNames["Data"] = true
	}
	workers := opts.Workers
	if workers <= 0 {
		workers = 2 * runtime.NumCPU()
	}
	if workers > 64 {
		workers = 64
	}
	jobs := make(chan dirJob)
	results := make(chan dirBatch, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var buf []byte
			if runtime.GOOS == "darwin" && !opts.Fallback {
				buf = make([]byte, 64<<10)
			}
			for job := range jobs {
				results <- readDirectory(ctx, job, opts.Fallback, buf, int(anchor.Fd()), root)
			}
		}()
	}
	queue := []dirJob{{0, root, rn.id}}
	head, active := 0, 0
	dirs := int64(0)
	seen := map[identity]bool{rn.id: true}
	hardlinks := map[identity]int32{}
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	sample := root
	progress := func(phase string) {
		if opts.Progress != nil {
			opts.Progress(Progress{Entries: res.Entries, Directories: dirs, Pending: len(queue) - head + active, Phase: phase, Path: sample})
		}
	}
	for head < len(queue) || active > 0 {
		if ctx.Err() != nil {
			close(jobs)
			for active > 0 {
				<-results
				active--
			}
			wg.Wait()
			return nil, ctx.Err()
		}
		var send chan dirJob
		var next dirJob
		if head < len(queue) {
			send = jobs
			next = queue[head]
		}
		select {
		case send <- next:
			queue[head] = dirJob{}
			head++
			active++
		case b := <-results:
			active--
			dirs++
			sample = b.job.path
			parent := t.At(b.job.id)
			if b.err != nil {
				parent.Flags |= Incomplete
				res.Skipped = append(res.Skipped, Skipped{b.job.path, b.err.Error()})
			}
			if len(b.skipped) > 0 {
				parent.Flags |= Incomplete
			}
			res.Skipped = append(res.Skipped, b.skipped...)
			var names strings.Builder
			totalNames := 0
			for _, m := range b.entries {
				totalNames += len(m.name)
			}
			names.Grow(totalNames)
			for _, m := range b.entries {
				names.WriteString(m.name)
			}
			block := uint32(len(t.Names))
			t.Names = append(t.Names, names.String())
			offset := uint32(0)
			prev := int32(-1)
			for _, m := range b.entries {
				e := m.entry
				e.Parent = b.job.id
				e.NameBlock = block
				e.NameStart = offset
				e.NameLen = uint32(len(m.name))
				offset += e.NameLen
				p := ""
				if excludeNames[m.name] || e.Kind == Directory {
					p = filepath.Join(b.job.path, m.name)
				}
				reason := ""
				if excludes[p] {
					reason = "excluded application state/output or macOS Data firmlink alias"
				} else if !opts.CrossMounts && !devices[m.id.dev] {
					reason = "different filesystem device"
				}
				if reason != "" {
					if p == "" {
						p = filepath.Join(b.job.path, m.name)
					}
					res.Skipped = append(res.Skipped, Skipped{p, reason})
					parent.Flags |= Incomplete
					continue
				}
				id := t.Add(e)
				res.Entries++
				if prev < 0 {
					parent.First = id
				} else {
					t.At(prev).Next = id
				}
				prev = id
				if e.Kind == Directory {
					if e.Flags&Dataless != 0 {
						t.At(id).Flags |= Incomplete
						res.Skipped = append(res.Skipped, Skipped{p, "dataless directory: not enumerated to avoid cloud downloads"})
					} else if seen[m.id] {
						t.At(id).Flags |= Incomplete
						t.At(id).OwnBytes = 0
						res.Skipped = append(res.Skipped, Skipped{p, "directory inode already visited (mount/firmlink alias)"})
					} else {
						seen[m.id] = true
						queue = append(queue, dirJob{id, p, m.id})
					}
				} else if m.links > 1 {
					if first, ok := hardlinks[m.id]; !ok {
						hardlinks[m.id] = id
					} else if t.LessPath(id, first) {
						t.At(first).OwnBytes = 0
						t.At(first).Flags |= Duplicate
						hardlinks[m.id] = id
					} else {
						t.At(id).OwnBytes = 0
						t.At(id).Flags |= Duplicate
					}
				}
			}
			// Reuse the coordinator's queue without retaining processed path strings.
			if head > 4096 && head*2 > len(queue) {
				copy(queue, queue[head:])
				queue = queue[:len(queue)-head]
				head = 0
			}
		case <-ticker.C:
			progress("walking")
		case <-ctx.Done():
		}
	}
	close(jobs)
	wg.Wait()
	res.WalkDuration = time.Since(res.Started)
	aggregateStart := time.Now()
	progress("aggregating")
	t.Aggregate()
	sort.Slice(res.Skipped, func(i, j int) bool { return res.Skipped[i].Path < res.Skipped[j].Path })
	res.AggregateDuration = time.Since(aggregateStart)
	res.Finished = time.Now()
	res.Root = t.View(0, 1, 0)
	progress("aggregating")
	return res, nil
}
