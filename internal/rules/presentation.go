package rules

import (
	"encoding/json"
	"fmt"
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
func (s Suggestion) MarshalJSON() ([]byte, error) {
	type plain Suggestion
	return json.Marshal(struct {
		plain
		Score float64 `json:"score"`
		Badge string  `json:"badge"`
	}{plain(s), CleanupScore(s.Score), fmt.Sprintf("%s · stale %dd · %.1f GB", s.Safety, s.StaleDays, float64(s.Bytes)/1e9)})
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
		if (category == "" || s.Category == category) && (safety == "" || s.Safety == safety) {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
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
