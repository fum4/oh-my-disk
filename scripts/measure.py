#!/usr/bin/env python3
"""Resource accounting fallback when sandboxed macOS time -l cannot use sysctl."""
import json, resource, subprocess, sys, time
start = time.monotonic()
p = subprocess.run(sys.argv[1:])
r = resource.getrusage(resource.RUSAGE_CHILDREN)
print(json.dumps(dict(wall_seconds=time.monotonic()-start, user_seconds=r.ru_utime,
                      sys_seconds=r.ru_stime, peak_rss_bytes=r.ru_maxrss)), file=sys.stderr)
sys.exit(p.returncode)
