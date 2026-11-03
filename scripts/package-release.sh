#!/bin/sh
# Run on macOS after make app-universal. Apple signing/notarization is deferred.
set -eu
cd "$(dirname "$0")/.."
: "${RELEASE_VERSION:?set RELEASE_VERSION to vMAJOR.MINOR.PATCH}"
version=${RELEASE_VERSION#v}
python3 - "$version" <<'PY'
import re, sys
if not re.fullmatch(r"\d+\.\d+\.\d+", sys.argv[1]):
    raise SystemExit("invalid release version")
PY
out=dist/release
mkdir -p "$out"
ldflags="-s -w -X github.com/fum4/oh-my-disk/internal/version.Version=$version"
for arch in amd64 arm64; do
 CGO_ENABLED=0 GOOS=linux GOARCH=$arch go build -trimpath -ldflags "$ldflags" -o "$out/oh-my-disk-linux-$arch" .
 # Preserve native macOS Trash support in both CLI architectures.
 CGO_ENABLED=1 GOOS=darwin GOARCH=$arch go build -trimpath -ldflags "$ldflags" -o "$out/oh-my-disk-darwin-$arch" .
 codesign --force --sign - "$out/oh-my-disk-darwin-$arch"
done
bundle=build/bin/oh-my-disk.app
/usr/libexec/PlistBuddy -c "Set :CFBundleShortVersionString $version" "$bundle/Contents/Info.plist"
/usr/libexec/PlistBuddy -c "Set :CFBundleVersion $version" "$bundle/Contents/Info.plist"
lipo -create "$out/oh-my-disk-darwin-arm64" "$out/oh-my-disk-darwin-amd64" -output "$bundle/Contents/MacOS/oh-my-disk-cli"
codesign --force --deep --sign - "$bundle"
codesign --verify --deep --strict "$bundle"
lipo "$bundle/Contents/MacOS/oh-my-disk" -verify_arch arm64 x86_64
lipo "$bundle/Contents/MacOS/oh-my-disk-cli" -verify_arch arm64 x86_64
stage=$(mktemp -d "${TMPDIR:-/tmp}/oh-my-disk-dmg.XXXXXXXX")
trap 'rm -rf "$stage"' EXIT
mkdir "$stage/image"
ditto "$bundle" "$stage/image/oh-my-disk.app"
ln -s /Applications "$stage/image/Applications"
# hdiutil writes a temporary image so retries cannot reuse an older DMG.
hdiutil create -volname "oh-my-disk $version" -srcfolder "$stage/image" -format UDZO -ov "$stage/release.dmg"
hdiutil verify "$stage/release.dmg"
mv "$stage/release.dmg" "$out/oh-my-disk-darwin-universal.dmg"
