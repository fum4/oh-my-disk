package scanner

import (
	"context"
	"golang.org/x/sys/unix"
	"os"
	"time"
)

func readBulk(_ context.Context, _ *os.File, _ *dirBatch, _ []byte) bool { return false }
func statMetadata(name string, s *unix.Stat_t) metadata {
	m := statBase(name, s.Mode, uint64(s.Dev), s.Ino, uint64(s.Nlink), s.Blocks)
	m.entry.SetTime(0, time.Unix(s.Mtim.Sec, s.Mtim.Nsec))
	m.entry.SetTime(1, time.Unix(s.Atim.Sec, s.Atim.Nsec))
	return m
}
