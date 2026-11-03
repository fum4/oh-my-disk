package memlimit

import (
	"runtime"
	"runtime/debug"
	"testing"
)

var keep [][]byte

func TestApplyAddsLiveHeapAndHonorsEnv(t *testing.T) {
	defer debug.SetMemoryLimit(debug.SetMemoryLimit(-1))
	t.Setenv("GOMEMLIMIT", "")
	keep = append(keep, make([]byte, 64<<20))
	runtime.GC()
	if got := Apply(); got < Base+64<<20 || debug.SetMemoryLimit(-1) != got {
		t.Fatalf("limit %d does not cover Base plus the retained heap", got)
	}
	keep = nil
	debug.SetMemoryLimit(123 << 20)
	t.Setenv("GOMEMLIMIT", "123MiB")
	if got := Apply(); got != 123<<20 {
		t.Fatalf("GOMEMLIMIT must win, got %d", got)
	}
}
