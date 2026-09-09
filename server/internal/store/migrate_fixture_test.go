package store

import (
	"context"
	"path/filepath"
	"testing"
)

// TestMigrate_UpgradesFromPreviousLevel builds a database containing only
// the schema produced by every migration up to (but excluding) the latest
// one, then runs Migrate and checks it reaches the current schema without
// error. Today there is exactly one migration, so the "previous level" is
// the empty schema; as soon as a second migration is added, this test
// should instead apply migrations[:len-1] directly and assert the last one
// still upgrades a database seeded with realistic pre-upgrade data.
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

	// Now upgrade to the current schema.
	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("Migrate() upgrade from previous level error = %v", err)
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
