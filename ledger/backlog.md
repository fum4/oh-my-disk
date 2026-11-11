# Backlog

Follow-ups nobody is working on yet. `cerebro items file` adds to it. Done items live in `git log`.

## Apple Developer ID signing and notarization · 2026-11-04 · M

**Why:** the DMG is ad-hoc signed, so macOS may warn on download. Deferred by the owner on 2026-11-03.
**First step:** provision a Developer ID Application certificate and notarization credentials; sign the app and embedded CLI with hardened runtime and secure timestamp; notarize and staple the app/DMG; verify with Gatekeeper before publishing.

## A shared docs-page template for Waker and oh-my-disk · 2026-11-04 · M

**Why:** requested on 2026-11-03: title, summary/use case and command reference. No separate product/docs page shipped during release integration.
**First step:** present a visual proposal for owner feedback before implementation.

## Windows support · 2026-11-04 · L

**Why:** the scanner targets macOS/Linux only.
**First step:** don't advertise Windows downloads until it lands.

## Support more agent providers · 2026-11-04 · M

**Why:** a product item from the backlog's Product list.
**First step:** none recorded; which providers is the owner's pick.

## More icons, categories, CLI commands and explanations · 2026-11-04 · M

**Why:** add them where useful (Product list).
**First step:** none recorded.

## Explain what each "What you can clean" CLI command does · 2026-11-04 · S

**Why:** the suggested commands are shown as text to review, without saying what they do.
**First step:** a one-line explanation per suggestion.

## Safe clean banner: more useful content · 2026-11-05 · S

**Why:** it feels too empty.
**First step:** propose what it should show.

## `PersonalData` matches `downloads` anywhere in a path · 2026-11-05 · S

**Why:** `Library/Caches/Homebrew/downloads`, a `cache` item, shows as "Your files" and is left out of "Safe clean".
**First step:** anchor personal folder names to the home directory.

## Don't count shared pnpm store bytes as reclaimable · 2026-11-05 · M

**Why:** on a real store 82% of files were hardlinked into `node_modules`, so neither `pnpm store prune` nor deleting the store frees those blocks. APFS clones hide the same sharing.
**First step:** record link count at scan time and report the shared share per suggestion.

## Remove tooltips across the app · 2026-11-04 · S

**Why:** UI polish list.
**First step:** remove them.

## "How we counted" panel: rename it · 2026-11-05 · S

**Why:** UI polish list.
**First step:** propose names.

## "How we counted" panel: show the resize bar on hover · 2026-11-04 · S

**Why:** UI polish list: the resize bar should show whenever the panel is hovered.
**First step:** the hover style.

## "How we counted" panel: align the two value columns · 2026-11-04 · S

**Why:** the "What's included" values are right-aligned, the "What's skipped" values left-aligned.
**First step:** align them.

## Sunburst: enlarge the hovered segment slightly · 2026-11-04 · S

**Why:** UI polish list.
**First step:** the hover transform.

## Sunburst and treemap tooltips: a swatch in the segment's color · 2026-11-04 · S

**Why:** UI polish list.
**First step:** add the swatch to the shared tooltip.

## Category map and Space map: move the hint next to the left-side text · 2026-11-04 · S

**Why:** "Click to explore · center to go back" sits apart from it.
**First step:** move it.

## Remove the divider between the header and the page content · 2026-11-04 · S

**Why:** UI polish list.
**First step:** remove it.

## A folder icon for "Show in Finder" actions · 2026-11-04 · S

**Why:** UI polish list.
**First step:** swap the icon.

## Give tabs more contrast · 2026-11-04 · S

**Why:** UI polish list.
**First step:** adjust the tab tokens, light and dark.

## Categories: remove "Current folder: All categories" · 2026-11-04 · S

**Why:** UI polish list.
**First step:** remove the line.
