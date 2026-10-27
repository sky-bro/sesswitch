#!/bin/bash
# @vicinae.schemaVersion 1
# @vicinae.title Repair AI Sessions
# @vicinae.mode silent
# @vicinae.exec ["/bin/bash"]
# @vicinae.description Restart Vicinae and reopen AI Sessions when extensions stop working
# @vicinae.packageName Sesswitch

set -euo pipefail

command_timeout() {
  local seconds="$1" command_pid timer_pid status=0
  shift
  "$@" &
  command_pid=$!
  (
    command sleep "$seconds"
    kill -KILL "$command_pid" 2>/dev/null || true
  ) &
  timer_pid=$!
  wait "$command_pid" || status=$?
  kill "$timer_pid" 2>/dev/null || true
  wait "$timer_pid" 2>/dev/null || true
  return "$status"
}

restart_vicinae() {
  local vicinae="$1" user_id attempt deadline
  user_id="$(id -u)"
  if pgrep -u "$user_id" -x Vicinae >/dev/null; then
    if ! pkill -TERM -u "$user_id" -x Vicinae; then
      if pgrep -u "$user_id" -x Vicinae >/dev/null; then
        echo 'Could not stop Vicinae.' >&2
        return 1
      fi
    fi
  fi
  for ((attempt = 0; attempt < 50; attempt++)); do
    if ! pgrep -u "$user_id" -x Vicinae >/dev/null; then
      break
    fi
    sleep 0.1
  done
  if pgrep -u "$user_id" -x Vicinae >/dev/null; then
    echo 'Vicinae did not exit within 5 seconds; refusing to start a competing instance.' >&2
    return 1
  fi

  command_timeout 5 open -a "${SESSWITCH_VICINAE_APP:-Vicinae}" || return 1
  deadline=$((SECONDS + 10))
  for ((attempt = 0; attempt < 100 && SECONDS < deadline; attempt++)); do
    if command_timeout 1 "$vicinae" ping >/dev/null 2>&1; then
      command_timeout 5 "$vicinae" cmd launch '@sky-bro/sesswitch:ai-sessions'
      return $?
    fi
    sleep 0.1
  done
  echo 'Vicinae did not become ready within 10 seconds.' >&2
  return 1
}

repair_worker() (
  local vicinae="$1" state_dir="$2" lock
  lock="$state_dir/vicinae-repair.lock"
  # macOS shlock rejects a live owner and reclaims locks left by dead workers.
  if ! shlock -p "$$" -f "$lock"; then
    echo 'A Vicinae repair is already running.'
    return 0
  fi
  trap 'rm -f "$lock"' EXIT
  # Let the Launcher finish its script callback before terminating its process.
  sleep 0.3
  echo "$(date '+%Y-%m-%d %H:%M:%S') Restarting Vicinae"
  restart_vicinae "$vicinae"
)

main() {
  if [[ "$(uname -s)" != Darwin ]]; then
    echo 'This recovery shortcut currently supports macOS only.' >&2
    return 1
  fi
  local state_dir vicinae
  umask 077
  state_dir="${XDG_STATE_HOME:-$HOME/.local/state}/sesswitch"
  mkdir -p "$state_dir"
  if [[ "${1:-}" == --worker ]]; then
    repair_worker "$2" "$state_dir"
    return $?
  fi
  if [[ -n "${SESSWITCH_VICINAE:-}" ]]; then
    vicinae="$SESSWITCH_VICINAE"
  elif ! vicinae="$(command -v vicinae)"; then
    vicinae='/Applications/Vicinae.app/Contents/MacOS/vicinae-cli'
  fi
  if ! command -v "$vicinae" >/dev/null 2>&1; then
    echo "Vicinae CLI is unavailable: $vicinae" >&2
    return 1
  fi
  # The worker must survive the Launcher closing, including its stdio pipes.
  nohup /bin/bash "${BASH_SOURCE[0]}" --worker "$vicinae" \
    </dev/null >>"$state_dir/vicinae-repair.log" 2>&1 &
  echo 'Restarting Vicinae; AI Sessions will reopen shortly.'
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
