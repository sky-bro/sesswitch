# User guide

[Home](../README.md) · [Troubleshooting](troubleshooting.md) · [CLI](cli.md)

## Install and update

Requirements: macOS, Go 1.24+, and Codex or Claude Code. Terminal focus/resume
uses tmux and WezTerm. Vicinae needs Node.js/npm; hook installers need `jq`.
Install these tools separately before following the [quick start](../README.md).

`make install` builds the CLI and links `~/.local/bin/sesswitch` to this checkout.
Keep the checkout and add `~/.local/bin` to PATH. Set Vicinae's **Sesswitch Binary**
preference if using a different path.

Install only the hooks you need:

```sh
make install-codex-hook
make install-claude-hook
```

The installers preserve unrelated settings and save timestamped backups under
`~/.local/state/sesswitch/backups/`. In Codex, open `/hooks` and trust the seven
Sesswitch entries if prompted. Restart Claude Code after installing its hooks.
Claude sessions become discoverable after hooks observe them; Codex also lists
saved history.

To update an existing checkout:

```sh
git pull --ff-only
make install
make install-vicinae-extension
```

Reopen AI Sessions after updating. Re-run hook installation if the binary moved.
If the command is missing, quit Vicinae and reopen it from Applications.

## Choose and open a session

Open **AI Sessions** in Vicinae; assign a direct hotkey in Vicinae settings.
Search by session title, project path, provider, or host. Return opens the
selected session and closes the launcher.

Use the search-bar dropdown (`⌘P`) to choose:

| View | Order |
| --- | --- |
| Focus | Needs You, Active, Recent, Done |
| Recent | Most recently updated first |
| Project | Full project paths, abbreviated under your home directory |
| Agent | Codex or Claude |

Project view folds inactive and done history into **Show past sessions**.
Return expands or collapses it; searching includes folded history. Status
filtering lives in the actions menu, independently of grouping. Both choices
are remembered; an active status filter appears in the bottom title.

| Shortcut | Action |
| --- | --- |
| Return | Open selected session |
| `⌘↓` / `⌘↑` | Jump between groups (macOS) |
| `⌘B` | Open actions (with the configuration below) |
| `⌘E` | Rename a Codex session |
| `⌘⇧R` | Mark completed turn read |
| `⌘⇧D` | Mark task done or clear Done |
| `⌘⇧G` | Open Organize By |
| `⌘⇧O` | Cycle grouping |

Group navigation keeps all sessions visible: in Agent view, `⌘↓` jumps to the
next agent's first session, following the displayed group order. In Project
and Focus views, it jumps between projects and status groups. `⌘↑` first
returns to the start of the current group, then moves to the previous group;
it may first reveal a hidden group header. On Linux, use `Ctrl+↓` / `Ctrl+↑`.
These are Vicinae's built-in list controls.

Vicinae's own action-panel and dropdown shortcuts are configurable. See
[configuration](configuration.md#vicinae-shortcuts) if they differ on your setup.

## Understand states

| Label | Meaning |
| --- | --- |
| Needs input / Action needed | Waiting for your reply or approval |
| Working | Agent activity is observed |
| Ready to review | A turn ended and has not been marked read |
| Reviewed | You acknowledged that completed turn |
| Open | A live CLI is confirmed; no stronger runtime state is available |
| Idle | The provider reports idle |
| Closed / Saved | Closed CLI or saved history |
| Done | You explicitly marked the task finished |

Opening a session does not mark it read. **Ready to review** stays until you
mark it read or another runtime event changes its state. It does not mean the
whole task is finished. New agent activity clears an older Done mark.

The list reads a fresh catalog on open or **Refresh Sessions**, then streams
local hook/mark changes. It does not poll processes or providers. Refresh after
host changes that emit no hook. A review reminder can outlive its browser tab;
see [known limitations](troubleshooting.md#known-limitations).

A **Background** session shares a CLI currently displaying another thread;
opening it uses its exact resume command. **No saved data** sessions appear only
for input/approval requests and offer **Go to Original CLI**. That action focuses
the carrier; use the CLI itself to switch threads.
