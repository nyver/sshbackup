package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureDataDir(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	dir, err := EnsureDataDir(root)
	if err != nil {
		t.Fatalf("EnsureDataDir() error = %v", err)
	}
	want := filepath.Join(root, AppDirName)
	if dir != want {
		t.Errorf("EnsureDataDir() = %q, want %q", dir, want)
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Fatalf("expected %q to exist as a directory, stat error = %v", dir, err)
	}
}

func TestEnsureDataDir_Idempotent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	if _, err := EnsureDataDir(root); err != nil {
		t.Fatalf("first EnsureDataDir() error = %v", err)
	}
	dir, err := EnsureDataDir(root)
	if err != nil {
		t.Fatalf("second EnsureDataDir() error = %v", err)
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Fatalf("expected %q to still exist as a directory, stat error = %v", dir, err)
	}
}

func TestDataDir_MissingProgramData(t *testing.T) {
	t.Setenv("ProgramData", "")
	if _, err := DataDir(); err == nil {
		t.Fatal("expected an error when %ProgramData% is not set")
	}
}

func TestDatabasePath(t *testing.T) {
	t.Parallel()
	got := DatabasePath(`C:\ProgramData\VPSBackupManager`)
	want := filepath.Join(`C:\ProgramData\VPSBackupManager`, "backup.db")
	if got != want {
		t.Errorf("DatabasePath() = %q, want %q", got, want)
	}
}
