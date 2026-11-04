package rules

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
)

// CleanupScore is a fixed saturating scale: one weighted decimal GB scores 50.
// It does not depend on the scan's largest item or the active filters.
func CleanupScore(raw float64) float64 {
	if raw <= 0 {
		return 0
	}
	return 100 * raw / (raw + 1e9)
}

// Reclaimable reports whether a suggestion counts toward "You could get back":
// complete, regenerable and not personal. Anything else is listed for review only.
func (s Suggestion) Reclaimable() bool {
	return !s.Incomplete && !PersonalData(s.Path) && (s.Safety == "cache" || s.Safety == "reinstallable")
}

// Totals splits disjoint suggestions into reclaimable bytes and everything else.
func Totals(in []Suggestion) (reclaimable, other int64) {
	for _, s := range in {
		if s.Reclaimable() {
			reclaimable += s.Bytes
		} else {
			other += s.Bytes
		}
	}
	return reclaimable, other
}

func (s Suggestion) MarshalJSON() ([]byte, error) {
	effective := math.Min(s.Score, rankScore(s.Path, s.Bytes, s.Safety, s.Rule, s.StaleDays, s.Incomplete))
	type plain Suggestion
	return json.Marshal(struct {
		plain
		Score       float64 `json:"score"`
		Badge       string  `json:"badge"`
		Personal    bool    `json:"personal"`
		Reclaimable bool    `json:"reclaimable"`
	}{plain(s), CleanupScore(effective), fmt.Sprintf("%s · stale %dd · %.1f GB", s.Safety, s.StaleDays, float64(s.Bytes)/1e9), PersonalData(s.Path) || s.Safety == "user-data", s.Reclaimable()})
}
func Select(in []Suggestion, key, category, safety string, limit int) ([]Suggestion, error) {
	if key == "" {
		key = "score"
	}
	if key != "score" && key != "size" && key != "stale" {
		return nil, fmt.Errorf("sort must be score, size or stale")
	}
	out := []Suggestion{}
	for _, s := range in {
		// Personal content remains discoverable under Largest and Unused Longest.
		if key == "score" && (PersonalData(s.Path) || s.Safety == "user-data") {
			continue
		}
		if (category == "" || s.Category == category) && (safety == "" || s.Safety == safety) {
			s.Score = math.Min(s.Score, rankScore(s.Path, s.Bytes, s.Safety, s.Rule, s.StaleDays, s.Incomplete))
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if key == "score" {
			pa, pb := rankPriority(a.Path, a.Safety, a.Incomplete), rankPriority(b.Path, b.Safety, b.Incomplete)
			if pa != pb {
				return pa > pb
			}
		}
		x, y := a.Score, b.Score
		if key == "size" {
			x, y = float64(a.Bytes), float64(b.Bytes)
		}
		if key == "stale" {
			x, y = float64(a.StaleDays), float64(b.StaleDays)
		}
		if x == y {
			return a.Path < b.Path
		}
		return x > y
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
