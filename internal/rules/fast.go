package rules

import (
	"path/filepath"
	"regexp"
	"strings"
)

type fastPattern struct {
	rule    int
	literal string
	suffix  bool
	re      *regexp.Regexp
}

func (e *Engine) compileFast(home string) {
	e.byName = map[string][]fastPattern{}
	e.wildParents = map[string][]int{}
	for i, r := range e.Rules {
		for j, p := range r.Patterns {
			if strings.HasPrefix(p, "~/") {
				p = filepath.Join(home, p[2:])
			}
			p = filepath.ToSlash(p)
			f := fastPattern{rule: i, literal: p}
			if strings.HasPrefix(p, "**/") && !strings.Contains(p[3:], "*") {
				f.literal = "/" + p[3:]
				f.suffix = true
				e.byName[filepath.Base(p)] = append(e.byName[filepath.Base(p)], f)
			} else if !strings.Contains(p, "*") {
				e.byName[filepath.Base(p)] = append(e.byName[filepath.Base(p)], f)
			} else if strings.HasSuffix(p, "/*") && !strings.Contains(p[:len(p)-2], "*") {
				parent := p[:len(p)-2]
				if parent == "" {
					parent = "/"
				}
				e.wildParents[parent] = append(e.wildParents[parent], i)
			} else {
				f.re = r.matchers[j]
				e.generic = append(e.generic, f)
			}
		}
	}
}
func (e *Engine) parentMatch(path string, project bool) int {
	for _, i := range e.wildParents[path] {
		if !e.Rules[i].Project || project {
			return i
		}
	}
	return -1
}
func (e *Engine) matchName(name string, project bool, best int, path func() string) int {
	var full string
	get := func() string {
		if full == "" {
			full = path()
		}
		return full
	}
	for _, p := range e.byName[name] {
		if best >= 0 && p.rule >= best || e.Rules[p.rule].Project && !project {
			continue
		}
		ok := false
		if p.suffix {
			// A single **/basename needs neither path construction nor regexp matching.
			if strings.Count(p.literal, "/") == 1 {
				ok = true
			} else {
				ok = strings.HasSuffix(get(), p.literal) || get() == p.literal[1:]
			}
		} else {
			ok = get() == p.literal
		}
		if ok {
			best = p.rule
		}
	}
	for _, p := range e.generic {
		if (best < 0 || p.rule < best) && (!e.Rules[p.rule].Project || project) && p.re.MatchString(get()) {
			best = p.rule
		}
	}
	return best
}
