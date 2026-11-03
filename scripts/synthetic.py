#!/usr/bin/env python3
"""Create a realistic, bounded fixture in a NEW directory. Never touches a real home."""
import hashlib
import json
import os
from pathlib import Path
import sys
import time

# Large mode uses mostly empty files so metadata scale does not require GBs of data.
if '--large' in sys.argv:
    import argparse
    ap = argparse.ArgumentParser()
    ap.add_argument('root')
    ap.add_argument('--large', action='store_true')
    ap.add_argument('--projects', type=int, default=1000)
    ap.add_argument('--packages', type=int, default=50)
    ap.add_argument('--files', type=int, default=18)
    args = ap.parse_args()
    root = Path(args.root).absolute()
    if '.fixtures' not in root.parts:
        ap.error('large fixtures must be under .fixtures/')
    root.mkdir(parents=True, exist_ok=False)
    started = time.monotonic()
    count = 1
    for i in range(args.projects):
        project = root / f'app-{i:05}'
        project.mkdir()
        (project / 'package.json').write_bytes(b'{}')
        modules = project / 'node_modules'
        modules.mkdir()
        count += 3
        for j in range(args.packages):
            package = modules / f'package-{j:04}'
            package.mkdir()
            count += 1
            for k in range(args.files):
                with (package / ('package.json' if k == 0 else f'file-{k:03}.js')).open('xb') as f:
                    if k == 0:
                        f.write(b'{}')
                count += 1
        if i % 100 == 0:
            print(f'{count:,} entries', file=sys.stderr)
    os.link(root / 'app-00000/package.json', root / 'hardlink')
    os.symlink(root, root / 'loop')
    print(json.dumps({'fixture': str(root), 'entries': count + 2,
                      'seconds': time.monotonic() - started}))
    sys.exit(0)

root = Path(sys.argv[1] if len(sys.argv) > 1 else '.fixtures/home').absolute()
root.mkdir(parents=True, exist_ok=False)
layout = {
    'Library/Caches/Homebrew/downloads/tool.tar': 6,
    'Library/Caches/com.apple.dt.Xcode/index.bin': 4,
    'Library/Developer/Xcode/DerivedData/App/Build/output.o': 8,
    'Library/Developer/Xcode/Archives/release.xcarchive/app': 4,
    'Library/Developer/CoreSimulator/Devices/device/data': 3,
    'Library/Containers/com.docker.docker/Data/vms/0/Docker.raw': 10,
    'Library/Application Support/MobileSync/Backup/device/data': 3,
    'Library/Messages/Attachments/photo.jpg': 2,
    'Library/Android/sdk/system-images/android-35/image': 3,
    '.npm/_cacache/content/blob': 8,
    '.cache/huggingface/hub/model.safetensors': 7,
    '.cache/uv/archive/wheel': 2,
    '.cache/ms-playwright/chromium/browser': 2,
    '.cache/custom-app/cache': 2,
    '.lmstudio/models/org/model.gguf': 5,
    '.ollama/models/blobs/model': 4,
    '.codeium/cache/index': 2,
    '.codex/sessions/session.jsonl': 1,
    '.gradle/caches/modules/dependency.jar': 2,
    '.android/avd/pixel.avd/userdata.img': 2,
    'go/pkg/mod/example/module/source': 2,
    'Downloads/installer.dmg': 3,
    'Pictures/family.jpg': 2,
    'mystery/large-unknown.bin': 5,
    '_work/old-app/node_modules/package/index.js': 4,
    '_work/old-app/.next/cache/build': 2,
    '_work/rust-app/target/debug/app': 3,
    '_work/python-app/.venv/lib/site-packages/pkg': 2,
}
old = time.time() - 240 * 86400
for rel, mib in layout.items():
    p = root / rel
    p.parent.mkdir(parents=True, exist_ok=True)
    # Deterministic nonzero data, physically allocated rather than sparse/truncated.
    with p.open('xb') as f:
        for n in range(mib * 16):
            f.write(hashlib.shake_256((rel + str(n)).encode()).digest(65536))
    os.utime(p, (old, old))
for rel, content in {
    '_work/old-app/package.json': '{"name":"old-app"}',
    '_work/rust-app/Cargo.toml': '[package]\nname="rust-app"\nversion="0.1.0"',
    '_work/python-app/pyproject.toml': '[project]\nname="python-app"',
}.items():
    p = root / rel
    p.write_text(content)
    os.utime(p, (old, old))
# A wide subtree exercises the parallel work queue, without allocating much disk.
for i in range(300):
    p = root / '_work/old-app/node_modules' / f'package-{i}'
    p.mkdir()
    (p / 'package.json').write_text('{"name":"fixture"}')
os.link(root / '.npm/_cacache/content/blob', root / 'npm-hardlink')
os.symlink(root, root / '_work/loop')
os.symlink('missing-target', root / 'broken-link')
with (root / 'sparse.img').open('xb') as f:
    f.truncate(2 * 1024**3)
# Make project staleness explicit across directory and child metadata.
for base, dirs, files in os.walk(root, followlinks=False):
    for name in dirs + files:
        p = Path(base) / name
        if not p.is_symlink():
            os.utime(p, (old, old))
os.utime(root, (old, old))
print(json.dumps({'fixture': str(root), 'logical_data_mib': sum(layout.values()), 'wide_packages': 300}))
