#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
go run scripts/icon.go
# Keep the embedded sidebar identity in sync with the Dock icon.
sips -z 128 128 build/appicon.png --out internal/web/assets/brand-icon.png >/dev/null
iconset=".build/oh-my-disk.iconset"
mkdir -p "$iconset" build/darwin
for size in 16 32 128 256 512; do
 sips -z "$size" "$size" build/appicon.png --out "$iconset/icon_${size}x${size}.png" >/dev/null
 double=$((size * 2))
 sips -z "$double" "$double" build/appicon.png --out "$iconset/icon_${size}x${size}@2x.png" >/dev/null
done
if ! iconutil -c icns "$iconset" -o build/darwin/icon.icns; then
 echo "iconutil unavailable in this environment; packing standard PNG ICNS entries" >&2
 python3 scripts/pack-icon.py "$iconset" build/darwin/icon.icns
fi
