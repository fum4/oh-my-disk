package report

import (
	"container/heap"
	"github.com/fum4/oh-my-disk/internal/scanner"
	"sort"
)

type topItem struct {
	id    int32
	bytes int64
	path  string
}
type topHeap []topItem

func (h topHeap) Len() int { return len(h) }
func (h topHeap) Less(i, j int) bool {
	if h[i].bytes == h[j].bytes {
		return h[i].path > h[j].path
	}
	return h[i].bytes < h[j].bytes
}
func (h topHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *topHeap) Push(v any)   { *h = append(*h, v.(topItem)) }
func (h *topHeap) Pop() any     { a := *h; v := a[len(a)-1]; *h = a[:len(a)-1]; return v }
func topCompact(t *scanner.Tree, limit int) []*scanner.Node {
	h := topHeap{}
	if limit < 0 {
		limit = int(t.Count)
	}
	if limit == 0 {
		return []*scanner.Node{}
	}
	for id := int32(1); id < t.Count; id++ {
		size := t.Bytes(id)
		if size <= 0 || len(h) == limit && size < h[0].bytes {
			continue
		}
		v := topItem{id, size, t.Path(id)}
		if len(h) < limit {
			heap.Push(&h, v)
		} else if v.bytes > h[0].bytes || v.path < h[0].path {
			h[0] = v
			heap.Fix(&h, 0)
		}
	}
	sort.Slice(h, func(i, j int) bool {
		if h[i].bytes == h[j].bytes {
			return h[i].path < h[j].path
		}
		return h[i].bytes > h[j].bytes
	})
	out := make([]*scanner.Node, 0, len(h))
	for _, v := range h {
		out = append(out, t.View(v.id, 0, 0))
	}
	return out
}
