# Vendored xterm.js

Upstream: https://github.com/xtermjs/xterm.js (MIT; see xterm-LICENSE and addon-*-LICENSE).

- @xterm/xterm 5.5.0 — xterm.js, xterm.css
- @xterm/addon-fit 0.10.0 — addon-fit.js
- @xterm/addon-web-links 0.11.0 — addon-web-links.js
- @xterm/addon-unicode11 0.8.0 — addon-unicode11.js
- @xterm/addon-webgl 0.18.0 — addon-webgl.js

UMD bundles copied unmodified from the corresponding npm package's lib/ directory.
WebGL falls back to xterm's built-in DOM renderer if unavailable or on context loss.
No CDN or runtime package downloads are needed.
