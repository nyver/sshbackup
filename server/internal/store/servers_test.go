package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"vpsbackupmanager/internal/domain"
)

func openMigratedTestDB(t *testing.T) *DB {
	t.Helper()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "backup.db")
	db, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	registerDBCleanup(t, db, path)
	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	return db
}

func newTestServer(t *testing.T, name string) *domain.Server {
	t.Helper()
	s, err := domain.NewServer(time.Now(), domain.NewServerParams{
		Name: name, Host: "1.2.3.4", Username: "deploy",
		AuthType: domain.AuthPrivateKey, PrivateKeyPath: `C:\keys\id_ed25519`,
	})
	if err != nil {
		t.Fatalf("domain.NewServer() error = %v", err)
	}
	return s
}

func TestServerRepository_CreateGetListDelete(t *testing.T) {
	ctx := context.Background()
	db := openMigratedTestDB(t)
	repo := NewServerRepository(db)

	s := newTestServer(t, "prod")
	if err := repo.Create(ctx, s); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := repo.Get(ctx, s.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Name != s.Name || got.Host != s.Host || got.Port != s.Port {
		t.Errorf("Get() = %+v, want fields matching %+v", got, s)
	}
	if !got.CreatedAt.Equal(s.CreatedAt) {
		t.Errorf("CreatedAt round-trip mismatch: got %v, want %v", got.CreatedAt, s.CreatedAt)
	}

	list, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("List() returned %d servers, want 1", len(list))
	}

	if err := repo.Delete(ctx, s.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := repo.Get(ctx, s.ID); err == nil {
		t.Fatal("expected Get() to fail after Delete()")
	}
}

func TestServerRepository_Get_NotFound(t *testing.T) {
	ctx := context.Background()
	db := openMigratedTestDB(t)
	repo := NewServerRepository(db)

	_, err := repo.Get(ctx, "does-not-exist")
	code, ok := domain.CodeOf(err)
	if !ok || code != domain.ErrNotFound {
		t.Fatalf("Get() error = %v, want a NOT_FOUND coded error", err)
	}
}

func TestServerRepository_Delete_RefusedWhenReferencedByJob(t *testing.T) {
	ctx := context.Background()
	db := openMigratedTestDB(t)
	serverRepo := NewServerRepository(db)
	jobRepo := NewJobRepository(db)

	s := newTestServer(t, "prod")
	if err := serverRepo.Create(ctx, s); err != nil {
		t.Fatalf("Create() server error = %v", err)
	}

	j, err := domain.NewJob(time.Now(), domain.NewJobParams{
		Name: "beresta", ServerID: s.ID,
		Sources:          []domain.Source{{RemotePath: "/docker/volumes"}},
		Schedule:         domain.Schedule{Type: domain.ScheduleManual},
		LocalDestination: `D:\Backups\beresta`,
	})
	if err != nil {
		t.Fatalf("domain.NewJob() error = %v", err)
	}
	if err := jobRepo.Create(ctx, j); err != nil {
		t.Fatalf("Create() job error = %v", err)
	}

	err = serverRepo.Delete(ctx, s.ID)
	code, ok := domain.CodeOf(err)
	if !ok || code != domain.ErrInvalidConfig {
		t.Fatalf("Delete() error = %v, want an INVALID_CONFIGURATION coded error", err)
	}
}

func TestServerRepository_Update(t *testing.T) {
	ctx := context.Background()
	db := openMigratedTestDB(t)
	repo := NewServerRepository(db)

	s := newTestServer(t, "prod")
	if err := repo.Create(ctx, s); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	s.Host = "5.6.7.8"
	s.UpdatedAt = time.Now()
	if err := repo.Update(ctx, s); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	got, err := repo.Get(ctx, s.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Host != "5.6.7.8" {
		t.Errorf("Host = %q after update, want %q", got.Host, "5.6.7.8")
	}
}
