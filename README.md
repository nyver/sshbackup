# SSH Backup Manager

An agentless, SSH-based backup orchestrator for self-hosted VPS servers on
Windows. A background Windows Service runs scheduled backups — stop a
remote service, archive its data, verify it, bring the service back up —
even when the desktop UI is closed or the archive step fails. A Flutter
desktop client provides configuration, live run progress, and history.

## Why

Ad-hoc scripts and Scheduled Tasks copy files but silently leave a
`docker compose down` service stopped when archiving breaks. VPS Backup
Manager treats "the service was restored" as a first-class, verified
outcome, reported separately from whether the archive itself succeeded.

## Requirements

- **To build/run the service:** Windows 10/11 x64, Go 1.26+ (see
  `server/go.mod`). No C compiler is required — the SQLite driver
  (`modernc.org/sqlite`) is pure Go.
- **To build/run the desktop client:** Flutter 3.35+ with the Windows
  desktop toolchain enabled (Visual Studio Build Tools with the "Desktop
  development with C++" workload, per the Flutter Windows setup docs).
- **Remote targets:** any host reachable over SSH with a POSIX shell
  providing `tar`, `gzip`, `sha256sum`, `du`, and `df`. Tested against
  Ubuntu/Debian.

## Install

There is no installer yet (see `docs/adr/` for packaging status). To run
from a build:

1. Build both binaries (see **Building** below).
2. Register the Windows Service — see **Service registration**.
3. Run the Flutter client to configure servers and jobs.

The service creates `%ProgramData%\VPSBackupManager\` (database, DPAPI
secrets, logs) on first start; no manual setup is needed beyond
registering the service.

## Building

### Server (Go)

```powershell
cd server
go build -o bin\vpsbackupservice.exe .\cmd\vpsbackupservice
```

Cross-compiling explicitly for Windows amd64 (useful from a non-Windows
build host, though the service only *runs* on Windows):

```powershell
$env:GOOS = "windows"; $env:GOARCH = "amd64"
go build -o bin\vpsbackupservice.exe -ldflags "-s -w" .\cmd\vpsbackupservice
```

### Client (Flutter)

```powershell
cd apps\client
flutter pub get
flutter build windows --release
```

The built app is at
`apps\client\build\windows\x64\runner\Release\vps_backup_manager.exe`.

## Service registration

Run from an elevated (Administrator) PowerShell prompt:

```powershell
.\scripts\install-service.ps1          # registers, Automatic startup
.\scripts\start-service.ps1
```

`install-service.ps1` defaults to `server\bin\vpsbackupservice.exe`; pass
`-ExePath` to point at a different build output (e.g. after copying it
into `C:\Program Files\VPSBackupManager\`). `stop-service.ps1` and
`uninstall-service.ps1` reverse the corresponding step. None of the
scripts copy files themselves — deploy the exe first, then register it.

For development and diagnostics, run the same binary directly instead of
as a service:

```powershell
.\server\bin\vpsbackupservice.exe --console
```

Console mode shares the exact same startup/wiring as the service; only
the process host differs, so it is a reliable way to reproduce a service
issue with visible stdout/stderr.

## Configuration

There is **no `config.yaml`** in this version — that is a deliberate
choice, not an oversight:

- The data directory is fixed at `%ProgramData%\VPSBackupManager\` (not
  configurable) so backups, secrets, and logs always live in a
  predictable, non-roaming location regardless of which user account
  starts the service.
- Everything a user can actually change at runtime — schedule pause,
  global concurrency limit, notification preferences, stale remote
  cleanup — is a row in the service's own SQLite `settings` table,
  edited from the desktop client's **Settings** screen and taking effect
  immediately (no restart, no file to hand-edit).
- Servers, jobs, schedules, and retention policies are configured
  entirely through the desktop client, not a file.
- The only command-line flag is `--console` (see **Service
  registration**).

If a later version adds file-based configuration, a `config.example.yaml`
will be added alongside it with every key documented; nothing here should
be read as implying one is planned.

## Running

1. Start the service (as a Windows Service, or `--console` for
   development).
2. Launch the Flutter client. It connects to the service over a local
   Named Pipe (`\\.\pipe\VPSBackupManager`); if the service is not
   reachable, the client shows a clear "service unavailable" state with a
   retry action and performs no backup work of its own.
3. Add a server (SSH host, username, and either a private key path or a
   password), confirm its host key fingerprint, then add a job (source
   paths, optional pre/post scripts, schedule, local destination).
4. Use `Validate job` to dry-run pre-flight checks (SSH reachability,
   host key, tooling, free space) without touching backup scripts, or
   `Run now` to start a real backup immediately.

Closing the client window does not stop the service — scheduled backups
keep running. The tray icon offers **Open**, **Run a job**, **Pause
schedules**, and **Exit UI** (which only closes the UI, never the
service).

## Least-privilege SSH user

The service executes your configured scripts and archive commands
**verbatim** with whatever permissions the SSH user has — this is
intentional (see `docs/adr/`) and is not filtered or sandboxed. Compensate
by using a dedicated, least-privilege SSH user for backups rather than
`root`:

- Grant only what the job actually needs: read access to the source
  paths, write access to the remote temp directory, and — if a script
  runs `docker compose down/up` or similar — membership in the relevant
  group (e.g. `docker`) rather than blanket `sudo`.
- Prefer private-key authentication with a dedicated key pair per server
  over reusing a personal key, and over password authentication —
  passwords are supported for servers that only offer them, but a key is
  not guessable/brute-forceable and can be revoked independently of the
  account's login password.
- Review each script's command text before saving; the client shows an
  unobtrusive warning for commands that look destructive (e.g. `rm -rf`,
  `docker compose down`) but never blocks or rewrites them.

## DPAPI machine-scope caveat

SSH key passphrases are stored using Windows DPAPI at
`CRYPTPROTECT_LOCAL_MACHINE` scope, under
`%ProgramData%\VPSBackupManager\secrets\`. These blobs are tied to the
machine they were created on: **they do not survive moving the data
directory to another machine, reinstalling Windows, or a bare-metal
restore.** After such an event, the service reports an actionable
"re-enter the credential on this machine" error for affected servers;
your job configuration is not lost, only the stored passphrase. Back up
your private key files separately — they are never stored by the
service, only referenced by path.

## Tests

```powershell
# Go
cd server
gofumpt -l .              # expect no output
go vet ./...
golangci-lint run
go test ./... -count=1
go test ./... -race -count=1   # requires CGO_ENABLED=1 / a C toolchain

# Flutter
cd apps\client
dart format --set-exit-if-changed .
flutter analyze
flutter test
```

## Project structure

```text
/apps/client/           Flutter desktop client (lib/, test/, windows/)
/server/                Go module (own go.mod)
    cmd/vpsbackupservice/    flags, service host, composition root
    internal/                domain, store, remote, backup, scheduler,
                              retention, secrets, ipc, notify, config
/protocol/               IPC wire contract: schemas, fixtures, README
/docs/
    architecture/         persisted-format documentation
    adr/                  architecture decision records
/scripts/                 service install/start/stop/uninstall
```

See `AGENTS.md` for the full engineering conventions this repository
follows (layering, testing, security, and commit rules).

## Troubleshooting

**Client shows "Background service unavailable".**
The service is not running, or its named pipe isn't reachable. Check
`Get-Service "SSH Backup Manager Service"`; if stopped, start it. In
`--console` mode, check stderr directly; otherwise check the structured
logs under `%ProgramData%\VPSBackupManager\logs\`.

**"SSH_HOST_KEY_UNVERIFIED" / "SSH_HOST_KEY_CHANGED" on connect.**
Expected on first connection to a server (trust-on-first-use — confirm
the fingerprint shown against what you expect from the host) or after the
remote host key genuinely changed (e.g. server reinstalled). The client
never offers a one-click accept for a *changed* key; verify the new
fingerprint out of band before trusting it.

**A job fails with a credential error after restoring the data
directory on a new machine.**
See **DPAPI machine-scope caveat** above — re-enter the affected
server's passphrase (or reconfirm a key with no passphrase) from the
client.

**`golangci-lint run` or `go test ./... -race` isn't available.**
The race detector and some `golangci-lint` checks require `CGO_ENABLED=1`
and a C toolchain. Where unavailable, note it explicitly in your report
rather than skipping silently.

## License

MIT — see [LICENSE](LICENSE).
