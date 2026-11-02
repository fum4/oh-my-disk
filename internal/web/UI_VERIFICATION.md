# Shared Calm UI verification

Wails and `oh-my-disk ui` embed the same `assets/` directory. Calm's tokens, type, ring geometry, spacing, cards, folder rows, cleanup rows, badges, and buttons come from the approved `.build/design/03-calm-v2.html`. All icons are locally inlined Phosphor SVG symbols (MIT license in `assets/Phosphor-LICENSE.txt`). System fonts take precedence on macOS; bundled Inter and JetBrains Mono remain available offline. There are no runtime network dependencies.

## Page coverage

- **Overview:** live whole-filesystem ring, free-space center, percent-full status, four-part legend, top seven folders plus Everything else, reclaim panel and before/after bar, seven cleanup suggestions, sort segments, and shared selection/Trash controls.
- **Folders:** breadcrumbs, icon and share-bar rows, share percentages, size/files/modified sorting, 200-entry lazy loading through `/api/node`, selection, and both interactive canvas maps.
- **Categories:** the same browser and maps, with lazy category details through `/api/category`.
- **Cleanup:** the same compact cleanup row component, filters, sort and reverse order, expandable explanations/path/project activity/commands/native actions, and 100-row display increments up to the existing API's 500-match cap.
- **Shared:** sidebar locations/counts/disk bar, folder chooser, Scan Again and Command-R/Control-R, cached/native welcome, progress and cancel, skeleton treatment, recovery messages, Full Disk Access, exports, accounting notes, dark appearance, reduced motion, focus rings, and arrow/Home/End row navigation. Native row actions appear on hover or keyboard focus; cleanup details expand with a real button.

The existing backend, API shapes, scanner, blocklist, Trash revalidation, MCP, and CLI semantics are unchanged. Checkbox state is synchronized across views; overlapping ancestor/descendant selections are still rejected. Trash always requires a confirmation dialog, with server checks unchanged.

## Intentional differences from the static mockup

- Values and filenames are live, with no preselected cleanup items or simulated hover/focus. Folder descriptions refer to the scanned folder because scanning outside Home is supported. The toolbar shows actual scan age and duration; its tooltip contains the full root and timestamp.
- The sidebar uses an application label in place of decorative traffic lights; native window controls remain the responsibility of Wails.
- Secondary scan roots, cache opening, unit preference, and exports live in **Scan options and exports**. Canvas maps remain in Folders/Categories to preserve their existing functionality.
- Longer cleanup details expand below the compact row. Folder/category pages keep file counts and modified dates. Narrow windows stack the Overview cards below 1150px rather than squeeze their text into fixed columns.
- Progress is indeterminate, with actual phase and entry/folder counts in Wails. The existing scanner API supplies no reliable total or percentage. Browser rescans expose an indeterminate busy state while their synchronous API request completes.
- Snapshot suggestions retain the existing 100-item cap: reclaim values display `≥` at the cap. Ring segments are clamped to the live disk's used space when APFS allocations exceed it; the before/after estimate is capped at capacity. Accounting notes explain that allocated bytes are not guaranteed reclaimed bytes.

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
