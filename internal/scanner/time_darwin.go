package scanner

import (
	"os"
	"syscall"
	"time"
)

func accessTime(info os.FileInfo) time.Time {
	s := info.Sys().(*syscall.Stat_t)
	return time.Unix(s.Atimespec.Sec, s.Atimespec.Nsec)
}
