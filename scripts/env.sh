#!/bin/sh
# Optional toolchain/cache overrides; ordinary installations can use PATH directly.
if [ -n "${RADAR_TOOLCHAIN_DIR:-}" ]; then
  export PATH="$RADAR_TOOLCHAIN_DIR/go/bin:$RADAR_TOOLCHAIN_DIR/tinygo/bin:$PATH"
fi
radar_cache_root="${RADAR_CACHE_ROOT:-${XDG_CACHE_HOME:-$HOME/.cache}/navidrome-upcoming-releases-plugin}"
export GOPATH="${RADAR_GOPATH:-${GOPATH:-$radar_cache_root/gopath}}"
export GOCACHE="${RADAR_GOCACHE:-$radar_cache_root/go-build}"
export GOMODCACHE="$GOPATH/pkg/mod"
export XDG_CACHE_HOME="$radar_cache_root"
