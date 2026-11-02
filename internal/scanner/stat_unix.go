//go:build darwin || linux

package scanner

import (
	"os"
	"syscall"
)

type identity struct{ dev, ino uint64 }

func diskInfo(info os.FileInfo) (bytes int64, id identity, links uint64) {
	s := info.Sys().(*syscall.Stat_t)
	bytes = s.Blocks * 512
	if isDataless(info) {
		bytes = info.Size()
	}
	return bytes, identity{uint64(s.Dev), uint64(s.Ino)}, uint64(s.Nlink)
}
