# Calm polish report

Base: `4d846c1` on `v1`. Shared Wails/browser assets; all eight feedback screenshots and the approved Calm reference inspected.

## Feedback items

1. Full Disk Access: renamed to **Open Settings**, a 32px secondary button at the right, vertically centered across the banner copy. It wraps beneath the copy on mobile.
2. Toolbar actions: icon, label, and shortcut are centered as one inline-flex group, with symmetric padding; Choose Folder uses the same alignment.
3. Sidebar: 2px separation between adjacent navigation items, including active/hover fills.
4. Scan options: remains collapsible and spans the content width, in normal flow above the footer. Selects, scan/chooser/cancel/export buttons and labels align at 32px. Progress spans the panel width at 8px high. Long paths wrap, empty chosen-path slots collapse, and the expanded panel can scroll in a constrained desktop window.
5. Contrast: solid dark content background `#1c1e24`, cards `#2b2e35`, subtle brighter hairlines. Light cards remain white on a neutral window; column labels are darker for legibility.
6. Controls: consistent 28px compact and 32px standard controls, centered labels/icons, explicit pointer cursors, disabled cursors/opacity, focus-visible rings. Checkbox labels, summary, selects, chart segments and sort buttons are included. Selection and destructive confirmation behavior remains intact.
7. Show All: symmetric 8px horizontal padding, 28px minimum height, centered label, no flex shrinking.
8. Overview scrolling: folder header/column labels sit outside `#overview-folders`; cleanup title, reclaim panel, sort/filter controls and column labels sit outside its independently scrolling list. Selection bar remains outside the list. Desktop panels use the remaining height; narrow/mobile and short-window fallbacks keep controls reachable.
9. Folders/Categories: bounded explorer/workspace; only the table list scrolls, with sticky column headers. Footer, options, accounting notes, breadcrumbs and current-folder header stay outside it. Footer uses normal layout flow, eliminating its absolute overlap with rows.
10. Sort: **Best / Largest / Unused Longest**. The hint and empty-filter message explain that personal files are available under Largest/Unused Longest.
11. Folder actions: bundled Phosphor eye/copy icon buttons, 28px, on the name line with reserved width. Hover/focus changes opacity without row reflow; touch devices keep actions visible. Tooltips/aria-labels identify actions; copying shows brief Copied feedback. Share bars align under the folder icon.
12. Chart tooltips: overview ring segments have label, size, percentage of disk, native SVG titles, and shared pointer tooltips. Canvas charts also include disk percentage. Sidebar and before/after bar segments expose size/percentage tooltips. Tooltip overlay does not intercept input; ring-center text no longer blocks segments.
13. Explorer composition: centered Sunburst/Treemap segmented control directly beneath the chart, centered wrapping legend beneath it, bounded chart size, and a persistent side-by-side map/table on desktop instead of a tall stacked chart pushing the table/footer away.

## Additional sweep fixes

- Reclaim estimate, overview cleanable ring portion and before/after bar now count only complete cache/reinstallable candidates, excluding personal, unknown and partial content. A list of reviewable files no longer implies all those bytes can be safely cleaned.
- Prevent unknown aggregate ancestors from hiding personal folders and returning them indirectly to Best.
- Light column-header contrast increased; personal badges avoid squeezed icon/text combinations.
- Accounting notes expose `aria-expanded`/`aria-controls` and use a bounded scroll area.
- Dialog height is bounded with internal scrolling for long confirmations/small windows.
- Short desktop windows allow page scrolling instead of clipping controls. Mobile keeps bounded lists and exposes hover actions on devices without hover.
- Expanded scan options no longer cover panels. Footer remains outside all lists, including while notes/options are open.
- Old cloud-size cache replacement is validated so a fresh scan can replace an obsolete cache safely.

## Ranking and local allocation

- One scoring implementation shared by Analyze, Explain and ExplainTree. Complete cache/reinstallable candidates rank ahead of unknown/partial items in Best.
- Personal content is excluded from Best and the overview snapshot suggestions. Largest and Unused Longest retain it, capped at 30 and labeled **Your files**. User-data rules and path components Music, Pictures, Movies, Documents, Desktop, Photos, Downloads, Mobile Documents, CloudStorage, and `.photoslibrary` packages are protected conservatively at any scan root.
- Personal items receive no staleness multiplier. Synced mtimes therefore cannot raise their score. Unknown/big-unknown and incomplete candidates cap at 40. JSON presentation and selection apply the limits to stored scores too.
- Expandable details show rule identifier, reason, score, safety in plain words, local allocated size and unused days, including applicable caps and the synced-deletion warning.
- Darwin bulk, stat fallback, and Unix file-info accounting now retain allocated bytes for dataless files instead of substituting logical size. Dataless directories remain unenumerated to avoid downloads. A 79 GB placeholder with 4 KB allocated contributes 4 KB.
- Binary cache moves from OMDCACH5 to OMDCACH6; old binary caches require a fresh scan. Legacy JSON caches containing dataless nodes also require a rescan. Valid old caches remain replaceable after that scan.

Rounded scoring examples using the byte sizes supplied with the feedback:

| Item | Previous | New | Best |
|---|---:|---:|---|
| DEEP, 79.1 GB, 314 days | 86 | 30 maximum | Excluded |
| Phone, 28.5 GB, 314 days | 70 | 30 maximum | Excluded |
| Pictures, 20.2 GB, 0 days | 71 | 30 maximum | Excluded |
| _cacache, 8.1 GB, 0 days | 89 | 89 | Eligible |

These are deterministic regression inputs, not a scan of the user's files. DEEP/Phone's exact new local allocation is unknown until a fresh scan; their scores can be substantially lower than 30 after the allocation correction.

Tests cover caps, no personal age boost, protected paths, fresh safe data ahead of huge unknown/partial data, personal exclusion from Best and presence in Largest, cached-score presentation, Analyze/Explain equivalence, aggregate splitting, dataless allocation (bulk/stat), and old binary/JSON cache recovery.

## Validation and limitation

Passed: `go vet ./...`, `go test ./...`, `make cli`, `make app`, `codesign --verify --deep --strict build/bin/oh-my-disk.app`, web smoke, native bridge smoke, empty-start smoke, large-list smoke, and whitespace checks. Large fixture contains 4,053 entries across 450 projects; lazy Folders/Categories pagination 200→400, global sorting and treemap pass. Native methods are stubbed in native smoke. No Trash operation was confirmed.

**Rendered sweep remains blocked.** Chromium aborts before page load under the sandbox; the browser access host is unavailable. Automatic approval review also rejected the native Chrome fallback because Computer Use was not approved for Chrome. Therefore actual rendered light/dark, wide/narrow and state pixel QA is not complete. The extended capture script defines that matrix and layout/cursor/scroll assertions for a permitted run; it has not passed here. Static inspection and DOM/bridge smoke checks do not establish final rendered fidelity.

Logs are under `.build/polish-*.log`. Interaction DOM output is `.build/screens/overview.dom.html`.

## Git delivery

Small conventional commits, author/committer `fum4 <52106389+fum4@users.noreply.github.com>`, branch `polish` based on `4d846c1` in `.build/polish-git`. Bundle: `.build/polish.bundle`. The primary `.git` was not written. `build/darwin/icon.icns` is not committed.
