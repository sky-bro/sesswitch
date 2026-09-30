# Security

## Reporting a vulnerability

Please do not open a public issue for a vulnerability that could expose local
session metadata, execute an unintended command, or focus an unverified process.
Use GitHub's private vulnerability reporting for this repository instead.

Include the affected Sesswitch version, operating system, reproduction steps,
and impact. Do not include API keys, transcripts, private prompts, or other
credentials.

## Local data boundary

Sesswitch does not need provider API keys. Chrome side-panel routing scans local
Codex rollouts on demand for selected-tab context and does not persist transcript
contents or browser URLs.
Provider hooks accept lifecycle metadata on standard input, normalize the
fields they need, and store local registry files with mode `0600` under
`~/.local/state/sesswitch/` by default.
