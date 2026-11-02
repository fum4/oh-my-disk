#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
# Wails creates its own icon from the PNG. Install our iconutil-produced ICNS
# (or its portable fallback) and renew the local ad-hoc signature afterward.
bundle=build/bin/oh-my-disk.app
if test -f build/darwin/icon.icns; then
 cp build/darwin/icon.icns "$bundle/Contents/Resources/iconfile.icns"
 codesign --force --sign - "$bundle"
fi
