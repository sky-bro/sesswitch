# Sesswitch

[![CI](https://github.com/sky-bro/sesswitch/actions/workflows/ci.yml/badge.svg)](https://github.com/sky-bro/sesswitch/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Jump to the AI-agent session that needs you.

Sesswitch builds one searchable list of Codex and Claude Code sessions, shows
whether each session is working, waiting for approval, or ready to review, and
opens the best available location. A live tmux/WezTerm pane is focused in place;
otherwise Sesswitch resumes the session or uses an available app route.

## What works today

| Capability | Codex | Claude Code |
| --- | --- | --- |
| Session discovery | Full history through `codex app-server` | Sessions observed after hook installation |
| Runtime state | Hooks plus live tmux title | Hooks |
| Exact tmux/WezTerm focus | Yes | Yes |
| Terminal resume | `codex resume` | `claude --resume` |
| Rename | Yes | Not exposed by a supported API |
| Desktop deep link | Codex Desktop | Not available |

The polished launcher integration is currently Vicinae on macOS. The Go CLI and
JSON boundary are designed for future Raycast and Rofi adapters, but those
adapters are not shipped yet. Browser and VS Code sessions can be listed when
the provider exposes them. Chrome side-panel threads can restore their
originating tab on macOS; exact editor-panel focus remains planned.

## Requirements

The current supported setup is macOS with:

- Go 1.24 or newer and `jq` for building and hook installation.
- At least one provider CLI: Codex or Claude Code.
- tmux and WezTerm for exact terminal-pane focus and terminal resume.
- Vicinae plus Node.js/npm only if you want the native launcher extension.

Sesswitch does not install or configure an AI provider, credentials, tmux,
WezTerm, Node.js, or Vicinae for you.

## Quick start

```sh
git clone https://github.com/sky-bro/sesswitch.git ~/repos/sesswitch
cd ~/repos/sesswitch
make test
make install
```

`make install` builds Sesswitch and links it to
`~/.local/bin/sesswitch`. Ensure that directory is on `PATH`, then verify the
local integrations:

```sh
sesswitch doctor
sesswitch list
```

Install the hooks for the providers you use:

```sh
make install-codex-hook
make install-claude-hook
```

Both installers preserve unrelated configuration, write a timestamped backup
under `~/.local/state/sesswitch/backups/`, and are safe to run again. Codex may
require interactive trust approval: open `/hooks` in Codex and trust the seven
Sesswitch entries. Restart Claude Code or open `/hooks` after installing its
hooks.

### Add the Vicinae extension

```sh
make install-vicinae-extension
```

This installs locked npm dependencies, type-checks the extension, and lets
`vici build` install the local extension into Vicinae. Open **AI Sessions** in
Vicinae. If it is already running and the command is missing, restart it with
`vicinae server --replace`. Assign **AI Sessions** a direct hotkey in
Vicinae settings if you want to bypass root search. The extension defaults to `~/.local/bin/sesswitch`; change the
**Sesswitch Binary** preference when using another path.

Inside the list:

- Return opens the selected session and closes the launcher.
- `⌘B` opens session actions (chosen to avoid Vicinae's `⌘J/K` navigation).
- `⌘E` renames a Codex session.
- `⌘⇧R` marks the current completed turn as read.
- `⌘⇧D` marks or unmarks your task as done.
- `⌘P` opens the organization picker; `⌘⇧O` cycles Focus, Recent, Project,
  and Agent.

The default **Focus** view puts approval requests, failures, interruptions, and
review-ready turns in **Needs You**, followed by active, recent, and explicitly
completed work.
The open list subscribes to local hook and mark events through `sesswitch watch`.
It obtains a fresh catalog when opened or manually refreshed; there is no timer
that polls providers or host state. Completed turns marked read move out of
Needs You; a later completed turn becomes review-ready again. New agent activity
reopens work previously marked Done.

## CLI

```text
sesswitch list [--json]
sesswitch watch [--limit N]
sesswitch pick
sesswitch open [--target auto|terminal|app] <provider:id>
sesswitch rename <provider:id> <name>
sesswitch mark read|done|clear <provider:id>
sesswitch hook codex|claude
sesswitch doctor
sesswitch version
```

Examples:

```sh
sesswitch open codex:<thread-id>
sesswitch open claude:<session-id>
sesswitch open --target app codex:<thread-id>
sesswitch rename codex:<thread-id> "Investigate notification delivery"
```

`list --json` returns a one-shot array. `watch` emits newline-delimited JSON
snapshots containing `sessions`, optional `warnings`, and `catalog_at`. Each
provider is queried independently; an unavailable provider retains cached
metadata and produces an explicit warning. The history limit does not exclude
hook-observed sessions.

These commands are the integration boundary for other launchers. During the 0.x
series its schema may gain fields; consumers should ignore unknown fields.

## How routing works

When opening a session, Sesswitch verifies the registered process identity and
TTY before touching tmux. It then prefers, in order:

1. An already attached tmux client in an existing WezTerm pane.
2. A new tab in the active WezTerm window attached to the existing tmux
   session.
3. A provider-specific app route or an exact CLI resume command.

If the registered coordinate is stale, Sesswitch does not silently focus an
unrelated pane. It can conservatively recover a missing Codex coordinate from a
unique match of project, pane title, and live provider process. A detached tmux
session is attached; a duplicate agent process is not started merely because
pane activation failed.

Title matching excludes the project and path suffixes. If multiple sessions
claim one pane, only a unique conversation-title match keeps the live binding;
ambiguous claims cannot focus that pane. Historical sessions remain in the catalog.

On macOS, Chrome side-panel sessions focus the originating tab by its recorded
ID, with an exact URL fallback only when one live tab matches. If the tab cannot
be resolved or automation is unavailable, Sesswitch raises Chrome. Browser
context is read on demand from the local Codex rollout, whose format may change.
VS Code routes currently focus the application rather than the exact panel.

## Tool resolution

All launchers use the Go CLI's tool resolver. It chooses executables in this
order: explicit environment overrides, the user configuration file, the current
`PATH`, then standard installation directories. `sesswitch doctor` shows the
selected absolute path and where it came from. Invalid explicit settings fail
with a diagnostic rather than silently selecting another installation.

The default configuration is `~/.config/sesswitch/config.json`, or
`$XDG_CONFIG_HOME/sesswitch/config.json`. For example:

```json
{
  "tools": {
    "codex": "~/.local/bin/codex",
    "claude": "~/.local/bin/claude",
    "node": "/opt/homebrew/bin/node",
    "tmux": "/opt/homebrew/bin/tmux",
    "wezterm": "/Applications/WezTerm.app/Contents/MacOS/wezterm"
  }
}
```

Each entry is optional. `SESSWITCH_CONFIG` selects another configuration file.
`SESSWITCH_CODEX`, `SESSWITCH_CLAUDE`, `SESSWITCH_NODE`, `SESSWITCH_TMUX`,
`SESSWITCH_WEZTERM`, and `SESSWITCH_VICINAE` override individual tools; the legacy
`AGENT_LOCATOR_CODEX` override remains supported. Paths may use `~/`. A bare
executable name is searched in PATH and standard directories.

Fallback locations include user local/npm prefixes, Homebrew, system binaries,
the WezTerm app bundle, Volta, and installed Node versions from mise, nvm, asdf,
and fnm. Within each version manager, numeric versions are tried newest first;
explicit settings and PATH always take precedence. Shell startup files are not
sourced, and relative or empty PATH entries are ignored.

List, doctor, focus, and resume share absolute tool paths and an execution PATH
with the selected Node first. npm agent entrypoints run through that Node;
missing Node is reported before launching. Native agents do not require Node.
Commands spawned through an already-running WezTerm receive an absolute command
and the same PATH and tool/provider configuration locations explicitly, so they
do not depend on the GUI's original environment.
Vicinae needs only the Sesswitch binary path.

## Update architecture

```text
Provider hooks → private local records → filesystem events
                                           ↓
                                    sesswitch watch
                                           ↓
                                    launcher snapshot
```

The subscription lives only as long as the launcher view. It watches directories
because records are replaced atomically, coalesces write bursts for 50 ms, and
projects local state without querying providers, probing processes, or writing
records back. Catalog queries and one shared process snapshot run only on open
or explicit refresh. Focus always revalidates the target immediately.

Hook writers capture arrival time before process inspection and serialize
activity updates. Delayed older writes and late tool events for a completed
turn cannot overwrite newer state. This orders observed hook arrivals; it does
not invent a provider event sequence when the provider supplies none.

Real-time updates require installed, loaded, trusted hooks that emit the relevant
events. Hosts without such events provide snapshot-only state: changes appear
on the next open or manual refresh. Closing a browser tab or forcibly killing an
agent may not emit SessionEnd. Those lifecycle changes are not detected while
the list remains open. A failed subscription is shown explicitly and requires
manual refresh; there is no periodic reconciliation or automatic retry service.

## Status semantics

`needs approval`, `working`, `turn ended · review`, `interrupted`, and
`session closed` are runtime observations, not guesses about task completion.
A `Stop` event means the current turn ended; it does not mean the user's task
is complete. A live tmux title containing `Action Required` overrides an older
turn-ended event. `mark read` acknowledges the observed completed turn without
finishing the task. `mark done` is a separate, explicit user judgment; newer
agent activity makes that mark inactive, while SessionEnd alone does not.

A tmux session verified during the current listing is shown as `session open`
when no stronger runtime signal exists. This confirms that its process is live;
it does not imply that the model is generating.

A newly started Codex app-server may report historical threads as `notLoaded`.
That means the thread is not loaded in that app-server process, not that the
session is broken or completed.

## Privacy and storage

Sesswitch is local-first. To open Chrome side-panel sessions, it scans the local
Codex rollout for selected-tab context; it does not persist transcript contents
or browser URLs. Its registry contains session IDs, titles, working directories, process identity,
host coordinates, lifecycle observations, and explicit task marks under
`~/.local/state/sesswitch/`; private registry files use mode `0600`. Titles and
previews for Codex are fetched from the local app-server while listing.

## Troubleshooting

- Run `sesswitch doctor` first; it reports missing commands and checks the Codex
  app-server when Codex is installed.
- If selection changes the tmux pane but not the foreground window, grant
  launcher/terminal automation permissions in macOS Privacy & Security.
- If an item opens a new WezTerm window, run `sesswitch list --json` and check
  whether it has a live `location`. Stale or ambiguous coordinates deliberately
  fall back instead of focusing a possibly wrong pane.
- Re-run the relevant hook installer after moving the repository or binary.
- In Vicinae, verify the command's **Sesswitch Binary** preference when the list
  fails to load.

See [the architecture notes](docs/architecture.md) for adapter boundaries and
implementation rationale, and [the Vicinae guide](extensions/vicinae/README.md)
for extension development.

## Development

```sh
make test
go test -race ./...
go vet ./...
make check-vicinae-extension
```

`make check` additionally builds the binary and runs the environment-dependent
doctor command. Contributions are welcome; please keep provider discovery,
host focus, and launcher rendering behind their existing adapter boundaries.

## License

MIT. Provider names and marks belong to their respective owners and are used
only to identify compatible services. Sesswitch is not affiliated with or
endorsed by OpenAI, Anthropic, tmux, WezTerm, or Vicinae.
