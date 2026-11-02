# Shared UI verification

The Wails application and `oh-my-disk ui` embed the same `assets/` directory, including the locally bundled Inter and JetBrains Mono fonts and their OFL license files. The overview uses allocated bytes from the scan, with white tiles showing the share covered by known cleanup candidates. Snapshot suggestions are capped at 100; the overview marks their total as a lower bound when that cap is reached. Volume HUD values always use decimal GB, independently of the optional binary setting used elsewhere.

## Reproduce

Build and start the UI against the existing synthetic home (generate it first using `scripts/synthetic.py` if needed):

```sh
make cli
./oh-my-disk ui --root "$PWD/.fixtures/home" --home "$PWD/.fixtures/home" \
  --cache "$PWD/.fixtures/state/latest.bin" --refresh --min-size 0 --no-open --json
```

In another terminal, set `OMD_UI_URL` to the private URL printed by that command, then run:

```sh
npm install --prefix .build/browser jsdom playwright
export NODE_PATH="$PWD/.build/browser/node_modules"
node scripts/ui-smoke.cjs "$OMD_UI_URL"
node scripts/ui-native-smoke.cjs "$OMD_UI_URL"
node scripts/ui-capture.cjs "$OMD_UI_URL"
```

The web smoke check rescans only `.fixtures/home`; it opens and cancels the Trash confirmation without sending a Trash request. The native smoke check stubs all Wails bridge methods and checks cached startup, navigation before opening results, the folder picker, scan/progress/cancel, Reveal, copy path/command, both exports, and the Full Disk Access banner. Neither smoke check moves files to Trash. Canvas drawing is stubbed in DOM checks, so they verify application behavior rather than actual rendering.

The capture script uses installed Google Chrome by default. Set `OMD_CHROME` to another Chromium executable if needed. It saves `.build/screens/overview.png`, `folders.png`, `categories.png`, `cleanup.png`, `treemap.png`, `confirmation.png`, and `mobile-overview.png`. It also checks mobile document overflow and browser script errors. Keep tokenized URLs private.

## This redesign's validation

- `make check` (`go vet ./...` and `go test ./...`): passed.
- `make cli`, `make app`, and `codesign --verify --deep --strict build/bin/oh-my-disk.app`: passed.
- Web and native bridge DOM smoke checks against `.fixtures/home`: passed, with no script errors.
- Additional DOM check against the 953,002-entry `.fixtures/large`: passed folder and category pagination from 200 to 400 entries, folder drill, global modified sort, and treemap creation.
- Screenshot capture: attempted using installed Chrome, direct Chrome headless, and installed Chromium headless shell. Browser startup is blocked by the execution sandbox. Chromium reports `bootstrap_check_in ... MachPortRendezvousServer ... Permission denied (1100)`. No PNGs were produced; visual rendering and responsive layout remain unverified in this session.

Build logs and DOM evidence are in `.build/redesign-*.log` and `.build/screens/overview.dom.html`. The Chromium launch failure is recorded in `.build/screens/capture.log`. These are ignored local artifacts.
