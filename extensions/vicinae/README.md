# Sesswitch for Vicinae

Native Vicinae UI for Sesswitch. It reads structured session data from the Go
CLI and delegates every focus, resume, rename, and task-mark action back to the
same binary.

Each row uses a provider mark as its leading icon, the project as its subtitle,
a separate host icon, and a colored state tag. Agent and host names remain
searchable and appear in icon tooltips without repeating text in every row.

## Install from the repository

Build and install the CLI first from the repository root:

```sh
make install
make install-vicinae-extension
```

If Vicinae is already running and **AI Sessions** is missing, restart it with
`vicinae server --replace`.

The list displays its last successful result immediately, then subscribes to
`sesswitch watch`. The subscription fetches a fresh catalog once and streams
local hook/mark changes as JSON snapshots. There is no periodic polling.
Provider failures retain cached sessions with a visible warning. If the stream
stops, use Refresh Sessions to reconnect. Hosts without lifecycle hooks update
only on open or manual refresh; opening always verifies the target's live host.

Use `⌘⇧R` to mark a completed turn read and remove its review reminder. A later
turn can request review again. `⌘⇧D` marks a task done; new agent activity reopens
it so future approval requests are not hidden.

The extension's **Sesswitch Binary** preference defaults to
`~/.local/bin/sesswitch` and accepts another absolute path. Assign a direct
hotkey to **AI Sessions** in Vicinae settings for one-keystroke access.

## Develop

```sh
npm ci
npm run typecheck
npm run dev
```

Build to an explicit directory without replacing the installed development
copy:

```sh
npm run build -- --out /tmp/sesswitch-vicinae
```

The Codex icons come from the official desktop application's
`icon-codex-light.png` and `icon-codex-dark-color.png` resources, and switch
with the launcher's light or dark theme. The Claude and tmux marks come from
Simple Icons; those names and marks remain
the property of their respective owners. Their use identifies compatible
services and does not imply endorsement.
