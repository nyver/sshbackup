package store

import (
	"context"
	"testing"
	"time"

	"vpsbackupmanager/internal/domain"
)

func newTestJobParams(serverID string) domain.NewJobParams {
	keepLast := 5
	return domain.NewJobParams{
		Name:     "beresta",
		ServerID: serverID,
		Enabled:  true,
		Sources: []domain.Source{
			{RemotePath: "/docker/volumes", Position: 0, Exclude: []string{"*.log", "cache/"}},
			{RemotePath: "/home/dev/config", Position: 1},
		},
		Scripts: []domain.Script{
			{Type: domain.ScriptPreBackup, Command: "docker compose down", Position: 0, RunCondition: domain.RunConditionAlways, CriticalCleanup: true},
			{Type: domain.ScriptPostBackup, Command: "docker compose up -d", Position: 0, RunCondition: domain.RunConditionAlways, CriticalCleanup: true},
		},
		Schedule: domain.Schedule{
			Type: domain.ScheduleWeekly, Hour: 3, Minute: 30,
			Weekdays: []time.Weekday{time.Monday, time.Wednesday, time.Friday},
		},
		RetentionPolicy:     domain.RetentionPolicy{KeepLast: &keepLast},
		LocalDestination:    `D:\Backups\beresta`,
		RemoteTempDirectory: "/tmp/vps-backup-manager",
	}
}

func createTestServerAndJob(t *testing.T, db *DB) (*domain.Server, *domain.Job) {
	t.Helper()
	ctx := context.Background()
	s := newTestServer(t, "prod")
	if err := NewServerRepository(db).Create(ctx, s); err != nil {
		t.Fatalf("create server: %v", err)
	}
	j, err := domain.NewJob(time.Now(), newTestJobParams(s.ID))
	if err != nil {
		t.Fatalf("domain.NewJob() error = %v", err)
	}
	if err := NewJobRepository(db).Create(ctx, j); err != nil {
		t.Fatalf("create job: %v", err)
	}
	return s, j
}

func TestJobRepository_CreateGetRoundTrip(t *testing.T) {
	ctx := context.Background()
	db := openMigratedTestDB(t)
	repo := NewJobRepository(db)

	_, j := createTestServerAndJob(t, db)

	got, err := repo.Get(ctx, j.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Name != j.Name || got.ServerID != j.ServerID {
		t.Errorf("Get() = %+v, want fields matching %+v", got, j)
	}
	if len(got.Sources) != 2 {
		t.Fatalf("Sources = %d, want 2", len(got.Sources))
	}
	if got.Sources[0].RemotePath != "/docker/volumes" || len(got.Sources[0].Exclude) != 2 {
		t.Errorf("Sources[0] = %+v, want remote_path=/docker/volumes with 2 excludes", got.Sources[0])
	}
	if len(got.Scripts) != 2 {
		t.Fatalf("Scripts = %d, want 2", len(got.Scripts))
	}
	if got.Schedule.Type != domain.ScheduleWeekly || len(got.Schedule.Weekdays) != 3 {
		t.Errorf("Schedule = %+v, want WEEKLY with 3 weekdays", got.Schedule)
	}
	if got.RetentionPolicy.KeepLast == nil || *got.RetentionPolicy.KeepLast != 5 {
		t.Errorf("RetentionPolicy.KeepLast = %v, want 5", got.RetentionPolicy.KeepLast)
	}
}

func TestJobRepository_List_BatchLoadsChildren(t *testing.T) {
	ctx := context.Background()
	db := openMigratedTestDB(t)
	_, _ = createTestServerAndJob(t, db)

	s := newTestServer(t, "staging")
	if err := NewServerRepository(db).Create(ctx, s); err != nil {
		t.Fatalf("create second server: %v", err)
	}
	params := newTestJobParams(s.ID)
	params.Name = "family-hub"
	j2, err := domain.NewJob(time.Now(), params)
	if err != nil {
		t.Fatalf("domain.NewJob() error = %v", err)
	}
	if err := NewJobRepository(db).Create(ctx, j2); err != nil {
		t.Fatalf("create second job: %v", err)
	}

	jobs, err := NewJobRepository(db).List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("List() returned %d jobs, want 2", len(jobs))
	}
	for _, j := range jobs {
		if len(j.Sources) != 2 {
			t.Errorf("job %q Sources = %d, want 2", j.Name, len(j.Sources))
		}
		if len(j.Scripts) != 2 {
			t.Errorf("job %q Scripts = %d, want 2", j.Name, len(j.Scripts))
		}
	}
}

func TestJobRepository_Update_ReplacesChildren(t *testing.T) {
	ctx := context.Background()
	db := openMigratedTestDB(t)
	repo := NewJobRepository(db)
	_, j := createTestServerAndJob(t, db)

	j.Sources = []domain.Source{{RemotePath: "/etc/nginx", Position: 0}}
	j.Scripts = nil
	j.UpdatedAt = time.Now()
	if err := repo.Update(ctx, j); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	got, err := repo.Get(ctx, j.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if len(got.Sources) != 1 || got.Sources[0].RemotePath != "/etc/nginx" {
		t.Errorf("Sources after update = %+v, want a single /etc/nginx source", got.Sources)
	}
	if len(got.Scripts) != 0 {
		t.Errorf("Scripts after update = %d, want 0", len(got.Scripts))
	}
}

func TestJobRepository_Delete_ConfigOnlyKeepsHistory(t *testing.T) {
	ctx := context.Background()
	db := openMigratedTestDB(t)
	jobRepo := NewJobRepository(db)
	runRepo := NewRunRepository(db)
	_, j := createTestServerAndJob(t, db)

	run, err := domain.NewRun(time.Now(), j.ID, j.ServerID, []string{"/docker/volumes"}, domain.TriggerManual)
	if err != nil {
		t.Fatalf("domain.NewRun() error = %v", err)
	}
	run.Finish(time.Now(), domain.RunSuccess)
	if err := runRepo.Create(ctx, run); err != nil {
		t.Fatalf("create run: %v", err)
	}

	if err := jobRepo.Delete(ctx, j.ID, false); err != nil {
		t.Fatalf("Delete(deleteHistory=false) error = %v", err)
	}

	if _, err := jobRepo.Get(ctx, j.ID); err == nil {
		t.Fatal("expected job to be gone after delete")
	}
	got, err := runRepo.Get(ctx, run.ID)
	if err != nil {
		t.Fatalf("expected run history to survive config-only delete, Get() error = %v", err)
	}
	if got.JobID != j.ID {
		t.Errorf("orphaned run JobID = %q, want %q", got.JobID, j.ID)
	}
}

func TestJobRepository_Delete_WithHistory(t *testing.T) {
	ctx := context.Background()
	db := openMigratedTestDB(t)
	jobRepo := NewJobRepository(db)
	runRepo := NewRunRepository(db)
	_, j := createTestServerAndJob(t, db)

	run, err := domain.NewRun(time.Now(), j.ID, j.ServerID, []string{"/docker/volumes"}, domain.TriggerManual)
	if err != nil {
		t.Fatalf("domain.NewRun() error = %v", err)
	}
	if err := runRepo.Create(ctx, run); err != nil {
		t.Fatalf("create run: %v", err)
	}

	if err := jobRepo.Delete(ctx, j.ID, true); err != nil {
		t.Fatalf("Delete(deleteHistory=true) error = %v", err)
	}
	if _, err := runRepo.Get(ctx, run.ID); err == nil {
		t.Fatal("expected run history to be removed when deleteHistory=true")
	}
}

func TestJobRepository_Delete_RefusedWhileRunning(t *testing.T) {
	ctx := context.Background()
	db := openMigratedTestDB(t)
	jobRepo := NewJobRepository(db)
	lockRepo := NewLockRepository(db)
	_, j := createTestServerAndJob(t, db)

	if err := lockRepo.Acquire(ctx, j.ID, "run-1", formatTime(time.Now()), 1234); err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}

	err := jobRepo.Delete(ctx, j.ID, false)
	code, ok := domain.CodeOf(err)
	if !ok || code != domain.ErrJobAlreadyRunning {
		t.Fatalf("Delete() error = %v, want a JOB_ALREADY_RUNNING coded error", err)
	}
}
