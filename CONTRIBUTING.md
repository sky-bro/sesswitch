# Contributing

Thanks for helping improve Sesswitch.

Before opening a pull request:

```sh
go test ./...
go vet ./...
make build
make check-vicinae-extension
shellcheck scripts/*.sh
```

Keep the three main concerns separate:

- Provider adapters discover and normalize sessions.
- Host adapters verify and focus live surfaces.
- Launcher adapters render the CLI model and delegate actions back to it.

Please add tests for routing, state, registry, and provider behavior. Never add
fixtures containing real session transcripts, API keys, prompts, private paths,
or credentials. When changing `list --json`, preserve existing fields during
the 0.x series and document any new launcher-facing behavior.

Bug reports are most useful with the Sesswitch version, macOS version, provider
and host involved, expected behavior, and sanitized command output. Report
security issues privately as described in [SECURITY.md](SECURITY.md).
