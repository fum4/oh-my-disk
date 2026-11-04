# Backlog

## Top priority

- [x] **Releases and CI.** Run tests in CI and ship builds for macOS and Linux.
- [x] **Publish to nomadware.io.** Publish in the existing releases catalogue.

## Distribution

- [ ] Apple Developer ID signing and notarization: provision a Developer ID Application certificate and notarization credentials; sign the app and embedded CLI with hardened runtime and secure timestamp; notarize and staple the app/DMG; verify with Gatekeeper before publishing. Deferred by the owner on 2026-11-03. Until then the DMG is ad-hoc signed and macOS may warn on download.
- [x] Audit and update the GitHub README after the first verified release.
- [ ] Design a shared, clean docs-page template for Waker and oh-my-disk with title, summary/use case and command reference. Present a visual proposal for owner feedback before implementation. Requested on 2026-11-03; no separate product/docs page ships during release integration.
- [ ] Windows support: the scanner currently targets macOS/Linux; do not advertise Windows downloads yet.

## Product

- [ ] Support more agent providers.
- [ ] Add more icons, categories, CLI commands and explanations where useful.
- [ ] Explain what each "What you can clean" CLI command does.
- [x] Reclaim banner: drop the "≥", rename it Safe clean and say what it counts.
- [ ] Safe clean banner: add more useful content because it feels too empty.
- [ ] Fix `PersonalData` matching `downloads` anywhere in a path: `Library/Caches/Homebrew/downloads`, a `cache` item, shows "Your files" and is left out of "Safe clean". First step: anchor personal folder names to the home directory.
- [ ] Don't count pnpm store bytes as reclaimable while projects still share them. On a real store 82% of files were hardlinked into `node_modules`, so neither `pnpm store prune` nor deleting the store frees those blocks. APFS clones hide the same sharing. First step: record link count at scan time and report the shared share per suggestion.
- [x] Finalize the app icon (Prism storage blocks).
- [x] Use the same disk usage percentage in explorer charts and lists, and label the share in Overview.
- [x] Index categories per scan and reuse sorted drill-down pages.

## UI polish

- [x] After the Nomadware release integration ships: remove the app icon from the sidebar and enlarge the “oh-my-disk” sidebar heading. Keep the finalized Prism app/Dock icon unchanged. Requested by the owner on 2026-11-03.
- [ ] Remove tooltips across the app.
- [x] Fix the glossy look of dropdowns.
- [x] "How we counted" panel: add a slide animation.
- [ ] "How we counted" panel: rename it.
- [ ] Sunburst: make the hovered segment slightly larger.
- [ ] Sunburst and treemap hover tooltips: add a small swatch in the segment's color.
- [x] Categories and Folders lists: the size bars are barely visible, and they disappear on hover.
- [ ] Category map and Space map: move "Click to explore · center to go back" next to the left-side text.
- [ ] Remove the divider between the header and the page content.
- [ ] Use a folder icon for "Show in Finder" actions.
- [ ] Give tabs more contrast.
- [ ] Categories: remove "Current folder: All categories".
- [ ] "How we counted" panel: show the resize bar whenever the panel is hovered.
- [ ] "How we counted" panel: align the "What's included" values (right-aligned) with the "What's skipped" values (left-aligned).
