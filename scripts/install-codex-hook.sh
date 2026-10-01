#!/usr/bin/env bash

set -euo pipefail

if [[ -n "${SESSWITCH_BINARY:-}" ]]; then
  binary="$SESSWITCH_BINARY"
elif [[ -n "${AGENT_LOCATOR_BINARY:-}" ]]; then
  binary="$AGENT_LOCATOR_BINARY"
elif binary_path="$(command -v sesswitch 2>/dev/null)"; then
  binary="$binary_path"
else
  binary="$HOME/.local/bin/sesswitch"
fi
hooks_file="${CODEX_HOOKS_FILE:-$HOME/.codex/hooks.json}"
state_dir="${XDG_STATE_HOME:-$HOME/.local/state}/sesswitch/backups"
legacy_command="$HOME/repos/agent-locator/bin/agent-locator hook"
previous_command="$binary hook"
repository_command="$HOME/repos/sesswitch/bin/sesswitch hook"

command -v jq >/dev/null 2>&1 || {
  echo "jq is required" >&2
  exit 1
}

[[ -x "$binary" ]] || {
  echo "sesswitch is not executable: $binary" >&2
  exit 1
}

mkdir -p "$state_dir" "$(dirname "$hooks_file")"
chmod 700 "$state_dir"

if [[ -e "$hooks_file" ]]; then
  backup="$state_dir/hooks.json.$(date +%Y%m%d-%H%M%S).bak"
  cp -p "$hooks_file" "$backup"
  input="$hooks_file"
  created_input=false
else
  input="$(mktemp)"
  created_input=true
  printf '%s\n' '{"hooks":{}}' > "$input"
fi

tmp="$(mktemp "$(dirname "$hooks_file")/.hooks.json.XXXXXX")"
cleanup() {
  rm -f "$tmp"
  if $created_input; then
    rm -f "$input"
  fi
}
trap cleanup EXIT

jq --arg command "$binary hook codex" --arg legacy_command "$legacy_command" --arg previous_command "$previous_command" --arg repository_command "$repository_command" '
  .hooks //= {} |
  reduce ["SessionStart", "UserPromptSubmit", "PermissionRequest", "PostToolUse", "Stop", "Interrupt", "SessionEnd"][] as $event (
    .;
    (if $event == "Interrupt" or $event == "SessionEnd" then 3 else 5 end) as $timeout |
    .hooks[$event] //= [] |
    .hooks[$event] |= map(.hooks |= map(select(.command != $legacy_command and .command != $previous_command and .command != $repository_command and .command != ($repository_command + " codex")))) |
    .hooks[$event] |= map(select((.hooks | length) > 0)) |
    if any(.hooks[$event][]?.hooks[]?; .type == "command" and .command == $command)
    then .hooks[$event] |= map(.hooks |= map(if .type == "command" and .command == $command then .timeout = $timeout else . end))
    else .hooks[$event] += [{"hooks":[{"type":"command","command":$command,"timeout":$timeout}]}]
    end
  )
' "$input" > "$tmp"

chmod 600 "$tmp"
mv "$tmp" "$hooks_file"
trap - EXIT

echo "installed Codex location hooks in $hooks_file"
echo "Open /hooks in Codex and trust each new Sesswitch hook before it can run."
