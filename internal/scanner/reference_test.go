package scanner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"syscall"
	"time"
)

func scanLegacy(ctx context.Context, root string, opts Options) (*Result, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("scan root must be a real directory (symlinks are not followed): %s", root)
	}
	n := fromInfo(root, info)
	result := &Result{Root: n, Started: time.Now(), CrossMounts: opts.CrossMounts, Skipped: []Skipped{}}
	excludes := map[string]bool{}
	for _, p := range opts.Exclude {
		p, e := filepath.Abs(p)
		if e != nil {
			return nil, e
		}
		excludes[filepath.Clean(p)] = true
	}
	if excludes[root] {
		return nil, fmt.Errorf("scan root is excluded: %s", root)
	}

	allowedDevices, skipData := devicePolicy(runtime.GOOS, root, n.id.dev, func() (uint64, error) {
		data, e := os.Lstat("/System/Volumes/Data")
		if e != nil {
			return 0, e
		}
		_, id, _ := diskInfo(data)
		return id.dev, nil
	})
	if skipData {
		excludes["/System/Volumes/Data"] = true
	}

	workers := opts.Workers
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	if workers > 64 {
		workers = 64
	}
	jobs := make(chan *Node)
	results := make(chan batch, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for dir := range jobs {
				b := batch{parent: dir}
				// O_NOFOLLOW rejects a directory replaced with a symlink since Lstat.
				var entries []os.DirEntry
				f, e := os.OpenFile(dir.Path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_DIRECTORY, 0)
				if e == nil {
					fi, se := f.Stat()
					if se != nil {
						e = se
					} else {
						_, id, _ := diskInfo(fi)
						if id != dir.id {
							e = fmt.Errorf("directory changed during scan")
						} else {
							entries, e = f.ReadDir(-1)
						}
					}
					f.Close()
				}
				if e != nil {
					b.err = e
				}
				for _, entry := range entries {
					if ctx.Err() != nil {
						break
					}
					p := filepath.Join(dir.Path, entry.Name())
					if excludes[p] {
						b.skipped = append(b.skipped, Skipped{p, "excluded application state/output or macOS Data firmlink alias"})
						continue
					}
					fi, e := os.Lstat(p)
					if e != nil {
						b.skipped = append(b.skipped, Skipped{p, e.Error()})
						continue
					}
					child := fromInfo(p, fi)
					if !opts.CrossMounts && !allowedDevices[child.id.dev] {
						b.skipped = append(b.skipped, Skipped{p, "different filesystem device"})
						continue
					}
					b.children = append(b.children, child)
				}
				results <- b
			}
		}()
	}
	queue := []*Node{n}
	active := 0
	dirs := int64(0)
	seenDirs := map[identity]bool{n.id: true}
	// A coordinator owns the unbounded directory queue. Workers never enqueue into
	// their own bounded work channel (which can deadlock on broad trees).
	for len(queue) > 0 || active > 0 {
		if ctx.Err() != nil {
			close(jobs)
			for active > 0 {
				<-results
				active--
			}
			wg.Wait()
			return nil, ctx.Err()
		}
		var send chan *Node
		var next *Node
		if len(queue) > 0 {
			send = jobs
			next = queue[0]
		}
		select {
		case send <- next:
			queue = queue[1:]
			active++
		case b := <-results:
			active--
			dirs++
			b.parent.Children = b.children
			if b.err != nil {
				b.parent.Incomplete = true
				result.Skipped = append(result.Skipped, Skipped{b.parent.Path, b.err.Error()})
			}
			if len(b.skipped) > 0 {
				b.parent.Incomplete = true
			}
			result.Skipped = append(result.Skipped, b.skipped...)
			result.Entries += int64(len(b.children))
			for _, c := range b.children {
				if c.Kind == "directory" {
					if seenDirs[c.id] {
						c.Incomplete = true
						c.OwnBytes = 0
						result.Skipped = append(result.Skipped, Skipped{c.Path, "directory inode already visited (mount/firmlink alias)"})
					} else {
						seenDirs[c.id] = true
						queue = append(queue, c)
					}
				}
			}
			if opts.Progress != nil {
				opts.Progress(Progress{Entries: result.Entries, Directories: dirs, Pending: len(queue) + active})
			}
		case <-ctx.Done():
		}
	}
	close(jobs)
	wg.Wait()
	aggregate(n, map[identity]bool{})
	sort.Slice(result.Skipped, func(i, j int) bool { return result.Skipped[i].Path < result.Skipped[j].Path })
	result.Finished = time.Now()
	return result, nil
}
