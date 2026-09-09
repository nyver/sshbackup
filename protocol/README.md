# VPS Backup Manager — IPC wire contract

This directory is the cross-language contract between the Go service
(`server/internal/ipc`) and the Flutter client (`apps/client/lib/core/ipc`).
Both sides implement the same JSON shapes described here and validate them
against the fixtures in `fixtures/`.

## Transport

Newline-delimited JSON over a Windows named pipe
(`\\.\pipe\VPSBackupManager`), local-only, restricted to local
administrators and interactively logged-on users.

## Envelope

Every line on the wire is one JSON object with a `type` field:

- `request` (client → service): `id`, `protocol_version`, `command`, `payload`.
- `response` (service → client): `id` (echoing the request), `success`,
  and either `payload` (on success) or `error: {code, message}` (on
  failure).
- `event` (service → client, unsolicited): `event`, `payload`. Carries no
  `id`.

`protocol_version` is currently `1`. A request whose version does not
match the service's is answered with error code
`PROTOCOL_VERSION_MISMATCH` and never reaches a handler. An unrecognized
`command` is answered with `UNSUPPORTED_COMMAND`; the connection remains
usable for further requests either way.

## Commands

`servers.list` / `servers.create` / `servers.update` / `servers.delete`,
`servers.testConnection`, `servers.confirmHostKey`,
`jobs.list` / `jobs.create` / `jobs.update` / `jobs.delete` /
`jobs.enable` / `jobs.disable`, `jobs.validate`,
`runs.start`, `runs.cancel`, `runs.list`, `runs.get`,
`settings.get`, `settings.set`, `service.status`.

See `server/internal/ipc/messages.go` for the exact request/response
payload shapes (Go is the source of truth; Dart mirrors them by hand).

## Events

`run.started`, `run.stepChanged`, `run.finished`, `settings.changed`.

`run.logLine` is reserved in the contract for a future incremental log
stream; the MVP surfaces step output through `run.stepChanged`'s `step`
payload once a step finishes, not line-by-line while it runs.

## Fixtures

`fixtures/*.json` are example envelopes for representative commands and
events, used by both `server/internal/ipc`'s Go tests and the Flutter
client's Dart tests to catch a contract drift on either side.
