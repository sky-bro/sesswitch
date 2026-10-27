#!/bin/bash
set -euo pipefail

repo_dir="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=scripts/repair-ai-sessions.sh
source "$repo_dir/scripts/repair-ai-sessions.sh"
test_dir="$(mktemp -d)"
trap 'rm -rf "$test_dir"' EXIT

# A CLI that connects but never replies must not hold the repair lock forever.
started=$SECONDS
if command_timeout 1 bash -c 'exec sleep 30' 2>/dev/null; then
  echo 'Expected an unresponsive command to time out' >&2
  exit 1
fi
[[ "$((SECONDS - started))" -lt 5 ]]
command_timeout 1 bash -c 'exit 0'
if command_timeout 1 bash -c 'exit 7'; then
  echo 'Expected command failures to propagate' >&2
  exit 1
else
  [[ "$?" == 7 ]]
fi

# Keep the restart state-machine cases synchronous; timeout behavior is tested above.
command_timeout() { shift; "$@"; }
pgrep() { [[ "$running" == true ]]; }
pkill() { stopped=true; [[ "$stuck" == true ]] || running=false; }
sleep() { :; }
open() {
  [[ "$running" == false ]] || return 1
  opened=true
  [[ "$open_fails" == false ]]
}
fake_vicinae() {
  if [[ "$1" == ping ]]; then
    probes=$((probes + 1))
    [[ "$ready" == true && "$probes" -ge 2 ]]
  else
    [[ "$*" == 'cmd launch @sky-bro/sesswitch:ai-sessions' ]] || return 1
    launched=true
  fi
}
reset_case() {
  running=true stopped=false stuck=false opened=false open_fails=false
  ready=true probes=0 launched=false
}

reset_case
restart_vicinae fake_vicinae
[[ "$stopped" == true && "$opened" == true && "$launched" == true ]]

reset_case
running=false
restart_vicinae fake_vicinae
[[ "$stopped" == false && "$opened" == true && "$launched" == true ]]

reset_case
stuck=true
if restart_vicinae fake_vicinae 2>/dev/null; then
  echo 'Expected shutdown timeout to fail' >&2
  exit 1
fi
[[ "$opened" == false && "$launched" == false ]]

reset_case
open_fails=true
if restart_vicinae fake_vicinae; then
  echo 'Expected application launch failure to propagate' >&2
  exit 1
fi
[[ "$launched" == false ]]

reset_case
ready=false
if restart_vicinae fake_vicinae 2>/dev/null; then
  echo 'Expected readiness timeout to fail' >&2
  exit 1
fi
[[ "$probes" == 100 && "$launched" == false ]]

# A second repair worker must not interrupt a repair that is already running.
reset_case
shlock() { return 1; }
repair_worker fake_vicinae "$test_dir" >/dev/null
[[ "$stopped" == false && "$opened" == false ]]

# Successful workers release their lock, including when restart fails.
shlock() { echo "$$" > "$4"; }
reset_case
repair_worker fake_vicinae "$test_dir" >/dev/null
[[ ! -e "$test_dir/vicinae-repair.lock" ]]
reset_case
open_fails=true
if repair_worker fake_vicinae "$test_dir" >/dev/null; then
  echo 'Expected worker failure to propagate' >&2
  exit 1
fi
[[ ! -e "$test_dir/vicinae-repair.lock" ]]

VICINAE_SCRIPTS_DIR="$test_dir/scripts" bash "$repo_dir/scripts/install-vicinae-repair.sh" >/dev/null
cmp "$repo_dir/scripts/repair-ai-sessions.sh" "$test_dir/scripts/repair-ai-sessions.sh"
[[ -x "$test_dir/scripts/repair-ai-sessions.sh" ]]
echo 'Vicinae recovery script tests passed'
