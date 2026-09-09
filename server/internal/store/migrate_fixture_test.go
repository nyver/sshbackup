package store

import (
	"context"
	"path/filepath"
	"testing"
)

// TestMigrate_UpgradesFromPreviousLevel builds a database containing only
// the schema produced by every migration up to (but excluding) the latest
// one, seeds it with a realistic pre-upgrade row, then runs Migrate and
// checks it reaches the current schema — and that the seeded row survives
// with the new column correctly defaulted — without error.
func TestMigrate_UpgradesFromPreviousLevel(t *testing.T) {
	ctx := context.Background()
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatalf("loadMigrations() error = %v", err)
	}
	if len(migrations) == 0 {
		t.Fatal("expected at least one embedded migration")
	}

	path := filepath.Join(t.TempDir(), "backup.db")
	db, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	registerDBCleanup(t, db, path)

	// Apply every migration except the last, simulating a database created
	// by the previous release.
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE schema_migrations (
			version    INTEGER PRIMARY KEY,
			name       TEXT NOT NULL,
			applied_at TEXT NOT NULL
		)`); err != nil {
		t.Fatalf("create schema_migrations: %v", err)
	}
	for _, m := range migrations[:len(migrations)-1] {
		if err := applyMigration(ctx, db, m); err != nil {
			t.Fatalf("apply fixture migration %04d_%s: %v", m.version, m.name, err)
		}
	}

	// Seed a realistic pre-upgrade row so the upgrade is proven against
	// existing data, not just an empty schema.
	if _, err := db.ExecContext(ctx, `INSERT INTO servers
		(id, name, host, port, username, authentication_type, connection_timeout_seconds, command_timeout_seconds, created_at, updated_at)
		VALUES ('srv-1', 'prod', '203.0.113.10', 22, 'deploy', 'PRIVATE_KEY', 10, 30, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`); err != nil {
		t.Fatalf("seed fixture server: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO backup_jobs
		(id, name, server_id, enabled, archive_format, remote_temp_directory, local_destination, archive_timeout_seconds, created_at, updated_at)
		VALUES ('job-1', 'nightly', 'srv-1', 1, 'tar.gz', '/tmp/vps-backup-manager', 'D:\Backups', 3600, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`); err != nil {
		t.Fatalf("seed fixture job: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO backup_runs
		(id, job_id, server_id, source_paths, trigger, started_at, status)
		VALUES ('run-1', 'job-1', 'srv-1', '', 'MANUAL', '2026-01-02T02:00:00Z', 'FAILED')`); err != nil {
		t.Fatalf("seed fixture run: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO run_steps
		(id, run_id, step_type, position, started_at, status)
		VALUES ('step-1', 'run-1', 'PRE_BACKUP_SCRIPT', 0, '2026-01-02T02:00:00Z', 'FAILED')`); err != nil {
		t.Fatalf("seed fixture step (pre-dates the command column): %v", err)
	}

	// Now upgrade to the current schema.
	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("Migrate() upgrade from previous level error = %v", err)
	}

	var stepCommand string
	if err := db.QueryRowContext(ctx, "SELECT command FROM run_steps WHERE id = 'step-1'").Scan(&stepCommand); err != nil {
		t.Fatalf("expected the pre-existing step row to survive the upgrade with a defaulted command column: %v", err)
	}
	if stepCommand != "" {
		t.Errorf("command column for a pre-upgrade step = %q, want empty default", stepCommand)
	}

	latest := migrations[len(migrations)-1]
	var appliedName string
	if err := db.QueryRowContext(ctx, "SELECT name FROM schema_migrations WHERE version = ?", latest.version).
		Scan(&appliedName); err != nil {
		t.Fatalf("expected migration %04d to be recorded as applied: %v", latest.version, err)
	}
	if appliedName != latest.name {
		t.Errorf("applied migration name = %q, want %q", appliedName, latest.name)
	}
}
