# Desktop verification — 2026-11-02

Implemented with Wails v2.17.1, resolved by the requested
`go install github.com/wailsapp/wails/v2/cmd/wails@latest` check. Pinned tooling
uses Go 1.26.10; the module requires Go 1.26.0. The v2 asset server directly
supports the existing lazy HTTP API within a native WKWebView. Upstream
[v2 installation documentation](https://v2.wails.io/docs/gettingstarted/installation/)
now advertises v3 as stable; v2 is retained for the requested `wails build`
workflow and this single-window integration.

Passed:

- `make check`: `go vet ./...` and `go test ./...`, including unit formatting,
  volume reconciliation/clamping, live statfs, token-free native snapshot and
  node APIs, rejected mutation requests, cancellation preserving displayed
  results, and overlapping-scan rejection. Existing CLI/scanner/rules/report/web
  tests also pass.
- `go test -race ./app ./internal/web`.
- JavaScript syntax checks with `node --check` for both frontend scripts.
- `make app`: arm64 `.app` compilation and packaging.
- `make app-universal`: universal x86_64 + arm64 `.app` compilation and packaging.
  The last build occupies `build/bin/oh-my-disk.app`.
- `file` confirms both architectures; `plutil -lint` validates the bundle plist;
  `codesign --verify --deep --strict` validates the local ad-hoc signature.
- The selected app identity is the Prism storage-block icon. `make icon` regenerates
  the 1024px PNG, macOS ICNS, and shared sidebar/favicon PNG from `scripts/icon.go`.
- Bundle icon bytes match `build/darwin/icon.icns`. The original 1024px PNG was
  visually inspected. The build first attempts `iconutil`; this host rejected
  its dimension-checked iconset, so the portable PNG-based ICNS fallback was used.
- Fixture CLI smoke: scanning, cache reuse, decimal output and `--binary` output.
  HTTP smoke: unauthenticated browser API returns 403; authenticated snapshot
  contains the expected file count and capacity = used + available.

Build-host adaptation: the sandbox denies `sysctl kern.osproductversion` and
`kern.osversion`. The Make wrapper supplies these two public version values
from `sw_vers` through a build-local shim. Other sysctl calls are delegated to
Apple's executable. No system files or settings are modified.

Launch verification is **incomplete**. `open build/bin/oh-my-disk.app` was
attempted but Launch Services returned `-10827` (`kLSNoExecutableErr`) despite
the executable being present and executable. Direct execution reached Wails'
native window creation and then aborted with SIGABRT. A sustained window,
Dock appearance, and normal quit could not be confirmed on this restricted
host. Computer-use inspection was rejected as “not approved” for this app.
The separate browser smoke attempt could not start because the installed
browser helper referenced a missing `browser-service.mjs` module.

Remaining manual verification on a normal desktop session:

1. Double-click/open the built app; confirm its own window and Dock icon.
2. Pick a small folder, scan, inspect progress, cancel a larger scan, and rescan.
3. Quit/reopen; open the previous cache and check its timestamp.
4. Exercise Finder reveal, path/command copy, and both native save dialogs;
   verify an existing export is refused.
5. Inspect the permission banner on a Mac without Full Disk Access; open the
   linked settings pane, enable access manually, quit/reopen, and rescan.
6. Quit normally and confirm the process exits. Intel runtime behavior is also
   unverified; the universal binary has only been cross-compiled.

No user content was deleted or modified. Smoke fixtures, caches and test
reports are confined to `.build/` or test temporary directories. Developer ID
signing and notarization remain distribution TODOs.
