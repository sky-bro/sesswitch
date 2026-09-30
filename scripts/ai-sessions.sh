#!/bin/bash
# @vicinae.schemaVersion 1
# @vicinae.title AI Sessions (Legacy)
# @vicinae.mode silent
# @vicinae.exec ["/bin/bash"]
# @vicinae.description Text fallback for the native Sesswitch extension
# @vicinae.packageName Sesswitch

set -euo pipefail

binary="${SESSWITCH_BINARY:-$(command -v sesswitch 2>/dev/null || true)}"
if [[ -z "$binary" ]]; then
  binary="$HOME/.local/bin/sesswitch"
fi
if [[ ! -x "$binary" ]]; then
  echo "sesswitch is not installed: $binary" >&2
  exit 1
fi

exec "$binary" pick
