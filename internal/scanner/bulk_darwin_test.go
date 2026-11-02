package scanner

import (
	"context"
	"encoding/binary"
	"os"
	"testing"
)

func TestBulkUsed(t *testing.T) {
	root := t.TempDir()
	write(t, root+"/file", 8192)
	os.Mkdir(root+"/dir", 0700)
	os.Symlink("file", root+"/link")
	f, e := os.Open(root)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	b := dirBatch{}
	if !readBulk(context.Background(), f, &b, make([]byte, 256<<10)) {
		t.Fatalf("bulk reader fell back: %v", b.err)
	}
	if b.err != nil || len(b.entries) != 3 {
		t.Fatalf("bulk=%+v", b)
	}
	for _, m := range b.entries {
		fi, e := os.Lstat(root + "/" + m.name)
		if e != nil {
			t.Fatal(e)
		}
		n := fromInfo(root+"/"+m.name, fi)
		if m.entry.OwnBytes != n.OwnBytes || kinds[m.entry.Kind] != n.Kind || !m.entry.Time(0).Equal(n.Modified) || !m.entry.Time(1).Equal(n.Accessed) || m.id != n.id || (n.Kind != "directory" && m.links != n.links) {
			t.Fatalf("%s bulk=%+v lstat=%+v", m.name, m, n)
		}
	}
}

func TestBulkDatalessAndCompressedMetadata(t *testing.T) {
	r := make([]byte, 112)
	put := func(at int, v uint32) { binary.LittleEndian.PutUint32(r[at:], v) }
	put(0, 112)
	put(4, commonAttrs)
	put(16, fileAttrs)
	put(24, 80)
	put(28, 5)
	put(32, 1)
	put(36, 1)
	put(72, sfDataless|ufCompressed)
	put(84, 1)
	binary.LittleEndian.PutUint64(r[96:], 123456)
	binary.LittleEndian.PutUint64(r[88:], 4096)
	copy(r[104:], "file\x00")
	m, e := parseBulk(r)
	if e != nil {
		t.Fatal(e)
	}
	if m.entry.OwnBytes != 123456 || m.entry.Flags&(Dataless|Compressed) != (Dataless|Compressed) {
		t.Fatal("cloud logical size/flags lost")
	}
	for n := 0; n < 104; n++ {
		if _, e = parseBulk(r[:n]); e == nil {
			t.Fatalf("accepted truncated record %d", n)
		}
	}
	put(24, 0xffffffff)
	if _, e = parseBulk(r); e == nil {
		t.Fatal("accepted invalid name offset")
	}
}
