package rules

import (
	"encoding/json"
	"github.com/fum4/oh-my-disk/internal/scanner"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestFastMatchersAndAnalyzeEquivalent(t *testing.T) {
	config := filepath.Join(t.TempDir(), "rules.yaml")
	os.WriteFile(config, []byte("rules:\n - id: custom\n   patterns: ['**/package-*/dist/**', '/h/custom/*/thing', '**/*.log', '/h/Library/Caches/*/special']\n   category: media\n   safety: user-data\n   explanation: custom pattern\n"), 0600)
	e, err := Load(config, "/h")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	rng := rand.New(rand.NewSource(10))
	names := []string{"node_modules", "build", "dist", "package.json", "Cargo.toml", "other", "package-1", "thing", "a.log", "special", "target", ".next", ".cache", "huggingface"}
	var makeTree func(string, int) *scanner.Node
	makeTree = func(path string, depth int) *scanner.Node {
		n := node(path, int64(rng.Intn(1000)), now.Add(-time.Duration(rng.Intn(365))*24*time.Hour))
		if depth > 0 {
			used := map[string]bool{}
			for i := 0; i < 5; i++ {
				name := names[rng.Intn(len(names))]
				if used[name] {
					continue
				}
				used[name] = true
				n.Children = append(n.Children, makeTree(path+"/"+name, depth-1))
				n.Bytes += n.Children[len(n.Children)-1].Bytes
			}
		} else {
			n.Kind = "file"
		}
		return n
	}
	for i := 0; i < 20; i++ {
		root := makeTree("/h", 4)
		scanner.Walk(root, func(n *scanner.Node) {
			for _, project := range []bool{false, true} {
				a, b := e.Match(n.Path, project), e.referenceMatch(n.Path, project)
				if a != b {
					t.Fatalf("matcher differs for %s project=%v: %v / %v", n.Path, project, a, b)
				}
			}
		})
		for _, min := range []int64{0, 500, 5000} {
			a, ac := e.analyzeReference(root, now, min)
			tr := scanner.Compact(root)
			b, bc := e.AnalyzeTree(tr, now, min)
			for i := range a {
				a[i].LastActivity = a[i].LastActivity.UTC()
			}
			for i := range b {
				b[i].LastActivity = b[i].LastActivity.UTC()
			}
			aj, _ := json.Marshal([]any{a, ac})
			bj, _ := json.Marshal([]any{b, bc})
			if string(aj) != string(bj) {
				t.Fatalf("analysis differs iteration %d threshold %d (%d vs %d candidates)", i, min, len(a), len(b))
			}
			for _, s := range b {
				want, err := e.Explain(root, s.Path, now)
				got, ge := e.ExplainTree(tr, s.Path, now)
				if want != nil {
					want.LastActivity = want.LastActivity.UTC()
				}
				if got != nil {
					got.LastActivity = got.LastActivity.UTC()
				}
				wj, _ := json.Marshal(want)
				gj, _ := json.Marshal(got)
				if err != nil || ge != nil || string(wj) != string(gj) {
					t.Fatalf("explain differs: %s %s: %v %v", wj, gj, err, ge)
				}
			}
		}
	}
}
func BenchmarkAnalyzeCompact(b *testing.B) {
	e, _ := Load("", "/h")
	tr := scanner.Compact(benchmarkTree())
	now := time.Now()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		e.AnalyzeTree(tr, now, 1)
	}
}
func TestNestedDirectoryRulesKeepSpecificSuggestions(t *testing.T) {
	e, _ := Load("", "/h")
	now := time.Now()
	child := node("/h/app/node_modules/p/dist", 100, now)
	pkg := node("/h/app/node_modules/p", 0, now, node("/h/app/node_modules/p/package.json", 1, now), child)
	mods := node("/h/app/node_modules", 0, now, pkg)
	root := node("/h", 0, now, node("/h/app", 0, now, node("/h/app/package.json", 1, now), mods))
	a, c := e.analyzeReference(root, now, 10)
	b, d := e.AnalyzeTree(scanner.Compact(root), now, 10)
	for i := range a {
		a[i].LastActivity = a[i].LastActivity.UTC()
	}
	for i := range b {
		b[i].LastActivity = b[i].LastActivity.UTC()
	}
	if !reflect.DeepEqual(a, b) || !reflect.DeepEqual(c, d) || len(b) != 1 || b[0].Path != child.Path {
		t.Fatalf("specific nested rule lost: %v %v", a, b)
	}
}
