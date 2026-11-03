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
| Analyze (103,001 nodes) | 904,587,750 | 5,465,672 | 14,213 |
| Match (one cold call) | 12,750 | 4,120 | 4 |

The benchmark harness was built against the original scanner/rules/cache before
implementation changes. A full baseline JSON cache is retained locally to compare
sizes, categories, suggestions and ranking after the changes.

## Semantics constraints

Unconditionally stopping matching at every recognized parent would change existing
rules. `.cache/huggingface`, `.codex/cache`, nested project outputs and arbitrary
custom patterns must still override parents. The implementation must preserve these
splits, using aggregate accounting only when no descendant override exists.

## Final pipeline

Default configuration: 2× CPU walk workers (28 here, capped at 64), 64 KiB bulk
buffers, 896 MiB Go soft memory limit (plus any heap already live, such as the
displayed scan the app keeps during a rescan) unless `GOMEMLIMIT` is already set.
The limit controls Go heap growth; it is not an RSS cap. No file details are collapsed.

The limit was 704 MiB until round 21. Adding device/inode to each entry for Trash
revalidation (16 B × ~6M entries) put the end-of-walk live heap (~634 MB) right at
that limit, so the collector ran continuously: on the real home (6.01M entries)
`GODEBUG=gctrace=1` showed 1,691 collections and 59 s user CPU per scan, against
173 collections and 11 s before the change. At 896 MiB the same scan runs 10
collections, 9 s user CPU, 29 s wall, 1.17 GB peak RSS (832 MiB: 15 collections,
1.07 GB; 1 GiB: 11 collections, 1.35 GB).

953,003-node fixture, same 208,896,000 bytes and same 2,000 ranked suggestions:

| Phase/resource | Original | Final default |
|---|---:|---:|
| Walk + aggregate | 4.881 s | 3.793 s |
| Walk alone (instrumented baseline rerun) | 5.674 s | 3.779 s |
| Aggregate alone (same rerun) | 56.62 ms | 13.51 ms |
| Classification | 19.758 s | 15.53 ms |
| Cache write | 1.101 s | 134.14 ms |
| Entire measured process | 25.80 s | 4.30 s |
| CPU user / system | 22.87 / 25.80 s | 0.73 / 17.01 s |
| Peak RSS | 1,941 MiB | 210 MiB |
| Cache bytes | 401,708,245 | 73,895,018 |
| Initial UI response bytes | ~401,708,245 (whole snapshot) | 314,303 |
| Initial UI response nodes | 953,003 | 601 (hard budget: 1,200) |

The separate baseline phase rerun uses original commit
`bbc9de52cf5a3f712de6a08a5db415a1f87f10a0`, adding only timers immediately before
and after aggregation. Its walk+aggregate took 5.730 s, classification 20.004 s.
This illustrates the filesystem timing variation; do not sum its phases with
those from the first baseline run. It did not write a cache (524 MiB peak RSS).
Raw results: `.build/baseline-phases.txt`, `.build/default-large.txt`.

A 56-worker profiled run took 3.71 s total, 15.45 ms classifying, 206 MiB peak RSS.
Default-profile repeats ranged from 2.87 s / 180 MiB to the final 4.30 s / 210 MiB
shown above. The latter replaces an existing cache, including checksum validation.
These are warm APFS measurements, not a
claim that adding workers always helps. Median walk+aggregate times over three
interleaved worker sweeps (before reducing buffers from 256 to 64 KiB):

| Workers | 1 | 7 | 14 | 28 | 56 |
|---|---:|---:|---:|---:|---:|
| Seconds | 7.880 | 4.483 | 4.276 | 3.719 | 3.139 |

On the real home, 28 workers used less system CPU and were at least as fast as
56, so the default remains 2× CPU. Explicit `--workers 56` is useful to try on
broad metadata fixtures. Raw sweep: `.build/worker-tuning.jsonl`.

Final Go benchmarks (`-benchtime=3x`; baseline above was `1x`, so cold-call
allocation effects especially affect the old Match result):

| Benchmark | ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| Scanner (5,100 entries) | 1,551,736 | 3,491,304 | 5,953 |
| Compact aggregate (101,001 nodes) | 1,464,708 | 32 | 0 |
| Analyze compact (103,001 nodes) | 3,444,097 | 7,225,088 | 20,085 |
| Match | 125 | 0 | 0 |
| Analyze legacy pointer-tree adapter | 9,749,653 | 32,975,264 | 15,166 |

The legacy adapter includes conversion into compact storage. Production scans
already use compact storage and call `AnalyzeTree` directly. The original
recursive aggregate benchmark remains available for comparison (2.95 ms in the
final run); production uses the reverse-index aggregate pass.

## Real home, metadata only

The final default run scanned **5,737,213 entries**, with **3,980 suggestions** at
the CLI's 1 MiB threshold:

| Phase/resource | Final default |
|---|---:|
| Walking | 28.185 s |
| Aggregating | 111.77 ms |
| Classifying | 543.00 ms |
| Writing binary cache | 1.028 s |
| Whole process, including profiles and summary | 30.20 s |
| Peak RSS | 908,869,632 B (867 MiB) |

Command (fixture/build artifacts deliberately excluded from the real scan):

```
python3 scripts/measure.py /usr/bin/time -l .build/perf -root "$HOME" -min 1048576 \
  -exclude "$PWD/.fixtures,$PWD/.build" -cache .build/home-default-cache/latest.bin \
  -cpu .build/home-default-cpu.pprof -heap .build/home-default-heap.pprof
```

This is a live home, not a frozen dataset. Repeated runs varied in entry count,
activity, skipped locations and totals as applications changed files. Permission
failures, excluded artifacts and unenumerated placeholders make totals partial.
The original real-home run was not repeated through the minutes-long rules/cache
pipeline; its observed problem is the user-provided baseline. The full identical
results comparison uses the frozen synthetic fixture instead.

An intermediate compact implementation still peaked at 1.89 GiB on this home.
Profiling led to storing directory aggregates separately, exact-capacity name
arenas, allocation-free path comparisons, an exclusion basename prefilter that
avoids constructing paths for ordinary files, compact candidate descriptors, and
building rich suggestion strings only after disjoint selection. The resulting
live heap on the fixture is about 80 MiB (down from 113 MiB in the first compact
prototype). These measurements predate the Trash follow-up: retaining device/inode adds 16 bytes per entry (80 bytes now). At the measured revision, each entry is 64 bytes; directories have an additional 40-byte
aggregate record. Names and indices contain no per-entry maps or pointers.

Using a **1-byte** threshold generated ~146,000 final home suggestions and used
more memory/time than the normal 1 MiB threshold; it is intentionally not the
reported default measurement. Custom globs and very low thresholds still have
cost proportional to the number of matches/candidates. The implementation keeps
all file detail and exact ranking rather than hiding that cost by approximation.

## Profiles and verification

The final fixture CPU profile attributes about 95% of sampled CPU to filesystem
syscalls. Regular-file regexp matching no longer appears as a hotspot. The live
heap is primarily entry chunks (~61 MiB), name arenas (~12 MiB), directory
aggregates (~3.5 MiB), plus runtime/profile/report overhead. Profile commands:

```
go tool pprof -top .build/perf .build/default-cpu.pprof
go tool pprof -top -inuse_space .build/perf .build/default-heap.pprof
# Initial and final real-home profiles are also under .build/home*-*.pprof.
```

Validation completed:

- `go vet ./...`, `go test ./...`, and `go test -race ./...` pass.
- Darwin bulk reader actually runs on APFS and agrees with `lstat`; mount and
  firmlink attributes are refreshed with descriptor-relative `fstatat` because
  bulk attributes can describe the underlying mountpoint rather than its target.
  See [Apple's getattrlistbulk documentation](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/man/man2/getattrlistbulk.2).
- Bulk and forced portable fallback agree across all 953,003 fixture nodes.
- Streaming comparison against the original full JSON cache verifies every
  fixture node, category totals, and all 2,000 suggestions in identical order.
  Raw directory atimes are excluded from this comparison because enumeration
  changes them; derived activity and file timestamps must still agree.
- Generated trees/custom globs compare against the original regexp/ranking
  implementation, including nested project outputs, wildcard scan roots, newly recognized projects,
  root/relative suffix patterns, and specific cache overrides.
- Compact chunk boundaries, 40,000-level iterative aggregation, exact timestamp
  nanoseconds/year 1600, replaced ancestor symlinks, hardlinks, sparse files,
  dataless flags, cache corruption, streamed export and bounded API pages pass.
- Linux amd64 scanner tests cross-compile and the full CLI cross-builds. Linux
  execution was not available on this Mac; existing CI runs it on Linux.
- JavaScript syntax check passes. A local Node controller smoke test against the
  running token-protected server passed initial canvas creation, folder navigation,
  400-child pagination, server sorting, category paging and treemap creation.
  The Browser plugin's runtime module was missing, so actual visual browser QA
  could not run. API tests include host/token/method checks and response budgets.

Re-run the full-size semantic checks using the preserved original cache:

```
OMD_FIXTURE="$PWD/.fixtures/large" OMD_BASELINE="$PWD/.build/baseline-cache/latest.json" \
  go test ./internal/report -run TestOriginalLargeCacheEquivalent -v
OMD_FIXTURE="$PWD/.fixtures/large" \
  go test ./internal/scanner -run TestLargeFixtureBulkFallbackEquivalence -v
OMD_CACHE="$PWD/.build/default-cache/latest.bin" \
  go test ./internal/web -run TestLargeCachedResponseBounded -v
```

## Intentional output/safety differences

- Dataless cloud files contribute logical data length, flagged `dataless`, as
  requested. They are never opened for content. Dataless directories are marked
  incomplete and not enumerated. Totals for homes with cloud placeholders can
  therefore differ from the former allocated-only total; they are not promises
  of reclaimable local storage. The report note explains this accounting.
- Relative directory opens now reject symlinks in **every** component beneath a
  pinned root, closing the old ancestor-replacement race. Such races yield a
  skipped/incomplete subtree, never traversal through the replacement symlink.
- Full JSON exports retain every requested file and exact totals; whitespace is
  now compact, streamed JSON. Node IDs are added for lazy navigation; pagination
  fields appear in API views. Timestamp instants/nanoseconds are retained; binary
  cache timestamps render using the local timezone.
- The default cache is `latest.bin`, versioned/checksummed. Explicit legacy JSON
  caches are readable; old default `latest.json` is not automatically overwritten.
  Binary cache validation before replacement streams the checksum instead of
  loading a second complete tree. Private directory and exclusive report checks
  remain in place.
- Built-in YAML and scoring semantics are unchanged. Literal basename/exact-path/
  suffix/parent-wildcard indices preserve rule priority. Generic custom globs
  retain precompiled regexps, including file matches. Descendant override indexing
  is necessary to preserve nested rules; homogeneous recognized subtrees use
  aggregate category accounting rather than constructing per-file suggestions.

## Cache load and full export measurements

All 953,003 fixture nodes, unpruned full export; the original command uses its
normal pretty JSON writer, final uses compact streaming JSON:

| Phase/resource | Original | Final |
|---|---:|---:|
| Load cache | 1.954 s | 140.70 ms |
| Copy/prune full tree for export | 46.89 ms | no copy |
| Encode/write full JSON to `/dev/null` / `io.Discard` | 2.480 s | 1.065 s |
| Peak RSS, load + full export process | 2,621 MiB | 183 MiB |

Raw measurements: `.build/baseline-export.txt`, `.build/cache-export.txt`.
The before export measurement calls the original `report.LoadCache`, `Export`
and `WriteJSON` directly, with stdout redirected to `/dev/null`. The final
measurement uses `cmd/perf -load .build/final-cache/latest.bin -export`.

Loading the **real 5.74M-entry binary cache**, reclassifying at 1 MiB, and producing
the summary took 1.56 s total: 809 ms loading, 305 ms classifying, 791 MiB peak RSS
(`.build/home-cache-load.txt`). It records 55,210 dataless entries contributing
151,522,790,906 logical bytes; this explains why that home's new mixed accounting
total must not be compared directly with its old allocated-only total.

The actual large-cache `/api/scan` handler took 943 µs to produce its 314,303-byte
response in a local `httptest` measurement after cache load. This excludes
network transit, browser parsing and drawing. Startup has no full-tree transfer
or eager frontend indexing. No visual rendering speed claim is made without a
working browser runtime.
