#!/usr/bin/env python3
"""Reuse this commit's version on retries; otherwise increment the latest stable patch."""
import re
import subprocess

def tags(*args):
    return subprocess.check_output(["git", "tag", *args], text=True).splitlines()

def stable(values):
    return sorted((tuple(map(int, tag[1:].split("."))), tag) for tag in values
                  if re.fullmatch(r"v\d+\.\d+\.\d+", tag))

current = stable(tags("--points-at", "HEAD"))
previous = stable(tags("--merged", "HEAD"))
if current:
    print(current[-1][1])
elif previous:
    (major, minor, patch), _ = previous[-1]
    print(f"v{major}.{minor}.{patch + 1}")
else:
    print("v1.0.0")
