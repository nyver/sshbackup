# 001. Service-plus-GUI architecture

## Context

VPS Backup Manager must keep running scheduled backups when the desktop UI is
closed, and must survive user logoff and machine reboot. It targets remote
Unix hosts over SSH/SFTP with no agent installed on the remote side, and must
be fully testable without a real VPS. See `proposal.md` and `design.md` for
the full rationale.

## Decision

- **Two processes, one binary.** `server/` builds a single
  `vpsbackupservice.exe` that runs either as a Windows Service
  (`golang.org/x/sys/windows/svc`) or in the foreground (`--console`) through
  the same composition root; only the lifecycle host differs. A separate
  `apps/client/` Flutter app is the GUI and performs no SSH work itself — it
  only talks to the service.
- **Named Pipes IPC.** The UI and service communicate over a Windows named
  pipe (`\\.\pipe\VPSBackupManager`) carrying newline-delimited JSON, with a
  security descriptor restricted to the interactive user and administrators,
  and an explicit `protocol_version` on every message. Rejected alternative:
  gRPC over localhost TCP, which would need firewalling and its own
  authentication where the named pipe gets OS-enforced ACLs for free.
- **Pure-Go SQLite driver.** Persistence uses `modernc.org/sqlite` (no CGO)
  instead of `mattn/go-sqlite3`, so `go test -race` and Windows
  cross-compilation work without a C toolchain. This is confined to the
  repository layer so the driver can be swapped later without touching engine
  code.

## Consequences

- The service is the only process that ever holds SSH credentials or performs
  SSH/SFTP operations; the UI can be closed, reinstalled, or absent entirely
  without affecting scheduled backups.
- Console mode and service mode share identical wiring, so console
  diagnostics are trustworthy stand-ins for the production service.
- A narrow consumer-side port around remote access (`Run`, `Download`,
  `Close`) is the one interface in the codebase introduced without a second
  production implementation, justified because it is what makes the critical
  failure scenario (specification §86) testable without a real VPS.
