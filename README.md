# Sesswitch

[![CI](https://github.com/sky-bro/sesswitch/actions/workflows/ci.yml/badge.svg)](https://github.com/sky-bro/sesswitch/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Find the AI session that needs you, then jump back to it.

Sesswitch puts Codex and Claude Code sessions in one searchable list. It shows
working, approval, and review states, focuses verified tmux/WezTerm panes, and
resumes saved sessions when their original terminal is unavailable.

## Quick start

The supported setup is **macOS**, with Go 1.24+, a provider CLI, and tmux/WezTerm
for terminal focus and resume. The optional Vicinae extension needs Node.js/npm;
hook installation needs `jq`.

```sh
git clone https://github.com/sky-bro/sesswitch.git
cd sesswitch
make install
sesswitch doctor
```

Install hooks for the providers you use, then add the launcher:

```sh
make install-codex-hook       # Codex
make install-claude-hook      # Claude Code
make install-vicinae-extension
```

Open **AI Sessions** in Vicinae and assign it a hotkey in settings. Codex may
require trusting the installed entries in `/hooks`. See the
[user guide](docs/user-guide.md) for setup and daily use.

## Support

| Capability | Codex | Claude Code |
| --- | --- | --- |
| Discovery | History through app-server | Hook-observed sessions |
| Exact terminal focus and resume | Yes | Yes |
| Rename | Yes | No supported API |
| Desktop/browser routing | Codex Desktop and Chrome | Terminal resume |

Vicinae is the shipped launcher. Exact VS Code panel focus, Raycast, and Rofi
adapters are not yet available.

## Documentation

- [User guide](docs/user-guide.md): setup, shortcuts, grouping, and status meanings.
- [CLI reference](docs/cli.md): commands and JSON integration.
- [Configuration](docs/configuration.md): tool paths and local storage.
- [Troubleshooting](docs/troubleshooting.md): missing sessions and opening failures.
- [Development](docs/development.md): checks, extension development, and layout.
- [Architecture](docs/architecture.md): state, ownership, routing, and recovery.

## License

[MIT](LICENSE). Provider names and marks identify compatible services; Sesswitch
is not affiliated with OpenAI, Anthropic, tmux, WezTerm, or Vicinae.
