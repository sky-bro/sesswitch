# Configuration

[Home](../README.md) · [User guide](user-guide.md) · [Troubleshooting](troubleshooting.md)

## Tool paths

Usually no configuration is needed. Run `sesswitch doctor` to see selected paths
and their source. Resolution order is environment override, configuration, PATH,
then standard installation directories. Invalid explicit paths fail visibly.

Configuration defaults to `~/.config/sesswitch/config.json`, respecting
`XDG_CONFIG_HOME`. Every entry is optional:

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

`SESSWITCH_CONFIG` selects another file. `SESSWITCH_CODEX`, `SESSWITCH_CLAUDE`,
`SESSWITCH_NODE`, `SESSWITCH_TMUX`, `SESSWITCH_WEZTERM`, and `SESSWITCH_VICINAE`
override individual tools. Paths accept `~/`; bare names are resolved as commands.
The legacy `AGENT_LOCATOR_CODEX` override remains supported.

Fallback directories cover Homebrew, user/system binaries, npm prefixes, WezTerm,
Volta, mise, nvm, asdf, and fnm. Shell startup files are not sourced. npm agent
entrypoints use the selected Node; native agents do not require Node. Children
launched through an existing WezTerm inherit explicit tool/configuration paths.

## Vicinae shortcuts

Set a direct hotkey for **AI Sessions** in Vicinae settings. The extension's
**Sesswitch Binary** preference defaults to `~/.local/bin/sesswitch`.

`make install-vicinae-extension` also installs the native **Repair AI Sessions**
script. To install just recovery, run `make install-vicinae-repair`. Assign it
**Cmd+Option+Shift+Space** in Vicinae settings for quick recovery when the
extension manager crashes. It restarts Vicinae and reopens AI Sessions.

For the `⌘P` dropdown and `⌘B` actions described in the user guide, merge these
keybindings into Vicinae's settings using its **Open Config File** command:

```json
{
  "keybinds": {
    "open-search-filter": "control+P",
    "toggle-action-panel": "control+B"
  }
}
```

Vicinae maps these control bindings to Command on macOS. Keeping actions on `⌘B`
avoids its `⌘J/K` navigation bindings.

## Storage and privacy

Private records live under `~/.local/state/sesswitch/`, respecting
`XDG_STATE_HOME`. They contain IDs, titles, paths, process identities, host
coordinates, lifecycle observations, and task marks; files use mode `0600`.

Sesswitch needs no provider API keys. Chrome routing reads selected-tab context
from local Codex rollouts on demand. Recovery reads process arguments on demand.
Neither browser URLs, transcripts, nor process arguments are stored in the
registry. See [Security](../SECURITY.md) for vulnerability reporting.
