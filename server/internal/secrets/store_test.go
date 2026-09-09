package secrets

import (
	"runtime"
	"testing"

	"vpsbackupmanager/internal/domain"
)

func skipIfNotWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "windows" {
		t.Skip("Store depends on Windows DPAPI")
	}
}

func TestStore_SaveLoadRoundTrip(t *testing.T) {
	skipIfNotWindows(t)
	t.Parallel()
	store := NewStore(t.TempDir())

	reference, err := store.Save([]byte("hunter2"))
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if reference == "" {
		t.Fatal("expected a non-empty reference")
	}

	got, err := store.Load(reference)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if string(got) != "hunter2" {
		t.Errorf("Load() = %q, want %q", got, "hunter2")
	}
}

func TestStore_Replace(t *testing.T) {
	skipIfNotWindows(t)
	t.Parallel()
	store := NewStore(t.TempDir())

	reference, err := store.Save([]byte("old-passphrase"))
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := store.Replace(reference, []byte("new-passphrase")); err != nil {
		t.Fatalf("Replace() error = %v", err)
	}

	got, err := store.Load(reference)
	if err != nil {
		t.Fatalf("Load() after Replace() error = %v", err)
	}
	if string(got) != "new-passphrase" {
		t.Errorf("Load() after Replace() = %q, want %q", got, "new-passphrase")
	}
}

func TestStore_DeleteLifecycle(t *testing.T) {
	skipIfNotWindows(t)
	t.Parallel()
	store := NewStore(t.TempDir())

	reference, err := store.Save([]byte("hunter2"))
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := store.Delete(reference); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, err = store.Load(reference)
	code, ok := domain.CodeOf(err)
	if !ok || code != domain.ErrSSHAuthFailed {
		t.Fatalf("Load() after Delete() error = %v, want an SSH_AUTH_FAILED coded error", err)
	}

	// Deleting an already-absent reference is not an error.
	if err := store.Delete(reference); err != nil {
		t.Errorf("Delete() of an already-deleted reference error = %v, want nil", err)
	}
}

func TestStore_Load_MissingReference(t *testing.T) {
	skipIfNotWindows(t)
	t.Parallel()
	store := NewStore(t.TempDir())

	id, err := domain.NewID()
	if err != nil {
		t.Fatalf("domain.NewID() error = %v", err)
	}

	_, err = store.Load(id)
	code, ok := domain.CodeOf(err)
	if !ok || code != domain.ErrSSHAuthFailed {
		t.Fatalf("Load() of a missing reference error = %v, want an SSH_AUTH_FAILED coded error", err)
	}
}

func TestStore_Load_InvalidReferenceFormat(t *testing.T) {
	t.Parallel()
	store := NewStore(t.TempDir())

	if _, err := store.Load("../../etc/passwd"); err == nil {
		t.Fatal("expected an error for a non-hex reference")
	}
}
