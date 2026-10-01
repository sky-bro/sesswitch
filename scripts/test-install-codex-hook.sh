#!/usr/bin/env bash
set -euo pipefail
repo_dir="$(cd "$(dirname "$0")/.." && pwd)"
test_dir="$(mktemp -d)"
trap 'rm -rf "$test_dir"' EXIT
export CODEX_HOOKS_FILE="$test_dir/hooks.json"
export XDG_STATE_HOME="$test_dir/state"
export SESSWITCH_BINARY="$test_dir/sesswitch"
printf '#!/bin/sh\nexit 0\n' > "$SESSWITCH_BINARY"
chmod +x "$SESSWITCH_BINARY"
bash "$repo_dir/scripts/install-codex-hook.sh" >/dev/null
jq --arg command "$SESSWITCH_BINARY hook codex" --arg repository_command "$HOME/repos/sesswitch/bin/sesswitch hook codex" '
  .hooks.SessionEnd[0].hooks[0].timeout = 5 |
  .hooks.Interrupt[0].hooks[0].timeout = 5 |
  .hooks.Stop += [{"hooks":[{"type":"command","command":"unrelated","timeout":17}]}] |
  .hooks.SessionEnd += [{"hooks":[{"type":"command","command":$repository_command,"timeout":5}]}]
' "$CODEX_HOOKS_FILE" > "$test_dir/old.json"
mv "$test_dir/old.json" "$CODEX_HOOKS_FILE"
bash "$repo_dir/scripts/install-codex-hook.sh" >/dev/null
bash "$repo_dir/scripts/install-codex-hook.sh" >/dev/null
jq -e --arg command "$SESSWITCH_BINARY hook codex" --arg repository_command "$HOME/repos/sesswitch/bin/sesswitch hook codex" '
  all(.hooks | to_entries[];
    .key as $event |
    [.value[].hooks[] | select(.command == $command)] as $ours |
    ($ours | length) == 1 and
    $ours[0].timeout == (if $event == "Interrupt" or $event == "SessionEnd" then 3 else 5 end)) and
  any(.hooks.Stop[].hooks[]; .command == "unrelated" and .timeout == 17) and
  all(.hooks.SessionEnd[].hooks[]; .command != $repository_command)
' "$CODEX_HOOKS_FILE" >/dev/null
printf 'Codex hook installer tests passed\n'
