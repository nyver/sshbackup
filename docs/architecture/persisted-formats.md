# Persisted formats

This document is the authoritative reference for the three formats VPS
Backup Manager persists across runs, and the compatibility rule each one
follows. It exists so a future change to any of them is made
deliberately, with a migration path, rather than by accident.

## SQLite schema

**Location:** `%ProgramData%\VPSBackupManager\backup.db`.
**Current version:** `1` (migration `0001_init.sql`).

The schema is versioned via a `schema_migrations` table
(`version INTEGER PRIMARY KEY, name TEXT, applied_at TEXT`), populated by
numbered SQL files embedded at build time in
`server/internal/store/migrations/*.sql`. Each file is named
`<4-digit version>_<name>.sql` (e.g. `0001_init.sql`); the runner in
`server/internal/store/migrate.go` reads every embedded file, sorts by
version, and applies any not yet recorded in `schema_migrations`, each in
its own transaction. Applying the same set of migrations twice is a
no-op — an already-recorded version is skipped, not re-run.

**Compatibility rule:**

- A released migration file is never edited after it ships. A schema
  change — including a destructive one — ships as a new, higher-numbered
  migration file.
- A destructive migration (dropping/renaming a column or table) needs an
  explicit backfill/rollback plan in its own commit description; this
  repository has not needed one yet (schema version 1 is the only one
  that exists).
- Migration 0001 creates: `servers`, `backup_jobs`, `backup_sources`,
  `job_scripts`, `schedules`, `retention_policies`, `backup_runs`,
  `run_steps`, `job_locks`, `settings`, plus indexes including
  `backup_runs(job_id, started_at)`.
- Connection settings (WAL journal mode, foreign keys on, a busy
  timeout, a single writer connection) are applied at connection open,
  not stored in the schema itself — see `server/internal/store`.

## IPC protocol version

**Constant:** `ProtocolVersion` in `server/internal/ipc/envelope.go`
(currently `1`); mirrored as `kProtocolVersion` in
`apps/client/lib/core/ipc/envelope.dart`.

Every `request` envelope on the Named Pipe carries `protocol_version`.
The service compares it against its own `ProtocolVersion` in
`server/internal/ipc/router.go` **before** dispatching to a handler; a
mismatch is answered with a `PROTOCOL_VERSION_MISMATCH` error and the
request never reaches a command handler. The client surfaces this as "the
component versions don't match — update the application" rather than a
generic failure.

**Compatibility rule:**

- `ProtocolVersion` is bumped whenever a request/response/event payload
  shape changes in a way an older client or an older service could not
  safely interpret (a field removed, a field's meaning changed, a new
  required field added). Purely additive, optional fields do not require
  a bump, since existing decoders on both sides already ignore unknown
  keys and default missing optional ones.
- The full command/event vocabulary and one example envelope per shape
  are documented in `protocol/README.md`, with round-trip fixtures in
  `protocol/fixtures/*.json` exercised by both the Go and Dart test
  suites — a fixture that stops parsing on either side is the signal that
  a bump (or a coordinated fix) is needed.
- `server/internal/ipc/messages.go` (Go structs) is the source of truth
  for exact field names and JSON tags; `apps/client/lib/core/ipc/models.dart`
  mirrors it by hand and is expected to be kept in sync manually, not
  code-generated.

## Archive naming convention

**Function:** `domain.ArchiveFileName(jobName, startedAt)` in
`server/internal/domain/archive_name.go`.

```text
<sanitized job name>_<YYYY-MM-DD_HH-MM-SS>.tar.gz
```

- `startedAt` is formatted in the pattern above (24-hour, host-local
  time at the moment the run started — the same timestamp recorded as
  the run's `started_at`, formatted for filesystem use rather than
  RFC3339).
- The job name is sanitized for Windows filename safety: control
  characters (`< 0x20`) and any of Windows' reserved filename characters
  are replaced with `_`. The job name is not otherwise deduplicated or
  truncated, so two jobs with names that sanitize to the same string
  starting in the same second could in principle collide; this has not
  been a problem in practice at the target scale (tens of jobs) and is
  not separately guarded against.
- The same name is used for the temporary remote archive
  (`<job's remote temp dir>/<archive name>`), the local in-progress
  download (`<local destination>\<archive name>.part`), and the final
  verified file (`<local destination>\<archive name>`) — the `.part`
  suffix is stripped only by the atomic rename that follows a successful
  checksum comparison (`server/internal/backup/download.go`).

**Compatibility rule:** the naming function is pure and has no persisted
version of its own; a future format change (e.g. adding compression
variants per `ArchiveFormat`) should extend the pattern rather than
reinterpret an existing one, since already-downloaded archives on users'
disks are never renamed retroactively.
