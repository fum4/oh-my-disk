package rules

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fum4/oh-my-disk/internal/scanner"
)

func node(path string, size int64, when time.Time, children ...*scanner.Node) *scanner.Node {
	n := &scanner.Node{Path: path, Name: filepath.Base(path), Kind: "directory", Bytes: size, OwnBytes: size, Modified: when, Accessed: when, LatestModified: when, LatestAccessed: when, Children: children}
	for _, c := range children {
		n.Bytes += c.Bytes
	}
	return n
}
func TestBuiltinCoverage(t *testing.T) {
	e, err := Load("",
		"/home/dev")
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"/home/dev/.npm":                                          "npm",
		"/home/dev/.yarn/cache":                                   "yarn",
		"/home/dev/Library/pnpm/store":                            "pnpm",
		"/home/dev/.bun/install/cache":                            "bun",
		"/home/dev/.cache/uv":                                     "python-cache",
		"/home/dev/go/pkg/mod":                                    "go-modules",
		"/home/dev/.cache/go-build":                               "go-build",
		"/home/dev/.gradle/caches":                                "gradle",
		"/home/dev/.android/avd":                                  "android-avd",
		"/home/dev/Library/Android/sdk/system-images":             "android-images",
		"/home/dev/Library/Developer/Xcode/DerivedData":           "xcode-derived",
		"/home/dev/Library/Developer/Xcode/Archives":              "xcode-archives",
		"/home/dev/Library/Developer/Xcode/watchOS DeviceSupport": "xcode-device-support",
		"/home/dev/Library/Developer/CoreSimulator/Devices":       "simulator-devices",
		"/Library/Developer/CoreSimulator/Images":                 "simulator-runtimes",
		"/home/dev/Library/Caches/com.apple.dt.Xcode":             "xcode-cache",
		"/home/dev/Library/Caches/CocoaPods":                      "cocoapods",
		"/work/app/Carthage/Build":                                "carthage",
		"/work/app/.build":                                        "swiftpm-project",
		"/home/dev/Library/Caches/Homebrew":                       "homebrew",
		"/home/dev/Library/Containers/com.docker.docker/Data":     "docker",
		"/home/dev/.cache/huggingface":                            "huggingface",
		"/home/dev/.lmstudio/models":                              "lmstudio",
		"/home/dev/.ollama/models":                                "ollama",
		"/home/dev/.cache/anything":                               "cache-folders",
		"/home/dev/Library/Caches/App":                            "cache-folders",
		"/work/app/node_modules":                                  "node-modules",
		"/work/app/target":                                        "project-outputs",
		"/work/app/.venv":                                         "project-outputs",
		"/work/app/build":                                         "project-outputs",
		"/work/app/dist":                                          "project-outputs",
		"/work/app/.next":                                         "project-outputs",
		"/work/app/.turbo":                                        "project-outputs",
		"/home/dev/.Trash":                                        "trash",
		"/home/dev/Downloads":                                     "downloads",
		"/home/dev/Library/Messages/Attachments":                  "mail-messages",
		"/home/dev/Library/Application Support/MobileSync/Backup": "ios-backups",
		"/home/dev/.expo":                                         "expo",
		"/home/dev/.cache/ms-playwright":                          "browser-downloads",
		"/home/dev/.codeium/cache":                                "ai-caches",
		"/home/dev/.codex":                                        "ai-tool-data",
		"/Applications/Xcode.app":                                 "apps",
	}
	for path, id := range cases {
		t.Run(id+path, func(t *testing.T) {
			r := e.Match(path, true)
			if r == nil || r.ID != id {
				t.Fatalf("%s => %#v; want %s", path, r, id)
			}
		})
	}
	if r := e.Match("/random/build", false); r != nil {
		t.Fatal("classified arbitrary build folder as regenerable")
	}
	if r := e.Match("/home/dev/.cache/huggingface-extra", false); r == nil || r.ID != "cache-folders" {
		t.Fatal("pattern crossed segment boundary")
	}
}
func TestRankingSafetyStalenessAndDisjointCategories(t *testing.T) {
	e, _ := Load("",
		"/h")
	now := time.Now()
	old := now.Add(-365 * 24 * time.Hour)
	npm := node("/h/.npm", 100, old)
	dl := node("/h/Downloads", 100, old)
	model := node("/h/.ollama/models", 100, old)
	unk := node("/h/mystery", 100, old)
	root := node("/h", 10, now, npm, dl, model, unk)
	got, c := e.Analyze(root, now, 0)
	if len(got) != 4 {
		t.Fatalf("%+v", got)
	}
	for i, want := range []string{"npm", "ollama", "downloads", "big-unknown"} {
		if got[i].Rule != want {
			t.Fatalf("ranking %d=%s want %s", i, got[i].Rule, want)
		}
	}
	if got[0].Score != 200 {
		t.Fatal("age multiplier must cap at 2")
	}
	var sum int64
	for _, b := range c {
		sum += b
	}
	if sum != root.Bytes {
		t.Fatalf("category bytes=%d total=%d", sum, root.Bytes)
	}
	npm.LatestModified = now
	fresh, _ := e.Analyze(root, now, 0)
	if fresh[0].Rule != "ollama" {
		t.Fatal("staleness not applied")
	}
	// Specific model rule must not disappear under a broad generic cache parent.
	hf := node("/h/.cache/huggingface", 100, old)
	cache := node("/h/.cache", 0, old, hf, node("/h/.cache/other", 100, old))
	got, _ = e.Analyze(node("/h", 0, now, cache), now, 0)
	if len(got) != 2 {
		t.Fatalf("nested contenders: %+v", got)
	}
	for i, a := range got {
		for j, b := range got {
			if i != j && (contains(a.Path, b.Path) || contains(b.Path, a.Path)) {
				t.Fatal("overlap")
			}
		}
	}
}
func TestProjectStalenessAndUnknowns(t *testing.T) {
	e, _ := Load("",
		"/h")
	now := time.Now()
	old := now.Add(-400 * 24 * time.Hour)
	output := node("/h/_work/app/target", 100, old)
	proj := node("/h/_work/app", 0, now, node("/h/_work/app/Cargo.toml", 1, now), output)
	got, _ := e.Analyze(node("/h", 0, now, proj), now, 10)
	if len(got) != 1 || got[0].Rule != "project-outputs" || got[0].StaleDays != 0 || got[0].ProjectPath != proj.Path {
		t.Fatalf("project activity: %+v", got)
	}
	v, err := e.Explain(node("/h", 0, now, proj), output.Path, now)
	if err != nil || v.Rule != "project-outputs" {
		t.Fatalf("explain: %+v %v", v, err)
	}
	if _, err = e.Explain(proj,
		"/absent", now); err == nil {
		t.Fatal("missing path explained")
	}
}
func TestCustomRulesAndValidation(t *testing.T) {
	p := filepath.Join(t.TempDir(), "rules.yaml")
	os.WriteFile(p, []byte("rules:\n  - id: npm\n    patterns: ['~/.npm']\n    category: unknown\n    safety: user-data\n    explanation: custom override\n    command: 'inspect {path}'\n"), 0600)
	e, err := Load(p,
		"/h")
	if err != nil {
		t.Fatal(err)
	}
	if e.Match("/h/.npm", false).Safety != "user-data" {
		t.Fatal("custom rule did not override")
	}
	os.WriteFile(p, []byte("rules:\n  - id: bad\n    patterns: ['**/x']\n    category: caches\n    safety: dangerous\n    explanation: bad\n"), 0600)
	if _, err = Load(p,
		"/h"); err == nil {
		t.Fatal("invalid safety accepted")
	}
	os.WriteFile(p, []byte("rules:\n  - id: x\n    effort: 1\n"), 0600)
	if _, err = Load(p,
		"/h"); err == nil {
		t.Fatal("unknown schema field accepted")
	}
	if Quote("a'b") != `'a'"'"'b'` {
		t.Fatal("shell quote unsafe")
	}
}

func TestInheritedRulesSplitAndExplain(t *testing.T) {
	e, _ := Load("", "/h")
	now := time.Now()
	cache := node("/h/.codex/cache", 200, now)
	sessions := node("/h/.codex/sessions", 100, now, node("/h/.codex/sessions/chat.json", 50, now))
	root := node("/h", 0, now, node("/h/.codex", 0, now, cache, sessions))
	got, _ := e.Analyze(root, now, 0)
	if len(got) != 2 || got[0].Rule != "ai-caches" || got[1].Safety != "user-data" {
		t.Fatalf("split lost state: %+v", got)
	}
	s, err := e.Explain(root, "/h/.codex/sessions/chat.json", now)
	if err != nil || s.Safety != "user-data" || s.Rule != "ai-tool-data" {
		t.Fatalf("inherit: %+v %v", s, err)
	}
	if e.Match("/random/.build", false) != nil {
		t.Fatal("arbitrary .build assumed safe")
	}
	// A whole-disk root must contain descendant paths for explain.
	if _, err = e.Explain(node("/", 0, now, root), "/h/.codex/cache", now); err != nil {
		t.Fatal(err)
	}
}
func TestSizeAndFutureActivity(t *testing.T) {
	e, _ := Load("", "/h")
	now := time.Now()
	root := node("/h", 0, now, node("/h/.npm", 100, now), node("/h/.cache/uv", 200, now.Add(24*time.Hour)))
	got, _ := e.Analyze(root, now, 0)
	if len(got) != 2 || got[0].Bytes != 200 || got[0].Score != 200 || got[0].StaleDays != 0 {
		t.Fatalf("size/future scoring: %+v", got)
	}
}
