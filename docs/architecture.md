# Architecture

[Home](../README.md) · [Development](development.md) · [CLI contract](cli.md)

## Boundaries

**Providers** discover and normalize sessions. **Hosts** verify and focus live
surfaces. **Launchers** render the model and delegate actions through the CLI.
Providers must not focus windows; launchers must not read registry or provider
files directly. Package locations are listed in the [developer guide](development.md#code-layout).

Codex uses app-server for history and exact thread reads. Claude uses
hook-observed metadata and `claude --resume`; it does not parse transcript
internals. Chrome routing reads selected-tab context on demand from the local
Codex rollout, an unstable format. Tab context is not serialized.

## Snapshot and event flow

```text
Provider catalog + process snapshot → local records → initial launcher snapshot
Provider hooks / task marks → atomic records → filesystem events → watch snapshot
```

Catalog queries and process discovery run on open/manual refresh. `watch`
subscribes before the initial load, watches directories for atomic replacements,
and coalesces bursts for 50 ms. Subsequent projections perform no provider
queries, process probes, or write-back. The view owns the subscription; closing
it stops the child. There is no polling or automatic reconnect service.

Each provider can fail independently; cached metadata and explicit warnings
remain available. Vicinae also caches the last successful array by binary path.
Cached states are advisory: opening revalidates the live target.

## Ownership and recovery

A location lease records provider, pane/TTY, PID, and process start time. A live
CLI may carry several threads. `display_state` separates carrier membership
from the currently displayed thread: `current`, `background`, or `unknown`.
Only a unique current conversation-title match inherits the pane's approval
indicator. Project/path suffixes are excluded from title matching.

An externally renamed Codex thread can leave an already-running TUI emitting
its old terminal title. On refresh/open, optional name history is read from
`$CODEX_HOME/session_index.jsonl` (default `~/.codex`), an unstable local format.
Old names apply only to an existing lease with unchanged verified PID/start/TTY,
matching cwd, and no peer with the same current or historical name. They cannot
discover a new lease or recover a replaced process. The launcher shows the new
name; focus rechecks the actual old title. Missing history keeps conservative
current-title matching. Historical alias lists are not serialized in Sesswitch records.

Hooks register terminal locations only from a verified CLI ancestor with a TTY.
Daemon/GUI hooks still record metadata and activity without trusting inherited
pane variables. Missing locations can be discovered from a unique title/cwd
match and a live provider process on the pane's TTY.

A changed PID/start time invalidates a registered lease. Recovery additionally
requires an exact `codex resume <thread-UUID>` argument match, current title/cwd,
and no competing title or lease on the new process. Native argv is inspected
only for recovery (Darwin sysctl or Linux procfs) and never retained. The process
identity/TTY is checked again after reading it. argv describes launch, so it
never overrides contrary current titles or existing leases after thread switches.
Picker/`--last`, inaccessible arguments, and ambiguous claims remain unrecovered.

## Opening

For provider-confirmed cached Codex threads, automatic opening first reconciles
all cached peers and tries a verified current tmux location, independently of
app-server availability. Hook-only/non-resumable records do not use this shortcut.
Provider lookup uses exact thread reads and includes cached peers when decorating
the result. A missing saved thread is never treated as proof that its record is
necessarily a `/side` conversation.

Focus verifies PID/start/provider/TTY and rechecks the conversation title. It
reuses an attached tmux client in WezTerm, or attaches the existing tmux session.
Stale/background targets can fall back to exact resume; other activation errors
stop to avoid duplicate agents. A retained `host` records terminal provenance
without asserting that its old location remains live. A new WezTerm GUI detaches
from request cancellation, process groups, and output pipes.

`--target host` explicitly focuses the carrier without selecting the thread.
Chrome focuses the recorded tab ID or a unique exact URL; failures raise Chrome.
VS Code routing currently raises the application.

## State and persistence

Runtime state, task completion, and host availability are independent. A Stop
hook means the turn ended. `mark read` acknowledges that turn; `mark done` is an
explicit task decision. New non-closed activity reopens Done. Browser closure
does not itself acknowledge a review reminder (see issue #6).

Private JSON records use `<provider>--<id>` names; reads accept legacy unscoped
records. Writers use atomic replacement; metadata/activity/task updates share a
file lock. Hook arrival timestamps are captured before process inspection;
older observations and late tool events cannot overwrite newer turn state.
This orders observed arrivals, not an invented provider event sequence.

## Execution environment

`internal/toolenv` resolves tools once per command and shares absolute paths,
configuration, and a child PATH across provider, host, and launcher operations.
npm agents run through the selected Node. WezTerm children receive explicit
tool/configuration locations because an existing GUI can have an older
environment. Only WezTerm calls drop an inherited socket when it no longer exists.
