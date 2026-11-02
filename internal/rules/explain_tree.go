package rules

import (
	"fmt"
	"github.com/fum4/oh-my-disk/internal/scanner"
	"strings"
	"time"
)

func (e *Engine) ExplainTree(t *scanner.Tree, path string, now time.Time) (*Suggestion, error) {
	id := t.Find(path)
	if id < 0 {
		return nil, fmt.Errorf("path not in cached scan: %s", path)
	}
	chain := []int32{}
	for cur := id; cur >= 0; cur = t.At(cur).Parent {
		chain = append(chain, cur)
	}
	project, matched := int32(-1), int32(-1)
	var rule *Rule
	for i := len(chain) - 1; i >= 0; i-- {
		cur := chain[i]
		n := t.At(cur)
		for c := n.First; c >= 0; c = t.At(c).Next {
			if markers[t.Name(c)] {
				project = cur
				break
			}
		}
		if r := e.Match(t.Path(cur), project >= 0); r != nil {
			rule = r
			matched = cur
		}
	}
	target := t.At(id)
	activity := t.Time(id, 2)
	if t.Time(id, 3).After(activity) {
		activity = t.Time(id, 3)
	}
	s := &Suggestion{Path: path, Bytes: t.Bytes(id), Category: "unknown", Safety: "unknown", Rule: "big-unknown", Explanation: "Big, check yourself: unrecognized content. Inspect before changing anything.", LastActivity: activity, Incomplete: target.Flags&scanner.Incomplete != 0}
	if rule != nil {
		s.Category = rule.Category
		s.Safety = rule.Safety
		s.Rule = rule.ID
		s.Explanation = rule.Explanation
		s.Command = strings.ReplaceAll(rule.Command, "{path}", Quote(t.Path(matched)))
		if matched != id {
			s.Explanation += " (Inherited from " + t.Path(matched) + "; suggested command applies to that location.)"
		}
		if rule.Project && project >= 0 {
			s.LastActivity = t.Time(project, 2)
			s.ProjectPath = t.Path(project)
		}
	}
	s.StaleDays = int(now.Sub(s.LastActivity).Hours() / 24)
	if s.StaleDays < 0 {
		s.StaleDays = 0
	}
	s.Score = rankScore(s.Path, s.Bytes, s.Safety, s.Rule, s.StaleDays, s.Incomplete)
	return s, nil
}
