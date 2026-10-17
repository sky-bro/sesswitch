# CLI reference

[Home](../README.md) · [Configuration](configuration.md) · [Architecture](architecture.md)

Session keys use `codex:<thread-id>` or `claude:<session-id>`. Copy a key from
Vicinae's actions menu or `sesswitch list --json`.

| Command | Purpose |
| --- | --- |
| `sesswitch list [--json] [--limit N]` | List sessions; default history limit 100, maximum 1000 |
| `sesswitch watch [--limit N]` | Stream snapshots on local record changes |
| `sesswitch open [--target auto\|terminal\|app\|host] <key>` | Focus or resume a session |
| `sesswitch pick` | Select through Vicinae dmenu (text fallback) |
| `sesswitch rename <key> <name>` | Rename a Codex thread |
| `sesswitch mark read\|done\|clear <key>` | Acknowledge a turn, finish a task, or clear its mark |
| `sesswitch hook codex\|claude` | Consume a lifecycle event from stdin; called by hooks |
| `sesswitch doctor` | Check tool resolution and Codex app-server connectivity |
| `sesswitch version` | Print the CLI version |

## Opening targets

- `auto` (default): focus a verified current host; otherwise use provider routing.
- `terminal`: resume the exact session in WezTerm.
- `app`: open a Codex Desktop deep link on macOS; unsupported for Claude.
- `host`: focus the original CLI without selecting a thread or resuming it.

```sh
sesswitch list --json
sesswitch open codex:THREAD_ID
sesswitch open --target terminal claude:SESSION_ID
sesswitch rename codex:THREAD_ID 'Investigate notification delivery'
sesswitch mark read codex:THREAD_ID
```

Host activation errors stop automatic opening. Stale or background locations may
fall back to exact resume; Sesswitch does not start another agent just because
window activation failed. Resuming requires saved data.

## JSON integration

`list --json` emits an array. `watch` emits one JSON object per line:

```json
{"sessions": [], "catalog_at": "2026-01-01T00:00:00Z"}
```

Snapshots can include `warnings`. Provider failures retain cached metadata and
report a warning. The history limit does not exclude hook-observed sessions
with non-closed activity. Each session includes its key, provider, title,
working directory, update time, runtime state, and optional location/task data.

`location.display_state` is `current`, `background`, or `unknown`. `host` retains
the last verified terminal type; it is not proof of a live location.
`resume: "unavailable"` means the provider confirmed no saved data. These records
are normally hidden unless there is an input/approval request with a carrier.

Consumers must ignore unknown fields during the 0.x series. Launchers should
use this CLI boundary rather than reading registry or provider files directly.
