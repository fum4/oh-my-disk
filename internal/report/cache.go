package report

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/fum4/oh-my-disk/internal/scanner"
	"hash/crc32"
	"io"
	"os"
	"strings"
)

// Version 6 stores local allocation for dataless files; older totals need a rescan.
const cacheMagic = "OMDCACH6"
const nodeChunk = 1 << 14

type cacheHeader struct {
	Snapshot   Snapshot
	RootPath   string
	Categories []string
	Paths      []string
}

func writeCache(w io.Writer, s *Snapshot) error {
	sum := crc32.NewIEEE()
	bw := bufio.NewWriterSize(io.MultiWriter(w, sum), 256<<10)
	if _, err := io.WriteString(bw, cacheMagic); err != nil {
		return err
	}
	t := s.Scan.Tree
	if t == nil {
		t = scanner.Compact(s.Scan.Root)
	}
	scan := *s.Scan
	scan.Root = nil
	scan.Tree = nil
	meta := *s
	meta.Scan = &scan
	h, err := json.Marshal(cacheHeader{meta, t.RootPath, t.Categories, t.Paths})
	if err != nil {
		return err
	}
	write := func(v any) error { return binary.Write(bw, binary.LittleEndian, v) }
	if err = write(uint32(len(h))); err != nil {
		return err
	}
	if _, err = bw.Write(h); err != nil {
		return err
	}
	if err = write(uint32(len(t.Names))); err != nil {
		return err
	}
	for _, s := range t.Names {
		if err = write(uint32(len(s))); err != nil {
			return err
		}
		if _, err = io.WriteString(bw, s); err != nil {
			return err
		}
	}
	if err = write(t.Count); err != nil {
		return err
	}
	remaining := int(t.Count)
	for _, chunk := range t.Chunks {
		n := len(chunk)
		if n > remaining {
			n = remaining
		}
		if err = write(chunk[:n]); err != nil {
			return err
		}
		remaining -= n
	}
	if err = write(t.DataCount); err != nil {
		return err
	}
	remaining = int(t.DataCount)
	for _, chunk := range t.DataChunks {
		n := len(chunk)
		if n > remaining {
			n = remaining
		}
		if err = write(chunk[:n]); err != nil {
			return err
		}
		remaining -= n
	}
	if err := bw.Flush(); err != nil {
		return err
	}
	return binary.Write(w, binary.LittleEndian, sum.Sum32())
}
func readCache(f *os.File) (*Snapshot, error) {
	var magic [8]byte
	if _, err := f.ReadAt(magic[:], 0); err != nil {
		return nil, err
	}
	if string(magic[:]) != cacheMagic {
		if string(magic[:7]) == "OMDCACH" {
			return nil, fmt.Errorf("older cache format; rescan with --refresh")
		}
		var s Snapshot
		if err := json.NewDecoder(f).Decode(&s); err != nil {
			return nil, err
		}
		if err := validSnapshot(&s); err != nil {
			return nil, err
		}
		// Legacy JSON caches used logical bytes for cloud placeholders too.
		pending := []*scanner.Node{s.Scan.Root}
		for len(pending) > 0 {
			n := pending[len(pending)-1]
			pending = pending[:len(pending)-1]
			if n.Dataless {
				return nil, fmt.Errorf("older cache format with cloud sizes; rescan with --refresh")
			}
			pending = append(pending, n.Children...)
		}
		s.Scan.Tree = scanner.Compact(s.Scan.Root)
		s.Scan.Root = s.Scan.Tree.View(0, 1, 0)
		return &s, nil
	}
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if fi.Size() < 12 {
		return nil, fmt.Errorf("truncated binary cache")
	}
	size := fi.Size()
	sum := crc32.NewIEEE()
	r := bufio.NewReaderSize(io.TeeReader(io.LimitReader(f, size-4), sum), 256<<10)
	r.Discard(len(cacheMagic))
	read := func(v any) error { return binary.Read(r, binary.LittleEndian, v) }
	readString := func() (string, error) {
		var n uint32
		if err := read(&n); err != nil {
			return "", err
		}
		if int64(n) > size {
			return "", fmt.Errorf("invalid cache string length")
		}
		b := make([]byte, n)
		_, err := io.ReadFull(r, b)
		return string(b), err
	}
	hdata, err := readString()
	if err != nil {
		return nil, err
	}
	var h cacheHeader
	if err = json.Unmarshal([]byte(hdata), &h); err != nil {
		return nil, err
	}
	var count uint32
	if err = read(&count); err != nil {
		return nil, err
	}
	if int64(count) > size/4 {
		return nil, fmt.Errorf("invalid name count")
	}
	t := &scanner.Tree{RootPath: h.RootPath, Categories: h.Categories, Paths: h.Paths, Names: make([]string, count)}
	for i := range t.Names {
		t.Names[i], err = readString()
		if err != nil {
			return nil, err
		}
	}
	if err = read(&t.Count); err != nil {
		return nil, err
	}
	if t.Count <= 0 || int64(t.Count) > size/int64(binary.Size(scanner.Entry{})) {
		return nil, fmt.Errorf("invalid cache node count")
	}
	remaining := int(t.Count)
	for remaining > 0 {
		n := nodeChunk
		if n > remaining {
			n = remaining
		}
		chunk := make([]scanner.Entry, n)
		if err = read(chunk); err != nil {
			return nil, err
		}
		t.Chunks = append(t.Chunks, chunk)
		remaining -= n
	}
	if err = read(&t.DataCount); err != nil {
		return nil, err
	}
	if t.DataCount < 0 || int64(t.DataCount) > size/int64(binary.Size(scanner.DirectoryData{})) {
		return nil, fmt.Errorf("invalid aggregate count")
	}
	remaining = int(t.DataCount)
	for remaining > 0 {
		n := nodeChunk
		if n > remaining {
			n = remaining
		}
		chunk := make([]scanner.DirectoryData, n)
		if err = read(chunk); err != nil {
			return nil, err
		}
		t.DataChunks = append(t.DataChunks, chunk)
		remaining -= n
	}
	if _, err = r.ReadByte(); err != io.EOF {
		return nil, fmt.Errorf("trailing cache data")
	}
	var checksum uint32
	if err = binary.Read(f, binary.LittleEndian, &checksum); err != nil {
		return nil, err
	}
	if checksum != sum.Sum32() {
		return nil, fmt.Errorf("cache checksum mismatch")
	}
	if err = t.Validate(); err != nil {
		return nil, err
	}
	s := &h.Snapshot
	if s.Scan == nil {
		return nil, fmt.Errorf("missing scan")
	}
	s.Scan.Tree = t
	s.Scan.Root = t.View(0, 1, 0)
	if err = validSnapshot(s); err != nil {
		return nil, err
	}
	return s, nil
}
func validSnapshot(s *Snapshot) error {
	if s.Version != 1 || s.Scan == nil || s.Scan.Root == nil {
		return fmt.Errorf("invalid or unsupported scan cache")
	}
	return nil
}

// Validate existing binary caches in bounded memory. Loading a second multi-
// million-entry tree merely to replace application state doubles peak memory.
func validateExisting(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	var magic [8]byte
	if _, err = f.ReadAt(magic[:], 0); err != nil {
		return err
	}
	// A structurally valid legacy JSON cache may be replaced after a fresh
	// scan even when its old cloud allocation makes it unsuitable for loading.
	if !strings.HasPrefix(string(magic[:]), "OMDCACH") {
		var s Snapshot
		if err := json.NewDecoder(f).Decode(&s); err != nil {
			return err
		}
		return validSnapshot(&s)
	}
	if string(magic[:]) != cacheMagic && string(magic[:]) != "OMDCACH4" && string(magic[:]) != "OMDCACH5" {
		_, err = LoadCache(path)
		return err
	}
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	if fi.Size() < 12 {
		return fmt.Errorf("truncated cache")
	}
	sum := crc32.NewIEEE()
	r := io.TeeReader(io.LimitReader(f, fi.Size()-4), sum)
	var prefix [12]byte
	if _, err = io.ReadFull(r, prefix[:]); err != nil {
		return err
	}
	n := binary.LittleEndian.Uint32(prefix[8:])
	if int64(n) > fi.Size()-16 {
		return fmt.Errorf("invalid cache header")
	}
	data := make([]byte, n)
	if _, err = io.ReadFull(r, data); err != nil {
		return err
	}
	var h cacheHeader
	if err = json.Unmarshal(data, &h); err != nil {
		return err
	}
	if h.Snapshot.Version != 1 || h.Snapshot.Scan == nil || h.RootPath == "" {
		return fmt.Errorf("invalid cache header")
	}
	if _, err = io.Copy(io.Discard, r); err != nil {
		return err
	}
	var checksum uint32
	if err = binary.Read(f, binary.LittleEndian, &checksum); err != nil {
		return err
	}
	if checksum != sum.Sum32() {
		return fmt.Errorf("cache checksum mismatch")
	}
	return nil
}
