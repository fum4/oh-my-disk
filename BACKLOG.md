# Backlog

## Top priority

1. **Releases and CI.** Fix deploys and releases, run tests in CI, and ship builds for macOS, Linux and Windows.
2. **Publish to nomadware.io.** Publish in the existing releases catalogue first; a dedicated page is deferred.

## Distribution

- [ ] Apple Developer ID signing and notarization: provision a Developer ID Application certificate and notarization credentials; sign the app and embedded CLI with hardened runtime and secure timestamp; notarize and staple the app/DMG; verify with Gatekeeper before publishing. Deferred by the owner on 2026-11-03. Until then the DMG is ad-hoc signed and macOS may warn on download.
- [ ] Dedicated oh-my-disk product/docs page on nomadware.io (after the release catalogue integration).
- [ ] Windows support: the scanner currently targets macOS/Linux; do not advertise Windows downloads yet.

## Product

- [ ] Support more agent providers.
- [ ] Add more icons, categories, CLI commands and explanations where useful.
- [ ] Explain what each "What you can clean" CLI command does.
- [ ] "You could get back" banner: drop the "≥", and add more useful content because it feels too empty.
- [x] Finalize the app icon (Prism storage blocks).
- [ ] Calculate and show numbers consistently everywhere (e.g. a folder's disk share differs between charts and lists).
- [ ] Categories: drill-down works but takes a very long time; make it fast.

## UI polish

- [ ] Remove tooltips across the app.
- [ ] Fix the glossy look of dropdowns.
- [ ] "How we counted" panel: add a slide animation and rename it.
- [ ] Sunburst: make the hovered segment slightly larger.
- [ ] Sunburst and treemap hover tooltips: add a small swatch in the segment's color.
- [ ] Categories and Folders lists: the size bars are barely visible, and they disappear on hover.
- [ ] Category map and Space map: move "Click to explore · center to go back" next to the left-side text.
- [ ] Remove the divider between the header and the page content.
- [ ] Use a folder icon for "Show in Finder" actions.
- [ ] Give tabs more contrast.
- [ ] Categories: remove "Current folder: All categories".
- [ ] "How we counted" panel: show the resize bar whenever the panel is hovered.
- [ ] "How we counted" panel: align the "What's included" values (right-aligned) with the "What's skipped" values (left-aligned).
