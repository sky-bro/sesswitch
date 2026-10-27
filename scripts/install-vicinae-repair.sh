#!/bin/bash
set -euo pipefail

repo_dir="$(cd "$(dirname "$0")/.." && pwd)"
# Vicinae uses this data directory on macOS, independently of XDG_DATA_HOME.
scripts_dir="${VICINAE_SCRIPTS_DIR:-$HOME/.local/share/vicinae/scripts}"
mkdir -p "$scripts_dir"
install -m 755 "$repo_dir/scripts/repair-ai-sessions.sh" "$scripts_dir/repair-ai-sessions.sh"
echo "Installed Repair AI Sessions in $scripts_dir"
echo 'Assign a shortcut to Repair AI Sessions in Vicinae settings (suggested: Cmd+Option+Shift+Space).'
