# oh-my-disk

<img src="build/appicon.png" width="96" height="96" alt="oh-my-disk Prism storage-block icon" />

A local disk-space analyzer with read-only scanning and explicit Move to Trash for developers. Find the caches, SDKs, models, build outputs and forgotten downloads filling your home folder. Written in Go for macOS Apple Silicon and Linux, with a native Wails macOS desktop app and a DaisyDisk-inspired explorer shared with the CLI.

Scanning reads filesystem metadata. In the desktop app, **Move to Trash** moves selected items after you review the paths and confirm. Suggested cleanup commands are shown as text to review. A few that are a tool's own clean command, like `pnpm store prune`, can also be run with a button, with their output in a floating terminal.

## Install

Download the universal macOS DMG or the CLI for macOS or Linux (ARM64 or AMD64) from the [Nomadware releases catalogue](https://releases.nomadware.io/#oh-my-disk) or [GitHub Releases](https://github.com/fum4/oh-my-disk/releases/latest). The catalogue also provides a [CLI installer script](https://releases.nomadware.io/oh-my-disk/install.sh).

The macOS app is ad-hoc signed. Gatekeeper may block the downloaded app while Developer ID signing and notarization are deferred.

### Build the CLI from source

Requires Go 1.26 or newer (the current Wails v2.17.1 requirement). Go can download the toolchain automatically. The CLI needs no Node, frontend build, CDN or database.

```sh
git clone https://github.com/fum4/oh-my-disk.git
cd oh-my-disk
go build -o oh-my-disk .
./oh-my-disk --help
```

Or install with `go install github.com/fum4/oh-my-disk@latest`. To cross-compile:

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

Every real item (including the current folder and cleanup suggestions) has **Reveal in
Finder** and **Copy path** actions. Suggested commands can be copied as text.
**Export JSON** writes the full tree; **Export LLM summary** writes a bounded
summary with up to 50 top entries and 50 cleanup suggestions. Both use native save dialogs and refuse to overwrite an
existing file, even if the save dialog offers Replace. Choose a new filename.
Each cleanup suggestion's details also have **Open in Folders**, which jumps to that folder in the explorer with its breadcrumbs; a file opens its folder with its row marked. Select folder table rows or Cleanup suggestions with checkboxes. The sticky selection bar shows count and total size; **Move to Trash** opens a dialog listing every path and the total. Nothing moves until you confirm. Per-item failures remain visible. Successful moves trigger a fresh scan to update totals.

The app probes protected `~/Library` folders. If macOS denies access, a banner
links to **System Settings → Privacy & Security → Full Disk Access**. Enable the
built `.app`, quit and reopen it, then rescan. This is a best-effort permission
probe, not a guarantee that every folder is readable. The app never changes
permissions. The CLI's Terminal permission is separate from the app's permission.

Sizes default to decimal **MB/GB**, matching Finder (1 GB = 1,000,000,000 bytes).
The UI's optional **Binary units (GiB)** setting (off at each launch) and CLI `--binary` flag opt
into powers of 1024. JSON always contains integer bytes; input suffixes such as
`--min-size 1MiB` keep their explicit meaning.

The **Whole disk · containing volume** panel and CLI header use `statfs`:
capacity = blocks × block size, available = available blocks × block size,
used = capacity − available, and outside scanned folder = max(0, used − scanned
allocation). This residual includes System, Applications, other users, unreadable paths, unscanned folders,
snapshots and filesystem overhead. It is an accounting remainder rather than
an inventory of those files. APFS shared containers, purgeable space, clones
and cloud placeholders can differ from About This Mac; when allocation exceeds
used, the UI reports the excess rather than showing a negative remainder.
Whole-disk scans retain the engine's Data/firmlink deduplication and do not cross
unrelated mounted volumes by default.

Build prerequisites: macOS, Xcode command-line tools, Go 1.26+, and Node/npm for
Wails tooling (there is no npm install or frontend compilation for this app).
`make` installs the pinned Wails CLI and keeps Go caches/tools under `.build/`.
Each `make app` or `make app-universal` first deletes the previous `build/bin/`, so only the latest app is kept.
After installing Wails yourself, the direct equivalent is:

```sh
make icon
cd app
../.build/bin/wails build -platform darwin/arm64
```

The Make wrapper falls back to public `sw_vers` OS-version values if a build
sandbox blocks Wails' two `sysctl` version probes. The **Prism** storage-block icon
uses violet, turquoise, orange, blue, pink, lime and yellow tiles on a dark purple
background. It is generated by `scripts/icon.go` at 1024×1024, resized with `sips`,
and packaged with
`iconutil`. On hosts where `iconutil` rejects the iconset, a standard PNG-based
ICNS packer is used. A Wails post-build hook installs that ICNS and renews the
ad-hoc signature. `make icon` also updates `internal/web/assets/brand-icon.png`,
used by the browser favicon and touch icon.
No network artwork is used. The macOS plist uses the stable
bundle ID `com.fum4.oh-my-disk`. Local builds are ad-hoc signed.

See [desktop verification notes](app/VERIFICATION.md) for the checks and host
limitations recorded during implementation.

## Start here

```sh
./oh-my-disk scan                      # home directory, progress on stderr
./oh-my-disk top --limit 25            # largest entries in the latest scan
./oh-my-disk suggest --limit 20        # ranked, disjoint cleanup suggestions
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

Web mode also has a **Scan** button and an absolute root field; leaving the field empty rescans the displayed root. The UI has a clickable, zoomable sunburst, a squarified treemap toggle, breadcrumbs, a sortable child table, category breakdowns and ranked cleanup suggestions. Click a segment or folder to drill in; click the sunburst center to go back. Small segments and entries beyond the chart’s display budget remain in the paginated table. Category drill-down attributes each entry’s own allocated blocks, so it does not count parent totals a second time.

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

`--depth` and `--min-size` prune the exported tree without changing its aggregate totals or the full cached scan. Defaults: unlimited depth and 1 MiB minimum for tree entries and cleanup suggestions. Use `--min-size 0` for a full file-level export. `top` includes overlapping parent/child totals and explicitly labels them. `suggest` outputs only disjoint cleanup suggestions. `explain` also resolves inherited rules for files inside known locations. `ui --json` outputs connection information, not the tree.

`--summary FILE` emits compact, bounded JSON with the root, scan time, totals, category totals, top N entries, top N disjoint suggestions and an accounting note. The sample fixture’s eight-item summary is about 7 KB. Paths and explanations affect length; lower `--limit` to fit a smaller context. Summary generation never needs an LLM, API key or network connection.

## MCP server

`./oh-my-disk mcp` runs a stdio MCP server inside the CLI binary using the [official Go SDK](https://github.com/modelcontextprotocol/go-sdk). It exposes only six read-only tools: `scan(root, refresh)`, `summary(limit)`, `top(path, depth, limit)`, `suggest(sort, limit, category, safety)`, `explain(path)`, and `whole_disk`. **No Trash/deletion tool is registered.** Suggested command strings are informational text only.

Call `scan` with an absolute root before querying a new folder; `refresh: true` forces a fresh metadata walk. An existing private cache can be queried without rescanning. Scans in an MCP session stay in memory and do not replace the desktop/CLI cache. Results are compact JSON, with limits of 100 items, top depth 8 and 64 KiB per response; oversized responses return an error asking for a narrower request. Stdout contains protocol messages only. The client process’s OS permissions determine readable folders.

For [Claude Desktop](https://modelcontextprotocol.io/docs/develop/connect-local-servers), open its Developer configuration (`~/Library/Application Support/Claude/claude_desktop_config.json` on macOS). For [Cursor](https://docs.cursor.com/context/model-context-protocol), use `.cursor/mcp.json` in the project or `~/.cursor/mcp.json` globally. Merge this entry with existing servers and replace the binary path:

```json
{
  "mcpServers": {
    "oh-my-disk": {
      "command": "/absolute/path/to/oh-my-disk",
      "args": ["mcp"]
    }
  }
}
```

For [Codex](https://developers.openai.com/codex/config-reference), add to `~/.codex/config.toml` (or the trusted project config):

```toml
[mcp_servers.oh-my-disk]
command = "/absolute/path/to/oh-my-disk"
args = ["mcp"]
```

Restart or reload the client’s MCP connection after saving. Example request: “Scan /Users/me/Projects, then show 10 cleanup suggestions sorted by stale days.”

## Ranking and rules

The embedded YAML lives in [internal/rules/rules.yaml](internal/rules/rules.yaml). It covers npm, Yarn, pnpm, Bun, pip/uv, Go module/build caches, Gradle, Android SDK/images/AVDs, Xcode DerivedData/Archives/DeviceSupport/caches, simulators and runtimes, CocoaPods, Carthage, SwiftPM, Homebrew, Docker Desktop, Hugging Face, LM Studio, Ollama, generic macOS/Linux caches, Expo, browser test downloads, AI editor caches/state, project artifacts, Trash, Downloads, media, Mail/Messages and iOS backups. Unknown large branches appear as **“Big, check yourself.”**

The raw ranking score (JSON `raw_score`) is:

```text
allocated_bytes × safety_weight × (1 + min(stale_days / 180, 1))

cache:         1.00  (regenerable; still review first)
reinstallable: 0.55  (SDKs, downloaded models, applications)
user-data:     0.12  (archives, backups, Docker volumes, sessions, downloads)
unknown:       0.04  (inspect yourself)
```

The displayed **cleanup score** (JSON `score`) is `100 × raw_score / (raw_score + 1,000,000,000)`, rounded to an integer in the UI/CLI. One weighted decimal GB scores 50; nine score 90. This fixed, monotonic 0–100 scale stays comparable across scans and filters; 100 is the limiting value. It expresses cleanup priority, not confidence or guaranteed recoverable space. JSON retains full precision. A one-line badge gives safety, stale days and decimal GB.

Cleanup suggestions default to **Score**, with **Size** and **Time since last activity** sorts available in the segmented control and column headers (click again to reverse). Category and safety filters apply before limiting results. CLI equivalents:

```sh
./oh-my-disk suggest --sort score
./oh-my-disk suggest --sort size --category caches --safety cache --limit 30
./oh-my-disk suggest --sort stale
```

**Effort is never part of the score.** Staleness uses the later of latest subtree mtime/regular-file atime and clamps future timestamps to zero days. Project artifacts use the enclosing project’s latest subtree mtime, so old `target` files in an active project are not treated as an abandoned project. Projects are recognized by markers including `package.json`, `Cargo.toml`, `pyproject.toml`, `go.mod`, Gradle/Maven/Swift/CMake manifests and `.git`. Generic `build`, `dist`, `target`, `.venv`, `.next` and `.turbo` names outside a recognized project are not assumed safe.

More specific locations take precedence over enclosing aggregates: Hugging Face models are not mislabeled as expendable generic `.cache` contents. The final score pass selects disjoint paths. Category totals attribute each entry’s own blocks to its nearest matching rule and sum to the scan total. Ranking measures candidates, not a promise that every allocated byte can be freed. **Safe clean** counts every complete `cache` and `reinstallable` suggestion outside personal folders; `user-data` such as Docker disks and unrecognized folders appear for review but never count, so removing them leaves that total unchanged. Its note shows how much more can be cleaned after review, with a link to those items.

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

## Safety, privacy and accounting

- Scanning is read-only. Deletion requires an explicit UI confirmation or `oh-my-disk trash /absolute/path ...` with `--yes` or typing `yes` at the interactive prompt. macOS uses `NSFileManager.trashItemAtURL`, so items remain restorable in Finder’s Trash. There is no permanent deletion fallback; non-macOS builds refuse Trash operations.
- Every selected path must still exist in the latest scan with the same device/inode. Symlinks and paths containing symlink components are rejected. Protected paths include `/`, `/System`, `/Library`, `/Applications`, `/usr`, `/bin`, `/sbin`, `/private`, the home and `~/Library` folders themselves, Documents, Desktop, Photos Library bundles, Mobile Documents (iCloud), CloudStorage, and anything outside the scan root. Selecting a directory containing protected data is also rejected. The scan root itself cannot be trashed. `/private/var/folders` permits only current-user-owned caches matched by a cache rule. Scan identities survive the private binary cache; legacy JSON exports cannot authorize Trash without a fresh scan.
- Trash attempts append JSON records to `~/Library/Logs/oh-my-disk/trash.log`, including failures. A durable intent record is required before moving an item. The CLI invalidates its cache after successful moves; UI sessions rescan and recompute tree/category totals. Revalidation occurs immediately before each native move; concurrent filesystem activity can still race any path-based OS operation.
- Scanner operations are metadata reads (macOS bulk attributes, or directory reads plus descriptor-relative `fstatat`); it never opens regular file contents. Symlinks are recorded but not traversed, including loop/broken links. A symlink root is rejected. Directory opens are pinned to the root descriptor, reject symlinks in every relative path component, and verify inode identity; changing trees can still produce partial, non-atomic snapshots.
- Sizes use allocated blocks (`st_blocks × 512` or equivalent bulk attributes), including directory metadata. Dataless iCloud/File Provider placeholders are counted by logical file length and flagged `dataless`; their contents are never opened. Dataless directories are left unenumerated and marked incomplete to avoid hydration. Sparse files therefore reflect allocated storage. Hardlinks are deduplicated by `(device, inode)` and charged to the first lexical path, deterministically during the parallel walk. A category can inherit those blocks even when another hardlink appears elsewhere.
- Other devices are skipped by default. For macOS `/`, both the sealed System device and firmlinked Data device are included through the normal namespace, while `/System/Volumes/Data` is omitted. Repeated directory identities are skipped to prevent alias loops. `--cross-mounts` does not re-enable the duplicate Data subtree.
- APFS clones, snapshots, compression, purgeable storage and filesystem accounting can make these totals differ from Finder or actual reclaimable space. APFS clone-aware accounting is not implemented. A scan never represents a transactionally consistent snapshot.
- atime may be disabled, coarse, or updated by directory reads; it is only a conservative activity hint. Directory atimes are recorded but excluded from subtree activity for ranking, so scanning does not make an old cache look newly used. File metadata and mtime are not changed by the tool. OS-managed access-time updates can occur from reading directories.
- The private scan cache defaults to `$TMPDIR/oh-my-disk-<uid>/latest.bin` (via Go’s OS temp directory), outside the usual home scan. It survives normal command runs but may be cleared by OS temp housekeeping. `--cache /private/path/state/latest.bin` selects a different cache; its directory must have mode `0700`. The cache is a versioned binary stream with a checksum. Persisting device/inode adds 16 bytes per scanned entry (80-byte compact entries). The new identity-bearing format requires a fresh scan (`--refresh`) after upgrading from an older binary cache; legacy JSON caches remain readable when selected explicitly with `--cache`. Only valid application caches may be replaced. Use a dedicated state directory. `--no-cache` performs a fresh scan without loading or saving state.
- The application’s cache path/directory and requested export files are excluded before scanning. New reports are created with mode `0600`; an existing report is never replaced. The cache writer uses a private temporary report and atomic rename. It removes only its own temporary report on completion.
- The server listens on `127.0.0.1`, uses a random per-launch token and SameSite HTTP-only cookie, validates Host, uses GET/HEAD for reads and token-authenticated POST for explicit rescan/Trash actions, with strict same-origin JSON headers and Origin checks, and serves no external assets. No analytics or remote calls. The token URL and JSON reports expose private filenames; share them deliberately.

## Development and verification

```sh
go build ./...
go test ./...
go test -race ./...
go vet ./...
```

Tests cover allocated bytes, sparse files, deterministic hardlinks, non-followed symlinks, denied permissions, exclusions, cancellation, wide worker queues, macOS Data-device policy, built-in matching, custom overrides, conservative project detection, project staleness, ranking, disjointness, category totals, cache reuse, report protection, export pruning and the token-protected local server, CSRF rejection, protected Trash paths, inode revalidation, audit records and an MCP client driving the binary over stdio. CI runs tests on macOS and Linux.

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

### Chat about a scan in the desktop app

The welcome screen has one command bar: **Folder**, **Analyze with** (None, Claude or Codex), and **Scan ⌘R**. The folder and agent choices are remembered locally. A successful scan with an agent selected opens the dedicated **Agents** page; cancelled or failed scans never launch an agent. After scanning, Overview offers one quiet **Ask <agent> about this scan →** link. Results use decimal units and one toolbar row, with location and exports on the left and **Rescan** on the right.

**Agents** is always reachable below Cleanup. It contains a full-height xterm.js terminal, an agent picker, session status, **Restart session**, and **Stop**. One session runs at a time and persists across page navigation. Switching agents or restarting a running conversation asks first. The sidebar dot indicates a running session. Before a scan, the page offers the shared scan command bar and **Start without a scan**. Stop ends the process group and removes the private session workspace. Selection, desktop copy/paste shortcuts, terminal colors, scrollback and HTTP(S) links are supported. Terminal assets are bundled locally, including xterm.js 5.5.0, fit 0.10.0 and web-links 0.11.0 (MIT).

Install [Claude Code](https://code.claude.com/docs/en/setup) or [Codex CLI](https://developers.openai.com/codex/cli), sign in using the CLI, then reopen the picker to detect it. Missing agents appear as install links. Browser UI offers install links; agent sessions run in the desktop app, with no PTY HTTP/WebSocket endpoint.

Agent model defaults live as constants in `internal/agents/agents.go`: Codex uses `-m gpt-6.1-sol -c model_reasoning_effort=medium`; Claude uses `--model opus` (the latest Opus alias). Override them with `OMD_CODEX_MODEL` or `OMD_CLAUDE_MODEL` in the app’s environment. The app never edits `~/.codex` or `~/.claude` configuration.

Detection checks executable files on PATH, `~/.local/bin`, Homebrew, `/usr/local/bin`, common npm global locations, mise shims, and Codex inside `/Applications/ChatGPT.app/Contents/Resources/codex-cli/CodexCLI.app/Contents/MacOS/codex`. A bounded `$SHELL -lc 'command -v claude'` / `command -v codex` lookup also resolves installations exposed by login-shell PATH setup. The terminal environment adds the resolved CLI directory and standard binary paths for GUI launches.

To add an agent, add one `Definition` in `internal/agents/agents.go` with its name, label, icon, install URL, executable command, optional fallback path, and argv builder. Detection, both pickers, and launch dispatch use this registry.

`make app` bundles `oh-my-disk-cli` alongside the desktop executable. Each launch creates a private temporary workspace containing a copy of the displayed scan (when available), MCP JSON, and AGENTS.md / CLAUDE.md context. MCP receives `OMD_SESSION_SCAN` pointing to that copy, so it does not accidentally analyze another window's cached scan. No user MCP configuration is edited. The reusable advisor context lives at `internal/agents/context.md`; it can later be packaged as a skill. Session working directory is deliberately the private workspace, avoiding scanned-folder instructions and hooks. Agent authentication and normal conversation persistence remain managed by the installed CLI.

Launch argv (paths below are placeholders; these are arguments passed directly, without a shell):

```sh
claude --model opus --mcp-config <session>/mcp.json --strict-mcp-config --tools '' \
  --append-system-prompt '<contents of internal/agents/context.md>' -- \
  "<agents.InitialPrompt>"

codex -m gpt-6.1-sol -c model_reasoning_effort=medium --no-daemon --sandbox read-only -c 'mcp_servers={}' \
  -c 'mcp_servers.oh-my-disk.command="<app>/Contents/MacOS/oh-my-disk-cli"' \
  -c 'mcp_servers.oh-my-disk.args=["mcp"]' \
  -c 'mcp_servers.oh-my-disk.env.OMD_SESSION_SCAN="<session>/scan.bin"' \
  -c 'developer_instructions="<escaped advisor context>"' -- \
  "<agents.InitialPrompt>"
```

Flags were verified against the installed CLIs' `--help`. Claude's built-in tools are disabled and its MCP list is isolated. Codex uses its read-only sandbox and replaces the session MCP table before configuring the six read-only disk tools. The advisor never deletes or runs cleanup commands, keeps personal/iCloud/Photos/Music/Documents data off-limits, and tells users to review candidates in the app and use **Move to Trash**. These rules guide recommendations; existing app Trash checks and confirmation remain unchanged. Starting an agent sends the metadata it requests to your configured AI provider under that CLI's account and settings; ordinary scanning remains local. No terminal content or credentials are logged by the app.

Cleanup choices can be remembered locally. **Remember this choice** is on by default in the Trash confirmation. Only successfully trashed paths are saved, with their allocated size, kind, rule and cleanup time, in versioned JSON at `os.UserConfigDir()/oh-my-disk/remembered.json` (on macOS: `~/Library/Application Support/oh-my-disk/remembered.json`). The file is private and replaced atomically. Nothing is uploaded.

A later scan matches exact paths against its full scan index. Returning items appear first in Cleanup, selected by default, with current size and growth. A warning marks growth above twice the previous size or more than 1 GB. **Clean again** uses the same confirmation and filesystem validation as other cleanup actions. **Manage remembered** includes paths that have not returned; **Forget** and **Forget all** remove choices without touching the files.

Round 14 fixture checks: `scripts/ui-remembered-smoke.cjs` covers returning choices and root/depth-2/category/synthetic sorting using jsdom. `scripts/ui-round14-smoke.cjs` adds browser geometry checks for column alignment, the stationary accounting disclosure and the responsive Agent command bar. Both accept the private fixture UI URL; neither confirms Trash.

## Releases

Every push to `master` (normally a PR merge) runs the release workflow. Version allocation starts at `v1.0.0` and increments the stable patch version for each new release commit. A retry
of the same tagged commit reuses its version and draft assets rather than replacing
published bytes. `workflow_dispatch` on `master` can resume a failed release.

Releases include a universal macOS DMG and CLI binaries for macOS/Linux,
AMD64/ARM64. macOS CLI binaries preserve native Trash support. Windows is not
supported yet. The app bundle, embedded CLI, and standalone CLIs carry the same
release version (`oh-my-disk --version`).

CI runs on pull requests and manual dispatch only, never again on the merge push.
It includes release-version and trusted-retry regression tests, release shell lint,
all four CLI release builds and universal DMG verification. CI artifact metadata
uses the initial v1.0.0 as a fixture; production numbering remains tag-driven.
Active workflows
live in `.github/workflows/`; no activation copy step is needed.

The release workflow tests both platforms, creates the DMG, and prepares a draft
GitHub Release. One full-SHA checkout of `fum4/releases` into `.release-publisher`
supplies both the local publishing action and its trusted `lib/common.sh` signature
verifier, public key, platform list and public base URL. The pin is the reviewed
merge revision of publisher PR #11; update this single workflow pin deliberately.

Retries reuse the draft’s original five assets (DMGs are not reproducible).
Before treating a publish as completed, and again before exposing GitHub downloads,
verification requires a trusted signature, exact manifest coverage of four CLIs,
the universal DMG and `RELEASE`, the requested package/tag binding, and a full
read-back of every remote asset matching the draft bytes. Missing/extra entries,
invalid signatures, mismatches and network errors fail closed; only a 404 for
`latest` means a first publish. No uploaded artifact supplies a verification marker.

When latest is the same or newer, verify the immutable historical version and
skip publishing it (never roll latest backwards). A pinned mise setup action
installs rclone from the publisher’s existing toolchain definition, then its
`bin/publish --index-only` repairs the catalogue using the publishing secrets.
Older/missing latest resumes normal
publication. A historical GitHub retry is not marked latest. Only after verification
and a successful catalogue update does the GitHub Release become public.
The catalogue serves the CLI installer at [install.sh](https://releases.nomadware.io/oh-my-disk/install.sh).

Provision `RELEASES_SIGNING_KEY`, `R2_RELEASES_ACCESS_KEY_ID`, and
`R2_RELEASES_SECRET_ACCESS_KEY` from the releases repo using its secure
`secrets:gh-push` task. These sign download checksums, not Apple code signatures.
Apple Developer ID signing/notarization is explicitly deferred in `ledger/backlog.md`;
DMGs currently contain an ad-hoc-signed app and may be blocked by Gatekeeper.
