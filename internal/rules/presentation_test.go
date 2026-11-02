package rules

import (
	"encoding/json"
	"math"
	"testing"
)

func TestCleanupScoreAndOrdering(t *testing.T) {
	if CleanupScore(0) != 0 || CleanupScore(1e9) != 50 || math.Round(CleanupScore(9e9)) != 90 {
		t.Fatal("fixed score scale")
	}
	in := []Suggestion{{Path: "a", Bytes: 10, StaleDays: 3, Score: 100, Safety: "cache", Category: "caches"}, {Path: "b", Bytes: 20, StaleDays: 1, Score: 50, Safety: "unknown", Category: "unknown"}}
	for key, want := range map[string]string{"score": "a", "size": "b", "stale": "a"} {
		out, e := Select(in, key, "", "", 1)
		if e != nil || len(out) != 1 || out[0].Path != want {
			t.Fatal(key, out, e)
		}
	}
	out, e := Select(in, "score", "caches", "cache", 10)
	if e != nil || len(out) != 1 {
		t.Fatal(out, e)
	}
	data, e := json.Marshal(in[0])
	if e != nil {
		t.Fatal(e)
	}
	var decoded map[string]any
	json.Unmarshal(data, &decoded)
	if decoded["raw_score"] != float64(100) || decoded["score"].(float64) > 100 || decoded["badge"] == nil {
		t.Fatal(string(data))
	}
	var round Suggestion
	if e = json.Unmarshal(data, &round); e != nil || round.Score != in[0].Score {
		t.Fatal("raw score lost", e, round)
	}
}
