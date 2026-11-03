package scanner

import (
	"context"
	"encoding/binary"
	"fmt"
	"golang.org/x/sys/unix"
	"os"
	"time"
	"unsafe"
)

// See Apple's getattrlistbulk(2)/attr.h. All fields are packed at 4-byte
// alignment, including uint64 and timespec. Read them without aligned casts.
const commonAttrs = 0x80000000 | 0x1 | 0x2 | 0x8 | 0x400 | 0x1000 | 0x40000 | 0x02000000
const fileAttrs = 0x1 | 0x4 | 0x200
const dirAttrs = 0x1 | 0x4 | 0x8
const sfDataless = 0x40000000
const ufCompressed = 0x20

type attrList struct {
	Count, Reserved                 uint16
	Common, Volume, Dir, File, Fork uint32
}

func statMetadata(name string, s *unix.Stat_t) metadata {
	m := statBase(name, uint32(s.Mode), uint64(s.Dev), s.Ino, uint64(s.Nlink), s.Blocks)
	m.entry.SetTime(0, time.Unix(s.Mtim.Sec, s.Mtim.Nsec))
	m.entry.SetTime(1, time.Unix(s.Atim.Sec, s.Atim.Nsec))
	if s.Flags&sfDataless != 0 {
		m.entry.Flags |= Dataless
	}
	if s.Flags&ufCompressed != 0 {
		m.entry.Flags |= Compressed
	}
	return m
}
func readBulk(ctx context.Context, f *os.File, b *dirBatch, buf []byte) bool {
	attrs := attrList{Count: 5, Common: commonAttrs, Dir: dirAttrs, File: fileAttrs}
	for ctx.Err() == nil {
		count, _, errno := unix.Syscall6(unix.SYS_GETATTRLISTBULK, f.Fd(), uintptr(unsafe.Pointer(&attrs)), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), unix.FSOPT_PACK_INVAL_ATTRS, 0)
		if errno != 0 {
			if errno == unix.ENOTSUP || errno == unix.EINVAL || errno == unix.ENOSYS {
				return false
			}
			b.err = errno
			return true
		}
		if count == 0 {
			return true
		}
		off := 0
		for i := uintptr(0); i < count; i++ {
			if off+4 > len(buf) {
				b.err = fmt.Errorf("truncated bulk record")
				return true
			}
			size := int(binary.LittleEndian.Uint32(buf[off:]))
			if size < 24 || off+size > len(buf) {
				b.err = fmt.Errorf("invalid bulk record length")
				return true
			}
			m, err := parseBulk(buf[off : off+size])
			if err != nil {
				b.err = err
				return false
			}
			if m.refresh {
				var st unix.Stat_t
				if err := unix.Fstatat(int(f.Fd()), m.name, &st, unix.AT_SYMLINK_NOFOLLOW); err != nil {
					b.skipped = append(b.skipped, Skipped{b.job.path + "/" + m.name, err.Error()})
					off += size
					continue
				}
				m = statMetadata(m.name, &st)
			}
			b.entries = append(b.entries, m)
			off += size
		}
	}
	return true
}
func parseBulk(r []byte) (metadata, error) {
	// Returned attrs (20), name reference (8), dev (4), type (4),
	// mtime (16), atime (16), flags (4), fileid (8), then dir and file attrs.
	if len(r) < 96 {
		return metadata{}, fmt.Errorf("short bulk attributes")
	}
	u32 := func(p int) uint32 { return binary.LittleEndian.Uint32(r[p:]) }
	u64 := func(p int) uint64 { return binary.LittleEndian.Uint64(r[p:]) }
	common, dir, file := u32(4), u32(12), u32(16)
	if common&commonAttrs != commonAttrs {
		return metadata{}, fmt.Errorf("missing common attributes")
	}
	start := 24 + int(int32(u32(24)))
	length := int(u32(28))
	if length < 1 || start < 0 || start+length > len(r) || r[start+length-1] != 0 {
		return metadata{}, fmt.Errorf("invalid bulk name")
	}
	e := Entry{First: -1, Next: -1}
	m := metadata{name: string(r[start : start+length-1]), entry: e, id: identity{uint64(int64(int32(u32(32)))), u64(76)}}
	switch u32(36) {
	case 1:
		m.entry.Kind = File
	case 2:
		m.entry.Kind = Directory
	case 5:
		m.entry.Kind = Symlink
	default:
		m.entry.Kind = Special
	}
	m.entry.SetTime(0, time.Unix(int64(u64(40)), int64(u64(48))))
	m.entry.SetTime(1, time.Unix(int64(u64(56)), int64(u64(64))))
	// Darwin emits only the group appropriate to the vnode type.
	if m.entry.Kind == Directory {
		if dir&dirAttrs != dirAttrs {
			return metadata{}, fmt.Errorf("missing directory attributes")
		}
		if len(r) < 100 {
			return metadata{}, fmt.Errorf("short directory attributes")
		}
		m.links = uint64(u32(84))
		m.entry.OwnBytes = int64(u64(92))
		m.refresh = u32(88)&1 != 0 || u32(72)&0x00800000 != 0
	} else {
		if file&fileAttrs != fileAttrs {
			return metadata{}, fmt.Errorf("missing file attributes")
		}
		if len(r) < 104 {
			return metadata{}, fmt.Errorf("short file attributes")
		}
		m.links = uint64(u32(84))
		m.entry.OwnBytes = int64(u64(88))
	}
	if u32(72)&sfDataless != 0 {
		m.entry.Flags |= Dataless
		if m.entry.Kind == Directory {
			m.refresh = true
		}
	}
	if u32(72)&ufCompressed != 0 {
		m.entry.Flags |= Compressed
	}
	return m, nil
}
