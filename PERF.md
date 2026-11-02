# Performance work (2026-11-02)

Machine: Apple M4 Pro, darwin/arm64, Go 1.23.3 (14 logical CPUs).
Fixtures live only under ignored `.fixtures/`; measurements/profiles under `.build/`.
Results are warm-cache, single samples unless explicitly stated; not portable promises.

## Reproduction

```
export GOCACHE="$PWD/.build/go-cache" GOMODCACHE="$PWD/.build/go-mod" GOPATH="$PWD/.build/go"
python3 scripts/synthetic.py .fixtures/large --large
# default: 953,003 entries including root (1000 projects, 50 packages, 18 files)
# roughly two million: --projects 2000
# The generator refuses an existing destination; never removes/replaces fixtures.
go test ./...
go test ./internal/scanner ./internal/rules -run '^$' -bench . -benchmem -benchtime=3x
 go build -o .build/perf ./cmd/perf
python3 scripts/measure.py /usr/bin/time -l .build/perf -root "$PWD/.fixtures/large" \
  -cache .build/perf-cache/latest.bin -cpu .build/cpu.pprof -heap .build/heap.pprof
 go tool pprof -top .build/perf .build/cpu.pprof
 go tool pprof -top -inuse_space .build/perf .build/heap.pprof
```

`/usr/bin/time -l` runs, but this managed sandbox denies its `sysctl kern.clockrate` call,
so it stops after wall/user/system time and cannot print RSS. `scripts/measure.py`
records the child's peak RSS via `getrusage` instead (macOS reports bytes).
All timing commands and raw output are retained in `.build/` locally.

## Baseline (original v1 implementation)

953,002 entries excluding root, 901,001 regular-file paths, 208,896,000 allocated bytes.

| Phase | Wall time |
|---|---:|
| Walk + aggregation | 4.881 s |
| Classification | 19.758 s |
| JSON cache | 1.101 s |
| Complete harness (includes top summary) | 25.80 s |
| CPU user / system | 22.87 / 25.80 s |
| Peak RSS | 2,034,958,336 B (1,941 MiB) |

Baseline Go benchmarks (`-benchtime=1x`, fixture creation excluded):

| Benchmark | ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| Scanner (5,100 entries) | 8,496,708 | 4,048,536 | 32,052 |
| Aggregate (101,001 nodes) | 5,025,208 | 56,056 | 2,002 |
| Analyze (102,001 nodes) | 904,587,750 | 5,465,672 | 14,213 |
| Match (one cold call) | 12,750 | 4,120 | 4 |

The benchmark harness was built against the original scanner/rules/cache before
implementation changes. A full baseline JSON cache is retained locally to compare
sizes, categories, suggestions and ranking after the changes.

## Semantics constraints

Unconditionally stopping matching at every recognized parent would change existing
rules. `.cache/huggingface`, `.codex/cache`, nested project outputs and arbitrary
custom patterns must still override parents. The implementation must preserve these
splits, using aggregate accounting only when no descendant override exists.
