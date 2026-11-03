#!/usr/bin/env python3
"""Fail closed before retrying a publish or exposing a GitHub release."""
import hashlib
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile
import urllib.error
import urllib.request

PACKAGE = "oh-my-disk"
PUBLISHER = Path(".release-publisher").resolve()
ASSETS = Path("dist/release")
TAG_PATTERN = r"v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)"


def publisher(command, *args):
    return subprocess.check_output(
        ["sh", "-eu", "-c", '. "$ROOT/lib/common.sh"; ' + command,
         "verify-release", *map(str, args)],
        env={**os.environ, "ROOT": str(PUBLISHER)}, text=True,
    ).strip()


def tag_number(tag):
    match = re.fullmatch(TAG_PATTERN, tag)
    if not match:
        raise ValueError(f"Invalid release tag: {tag!r}")
    return tuple(map(int, match.groups()))


def download(url, destination, missing_ok=False):
    request = urllib.request.Request(url, headers={"User-Agent": "oh-my-disk-release-verifier/1.0"})
    try:
        with urllib.request.urlopen(request, timeout=60) as response:
            with destination.open("wb") as output:
                while chunk := response.read(1024 * 1024):
                    output.write(chunk)
    except urllib.error.HTTPError as error:
        if missing_ok and error.code == 404:
            return False
        raise
    return True


def digest(path):
    with path.open("rb") as source:
        return hashlib.file_digest(source, "sha256").hexdigest()


def verify(base, tag, assets, directory):
    url = f"{base}/{PACKAGE}/{tag}"
    for name in ("SHA256SUMS", "SHA256SUMS.sig"):
        download(f"{url}/{name}", directory / name)
    publisher('sig_verify "$1"', directory)
    checksums = {}
    for line in (directory / "SHA256SUMS").read_text(encoding="ascii").splitlines():
        match = re.fullmatch(r"([0-9a-f]{64})  ([A-Za-z0-9.-]+)", line)
        if not match or match[2] in checksums:
            raise ValueError("Malformed or duplicate checksum entry")
        checksums[match[2]] = match[1]
    if set(checksums) != {*assets, "RELEASE"}:
        raise ValueError("Signed manifest must cover exactly five assets and RELEASE")
    for name, checksum in checksums.items():
        remote = directory / name
        download(f"{url}/{name}", remote)
        if digest(remote) != checksum:
            raise ValueError(f"Remote checksum mismatch: {name}")
        if name != "RELEASE" and digest(ASSETS / name) != checksum:
            raise ValueError(f"Draft asset mismatch: {name}")
    if (directory / "RELEASE").read_bytes() != f"{PACKAGE} {tag}\n".encode():
        raise ValueError("Signed RELEASE names another package or version")


def run(mode, metadata=None):
    tag = os.environ["RELEASE_VERSION"]
    requested = tag_number(tag)
    targets = publisher('pkg_known "$1"; asset_targets "$1"', PACKAGE).split()
    binary = publisher('pkg_field "$1" 2', PACKAGE)
    assets = {f"{binary}-{target}" for target in targets}
    if len(assets) != 5 or {path.name for path in ASSETS.iterdir()} != assets:
        raise ValueError("Local release must contain exactly four CLIs and universal DMG")
    if any(not (ASSETS / name).is_file() or (ASSETS / name).is_symlink() for name in assets):
        raise ValueError("Local release assets must be regular files")
    base = publisher("config_get RELEASES_BASE")
    with tempfile.TemporaryDirectory(prefix="oh-my-disk-verify-") as temporary:
        directory = Path(temporary)
        present = download(f"{base}/{PACKAGE}/latest", directory / "latest", missing_ok=True)
        latest = (directory / "latest").read_text(encoding="ascii").removesuffix("\n") if present else None
        completed = latest is not None and tag_number(latest) >= requested
        if mode == "detect" and not completed:
            return False
        if not completed:
            raise ValueError("Publisher has not advanced latest to this version")
        verify(base, tag, assets, directory)
        if metadata is not None:
            for name in ("SHA256SUMS", "SHA256SUMS.sig", "RELEASE"):
                (metadata / name).write_bytes((directory / name).read_bytes())
        return True if mode == "detect" else latest == tag


if __name__ == "__main__":
    try:
        if sys.argv[1:] == ["detect"]:
            result = run("detect")
        elif len(sys.argv) == 3 and sys.argv[1] == "verify":
            result = run("verify", Path(sys.argv[2]))
        else:
            raise ValueError("usage: verify-release.py detect|verify <metadata-dir>")
        print(str(result).lower())
    except (OSError, ValueError, subprocess.CalledProcessError) as error:
        sys.exit(f"Release verification failed: {error}")
