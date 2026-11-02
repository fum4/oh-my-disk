#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
# Wails probes these two values through sysctl. Restricted build hosts may
# deny the keys even though sw_vers provides the same public OS information.
if ! /usr/sbin/sysctl kern.osproductversion kern.osversion >/dev/null 2>&1; then
 echo "Wails OS probe: using sw_vers for sandbox-restricted sysctl keys" >&2
 mkdir -p .build/compat
 cat > .build/compat/sysctl <<'SHIM'
#!/bin/sh
case "$1" in
 kern.osproductversion) printf 'kern.osproductversion: '; /usr/bin/sw_vers -productVersion ;;
 kern.osversion) printf 'kern.osversion: '; /usr/bin/sw_vers -buildVersion ;;
 *) exec /usr/sbin/sysctl "$@" ;;
esac
SHIM
 chmod +x .build/compat/sysctl
 PATH="$PWD/.build/compat:$PATH"
 export PATH
fi
cd app
exec ../.build/bin/wails build -platform "${1:-darwin/arm64}"
