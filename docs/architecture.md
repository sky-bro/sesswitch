# Architecture

Sesswitch normalizes three independent dimensions:

1. **Provider** — owns session discovery and provider-specific resume behavior
   (`codex`, `claude`).
2. **Host** — owns the live surface and focus mechanics (`tmux`, `wezterm`,
   later VS Code, browser, and other terminals).
3. **Launcher** — renders and selects normalized sessions (Vicinae today;
   Raycast and Rofi adapters later).

Providers must not focus windows. Hosts must not parse provider history.
Launchers must call the CLI contract instead of reading provider or registry
files directly. During the 0.x series, JSON consumers must ignore unknown
fields so the schema can evolve compatibly.

## Current package boundaries

- `internal/providers`: adapter contract, merge, global sort, de-duplication,
  and result limiting.
- `internal/providers/codex`: Codex app-server transport and normalization.
- `internal/providers/claude`: hook-observed Claude session catalog adapter.
- `internal/hosts`: conservative live tmux discovery plus verified tmux and
  WezTerm focus behavior.
- `internal/open`: provider fallback routing after live-host focus is not
  available, plus process execution with stale WezTerm socket recovery.
- `internal/presentation`: launcher-neutral display fields and the compact
  plain-text renderer used by dmenu-style launchers.
- `extensions/vicinae`: native React/TypeScript launcher adapter with provider
  image assets, subtitles, colored status tags, and actions backed by the Go
  CLI contract.
- `internal/registry`: provider-scoped location, activity, and explicit task
  state.
- `internal/state`: converts raw runtime observations into user-facing state.

Registry filenames use `<provider>--<session-id>` so equal native IDs from two
providers cannot collide. Reads retain compatibility with the original
unscoped Codex filenames.

## Live location reconciliation

Provider hooks remain the preferred source of terminal coordinates. Some Codex
versions execute hooks from a managed app-server, so the hook process tree has
no relationship to the CLI's TTY. For sessions without a registered location,
the host layer scans live tmux panes and requires all of the following:

- the pane working directory equals the session working directory;
- the pane title contains the normalized session title;
- the pane TTY contains a live executable for the same provider; and
- exactly one pane satisfies those checks.

The recovered location carries the agent PID and start time and is subject to
the normal focus-time identity check. Ambiguous matches are ignored and fall
back to provider routing rather than risking focus on the wrong conversation.
For a registered pane, discovery matches the pane ID and process identity
directly. This refreshes live title-derived state even after a provider-side
session rename, without weakening the identity check.
For a verified live Codex pane, `Action Required` in the terminal title is an
ephemeral host-state signal. It outranks historical turn-ended hook activity
and is cleared on the next scan when the title no longer requests action.

## Adapter contracts

A provider implements:

```go
type Adapter interface {
    Name() string
    List(context.Context, int) ([]session.Session, error)
}
```

Each provider adds its own hook subcommand and fallback opener without adding
provider conditionals to a host adapter. Hook entry points are provider-qualified
(`sesswitch hook codex`, `sesswitch hook claude`) and registry records carry the
provider used to verify the live executable.

## Claude integration

Claude sessions are indexed from documented hook events and resumed with
`claude --resume <session-id>`. Hooks fire in terminal, IDE, Desktop, and cloud
surfaces, but only a local CLI ancestor supplies a verifiable TTY/pane location.
The adapter deliberately does not parse transcript internals. Historical
discovery can be added when a documented machine-readable CLI/API is available.

This creates an explicit capability distinction: Codex currently provides full
history plus live observations, while Claude provides hook-observed sessions
plus live observations. The normalized output schema remains the same.

## Persistence

JSON files remain adequate while hooks are the only writers and the data set is
small. Adopt SQLite behind the registry interface when concurrent providers,
event history queries, or a watcher daemon make transactions and migrations
valuable. Do not introduce a daemon until push notifications or continuously
fresh launcher results require it.
