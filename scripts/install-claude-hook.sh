#!/usr/bin/env bash

set -euo pipefail

if [[ -n "${SESSWITCH_BINARY:-}" ]]; then
  binary="$SESSWITCH_BINARY"
elif binary_path="$(command -v sesswitch 2>/dev/null)"; then
  binary="$binary_path"
else
  binary="$HOME/.local/bin/sesswitch"
fi
settings_file="${CLAUDE_SETTINGS_FILE:-${CLAUDE_CONFIG_DIR:-$HOME/.claude}/settings.json}"
state_dir="${XDG_STATE_HOME:-$HOME/.local/state}/sesswitch/backups"
repository_binary="$HOME/repos/sesswitch/bin/sesswitch"

command -v jq >/dev/null 2>&1 || {
  echo "jq is required" >&2
  exit 1
}

[[ -x "$binary" ]] || {
  echo "sesswitch is not executable: $binary" >&2
  exit 1
}

mkdir -p "$state_dir" "$(dirname "$settings_file")"
chmod 700 "$state_dir"

if [[ -e "$settings_file" ]]; then
  backup="$state_dir/claude-settings.json.$(date +%Y%m%d-%H%M%S).bak"
  cp -p "$settings_file" "$backup"
  input="$settings_file"
  created_input=false
else
  input="$(mktemp)"
  created_input=true
  printf '%s\n' '{}' > "$input"
fi

tmp="$(mktemp "$(dirname "$settings_file")/.settings.json.XXXXXX")"
cleanup() {
  rm -f "$tmp"
  if $created_input; then
    rm -f "$input"
  fi
}
trap cleanup EXIT

jq --arg command "$binary" --arg repository_binary "$repository_binary" '
  .hooks //= {} |
  reduce ["SessionStart", "UserPromptSubmit", "PermissionRequest", "PostToolUse", "Stop", "StopFailure", "CwdChanged", "SessionEnd"][] as $event (
    .;
    .hooks[$event] //= [] |
    .hooks[$event] |= map(.hooks |= map(select(.command != $repository_binary or .args != ["hook", "claude"]))) |
    .hooks[$event] |= map(select((.hooks | length) > 0)) |
    if any(.hooks[$event][]?.hooks[]?; .type == "command" and .command == $command and .args == ["hook", "claude"])
    then .
    else .hooks[$event] += [{"hooks":[{"type":"command","command":$command,"args":["hook","claude"],"timeout":5}]}]
    end
  ) |
  .hooks.Notification //= [] |
  .hooks.Notification |= map(.hooks |= map(select(.command != $repository_binary or .args != ["hook", "claude"]))) |
  .hooks.Notification |= map(select((.hooks | length) > 0)) |
  if any(.hooks.Notification[]?.hooks[]?; .type == "command" and .command == $command and .args == ["hook", "claude"])
  then .
  else .hooks.Notification += [{"matcher":"permission_prompt|idle_prompt|agent_needs_input|agent_completed","hooks":[{"type":"command","command":$command,"args":["hook","claude"],"timeout":5}]}]
  end
' "$input" > "$tmp"

chmod 600 "$tmp"
mv "$tmp" "$settings_file"
trap - EXIT

echo "installed Claude session hooks in $settings_file"
echo "Restart Claude Code or open /hooks to load and inspect the new hooks."
