package report

import (
	"bufio"
	"encoding/json"
	"github.com/fum4/oh-my-disk/internal/scanner"
	"io"
)

func encodeOutput(w io.Writer, enc *json.Encoder, v any) error {
	switch x := v.(type) {
	case ExportView:
		return StreamSnapshot(w, x.Snapshot, x.Depth, x.Min)
	case *Snapshot:
		return StreamSnapshot(w, x, -1, 0)
	case []*scanner.Node:
		bw := bufio.NewWriterSize(w, 256<<10)
		if x == nil {
			_, e := io.WriteString(bw, "null\n")
			if e != nil {
				return e
			}
			return bw.Flush()
		}
		if e := bw.WriteByte('['); e != nil {
			return e
		}
		for i, n := range x {
			if i > 0 {
				if e := bw.WriteByte(','); e != nil {
					return e
				}
			}
			t, id := n.CompactTree(), n.ID
			if t == nil {
				t = scanner.Compact(n)
				id = 0
			}
			if e := streamTree(bw, t, id, -1, 0); e != nil {
				return e
			}
		}
		if _, e := io.WriteString(bw, "]\n"); e != nil {
			return e
		}
		return bw.Flush()
	}
	return enc.Encode(v)
}

// StreamSnapshot writes bounded node buffers, never a whole-tree JSON buffer or
// a pruned copy. Full exports use O(depth) traversal memory, even on deep trees.
func StreamSnapshot(w io.Writer, s *Snapshot, depth int, min int64) error {
	bw := bufio.NewWriterSize(w, 256<<10)
	write := func(v any) error {
		b, e := json.Marshal(v)
		if e != nil {
			return e
		}
		_, e = bw.Write(b)
		return e
	}
	if _, e := io.WriteString(bw, `{"version":1,"scan":{"root":`); e != nil {
		return e
	}
	t := s.Scan.Tree
	if t == nil {
		t = scanner.Compact(s.Scan.Root)
	}
	if e := streamTree(bw, t, 0, depth, min); e != nil {
		return e
	}
	// Marshal the small scan metadata separately from the tree.
	scan := *s.Scan
	scan.Root = nil
	scan.Tree = nil
	var e error
	if _, e = io.WriteString(bw, `,"started":`); e != nil {
		return e
	}
	if e = write(scan.Started); e != nil {
		return e
	}
	if _, e = io.WriteString(bw, `,"finished":`); e != nil {
		return e
	}
	if e = write(scan.Finished); e != nil {
		return e
	}
	if _, e = io.WriteString(bw, `,"skipped":`); e != nil {
		return e
	}
	if e = write(scan.Skipped); e != nil {
		return e
	}
	if _, e = io.WriteString(bw, `,"entries":`); e != nil {
		return e
	}
	if e = write(scan.Entries); e != nil {
		return e
	}
	if _, e = io.WriteString(bw, `,"cross_mounts":`); e != nil {
		return e
	}
	if e = write(scan.CrossMounts); e != nil {
		return e
	}
	if _, e = io.WriteString(bw, `},"suggestions":[`); e != nil {
		return e
	}
	for i, v := range s.Suggestions {
		if i > 0 {
			if e = bw.WriteByte(','); e != nil {
				return e
			}
		}
		if e = write(v); e != nil {
			return e
		}
	}
	if _, e = io.WriteString(bw, `],"categories":`); e != nil {
		return e
	}
	if e = write(s.Categories); e != nil {
		return e
	}
	if _, e = io.WriteString(bw, `,"note":`); e != nil {
		return e
	}
	if e = write(s.Note); e != nil {
		return e
	}
	if _, e = io.WriteString(bw, "}\n"); e != nil {
		return e
	}
	return bw.Flush()
}

func streamTree(bw *bufio.Writer, t *scanner.Tree, root int32, depth int, min int64) error {
	type frame struct {
		id, next int32
		depth    int
		opened   bool
		first    bool
	}
	stack := []frame{{id: root, depth: depth, first: true}}
	for len(stack) > 0 {
		f := &stack[len(stack)-1]
		if !f.opened {
			n := t.View(f.id, 0, 0)
			b, e := json.Marshal(n)
			if e != nil {
				return e
			}
			if _, e = bw.Write(b[:len(b)-1]); e != nil {
				return e
			}
			f.opened = true
			f.next = t.At(f.id).First
			if f.depth == 0 {
				f.next = -1
			}
		}
		for f.next >= 0 && t.Bytes(f.next) < min {
			f.next = t.At(f.next).Next
		}
		if f.next < 0 {
			if !f.first {
				if _, e := io.WriteString(bw, "]"); e != nil {
					return e
				}
			}
			if _, e := io.WriteString(bw, "}"); e != nil {
				return e
			}
			stack = stack[:len(stack)-1]
			continue
		}
		if f.first {
			if _, e := io.WriteString(bw, `,"children":[`); e != nil {
				return e
			}
			f.first = false
		} else {
			if e := bw.WriteByte(','); e != nil {
				return e
			}
		}
		id := f.next
		f.next = t.At(id).Next
		d := f.depth
		if d > 0 {
			d--
		}
		stack = append(stack, frame{id: id, depth: d, first: true})
	}

	return nil
}
