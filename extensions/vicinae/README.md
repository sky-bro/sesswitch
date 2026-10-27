# Sesswitch for Vicinae

[English](README.md) | [简体中文](README.zh-CN.md)

The native launcher adapter for Sesswitch. It renders CLI snapshots and delegates
open, rename, and mark actions to the same binary.

- [Install and use](../../docs/user-guide.md)
- [Shortcuts and binary configuration](../../docs/configuration.md#vicinae-shortcuts)
- [Troubleshooting](../../docs/troubleshooting.md)
- [Development and checks](../../docs/development.md)

From the repository root:

```sh
make install
make install-vicinae-extension
```

For development in this directory:

```sh
npm ci
npm test
npm run typecheck
npm run dev
```

`npm run build -- --out /tmp/sesswitch-vicinae` builds without replacing the
installed copy. The generated `vicinae-env.d.ts` follows `package.json`.

Codex light/dark marks come from the official desktop application's icon
resources. Claude and tmux marks come from Simple Icons. These names and marks
belong to their owners and identify compatible services without endorsement.
