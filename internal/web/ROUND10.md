# Round 10 — job J

Based on v1 HEAD `42cbdf6`.

- Every pre-scan sidebar page keeps its own heading and copy above the same variant C command bar. The folder picker, persisted scan target, agent picker, and Scan handler are shared; the previous empty-state action row is removed.
- Scanning uses the normal window surface, a centered animated ring, tabular count smoothed at 10 Hz, compact folder/rate metrics, a middle-shortened path, and plain phase names. Navigation remains usable; the subtitle tracks the scan target. Cancellation/errors retain the previous results. Completion keeps the scanning view until the new snapshot is loaded, then fades into results. Reduced motion stops rotation and count interpolation. The native status API has neither a total nor a scanned-byte estimate, so progress is honestly indeterminate.
- `COUNTING_LABEL` holds the final “How we counted” label. Its active trigger and close X sit inside the connected, theme-adaptive deep surface. The panel expands in flow and only its content scrolls. Trigger, X, and Escape close it. Accounting is grouped into short labeled rows with aligned tabular values; units follow the display preference.
- Overview links explicitly name Cleanup/Folders, update the page header and active navigation, animate the page change, and create a history entry. The quiet Overview link, Escape, and browser back restore Overview and its scroll positions; forward navigation also works.
- Removed sidebar app branding. Corrected shortcut and quiet-label contrast, retained fixed-size hover paint and keyboard focus, and kept the existing top-level menu portals. The scan target select fills its row alongside Change folder. Drawer height is constrained to preserve card headings; compact layouts hide secondary controls before clipping headings. The terminal retains inner padding.
- Maps use a square canvas sized to the column and available height, with a DPR backing store and CSS-pixel text sizes. Sunburst bands fill the available radius at the actual visible depth, including single-level category summaries. Switch and legend follow the chart. Category labels are capitalized for display (including AI models); stored keys and API values stay unchanged. Category summaries hide unavailable Files/Modified columns. Breadcrumbs use primary text with muted separators and a quiet hover fill.

## Verification

Passed:

- `make check`: `go vet ./...` and `go test ./...`.
- `make app`; `codesign --verify --deep --strict build/bin/oh-my-disk.app`.
- JavaScript syntax checks.
- `ui-smoke.cjs`: real fixture APIs, Overview navigation/back/Escape/scroll restoration, maps, category drill-down, filters, units, disclosure trigger/X/Escape, and fixture-only rescans. No Trash action.
- `ui-native-smoke.cjs`, normal, empty, and dismissed-access variants: bridge mocks cover shared command identity, pre-scan navigation, selected targets, live counts/phases/header, navigation during scanning, cancellation, completion, exports, and Finder/copy actions.
- `ui-agents-smoke.cjs`, default, remembered, remembered-none, fresh-installed, and missing-remembered variants: persisted agent choice, shared pre-scan control, scan handoff, cancellation, and mocked terminal lifecycle.
- `ui-smoke.cjs --large`: folder/category pagination from 200 to 400, global sort, and treemap on the existing large fixture.
- `ui-terminal-smoke.cjs`: actual vendored xterm parser, alternate screen, Unicode, streaming UTF-8, resize, and scrollback.

Rendered visual QA remains blocked: both Playwright capture and hover sweeps abort at Chrome startup with sandbox `bootstrap_check_in … Permission denied` / `SIGABRT`. The in-app browser fallback reports “The browser access host is unavailable.” The capture script has been updated for the shared command bar and real scanning presentation. DOM/bridge tests do not establish rendered pixel fidelity, overflow, or animation quality.

Logs are ignored `.build/round10-*.log` artifacts. Generated `build/darwin/icon.icns` is excluded from the commit.
