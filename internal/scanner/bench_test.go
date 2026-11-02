package scanner

import (
 "context"
 "fmt"
 "os"
 "path/filepath"
 "testing"
)

func BenchmarkScan(b *testing.B) {
 root := os.Getenv("OMD_FIXTURE")
 if root == "" {
  root = b.TempDir()
  for i:=0;i<100;i++ { d:=filepath.Join(root,fmt.Sprint(i)); os.Mkdir(d,0700); for j:=0;j<50;j++ { f,e:=os.Create(filepath.Join(d,fmt.Sprint(j))); if e!=nil {b.Fatal(e)}; f.Close() } }
 }
 b.ResetTimer(); b.ReportAllocs()
 for i:=0;i<b.N;i++ { if _,e:=Scan(context.Background(),root,Options{});e!=nil {b.Fatal(e)} }
}
func BenchmarkAggregate(b *testing.B) {
 root:=&Node{Kind:"directory"}
 for i:=0;i<1000;i++ {d:=&Node{Name:fmt.Sprint(i),Kind:"directory"};root.Children=append(root.Children,d);for j:=0;j<100;j++ {d.Children=append(d.Children,&Node{Name:fmt.Sprint(j),Kind:"file",OwnBytes:4096})}}
 b.ResetTimer();b.ReportAllocs()
 for i:=0;i<b.N;i++ { aggregate(root,map[identity]bool{}) }
}
