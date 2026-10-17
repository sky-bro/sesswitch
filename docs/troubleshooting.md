# Troubleshooting

[Home](../README.md) · [User guide](user-guide.md) · [Configuration](configuration.md)

Start with `sesswitch doctor`. It reports tool paths, missing commands, and
Codex app-server connectivity. Share sanitized output when reporting a bug.

## AI Sessions does not open

1. Check `vicinae ping`. If it cannot connect, launch Vicinae from Applications.
2. If Vicinae runs but AI Sessions fails, check its logs for extension errors.
3. If `extension-manager.js` is missing or the extension manager crashed, quit
   Vicinae completely and reopen it from Applications.
4. Verify the AI Sessions hotkey and **Sesswitch Binary** preference.

On macOS, use the normal application launch for a persistent Vicinae instance.
A foreground `vicinae server --replace` stays attached to the terminal or tool
session that starts it.

After changing source, `make install` updates the CLI and
`make install-vicinae-extension` updates the UI. Reopen AI Sessions afterward.

## A session is missing or stale

- Verify hooks are installed, loaded, and trusted; re-run the installer if the
  binary moved. Claude history before hook installation is not indexed.
- Clear **Filter Status** to **All**. Expand project history or search for it.
- Use **Refresh Sessions** after changes that do not emit hooks, or to reconnect
  a stopped subscription. Provider warnings mean cached metadata is being used.
- Provider-confirmed non-resumable history is hidden. An active input/approval
  request can still offer **Go to Original CLI**; it cannot be resumed separately.

## Opening chooses a new terminal or the wrong application

Inspect `sesswitch list --json` for the selected session's `location`, `host`,
and `display_state`. A stale, background, or ambiguous location uses provider
routing instead of focusing a possibly unrelated thread. Terminal provenance is
retained after a CLI closes, so its saved thread resumes in the terminal.

For a restarted CLI, automatic rebinding supports explicit
`codex resume <thread-UUID>` only when the new process, current title, cwd, and
competing claims can all be checked. Picker/`--last` recovery is not inferred.

If the pane changes but the window stays behind, check macOS automation
permissions for your launcher/terminal. Chrome tab focus also needs automation;
if exact tab routing fails, Sesswitch raises Chrome instead.

## Known limitations

- No polling: process exits and browser-tab closure may need a manual refresh.
- A Chrome **Ready to review** reminder can survive tab closure because review
  state and tab liveness are separate ([#6](https://github.com/sky-bro/sesswitch/issues/6)).
  Mark it read to acknowledge the completed turn; opening alone does not clear it.
- VS Code routing raises the app; exact editor-panel focus is not available.
- Sesswitch cannot remotely select another thread inside a running CLI. A
  background thread uses exact resume, or you switch it in the original CLI.
