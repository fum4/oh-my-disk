# Shared Calm UI verification

Wails and `oh-my-disk ui` embed the same `assets/` directory. Calm's tokens, type, ring geometry, spacing, cards, folder rows, cleanup rows, badges, and buttons come from the approved `.build/design/03-calm-v2.html`. All icons are locally inlined Phosphor SVG symbols (MIT license in `assets/Phosphor-LICENSE.txt`). System fonts take precedence on macOS; bundled Inter and JetBrains Mono remain available offline. There are no runtime network dependencies.

## Page coverage

- **Overview:** live whole-filesystem ring, free-space center, percent-full status, four-part legend, top seven folders plus Everything else, reclaim panel and before/after bar, seven cleanup suggestions, sort segments, and shared selection/Trash controls.
- **Folders:** breadcrumbs, icon and share-bar rows, share percentages, size/files/modified sorting, 200-entry lazy loading through `/api/node`, selection, and both interactive canvas maps.
- **Categories:** the same browser and maps, with lazy category details through `/api/category`.
- **Cleanup:** the same compact cleanup row component, filters, sort and reverse order, expandable explanations/path/project activity/commands/native actions, and 100-row display increments up to the existing API's 500-match cap.
- **Shared:** sidebar locations/counts/disk bar, folder chooser, Scan Again and Command-R/Control-R, cached/native welcome, progress and cancel, skeleton treatment, recovery messages, Full Disk Access, exports, accounting notes, dark appearance, reduced motion, focus rings, and arrow/Home/End row navigation. Native row actions appear on hover or keyboard focus; cleanup details expand with a real button.

The shared UI preserves scan/cancel, lazy APIs, blocklist, Trash revalidation, MCP, and CLI features. The polish pass adds personal-content metadata and conservative ranking; dataless files now use local allocation and old binary caches require a rescan. Checkbox state is synchronized across views; overlapping ancestor/descendant selections are still rejected. Trash always requires a confirmation dialog, with server checks unchanged.

## Intentional differences from the static mockup

- Values and filenames are live, with no preselected cleanup items or simulated hover/focus. Folder descriptions refer to the scanned folder because scanning outside Home is supported. The toolbar shows actual scan age and duration; its tooltip contains the full root and timestamp.
- The sidebar uses an application label in place of decorative traffic lights; native window controls remain the responsibility of Wails.
- Secondary scan roots, cache opening, unit preference, and exports live in **Scan options and exports**. Canvas maps remain in Folders/Categories to preserve their existing functionality.
- Longer cleanup details expand below the compact row. Folder/category pages keep file counts and modified dates. Desktop windows retain two compact Overview columns with independent list scrolling. Windows at 700px or below stack cards; short desktop windows (700px tall or below) allow page scrolling to keep all controls reachable.
- Progress is indeterminate, with actual phase and entry/folder counts in Wails. The existing scanner API supplies no reliable total or percentage. Browser rescans expose an indeterminate busy state while their synchronous API request completes.
- Snapshot suggestions exclude personal files from Best and retain the existing 100-item cap: the reclaim total comes from the server, covers every suggestion rather than the listed 100, and includes only complete cache/reinstallable candidates; its note states how much isn't counted. Ring segments are clamped to the live disk's used space when APFS allocations exceed it; the before/after estimate is capped at capacity. Accounting notes explain that allocated bytes are not guaranteed reclaimed bytes.

## Reproduce

Build and start against the synthetic home (generate it with `scripts/synthetic.py` if needed):

```sh
make cli
./oh-my-disk ui --root "$PWD/.fixtures/home" --home "$PWD/.fixtures/home" \
  --cache "$PWD/.fixtures/state/latest.bin" --refresh --min-size 0 --no-open --json
```

Set `OMD_UI_URL` to the private URL printed by that process, then run:

```sh
npm install --prefix .build/browser jsdom playwright
export NODE_PATH="$PWD/.build/browser/node_modules"
node scripts/ui-smoke.cjs "$OMD_UI_URL"
node scripts/ui-native-smoke.cjs "$OMD_UI_URL"
node scripts/ui-native-smoke.cjs "$OMD_UI_URL" --empty
node scripts/ui-capture.cjs "$OMD_UI_URL"
```

The web smoke checks the real fixture APIs, bundled icons/fonts, ring and decimal values, folder/category navigation, global sorting, treemap, cleanup filters, row expansion, keyboard row navigation, selection across pages, confirmation/cancel, binary units, accounting notes, rescan, and Command-R. It only rescans `.fixtures/home` and never confirms Trash. The native checks stub bridge methods and verify cache opening, chosen root preservation, the native picker, progress/cancel/completion, Reveal, copy path/command, exports, protected-folder access, and the old-cache rescan action. `--empty` exercises startup without a cached scan. Canvas is stubbed in DOM checks.

For large-list verification, start another UI against `.fixtures/large` with a separate cache, then run:

```sh
node scripts/ui-smoke.cjs "$OMD_LARGE_UI_URL" --large
```

This checks folder and category pagination from 200 to 400 entries, global sort resetting the page, and treemap creation. It sends no mutation requests.

The capture script uses installed Google Chrome by default (`OMD_CHROME` can select another Chromium executable). It targets 1440×900, all four pages, treemap, Trash confirmation, all pages at 900px width with overflow assertions, dark Overview, and a 390px narrow-window fallback. Files go into `.build/screens/`. Tokenized URLs remain private.

## Validation for Calm (2026-11-02)

- `go test ./...` and `go vet ./...` through `make check`: passed.
- `make cli`, `make app`, and `codesign --verify --deep --strict build/bin/oh-my-disk.app`: passed.
- Updated browser and native bridge smoke checks, including the separate empty-startup check: passed, with no script errors or Trash requests.
- 953,002-entry fixture: folder/category pagination, sort reset, and treemap passed, without mutations.
- Screenshot capture: blocked at Chromium startup by the execution sandbox (`SIGABRT`; attempted process cleanup reports `EPERM`). The Browser skill fallback also failed before connection because its installed runtime references a missing `browser-service.mjs` file. No screenshot or actual layout assertions could run. Pixel fidelity, dark-mode rendering, and rendered overflow remain unverified; the DOM checks do not establish these.

Logs are `.build/calm-{check,cli,app,codesign,web-smoke,native-smoke,empty-smoke,large-smoke,capture}.log`. `.build/screens/overview.dom.html` records the web smoke's final DOM. These are ignored artifacts.

Commits are on branch `calm`, based on `5df2820`, in `.build/calm-git`; `.build/calm.bundle` contains the branch. The primary `.git` was not written.


## Polish verification (2026-11-02)

See [POLISH_REPORT.md](POLISH_REPORT.md) for all thirteen feedback items, extra sweep fixes, scoring examples, cache migration, and validation results.

- `make check` (`go vet ./...`, `go test ./...`), `make cli`, `make app`, and strict/deep signature verification: passed.
- Updated web, native bridge, empty-start, and large-list smoke checks: passed. Large fixture: 4,053 entries, 450 projects; folder/category pagination from 200 to 400, global sort reset, and treemap. No Trash action confirmed.
- New DOM assertions check Open Settings, Best, the chart-local segmented control, ring tooltip label/size/percentage, rule/score details, icon copy action and Copied feedback.
- `ui-capture.cjs` now defines a light/dark × 1440/900/390px matrix across every page, maps, details, confirmation, access/options, and presentation fixtures for scanning/empty/error. It asserts cursor coverage, horizontal/page overflow, footer separation and stability when lists scroll. The state fixtures exercise rendering only; native bridge smoke separately drives real UI scan/cancel/completion and error transitions with stubbed native methods.
- Rendered matrix execution remains blocked before page load: sandboxed Chromium aborts (`SIGABRT`, crashpad permission errors), browser access host is unavailable, and native Chrome fallback was denied because Computer Use was not approved for Chrome. These checks are prepared, **not passed**. Actual rendered light/dark layout, window geometry, and native hit-target appearance still need a permitted browser/native run. Static CSS/DOM inspection is not a substitute for pixel QA.
- Logs: `.build/polish-{check,cli,app,codesign,web-smoke,native-smoke,empty-smoke,large-smoke,capture}.log`.
- Delivery: branch `polish`, base `4d846c1`, side Git directory `.build/polish-git`, bundle `.build/polish.bundle`. Primary `.git` is untouched; icon.icns is excluded.

## Embedded agent terminal verification (2026-11-02)

- Three equal welcome options, persistent post-scan Ask buttons, missing-agent install links, Check Again, and remembered dismissal. Browser mode disables agent actions and says “Available in the desktop app.”
- Desktop terminal uses local xterm.js plus fit/web-links addons and a Go PTY through Wails bindings. No network terminal service. Drawer resize is pointer/keyboard accessible; theme uses Calm light/dark tokens. Closing terminates the process group; app shutdown also closes it. One session, private workspace rather than scanned-root cwd; optional row context actions are deferred.
- `go test ./...`: detection including GUI fallback, command/config escaping, private MCP snapshot selection, and real PTY I/O/resize/close tests added. Existing tests retained.
- Existing web, native bridge, and native empty-start smoke scripts pass. `scripts/ui-agents-smoke.cjs <fixture-url>` adds missing CLI/check/dismiss, successful scan handoff, cancelled scan, terminal close/reopen checks. Its terminal/CLI are mocked; it uses real fixture APIs and local UI sources.
- `go vet ./...`, `go test ./...`, `make app`, deep/strict codesign verification, and `go test -race ./app ./internal/agents`: passed (see `.build/agents-*.log`). Desktop bundle includes the CLI MCP helper.
- Rendered QA remains blocked: Chrome/Playwright aborts during startup under this sandbox; the computer-use in-app browser reports “browser access host is unavailable.” Light/dark pixel rendering, actual WebKit terminal interactions, and authenticated agent conversations were not verified. The automated PTY test uses `/bin/sh` without calling a provider; no account/auth settings are changed.
- Side git branch `agents` is stored in `.build/agents-git` with bundle `.build/agents.bundle`, based on the original v1 HEAD. No icon changes are included; `make app` regenerates the build icon.

## Round 4: merged scan and agent controls (2026-11-02)

This supersedes the three welcome cards and separate Ask buttons described above. One **Scan my disk** card contains **Scan** and an **Analyze with** dropdown. None is the initial choice; installed agents are selectable, missing agents are install links. The choice persists locally and falls back to None if that CLI is unavailable. The missing-agent nudge appears only on the first eligible visit, remains stable during that visit, and can be dismissed. Check again lives inside the picker.

The same control moves beside the cleanup results on Overview/Cleanup, and into the toolbar on Folders/Categories. **Ask agent** is hidden until a snapshot has actually loaded: a native `hasScan` flag alone does not reveal it. With None selected, Ask opens the picker; choosing an installed agent and pressing Ask starts the conversation. Scan-only works without any agent. Cancelled/failed scans never hand off. Agent options are not rebuilt during native status polling, preserving keyboard focus.

### Action hierarchy by state

| State / page | Focus and next step | Secondary / hidden controls |
| --- | --- | --- |
| Before any scan, every page | One merged Scan card; Scan is the only filled button. | Analyze with is optional; Choose Folder is a quiet link and Options holds details. Ask agent and result statistics are hidden. |
| Saved scan available, unopened | Same Scan card until a valid snapshot is opened. | Open last scan lives in Options. No Ask action based solely on cache availability. |
| Scanning / opening saved scan | Progress and current phase; Cancel when cancellable. | Results, Scan card, toolbar actions, terminal and footer are hidden; sidebar is quiet. No competing filled button. |
| Overview after scan | “You could get back X” and top cleanup candidates lead in both DOM and visual order. Select candidates, then Move to Trash. | Disk/folder panels follow. Ask agent sits just below the opportunity; rescan is secondary. Overview hides sorting tools, redundant before/after figures and sidebar counts. |
| Cleanup after scan | Review/filter candidates and select items. Move to Trash is the sole filled action once selected. | Ask agent stays near the cleanup opportunity; filters and sorting serve review. Explanation/path/commands expand on demand. |
| Folders / Categories after scan | Explore the map/table and select items. Move to Trash is the sole filled action once selected. | Ask agent and rescan are secondary toolbar controls; metadata/actions retain hover/expand behavior. |
| Trash confirmation | Confirm Move to Trash in the modal. | Cancel is secondary; the background is modal. |
| After Trash | Concise amount moved, then review more or scan again. Per-path results expand under Details. | Empty selection bar is hidden. A failed action retains the selection action if items are still selected. Rescan remains secondary. |
| Cancel / scan failure | Restore the prior results, or the one Scan card if no snapshot exists. | A concise error explains recovery; no agent handoff. |
| Agent conversation | Terminal conversation and Close. | No new filled action; existing cleanup controls remain secondary to the active conversation. |

Trash confirmation reports allocated bytes **moved to Trash**, not a claim that disk space was freed: recoverable files may still occupy disk space. Selection is cleared after refreshing the snapshot. No tests move real fixture files to Trash.

### Registry and verification

`internal/agents/agents.go` is the only agent registration point: name, label, icon, install URL, detection command/fallback, and launch argv builder. Native Agents returns those definitions with detection paths; `/api/agents` exposes presentation metadata for the browser, without paths or command arguments. A new agent requires one definition, with no frontend card, detection allowlist, or launch switch to update. Go tests cover registry completeness/uniqueness, copy isolation, presentation JSON, detection/launch dispatch, unknown names, and an additional synthetic entry with a distinct executable name.

- `make check` (`go vet ./...`, `go test ./...`), `make cli`, `make app`, and strict/deep signature verification pass.
- Web, native saved-scan, native empty-start, agent-flow, remembered-choice and missing-remembered-choice smoke checks pass. They cover pre-scan visibility on every page, a single card, installed/missing options, persistence/fallback, once-only nudge, post-scan placement, scan-only, successful handoff, cancellation, terminal close/reopen, and a mocked Trash response with collapsed details and cleared selection. Provider CLIs and native bridge/terminal are mocked; fixture read/rescan APIs are real. The Trash response is intercepted before any network mutation.
- Rendered verification remains blocked: Chrome aborts before page load with crashpad/bootstrap sandbox permission errors; the computer-use in-app browser reports “browser access host is unavailable.” Actual light/dark pixels, responsive geometry and native WebKit terminal behavior remain unverified. The DOM checks do not establish visual fidelity.
- Logs: `.build/round4-{check,cli,app,codesign,web-smoke,native-smoke,empty-smoke,agents-smoke,remembered-smoke,missing-smoke,capture}.log`.
- Delivery: branch `round4`, based on v1 HEAD `3fc76b5`, in `.build/round4-git`; bundle `.build/round4.bundle`. Primary `.git` is not written; generated icon artifacts are excluded from the commit.

## Round 7, job G: command bar (variant C)

The landing card is now the heading and copy from `.build/design/scan-card-C.html`, followed by one 56px command bar with 44px segments: **Folder Home**, **Analyze with None**, **Scan ⌘R**. The bar uses the reference padding/radii, Calm theme tokens, hairline dividers, stationary text/icons on hover, and visible keyboard focus. Only Scan is primary. Folder names truncate without moving the bar beyond the available space; compact widths hide segment labels and keep menus within the viewport.

The Folder menu mirrors sidebar Locations, including add/remove/undo changes, and offers the native Choose Folder picker. Selecting a location or choosing a folder updates the scan target without starting a scan or opening Options. Escape and outside clicks dismiss the menus; only one picker is open at a time. Other empty pages retain their paired Choose Folder / Scan actions. The FDA banner and toolbar Options remain in place.

The existing agent picker, real agent marks, remembered choice, missing-CLI fallback to None, install links, Check again, and successful-scan handoff remain. The old standalone installation tip and dismissal logic are removed; install hints live inside the picker.

Validation:

- `make check` (`go vet ./...`, `go test ./...`), `make app`, `make cli`, and strict/deep codesign verification passed.
- Web, native, empty-start, dismissed-FDA, agent flow, remembered/missing/None/fresh-installed agent choice, and actual vendored terminal parser smoke scripts passed. New agent-flow assertions cover matching sidebar/menu Locations, selection without scanning, native picking with Options closed, add/remove/undo synchronization, Escape focus, and the target passed to Scan. Native bridge/provider processes are mocked; fixture APIs are real. No real Trash request is sent.
- `ui-hover-smoke.cjs` now includes command-bar geometry, fit, hover stability, and keyboard focus checks at 1200/900/390px in both themes. It and `ui-capture.cjs` are blocked before page load: Chrome aborts with sandbox crashpad/bootstrap permission errors. The in-app browser fallback reports that its browser access host is unavailable. These rendered checks have **not passed**; actual light/dark pixels and native WebKit geometry remain unverified.
- Logs: `.build/round7-{check,app,cli,codesign}.log`, `.build/round7-*-smoke.log`.
- Delivery: branch `round7` in `.build/round7-git`, with `.build/round7.bundle`, based on v1 HEAD `358a43ea19acb7635980c5b7750410b69805d611`. Author and committer: fum4 <52106389+fum4@users.noreply.github.com>. No generated icons or application binaries are committed; the primary `.git` is untouched.
