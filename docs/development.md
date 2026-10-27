# Development

[English](development.md) | [简体中文](zh-CN/development.md)

[Home](../README.md) · [Architecture](architecture.md) · [Contributing](../CONTRIBUTING.md)

Use Go 1.24+, Node.js 24+ with npm, `jq`, and ShellCheck. Local host integration
is supported on macOS; unit tests use fake providers, processes, and commands.
Provider credentials are not needed for the test suite.

## Checks

From the repository root:

```sh
make test                         # Go, hook installer, and recovery script tests
go test -race ./...
go vet ./...
make build
shellcheck scripts/*.sh
```

Install the extension's locked dependencies once, then check it:

```sh
cd extensions/vicinae
npm ci
```

Back at the repository root:

```sh
make check-vicinae-extension      # tests, typecheck, build into bin/vicinae
```

`make check` additionally runs `sesswitch doctor`, which depends on locally
installed tools and a working provider. CI checks Go on macOS, the extension
on Linux with Node.js 24, and shell scripts with ShellCheck.

## Run and install

```sh
./bin/sesswitch list --json
./bin/sesswitch watch
make install                     # links ~/.local/bin/sesswitch to this checkout
make install-vicinae-extension    # replaces the installed local extension
```

For extension development, run `npm run dev` in `extensions/vicinae`. A standalone
build avoids replacing the installed extension:

```sh
npm run build -- --out /tmp/sesswitch-vicinae
```

`vicinae-env.d.ts` is generated from the manifest; do not edit it by hand.
Reopen AI Sessions to load an updated CLI/extension. Launch Vicinae through
Applications when it needs to persist independently of your development shell.

## Code layout

| Directory | Responsibility |
| --- | --- |
| `cmd/sesswitch` | Commands, catalog snapshots, hook handling, event stream |
| `internal/providers` | Provider discovery and normalization |
| `internal/hosts` | Live ownership discovery and verified host focus |
| `internal/process` | Process identity, TTY lookup, explicit resume evidence |
| `internal/open` | Fallback routing and detached GUI startup |
| `internal/registry`, `internal/state`, `internal/session` | Persistence, state resolution, shared model |
| `internal/toolenv` | Shared tool selection and execution environment |
| `internal/presentation`, `extensions/vicinae` | Text and native launcher adapters |

Add regression tests for ownership ambiguity, process reuse/exit, routing
fallbacks, and state transitions when changing those behaviors. Keep fixtures
synthetic and avoid storing prompts, transcripts, private paths, or credentials.
