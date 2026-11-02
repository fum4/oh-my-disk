package rules

import (
	"github.com/fum4/oh-my-disk/internal/scanner"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// AnalyzeTree matches each location once. Literal basename buckets eliminate
// all path construction for ordinary files. A bottom-up bit marks only branches
// needing a split; homogeneous matched subtrees use their aggregate byte count.
func (e *Engine) AnalyzeTree(t *scanner.Tree, now time.Time, min int64) ([]Suggestion, map[string]int64) {
	t.Categories = Categories
	type state struct {
		rule, project int32
		below         bool
	}
	states := make([]state, t.Count)
	type job struct {
		id      int32
		project int32
		wild    int
	}
	// Each root branch is independent. Workers own disjoint entries and states.
	roots := []int32{0}
	// Match root separately, then distribute its children across cores.
	process := func(root int32, project int32, wild int) {
		stack := []job{{root, project, wild}}
		for len(stack) > 0 {
			j := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			n := t.At(j.id)
			if n.Kind == scanner.Directory {
				for c := n.First; c >= 0; c = t.At(c).Next {
					if markers[t.Name(c)] {
						j.project = j.id
						break
					}
				}
			}
			r := e.matchName(t.Name(j.id), j.project >= 0, j.wild, func() string { return t.Path(j.id) })
			childWild := -1
			if n.Kind == scanner.Directory && len(e.wildParents) > 0 {
				childWild = e.parentMatch(t.Path(j.id), j.project >= 0)
			}
			states[j.id] = state{rule: int32(r), project: j.project}
			for c := n.First; c >= 0; c = t.At(c).Next {
				stack = append(stack, job{c, j.project, childWild})
			}
		}
	}
	// Prepare root without traversing children twice.
	root := t.At(0)
	project := int32(-1)
	for c := root.First; c >= 0; c = t.At(c).Next {
		if markers[t.Name(c)] {
			project = 0
			break
		}
	}
	states[0] = state{rule: int32(e.matchName(t.Name(0), project >= 0, -1, func() string { return t.RootPath })), project: project}
	roots = roots[:0]
	for c := root.First; c >= 0; c = t.At(c).Next {
		roots = append(roots, c)
	}
	rootWild := e.parentMatch(t.RootPath, project >= 0)
	jobs := make(chan int32)
	var wg sync.WaitGroup
	workers := runtime.GOMAXPROCS(0)
	if workers > len(roots) {
		workers = len(roots)
	}
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range jobs {
				process(id, project, rootWild)
			}
		}()
	}
	for _, id := range roots {
		jobs <- id
	}
	close(jobs)
	wg.Wait()
	for id := t.Count - 1; id > 0; id-- {
		s := states[id]
		if s.rule >= 0 || s.below {
			states[t.At(id).Parent].below = true
		}
	}
	categories := map[string]int64{}
	for _, c := range Categories {
		categories[c] = 0
	}
	type candidate struct {
		id, rule, project int32
		score             float64
	}
	candidates := []candidate{}
	type visit struct {
		id        int32
		inherited int32
		split     bool
	}
	stack := []visit{{0, -1, false}}
	for len(stack) > 0 {
		v := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		n := t.At(v.id)
		s := states[v.id]
		r := s.rule
		effective := r
		if effective < 0 {
			effective = v.inherited
		}
		category := "unknown"
		if effective >= 0 {
			category = e.Rules[effective].Category
		}
		for i, c := range Categories {
			if c == category {
				n.Category = uint8(i)
				break
			}
		}
		categories[category] += n.OwnBytes
		if t.Bytes(v.id) >= min && t.Bytes(v.id) > 0 && n.Kind != scanner.Symlink && n.Kind != scanner.Special && !s.below && (r >= 0 || v.inherited < 0 || v.split) && v.id != 0 {
			if r < 0 && v.inherited >= 0 {
				r = v.inherited
			}
			activity := t.Time(v.id, 2)
			if t.Time(v.id, 3).After(activity) {
				activity = t.Time(v.id, 3)
			}
			safety := "unknown"
			if r >= 0 {
				safety = e.Rules[r].Safety
				if e.Rules[r].Project && s.project >= 0 {
					activity = t.Time(s.project, 2)
				}
			}
			stale := int(now.Sub(activity).Hours() / 24)
			if stale < 0 {
				stale = 0
			}
			age := float64(stale) / 180
			if age > 1 {
				age = 1
			}
			candidates = append(candidates, candidate{v.id, r, s.project, float64(t.Bytes(v.id)) * safetyWeight[safety] * (1 + age)})
		}
		// Without a descendant override, category accounting takes the subtree's
		// aggregate. Set compact category bytes in a cheap linear subtree walk for UI.
		if effective >= 0 && !s.below {
			categories[category] += t.Bytes(v.id) - n.OwnBytes
			pending := []int32{}
			for c := n.First; c >= 0; c = t.At(c).Next {
				pending = append(pending, c)
			}
			for len(pending) > 0 {
				id := pending[len(pending)-1]
				pending = pending[:len(pending)-1]
				cn := t.At(id)
				cn.Category = n.Category
				for c := cn.First; c >= 0; c = t.At(c).Next {
					pending = append(pending, c)
				}
			}
			continue
		}
		for c := n.First; c >= 0; c = t.At(c).Next {
			stack = append(stack, visit{c, effective, s.below})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score == candidates[j].score {
			return t.LessPath(candidates[i].id, candidates[j].id)
		}
		return candidates[i].score > candidates[j].score
	})
	// Integer ancestry flags replace a per-component map trie. Build rich strings
	// and commands only for the final disjoint selection.
	for i := range states {
		states[i].below = false
		states[i].rule = -1
	}
	selected := []Suggestion{}
	for _, c := range candidates {
		if states[c.id].below {
			continue
		}
		blocked := false
		for p := t.At(c.id).Parent; p >= 0; p = t.At(p).Parent {
			if states[p].rule == -2 {
				blocked = true
				break
			}
		}
		if blocked {
			continue
		}
		states[c.id].rule = -2
		for p := t.At(c.id).Parent; p >= 0; p = t.At(p).Parent {
			states[p].below = true
		}
		n := t.At(c.id)
		path := t.Path(c.id)
		activity := t.Time(c.id, 2)
		if t.Time(c.id, 3).After(activity) {
			activity = t.Time(c.id, 3)
		}
		suggestion := Suggestion{Path: path, Bytes: t.Bytes(c.id), Category: "unknown", Safety: "unknown", Rule: "big-unknown", Explanation: "Big, check yourself: unrecognized content. Inspect and back up before making changes.", LastActivity: activity, Incomplete: n.Flags&scanner.Incomplete != 0, Score: c.score}
		if c.rule >= 0 {
			rule := &e.Rules[c.rule]
			suggestion.Category = rule.Category
			suggestion.Safety = rule.Safety
			suggestion.Rule = rule.ID
			suggestion.Explanation = rule.Explanation
			suggestion.Command = strings.ReplaceAll(rule.Command, "{path}", Quote(path))
			if rule.Project && c.project >= 0 {
				suggestion.LastActivity = t.Time(c.project, 2)
				suggestion.ProjectPath = t.Path(c.project)
			}
		}
		suggestion.StaleDays = int(now.Sub(suggestion.LastActivity).Hours() / 24)
		if suggestion.StaleDays < 0 {
			suggestion.StaleDays = 0
		}
		selected = append(selected, suggestion)
	}
	return selected, categories
}
