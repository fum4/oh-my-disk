package rules

import (
	"fmt"
	"github.com/fum4/oh-my-disk/internal/scanner"
	"testing"
	"time"
)

func benchmarkTree() *scanner.Node {
	root := &scanner.Node{Path: "/h", Name: "h", Kind: "directory"}
	for i := 0; i < 1000; i++ {
		path := fmt.Sprintf("/h/app-%d", i)
		project := &scanner.Node{Name: fmt.Sprintf("app-%d", i), Path: path, Kind: "directory", Children: []*scanner.Node{{Name: "package.json", Path: path + "/package.json", Kind: "file"}}}
		dir := &scanner.Node{Name: "node_modules", Path: path + "/node_modules", Kind: "directory"}
		for j := 0; j < 100; j++ {
			dir.Children = append(dir.Children, &scanner.Node{Name: fmt.Sprintf("f%d.js", j), Path: fmt.Sprintf("%s/f%d.js", dir.Path, j), Kind: "file", OwnBytes: 4096, Bytes: 4096, Files: 1})
			dir.Bytes += 4096
		}
		project.Children = append(project.Children, dir)
		project.Bytes = dir.Bytes
		root.Children = append(root.Children, project)
		root.Bytes += dir.Bytes
	}
	return root
}
func BenchmarkAnalyze(b *testing.B) {
	e, _ := Load("", "/h")
	root := benchmarkTree()
	now := time.Now()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		e.Analyze(root, now, 1)
	}
}
func BenchmarkMatch(b *testing.B) {
	e, _ := Load("", "/h")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.Match("/h/app/node_modules/package/index.js", true)
	}
}
