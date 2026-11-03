// Package memlimit sets the Go soft memory limit for scans.
package memlimit

import (
	"os"
	"runtime/debug"
	"runtime/metrics"
)

// Base is the soft limit for one full home scan. The compact tree for a ~6M-entry
// home keeps about 640 MB live at the end of the walk; a 704 MiB limit sat right
// on top of that, so the collector ran almost continuously (1,700 cycles and ~5x
// user CPU per scan). 896 MiB keeps ~10-15 collections per scan.
const Base = 896 << 20

// Apply sets the limit to Base plus the heap that is already live (for example a
// displayed scan kept during a rescan), so the collector never chases a target
// below data that must stay in memory. An explicit GOMEMLIMIT always wins.
func Apply() int64 {
	if os.Getenv("GOMEMLIMIT") != "" {
		return debug.SetMemoryLimit(-1)
	}
	limit := Base + live()
	debug.SetMemoryLimit(limit)
	return limit
}

func live() int64 {
	s := []metrics.Sample{{Name: "/gc/heap/live:bytes"}}
	metrics.Read(s)
	if s[0].Value.Kind() == metrics.KindUint64 {
		return int64(s[0].Value.Uint64())
	}
	return 0
}
