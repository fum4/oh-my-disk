package report

import "testing"

func TestUnits(t *testing.T) {
	for _, c := range []struct {
		b      int64
		binary bool
		want   string
	}{
		{0, false, "0 B"}, {999, false, "999 B"}, {1000, false, "1.0 kB"}, {1500000000, false, "1.5 GB"},
		{1023, true, "1023 B"}, {1024, true, "1.0 KiB"}, {1 << 30, true, "1.0 GiB"}, {1 << 60, true, "1.0 EiB"},
	} {
		if got := FormatBytes(c.b, c.binary); got != c.want {
			t.Errorf("%d: %s != %s", c.b, got, c.want)
		}
	}
}
func TestVolumeMath(t *testing.T) {
	for _, c := range []struct{ cap, avail, scanned, outside, excess int64 }{
		{1000, 400, 200, 400, 0}, {1000, 400, 600, 0, 0}, {1000, 400, 800, 0, 200},
		{1000, 1200, 0, 0, 0}, {1000, -5, 200, 800, 0}, {0, 0, 0, 0, 0},
	} {
		v := VolumeMath(c.cap, c.avail, c.scanned)
		if v.Outside != c.outside || v.Excess != c.excess || v.Used+v.Available != v.Capacity {
			t.Fatalf("%+v: %+v", c, v)
		}
	}
}
func TestVolumeStat(t *testing.T) {
	v, err := VolumeFor(t.TempDir(), 0)
	if err != nil || v.Capacity <= 0 || v.Used+v.Available != v.Capacity {
		t.Fatalf("%+v %v", v, err)
	}
}
