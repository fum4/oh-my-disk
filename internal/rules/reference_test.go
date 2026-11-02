package rules

import (
	"github.com/fum4/oh-my-disk/internal/scanner"
	"sort"
	"strings"
	"time"
)

func (e *Engine) analyzeReference(root *scanner.Node, now time.Time, min int64) ([]Suggestion, map[string]int64) {
	categories := map[string]int64{}
	for _, c := range Categories {
		categories[c] = 0
	}
	candidates := []Suggestion{}
	// Never recommend an aggregate that hides a more specific recognized location.
	hasMatchBelow := map[*scanner.Node]bool{}
	var index func(*scanner.Node, bool) bool
	index = func(n *scanner.Node, project bool) bool {
		project = project || isProject(n)
		below := false
		for _, c := range n.Children {
			below = index(c, project) || below
		}
		hasMatchBelow[n] = below
		return below || e.referenceMatch(n.Path, project) != nil || PersonalData(n.Name)
	}
	index(root, false)
	var walk func(*scanner.Node, *scanner.Node, *Rule, bool)
	walk = func(n, project *scanner.Node, inherited *Rule, split bool) {
		if isProject(n) {
			project = n
		}
		r := e.referenceMatch(n.Path, project != nil)
		effective := r
		if effective == nil {
			effective = inherited
		}
		category := "unknown"
		if effective != nil {
			category = effective.Category
		}
		categories[category] += n.OwnBytes
		n.Category = category
		if n.Bytes >= min && n.Bytes > 0 && n.Kind != "symlink" && n.Kind != "special" {
			// Unknown candidates are immediate branches/files, not every descendant of
			// an already recognized cache. The score pass selects disjoint paths.
			if !hasMatchBelow[n] && (r != nil || inherited == nil || split) {
				if r == nil && inherited != nil {
					r = inherited
				}
				s := Suggestion{Path: n.Path, Bytes: n.Bytes, Category: "unknown", Safety: "unknown", Rule: "big-unknown", Explanation: "Big, check yourself: unrecognized content. Inspect and back up before making changes.", LastActivity: latest(n), Incomplete: n.Incomplete}
				if r != nil {
					s.Category = r.Category
					s.Safety = r.Safety
					s.Rule = r.ID
					s.Explanation = r.Explanation
					s.Command = strings.ReplaceAll(r.Command, "{path}", Quote(n.Path))
					if r.Project && project != nil {
						s.LastActivity = project.LatestModified
						s.ProjectPath = project.Path
					}
				}
				s.StaleDays = int(now.Sub(s.LastActivity).Hours() / 24)
				if s.StaleDays < 0 {
					s.StaleDays = 0
				}
				s.Score = rankScore(s.Path, s.Bytes, s.Safety, s.Rule, s.StaleDays, s.Incomplete)
				// The scan root is context, never itself a cleanup candidate.
				if n != root {
					candidates = append(candidates, s)
				}
			}
		}
		for _, c := range n.Children {
			walk(c, project, effective, hasMatchBelow[n])
		}
	}
	walk(root, nil, nil, false)
	sort.Slice(candidates, func(i, j int) bool {
		pa, pb := rankPriority(candidates[i].Path, candidates[i].Safety, candidates[i].Incomplete), rankPriority(candidates[j].Path, candidates[j].Safety, candidates[j].Incomplete)
		if pa != pb {
			return pa > pb
		}
		if candidates[i].Score == candidates[j].Score {
			return candidates[i].Path < candidates[j].Path
		}
		return candidates[i].Score > candidates[j].Score
	})

	selected := []Suggestion{}
	used := &pathSet{children: map[string]*pathSet{}}
	for _, c := range candidates {
		if used.add(c.Path) {
			selected = append(selected, c)
		}
	}

	return selected, categories
}
func (e *Engine) referenceMatch(path string, project bool) *Rule {
	for i := range e.Rules {
		r := &e.Rules[i]
		if r.Project && !project {
			continue
		}
		for _, re := range r.matchers {
			if re.MatchString(path) {
				return r
			}
		}
	}
	return nil
}
