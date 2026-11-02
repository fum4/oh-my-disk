package rules

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/fum4/oh-my-disk/internal/scanner"
	"gopkg.in/yaml.v3"
)

//go:embed rules.yaml
var builtins []byte
var Categories = []string{"caches", "dev tooling", "AI models", "media", "apps", "downloads", "project build artifacts", "unknown"}
var safetyWeight = map[string]float64{"cache": 1, "reinstallable": .55, "user-data": .12, "unknown": .04}

type Rule struct {
	ID          string   `yaml:"id" json:"id"`
	Patterns    []string `yaml:"patterns" json:"patterns"`
	Category    string   `yaml:"category" json:"category"`
	Safety      string   `yaml:"safety" json:"safety"`
	Explanation string   `yaml:"explanation" json:"explanation"`
	Command     string   `yaml:"command" json:"command"`
	Project     bool     `yaml:"project" json:"project"`
	matchers    []*regexp.Regexp
}
type Engine struct{ Rules []Rule }
type Suggestion struct {
	Path         string    `json:"path"`
	Bytes        int64     `json:"bytes"`
	Category     string    `json:"category"`
	Safety       string    `json:"safety"`
	Rule         string    `json:"rule"`
	Explanation  string    `json:"explanation"`
	Command      string    `json:"suggested_command,omitempty"`
	LastActivity time.Time `json:"last_activity"`
	StaleDays    int       `json:"stale_days"`
	Score        float64   `json:"score"`
	ProjectPath  string    `json:"project_path,omitempty"`
	Incomplete   bool      `json:"incomplete,omitempty"`
}

func parse(data []byte) ([]Rule, error) {
	var doc struct {
		Rules []Rule `yaml:"rules"`
	}
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	dec.KnownFields(true)
	if err := dec.Decode(&doc); err != nil {
		return nil, err
	}
	return doc.Rules, nil
}
func Load(config, home string) (*Engine, error) {
	rr, err := parse(builtins)
	if err != nil {
		return nil, err
	}
	if config != "" {
		data, e := os.ReadFile(config)
		if e != nil {
			return nil, e
		}
		custom, e := parse(data)
		if e != nil {
			return nil, e
		}
		rr = append(custom, rr...)
	}
	eng := &Engine{}
	ids := map[string]bool{}
	for _, r := range rr {
		if ids[r.ID] {
			continue
		}
		ids[r.ID] = true
		if r.ID == "" || len(r.Patterns) == 0 || r.Explanation == "" {
			return nil, fmt.Errorf("rule needs id, patterns and explanation: %q", r.ID)
		}
		if _, ok := safetyWeight[r.Safety]; !ok {
			return nil, fmt.Errorf("rule %s: invalid safety %q", r.ID, r.Safety)
		}
		valid := false
		for _, c := range Categories {
			valid = valid || c == r.Category
		}
		if !valid {
			return nil, fmt.Errorf("rule %s: invalid category %q", r.ID, r.Category)
		}
		for _, p := range r.Patterns {
			if strings.HasPrefix(p, "~/") {
				p = filepath.Join(home, p[2:])
			}
			if !strings.HasPrefix(p, "/") && !strings.HasPrefix(p, "**/") {
				return nil, fmt.Errorf("rule %s: patterns must be absolute, ~/ or **/", r.ID)
			}
			pattern := regexp.QuoteMeta(filepath.ToSlash(p))
			pattern = strings.ReplaceAll(pattern, `\*\*/`, "\x01")
			pattern = strings.ReplaceAll(pattern, `\*\*`, "\x00")
			pattern = strings.ReplaceAll(pattern, `\*`, `[^/]*`)
			pattern = strings.ReplaceAll(pattern, "\x00", `.*`)
			pattern = strings.ReplaceAll(pattern, "\x01", `(?:.*/)?`)
			re, e := regexp.Compile("^" + pattern + "$")
			if e != nil {
				return nil, e
			}
			r.matchers = append(r.matchers, re)
		}
		eng.Rules = append(eng.Rules, r)
	}
	return eng, nil
}
func (e *Engine) Match(path string, project bool) *Rule {
	for i := range e.Rules {
		r := &e.Rules[i]
		if r.Project && !project {
			continue
		}
		for _, re := range r.matchers {
			if re.MatchString(filepath.ToSlash(path)) {
				return r
			}
		}
	}
	return nil
}

var markers = map[string]bool{"package.json": true, "Cargo.toml": true, "pyproject.toml": true, "go.mod": true, "pom.xml": true, "build.gradle": true, "build.gradle.kts": true, "Package.swift": true, "requirements.txt": true, "CMakeLists.txt": true, ".git": true}

func isProject(n *scanner.Node) bool {
	for _, c := range n.Children {
		if markers[c.Name] {
			return true
		}
	}
	return false
}
func latest(n *scanner.Node) time.Time {
	t := n.LatestModified
	if n.LatestAccessed.After(t) {
		t = n.LatestAccessed
	}
	return t
}
func Quote(path string) string { return "'" + strings.ReplaceAll(path, "'", `'"'"'`) + "'" }

// Analyze attributes each inode's allocated bytes to exactly one category, and
// returns disjoint contenders so their reported sizes can safely be summed.
func (e *Engine) Analyze(root *scanner.Node, now time.Time, min int64) ([]Suggestion, map[string]int64) {
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
		return below || e.Match(n.Path, project) != nil
	}
	index(root, false)
	var walk func(*scanner.Node, *scanner.Node, *Rule, bool)
	walk = func(n, project *scanner.Node, inherited *Rule, split bool) {
		if isProject(n) {
			project = n
		}
		r := e.Match(n.Path, project != nil)
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
						s.LastActivity = latest(project)
						s.ProjectPath = project.Path
					}
				}
				s.StaleDays = int(now.Sub(s.LastActivity).Hours() / 24)
				if s.StaleDays < 0 {
					s.StaleDays = 0
				}
				age := float64(s.StaleDays) / 180
				if age > 1 {
					age = 1
				}
				s.Score = float64(s.Bytes) * safetyWeight[s.Safety] * (1 + age)
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
func contains(a, b string) bool {
	return a == b || a == string(filepath.Separator) && filepath.IsAbs(b) || strings.HasPrefix(b, a+string(filepath.Separator))
}

// pathSet rejects ancestor/descendant overlaps in O(path depth), rather than
// comparing each contender with every previously selected path.
type pathSet struct {
	selected bool
	count    int
	children map[string]*pathSet
}

func (s *pathSet) add(path string) bool {
	parts := strings.Split(filepath.Clean(path), string(filepath.Separator))
	chain := []*pathSet{s}
	cur := s
	for _, part := range parts {
		if cur.selected {
			return false
		}
		next := cur.children[part]
		if next == nil {
			next = &pathSet{children: map[string]*pathSet{}}
			cur.children[part] = next
		}
		cur = next
		chain = append(chain, cur)
	}
	if cur.count > 0 {
		return false
	}
	cur.selected = true
	for _, n := range chain {
		n.count++
	}
	return true
}
func (e *Engine) Explain(root *scanner.Node, path string, now time.Time) (*Suggestion, error) {
	var target, project, matched *scanner.Node
	var rule *Rule
	var visit func(*scanner.Node) bool
	visit = func(n *scanner.Node) bool {
		if !contains(n.Path, path) {
			return false
		}
		if isProject(n) {
			project = n
		}
		if r := e.Match(n.Path, project != nil); r != nil {
			rule = r
			matched = n
		}
		if n.Path == path {
			target = n
			return true
		}
		for _, c := range n.Children {
			if visit(c) {
				return true
			}
		}
		return false
	}
	visit(root)
	if target == nil {
		return nil, fmt.Errorf("path not in cached scan: %s", path)
	}
	s := Suggestion{Path: path, Bytes: target.Bytes, Category: "unknown", Safety: "unknown", Rule: "big-unknown", Explanation: "Big, check yourself: unrecognized content. Inspect before changing anything.", LastActivity: latest(target), Incomplete: target.Incomplete}
	if rule != nil {
		s.Category = rule.Category
		s.Safety = rule.Safety
		s.Rule = rule.ID
		s.Explanation = rule.Explanation
		s.Command = strings.ReplaceAll(rule.Command, "{path}", Quote(matched.Path))
		if matched != target {
			s.Explanation += " (Inherited from " + matched.Path + "; suggested command applies to that location.)"
		}
		if rule.Project && project != nil {
			s.LastActivity = latest(project)
			s.ProjectPath = project.Path
		}
	}
	s.StaleDays = int(now.Sub(s.LastActivity).Hours() / 24)
	if s.StaleDays < 0 {
		s.StaleDays = 0
	}
	age := float64(s.StaleDays) / 180
	if age > 1 {
		age = 1
	}
	s.Score = float64(s.Bytes) * safetyWeight[s.Safety] * (1 + age)
	return &s, nil
}
