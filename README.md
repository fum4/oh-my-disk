# oh-my-disk

A local, read-only disk-space analyzer for developers. Find the caches, SDKs, models, build outputs and forgotten downloads filling your home folder. Written in Go for macOS Apple Silicon and Linux, with a native Wails macOS desktop app and a DaisyDisk-inspired explorer shared with the CLI.

**It never deletes, moves, or edits scanned content.** There is no cleanup subcommand, execution hook, cleanup API, or generated cleanup script. Suggested commands are plain text for a human to review. The desktop app only invokes macOS `open` for Reveal in Finder and Full Disk Access settings. The CLI browser UI invokes the platform browser opener. Suggested cleanup commands are never run.

## Install

Requires Go 1.26 or newer (the current Wails v2.17.1 requirement). Go can download the toolchain automatically. The CLI needs no Node, frontend build, CDN or database.

```sh
git clone https://github.com/fum4/oh-my-disk.git
cd oh-my-disk
go build -o oh-my-disk .
./oh-my-disk --help
```

Or install with `go install github.com/fum4/oh-my-disk@latest` after this version is published. To cross-compile:

```sh
GOOS=darwin GOARCH=arm64 go build -o oh-my-disk-macos .
GOOS=linux GOARCH=amd64 go build -o oh-my-disk-linux .
```

## Native macOS desktop app

```sh
make app                         # pinned Wails v2.17.1, arm64 .app
open build/bin/oh-my-disk.app     # or double-click in Finder
make app-universal               # Intel + Apple Silicon, same output path
make check                       # go vet ./... and go test ./...
make cli                         # CLI remains ./oh-my-disk
```

The app is a real Cocoa/WKWebView window with a Dock icon and standard Quit
behavior. It does not open a browser tab or expose a TCP listener. `app/main.go`
is a separate desktop entry point within the same Go module; the root `main.go`
remains the CLI. Both use `internal/scanner`, `internal/rules` and
`internal/report`. `internal/web/assets` supplies the same canvas charts and
lazy `/api/node` and `/api/category` endpoints inside Wails' embedded asset
server. Only native dialogs, clipboard, progress and scan lifecycle use bindings.
No token or browser session is needed inside the app. The CLI `ui` command
retains its private, token-protected browser mode.

On the welcome screen choose **Home**, **Whole disk (/)** or a folder from the
native picker, then **Scan**. Progress shows the phase, entries, folders, average
entries per second and current path. **Cancel** stops the walk; classification
and cache serialization finish their current phase before cancellation takes
effect. The displayed scan stays available. **Rescan** uses the current folder
selection. A previous compact CLI/app cache loads without rescanning and is
available through **Open last cached scan**. Cache timestamps remain visible;
volume totals are read live when the result is opened.

Every real item (including the current folder and contenders) has **Reveal in
Finder** and **Copy path** actions. Suggested commands can be copied as text.
**Export JSON** writes the full tree; **Export LLM summary** writes a bounded
summary with up to 50 top entries and 50 contenders. Both use native save dialogs and refuse to overwrite an
existing file, even if the save dialog offers Replace. Choose a new filename.
No deletion or Trash operations are implemented.

The app probes protected `~/Library` folders. If macOS denies access, a banner
links to **System Settings → Privacy & Security → Full Disk Access**. Enable the
built `.app`, quit and reopen it, then rescan. This is a best-effort permission
probe, not a guarantee that every folder is readable. The app never changes
permissions. The CLI's Terminal permission is separate from the app's permission.

Sizes default to decimal **MB/GB**, matching Finder (1 GB = 1,000,000,000 bytes).
The UI's persistent **Binary units (GiB)** setting and CLI `--binary` flag opt
into powers of 1024. JSON always contains integer bytes; input suffixes such as
`--min-size 1MiB` keep their explicit meaning.

The **Whole disk · containing volume** panel and CLI header use `statfs`:
capacity = blocks × block size, available = available blocks × block size,
used = capacity − available, and outside scanned folder = max(0, used − scanned
allocation). This residual includes unreadable paths, unscanned folders,
snapshots and filesystem overhead. It is an accounting remainder rather than
an inventory of those files. APFS shared containers, purgeable space, clones
and cloud placeholders can differ from About This Mac; when allocation exceeds
used, the UI reports the excess rather than showing a negative remainder.
Whole-disk scans retain the engine's Data/firmlink deduplication and do not cross
unrelated mounted volumes by default.

Build prerequisites: macOS, Xcode command-line tools, Go 1.26+, and Node/npm for
Wails tooling (there is no npm install or frontend compilation for this app).
`make` installs the pinned Wails CLI and keeps Go caches/tools under `.build/`.
After installing Wails yourself, the direct equivalent is:

```sh
make icon
cd app
../.build/bin/wails build -platform darwin/arm64
```

The Make wrapper falls back to public `sw_vers` OS-version values if a build
sandbox blocks Wails' two `sysctl` version probes. The original icon is generated
by `scripts/icon.go` at 1024×1024, resized with `sips`, and packaged with
`iconutil`. On hosts where `iconutil` rejects the iconset, a standard PNG-based
ICNS packer is used. A Wails post-build hook installs that ICNS and renews the
ad-hoc signature. No network artwork is used. The macOS plist uses the stable
bundle ID `com.fum4.oh-my-disk`. **TODO before distribution:** Developer ID
signing, hardened runtime/entitlements review, notarization and stapling.
Local builds are ad-hoc signed, without a Developer ID.

See [desktop verification notes](app/VERIFICATION.md) for the checks and host
limitations recorded during implementation.

## Start here

```sh
./oh-my-disk scan                      # home directory, progress on stderr
./oh-my-disk top --limit 25            # largest entries in the latest scan
./oh-my-disk suggest --limit 20        # ranked, disjoint free-space contenders
./oh-my-disk explain "$HOME/.npm"      # rule, safety, age, size, human-only command
./oh-my-disk ui                        # reuse the scan and open a local browser
```

`ui`, `top`, `suggest`, and `explain` reuse the latest cached scan. If there is no cache, they scan the home directory first. Use `--refresh` to rescan; supplying a different `--root` also triggers a scan. The UI displays the scan timestamp rather than implying a live view. Suggestions are recalculated with the current rules and current date when loading a cache.

```sh
./oh-my-disk scan /                    # whole-disk namespace; no sudo required
./oh-my-disk scan ~/Library --workers 12
./oh-my-disk ui --root "$HOME" --refresh
./oh-my-disk ui --no-open --port 8787   # prints a private local URL
```

Permission errors are reported with skipped paths, and the scan continues. Terminal may need macOS Full Disk Access to inspect protected Mail/Messages/backups. Partial totals are clearly marked; the tool never changes permissions or requests elevated privileges. Other filesystem devices are skipped unless `--cross-mounts` is set.

The UI has a clickable, zoomable sunburst, a squarified treemap toggle, breadcrumbs, a sortable child table, category breakdowns and ranked contenders. Click a segment or folder to drill in; click the sunburst center to go back. Small segments and entries beyond the chart’s display budget remain in the paginated table. Category drill-down attributes each entry’s own allocated blocks, so it does not count parent totals a second time.

## JSON and LLM exports

Every subcommand accepts `--json` (stdout) or `--json NEW-FILE.json`. Progress goes to stderr. Existing export files are **never overwritten**. JSON uses integer bytes and RFC 3339 timestamps.

```sh
./oh-my-disk scan --json scan.json --depth 5 --min-size 1MiB
./oh-my-disk scan ~/Library --json full-library.json --min-size 0 --depth -1
./oh-my-disk top --json --limit 30
./oh-my-disk suggest --json suggestions.json --limit 15
./oh-my-disk explain "$HOME/.npm" --json
./oh-my-disk suggest --summary brief.json --limit 8
./oh-my-disk ui --no-open --json       # prints URL JSON, then serves until Ctrl-C
```

`scan` JSON contains `version`, `scan` (tree, timestamps, skips, entry count), `suggestions`, `categories`, and an accounting note. Nodes include allocated `bytes` for the subtree, `own_bytes`, regular-file entry count, type, category, metadata timestamps, latest subtree activity and incomplete/hardlink flags. File counts include all regular-file directory entries, including hardlink aliases; their allocated blocks are counted once.

`--depth` and `--min-size` prune the exported tree without changing its aggregate totals or the full cached scan. Defaults: unlimited depth and 1 MiB minimum for tree entries and contenders. Use `--min-size 0` for a full file-level export. `top` includes overlapping parent/child totals and explicitly labels them. `suggest` outputs only disjoint contenders. `explain` also resolves inherited rules for files inside known locations. `ui --json` outputs connection information, not the tree.

`--summary FILE` emits compact, bounded JSON with the root, scan time, totals, category totals, top N entries, top N disjoint suggestions and an accounting note. The sample fixture’s eight-item summary is about 7 KB. Paths and explanations affect length; lower `--limit` to fit a smaller context. Summary generation never needs an LLM, API key or network connection.

## Ranking and rules

The embedded YAML lives in [internal/rules/rules.yaml](internal/rules/rules.yaml). It covers npm, Yarn, pnpm, Bun, pip/uv, Go module/build caches, Gradle, Android SDK/images/AVDs, Xcode DerivedData/Archives/DeviceSupport/caches, simulators and runtimes, CocoaPods, Carthage, SwiftPM, Homebrew, Docker Desktop, Hugging Face, LM Studio, Ollama, generic macOS/Linux caches, Expo, browser test downloads, AI editor caches/state, project artifacts, Trash, Downloads, media, Mail/Messages and iOS backups. Unknown large branches appear as **“Big, check yourself.”**

The score is:

```text
allocated_bytes × safety_weight × (1 + min(stale_days / 180, 1))

cache:         1.00  (regenerable; still review first)
reinstallable: 0.55  (SDKs, downloaded models, applications)
user-data:     0.12  (archives, backups, Docker volumes, sessions, downloads)
unknown:       0.04  (inspect yourself)
```

**Effort is never part of the score.** Staleness uses the later of latest subtree mtime/regular-file atime and clamps future timestamps to zero days. Project artifacts use the enclosing project’s latest subtree mtime, so old `target` files in an active project are not treated as an abandoned project. Projects are recognized by markers including `package.json`, `Cargo.toml`, `pyproject.toml`, `go.mod`, Gradle/Maven/Swift/CMake manifests and `.git`. Generic `build`, `dist`, `target`, `.venv`, `.next` and `.turbo` names outside a recognized project are not assumed safe.

More specific locations take precedence over enclosing aggregates: Hugging Face models are not mislabeled as expendable generic `.cache` contents. The final score pass selects disjoint paths. Category totals attribute each entry’s own blocks to its nearest matching rule and sum to the scan total. Ranking measures candidates, not a promise that every allocated byte can be freed.

Extend or override rules:

```yaml
# my-rules.yaml
rules:
  - id: my-project-cache
    patterns: ['~/_work/**/.generated-cache']
    category: project build artifacts
    safety: cache
    project: true
    explanation: Generated output; verify the source and lockfiles are backed up.
    command: 'du -sh {path}/*'  # human-only text, never executed
```

```sh
./oh-my-disk suggest --config my-rules.yaml
```

Patterns are anchored, case-sensitive paths: `~/` expands using the current home; `*` matches within one path component; `**` spans components. Absolute patterns and `**/name` anywhere patterns are supported. Rules are tested in order, custom rules first. Reusing a built-in `id` overrides it. Allowed categories are `caches`, `dev tooling`, `AI models`, `media`, `apps`, `downloads`, `project build artifacts`, `unknown`; allowed safety values are shown above. Unknown fields and invalid categories/safety levels fail clearly. `{path}` is replaced with a shell-quoted path in command text. `--home PATH` lets you classify another user’s home or a fixture without accessing your own home.

## Read-only behavior, privacy and accounting

- Scanner operations are metadata reads (macOS bulk attributes, or directory reads plus descriptor-relative `fstatat`); it never opens regular file contents. Symlinks are recorded but not traversed, including loop/broken links. A symlink root is rejected. Directory opens are pinned to the root descriptor, reject symlinks in every relative path component, and verify inode identity; changing trees can still produce partial, non-atomic snapshots.
- Sizes use allocated blocks (`st_blocks × 512` or equivalent bulk attributes), including directory metadata. Dataless iCloud/File Provider placeholders are counted by logical file length and flagged `dataless`; their contents are never opened. Dataless directories are left unenumerated and marked incomplete to avoid hydration. Sparse files therefore reflect allocated storage. Hardlinks are deduplicated by `(device, inode)` and charged to the first lexical path, deterministically during the parallel walk. A category can inherit those blocks even when another hardlink appears elsewhere.
- Other devices are skipped by default. For macOS `/`, both the sealed System device and firmlinked Data device are included through the normal namespace, while `/System/Volumes/Data` is omitted. Repeated directory identities are skipped to prevent alias loops. `--cross-mounts` does not re-enable the duplicate Data subtree.
- APFS clones, snapshots, compression, purgeable storage and filesystem accounting can make these totals differ from Finder or actual reclaimable space. APFS clone-aware accounting is not implemented. A scan never represents a transactionally consistent snapshot.
- atime may be disabled, coarse, or updated by directory reads; it is only a conservative activity hint. Directory atimes are recorded but excluded from subtree activity for ranking, so scanning does not make an old cache look newly used. File metadata and mtime are not changed by the tool. OS-managed access-time updates can occur from reading directories.
- The private scan cache defaults to `$TMPDIR/oh-my-disk-<uid>/latest.bin` (via Go’s OS temp directory), outside the usual home scan. It survives normal command runs but may be cleared by OS temp housekeeping. `--cache /private/path/state/latest.bin` selects a different cache; its directory must have mode `0700`. The cache is a versioned binary stream with a checksum; legacy JSON caches remain readable when selected explicitly with `--cache`. Only valid application caches may be replaced. Use a dedicated state directory. `--no-cache` performs a fresh scan without loading or saving state.
- The application’s cache path/directory and requested export files are excluded before scanning. New reports are created with mode `0600`; an existing report is never replaced. The cache writer uses a private temporary report and atomic rename. It removes only its own temporary report on completion.
- The server listens on `127.0.0.1`, uses a random per-launch token and SameSite HTTP-only cookie, validates Host, accepts GET/HEAD only, and serves no external assets. No analytics or remote calls. The token URL and JSON reports expose private filenames; share them deliberately.

## Development and verification

```sh
go build ./...
go test ./...
go test -race ./...
go vet ./...
```

Tests cover allocated bytes, sparse files, deterministic hardlinks, non-followed symlinks, denied permissions, exclusions, cancellation, wide worker queues, macOS Data-device policy, built-in matching, custom overrides, conservative project detection, project staleness, ranking, disjointness, category totals, cache reuse, report protection, export pruning and the read-only local server. CI runs tests on macOS and Linux.

Create and scan a realistic synthetic developer home without touching real user data:

```sh
python3 scripts/synthetic.py .fixtures/home
./oh-my-disk scan .fixtures/home --home "$PWD/.fixtures/home" \
  --cache "$PWD/.fixtures/state/latest.bin" --min-size 0 \
  --json .fixtures/scan.json --summary .fixtures/summary.json --limit 8
./oh-my-disk ui --home "$PWD/.fixtures/home" \
  --cache "$PWD/.fixtures/state/latest.bin"
```

The fixture generator refuses to reuse an existing directory. It creates approximately 103 MiB of nonzero cache/model/tool data, 300 package folders, hardlinks, symlink loops and a 2 GiB logical sparse image. `.fixtures/` and build artifacts are gitignored. Automated filesystem tests use fixtures or test temp directories. The performance work also measured a metadata-only real home scan.

## v2

- Diff between scans
- Duplicate finder
- APFS clone-aware accounting
- Generated cleanup scripts (explicit human review; the analyzer stays read-only)

The scanner keeps every entry in a compact indexed tree; it does not collapse file detail. The browser fetches bounded pages through `/api/node?id=…&depth=2` and draws charts on canvas. The folder table loads additional children on demand. JSON exports stream nodes without a second tree or whole-tree JSON buffer. There is one latest cached scan, no live rescan endpoint and no filesystem watcher.


Performance measurements and reproduction commands are in [PERF.md](PERF.md).
The default walk concurrency is twice the CPU count (maximum 64); use `--workers`
to tune for the filesystem. Progress on stderr refreshes about five times per
second through walking, aggregation, classification, cache writing and UI startup.
The CLI sets a 704 MiB Go soft memory limit unless `GOMEMLIMIT` is already set;
this controls heap growth, not a hard RSS ceiling. Every file remains available
in full exports and the explorer.

Generate a metadata-scale fixture separately from the realistic small fixture:

```sh
python3 scripts/synthetic.py .fixtures/large --large
# roughly two million entries: add --projects 2000
```
