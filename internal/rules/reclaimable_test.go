package rules

import (
	"encoding/json"
	"testing"
)

func TestTotalsCountOnlyCompleteRegenerableData(t *testing.T) {
	in := []Suggestion{
		{Path: "/home/u/.npm", Bytes: 10, Safety: "cache"},
		{Path: "/home/u/.cache/huggingface", Bytes: 20, Safety: "reinstallable"},
		{Path: "/home/u/Library/Containers/com.docker.docker/Data", Bytes: 400, Safety: "user-data"},
		{Path: "/home/u/mystery", Bytes: 800, Safety: "unknown"},
		{Path: "/home/u/.cargo", Bytes: 1600, Safety: "cache", Incomplete: true},
		{Path: "/home/u/Documents/.cache", Bytes: 3200, Safety: "cache"},
	}
	reclaimable, other := Totals(in)
	if reclaimable != 30 || other != 6000 {
		t.Fatalf("reclaimable %d, other %d", reclaimable, other)
	}
	var got struct {
		Reclaimable bool `json:"reclaimable"`
	}
	data, err := json.Marshal(in[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &got); err != nil || !got.Reclaimable {
		t.Fatalf("reclaimable missing from %s", data)
	}
}
