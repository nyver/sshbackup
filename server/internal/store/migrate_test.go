package store

import (
	"context"
	"path/filepath"
	"testing"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "backup.db")
	db, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	registerDBCleanup(t, db, path)
	return db
}

func TestMigrate_AppliesFromEmpty(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	tables := []string{
		"servers", "backup_jobs", "backup_sources", "job_scripts",
		"schedules", "retention_policies", "backup_runs", "run_steps",
		"job_locks", "settings", "schema_migrations",
	}
	for _, table := range tables {
		var name string
		err := db.QueryRowContext(ctx,
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?", table,
		).Scan(&name)
		if err != nil {
			t.Errorf("expected table %q to exist: %v", table, err)
		}
	}

	var settingsRows int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM settings").Scan(&settingsRows); err != nil {
		t.Fatalf("count settings rows: %v", err)
	}
	if settingsRows != 1 {
		t.Errorf("expected exactly one settings row, got %d", settingsRows)
	}
}

func TestMigrate_IdempotentOnRepeatedStart(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("first Migrate() error = %v", err)
	}
	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("second Migrate() error = %v", err)
	}

	var appliedCount int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&appliedCount); err != nil {
		t.Fatalf("count schema_migrations rows: %v", err)
	}
	if appliedCount != 1 {
		t.Errorf("expected exactly one applied migration row, got %d", appliedCount)
	}

	var settingsRows int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM settings").Scan(&settingsRows); err != nil {
		t.Fatalf("count settings rows: %v", err)
	}
	if settingsRows != 1 {
		t.Errorf("re-running migrations duplicated the seeded settings row: got %d", settingsRows)
	}
}

func TestParseMigrationFileName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		fileName    string
		wantVersion int
		wantName    string
		wantErr     bool
	}{
		{"0001_init.sql", 1, "init", false},
		{"0012_add_indexes.sql", 12, "add_indexes", false},
		{"init.sql", 0, "", true},
		{"0001.sql", 0, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.fileName, func(t *testing.T) {
			t.Parallel()
			version, name, err := parseMigrationFileName(tt.fileName)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseMigrationFileName(%q) error = %v, wantErr %v", tt.fileName, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if version != tt.wantVersion || name != tt.wantName {
				t.Errorf("parseMigrationFileName(%q) = (%d, %q), want (%d, %q)",
					tt.fileName, version, name, tt.wantVersion, tt.wantName)
			}
		})
	}
}
