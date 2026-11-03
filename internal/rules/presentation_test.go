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

func TestPersonalFilesExcludedFromBestButAvailableBySize(t *testing.T) {
	in := []Suggestion{
		{Path: "/h/Library/Mobile Documents/com~apple~CloudDocs/DEEP", Bytes: 79_100_000_000, Score: 6_328_000_000, StaleDays: 314, Safety: "unknown", Rule: "big-unknown"},
		{Path: "/h/Pictures", Bytes: 20_200_000_000, Score: 2_424_000_000, Safety: "user-data"},
		{Path: "/h/.npm/_cacache", Bytes: 8_100_000_000, Score: 8_100_000_000, Safety: "cache"},
	}
	best, _ := Select(in, "score", "", "", 100)
	if len(best) != 1 || best[0].Safety != "cache" {
		t.Fatalf("personal content in Best: %+v", best)
	}
	largest, _ := Select(in, "size", "", "", 100)
	if len(largest) != 3 || largest[0].Path != in[0].Path || CleanupScore(largest[0].Score) > 30 {
		t.Fatalf("Largest: %+v", largest)
	}
	data, _ := json.Marshal(in[0])
	var got map[string]any
	json.Unmarshal(data, &got)
	if got["score"].(float64) > 30 || got["personal"] != true {
		t.Fatalf("unsafe cached presentation: %s", data)
	}
}

func TestBestPutsCompleteSafeItemsBeforeHugeUnknown(t *testing.T) {
	in := []Suggestion{
		{Path: "/h/mystery", Safety: "unknown", Rule: "big-unknown", Bytes: 1e12, Score: 8e10},
		{Path: "/h/.cache/small", Safety: "cache", Bytes: 1e6, Score: 1e6},
		{Path: "/h/.cache/partial", Safety: "cache", Incomplete: true, Bytes: 1e12, Score: 1e12},
	}
	out, _ := Select(in, "score", "", "", 0)
	if out[0].Path != in[1].Path {
		t.Fatalf("safe candidate lost: %+v", out)
	}
	for _, s := range out[1:] {
		if CleanupScore(s.Score) > 40 {
			t.Fatal("review cap", s)
		}
	}
}
