// perf measures the real pipeline without UI or export noise.
package main
import (
 "context"
 "encoding/json"
 "flag"
 "fmt"
 "os"
 "runtime"
 "runtime/pprof"
 "time"
 "github.com/fum4/oh-my-disk/internal/report"
 "github.com/fum4/oh-my-disk/internal/rules"
 "github.com/fum4/oh-my-disk/internal/scanner"
)
func main(){
 root:=flag.String("root",".fixtures/large","fixture root");workers:=flag.Int("workers",0,"workers");cpu:=flag.String("cpu","","CPU profile");heap:=flag.String("heap","","heap profile");cache:=flag.String("cache","","cache output");flag.Parse()
 if *cpu!="" {f,e:=os.Create(*cpu);must(e);must(pprof.StartCPUProfile(f));defer f.Close();defer pprof.StopCPUProfile()}
 e,err:=rules.Load("",*root);must(err)
 t:=time.Now();s,err:=scanner.Scan(context.Background(),*root,scanner.Options{Workers:*workers});must(err);fmt.Printf("walk_and_aggregate=%s entries=%d\n",time.Since(t),s.Entries)
 t=time.Now();snap:=report.New(s,e,1);fmt.Printf("classify=%s suggestions=%d bytes=%d\n",time.Since(t),len(snap.Suggestions),s.Root.Bytes)
 if *cache!="" {t=time.Now();must(report.SaveCache(*cache,snap));fmt.Printf("cache=%s\n",time.Since(t))}
 if *heap!="" {runtime.GC();f,err:=os.Create(*heap);must(err);must(pprof.WriteHeapProfile(f));must(f.Close());runtime.KeepAlive(snap)}
 json.NewEncoder(os.Stdout).Encode(report.Summary(snap,10))
}
func must(e error){if e!=nil{panic(e)}}
