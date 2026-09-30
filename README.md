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
the provider exposes them; exact browser-tab or editor-panel focus remains a
planned host adapter.

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
Vicinae and assign it a direct hotkey in Vicinae settings if you want to bypass
root search. The extension defaults to `~/.local/bin/sesswitch`; change the
**Sesswitch Binary** preference when using another path.

Inside the list:

- Return opens the selected session and closes the launcher.
- `⌘B` opens session actions (chosen to avoid Vicinae's `⌘J/K` navigation).
- `⌘E` renames a Codex session.
- `⌘⇧D` marks or unmarks your task as done.
- `⌘P` opens the organization picker; `⌘⇧O` cycles Focus, Recent, Project,
  and Agent.

The default **Focus** view puts approval requests, failures, interruptions, and
review-ready turns in **Needs You**, followed by active, recent, and explicitly
completed work.

## CLI

```text
sesswitch list [--json]
sesswitch pick
sesswitch open [--target auto|terminal|app] <provider:id>
sesswitch rename <provider:id> <name>
sesswitch mark done|clear <provider:id>
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

`list --json` is the integration boundary for other launchers. During the 0.x
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

Chrome side panels and VS Code currently expose no stable conversation
coordinate, so those routes can focus the application but cannot select the
exact conversation.

## Status semantics

`needs approval`, `working`, `turn ended · review`, `interrupted`, and
`session closed` are runtime observations, not guesses about task completion.
A `Stop` event means the current turn ended; it does not mean the user's task
is complete. A live tmux title containing `Action Required` overrides an older
turn-ended event. `mark done` is a separate, explicit user judgment.

A newly started Codex app-server may report historical threads as `notLoaded`.
That means the thread is not loaded in that app-server process, not that the
session is broken or completed.

## Privacy and storage

Sesswitch is local-first. It never reads or stores transcript contents. Its
registry contains session IDs, titles, working directories, process identity,
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
