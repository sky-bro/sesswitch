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

The list displays its last successful result immediately and refreshes in the
background. Cached state may be outdated; opening still verifies the live host.

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

The Codex mark is a repository-owned compatibility glyph, not an OpenAI logo.
The Claude and tmux marks come from Simple Icons; those names and marks remain
the property of their respective owners. Their use identifies compatible
services and does not imply endorsement.
