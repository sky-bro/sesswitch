# Contributing

See [Development](docs/development.md) for setup and required checks, and
[Architecture](docs/architecture.md) for the provider/host/launcher boundaries.

Before submitting:

- Run Go tests with the race detector, `go vet`, a build, extension checks, and
  shell checks as listed in the developer guide.
- Add regression coverage for behavior changes, especially process ownership,
  routing, state transitions, and persistence.
- Preserve existing JSON fields during the 0.x series; consumers ignore additions.
- Use synthetic fixtures. Never commit transcripts, prompts, API keys, private
  paths, or credentials.
- Update the relevant guide when changing user-facing behavior.

Bug reports should include the Sesswitch/macOS versions, provider and host,
expected behavior, reproduction steps, and sanitized diagnostics.
Report security issues privately as described in [SECURITY.md](SECURITY.md).
