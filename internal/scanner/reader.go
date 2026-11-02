//go:build darwin || linux

package scanner

import (
	"context"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"sort"
)

type metadata struct {
	name    string
	entry   Entry
	id      identity
	links   uint64
	refresh bool
}
type dirJob struct {
	id       int32
	path     string
	identity identity
}
type dirBatch struct {
	job     dirJob
	entries []metadata
	skipped []Skipped
	err     error
}

func readDirectory(ctx context.Context, j dirJob, fallback bool, buf []byte, rootfd int, rootPath string) dirBatch {
	b := dirBatch{job: j}
	// No file contents are opened. O_NOFOLLOW and identity validation protect the
	// queued directory against replacement. Child metadata is relative to this fd.
	rel, err := filepath.Rel(rootPath, j.path)
	if err != nil {
		b.err = err
		return b
	}
	fd, err := openDirectoryAt(rootfd, rel)
	if err != nil {
		b.err = err
		return b
	}
	f := os.NewFile(uintptr(fd), j.path)
	defer func() { f.Close() }()
	var st unix.Stat_t
	if err = unix.Fstat(int(f.Fd()), &st); err != nil {
		b.err = err
		return b
	}
	if (identity{uint64(st.Dev), uint64(st.Ino)}) != j.identity {
		b.err = unix.ESTALE
		return b
	}
	if !fallback {
		ok := readBulk(ctx, f, &b, buf)
		if ok {
			sort.Slice(b.entries, func(i, j int) bool { return b.entries[i].name < b.entries[j].name })
			return b
		}
		// Do not mix bulk and readdir on one open description (Darwin ABI).
		f.Close()
		fd, err = openDirectoryAt(rootfd, rel)
		if err != nil {
			b.err = err
			return b
		}
		f = os.NewFile(uintptr(fd), j.path)
		if err = unix.Fstat(fd, &st); err != nil {
			b.err = err
			return b
		}
		if (identity{uint64(st.Dev), uint64(st.Ino)}) != j.identity {
			b.err = unix.ESTALE
			return b
		}
		b.entries = nil
		b.skipped = nil
		b.err = nil
	}
	entries, err := f.ReadDir(-1)
	b.err = err
	b.entries = make([]metadata, 0, len(entries))
	for _, de := range entries {
		if ctx.Err() != nil {
			break
		}
		var st unix.Stat_t
		if err := unix.Fstatat(int(f.Fd()), de.Name(), &st, unix.AT_SYMLINK_NOFOLLOW); err != nil {
			b.skipped = append(b.skipped, Skipped{j.path + "/" + de.Name(), err.Error()})
			continue
		}
		b.entries = append(b.entries, statMetadata(de.Name(), &st))
	}
	sort.Slice(b.entries, func(i, j int) bool { return b.entries[i].name < b.entries[j].name })
	return b
}
func statBase(name string, mode uint32, dev, ino, links uint64, blocks int64) metadata {
	e := Entry{OwnBytes: blocks * 512, First: -1, Next: -1}
	switch mode & unix.S_IFMT {
	case unix.S_IFDIR:
		e.Kind = Directory
	case unix.S_IFLNK:
		e.Kind = Symlink
	case unix.S_IFREG:
		e.Kind = File
	default:
		e.Kind = Special
	}
	return metadata{name: name, entry: e, id: identity{dev, ino}, links: links}
}
