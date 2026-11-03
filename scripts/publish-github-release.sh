#!/bin/sh
# Preserve draft assets across retries: DMG bytes are not reproducible.
set -eu
: "${RELEASE_VERSION:?}" "${RELEASE_COMMIT:?}" "${GH_REPO:?}"
case ${1:-} in
 prepare)
  mkdir -p dist/release
  notes=$(mktemp)
  trap 'rm -f "$notes"' EXIT
  cat >"$notes" <<'NOTES'
macOS desktop: download the universal DMG and drag oh-my-disk into Applications.

CLI: macOS and Linux, Intel/AMD64 and ARM64. Native Trash support requires macOS.

Downloads, signed checksums and CLI installer: https://releases.nomadware.io/#oh-my-disk
NOTES
  # Create the tag explicitly: draft GitHub releases do not always materialize it.
  if ! gh api "repos/$GH_REPO/git/ref/tags/$RELEASE_VERSION" >/dev/null 2>&1; then
   gh api --method POST "repos/$GH_REPO/git/refs" -f "ref=refs/tags/$RELEASE_VERSION" -f "sha=$RELEASE_COMMIT" >/dev/null
  fi
  actual=$(gh api "repos/$GH_REPO/commits/$RELEASE_VERSION" --jq .sha)
  [ "$actual" = "$RELEASE_COMMIT" ] || { echo 'Release tag points at another commit' >&2; exit 1; }
  if ! gh release view "$RELEASE_VERSION" >/dev/null 2>&1; then
   gh release create "$RELEASE_VERSION" --verify-tag --title "oh-my-disk $RELEASE_VERSION" --draft --notes-file "$notes"
  fi
  existing=$(gh release view "$RELEASE_VERSION" --json assets --jq '.assets[].name')
  for file in dist/release/oh-my-disk-*; do
   name=${file##*/}
   if printf '%s\n' "$existing" | grep -qxF "$name"; then
    gh release download "$RELEASE_VERSION" --pattern "$name" --dir dist/release --clobber
   else
    gh release upload "$RELEASE_VERSION" "$file"
   fi
  done
  ;;
 finish)
  verified=$(mktemp -d)
  trap 'rm -rf "$verified"' EXIT
  latest=$(python3 scripts/verify-release.py verify "$verified")
  for name in SHA256SUMS SHA256SUMS.sig RELEASE; do
   gh release upload "$RELEASE_VERSION" "$verified/$name" --clobber
  done
  gh release edit "$RELEASE_VERSION" --draft=false --latest="$latest"
  ;;
 *) echo 'usage: publish-github-release.sh prepare|finish' >&2; exit 1 ;;
esac
