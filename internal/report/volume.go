package report

import "golang.org/x/sys/unix"

// Volume is a live filesystem reading; Outside also includes inaccessible data,
// snapshots and filesystem overhead. Excess records clone/placeholder overcounts.
type Volume struct {
	Capacity  int64 `json:"capacity"`
	Used      int64 `json:"used"`
	Available int64 `json:"available"`
	Outside   int64 `json:"outside"`
	Excess    int64 `json:"excess"`
}

func VolumeMath(capacity, available, scanned int64) Volume {
	capacity = max(0, capacity)
	available = min(capacity, max(0, available))
	used := capacity - available
	return Volume{capacity, used, available, max(0, used-max(0, scanned)), max(0, scanned-used)}
}

func VolumeFor(path string, scanned int64) (Volume, error) {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return Volume{}, err
	}
	return VolumeMath(int64(stat.Blocks)*int64(stat.Bsize), int64(stat.Bavail)*int64(stat.Bsize), scanned), nil
}
