package store

import (
	"context"
	"testing"
	"time"

	"vpsbackupmanager/internal/domain"
)

func TestRunRepository_CreateUpdateGet(t *testing.T) {
	ctx := context.Background()
	db := openMigratedTestDB(t)
	repo := NewRunRepository(db)

	run, err := domain.NewRun(time.Now(), "job-1", "server-1", []string{"/docker/volumes"}, domain.TriggerSchedule)
	if err != nil {
		t.Fatalf("domain.NewRun() error = %v", err)
	}
	if err := repo.Create(ctx, run); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	run.Finish(time.Now(), domain.RunFailed)
	run.ErrorCode = domain.ErrArchiveFailed
	run.ErrorMessage = "no space left on device"
	run.RecoveryOutcome = domain.RecoverySuccess
	if err := repo.Update(ctx, run); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	got, err := repo.Get(ctx, run.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Status != domain.RunFailed || got.ErrorCode != domain.ErrArchiveFailed {
		t.Errorf("Get() = %+v, want FAILED/ARCHIVE_FAILED", got)
	}
	if got.RecoveryOutcome != domain.RecoverySuccess {
		t.Errorf("RecoveryOutcome = %q, want SUCCESS", got.RecoveryOutcome)
	}
	if len(got.SourcePaths) != 1 || got.SourcePaths[0] != "/docker/volumes" {
		t.Errorf("SourcePaths = %v, want [/docker/volumes]", got.SourcePaths)
	}
}

func TestRunRepository_List_FiltersByJobAndStatus(t *testing.T) {
	ctx := context.Background()
	db := openMigratedTestDB(t)
	repo := NewRunRepository(db)

	r1, _ := domain.NewRun(time.Now(), "job-1", "server-1", nil, domain.TriggerManual)
	r1.Finish(time.Now(), domain.RunSuccess)
	r2, _ := domain.NewRun(time.Now().Add(time.Minute), "job-1", "server-1", nil, domain.TriggerManual)
	r2.Finish(time.Now(), domain.RunFailed)
	r3, _ := domain.NewRun(time.Now().Add(2*time.Minute), "job-2", "server-1", nil, domain.TriggerManual)
	r3.Finish(time.Now(), domain.RunSuccess)
	for _, r := range []*domain.Run{r1, r2, r3} {
		if err := repo.Create(ctx, r); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	}

	byJob, err := repo.List(ctx, RunFilter{JobID: "job-1"})
	if err != nil {
		t.Fatalf("List(job-1) error = %v", err)
	}
	if len(byJob) != 2 {
		t.Fatalf("List(job-1) returned %d runs, want 2", len(byJob))
	}
	// Most recently started first.
	if byJob[0].ID != r2.ID {
		t.Errorf("List(job-1)[0] = %q, want most recent run %q", byJob[0].ID, r2.ID)
	}

	byStatus, err := repo.List(ctx, RunFilter{Status: domain.RunFailed})
	if err != nil {
		t.Fatalf("List(FAILED) error = %v", err)
	}
	if len(byStatus) != 1 || byStatus[0].ID != r2.ID {
		t.Errorf("List(FAILED) = %v, want just %q", byStatus, r2.ID)
	}
}

func TestRunRepository_InterruptRunningRuns(t *testing.T) {
	ctx := context.Background()
	db := openMigratedTestDB(t)
	repo := NewRunRepository(db)

	running, _ := domain.NewRun(time.Now(), "job-1", "server-1", nil, domain.TriggerSchedule)
	running.Status = domain.RunRunning
	if err := repo.Create(ctx, running); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	finished, _ := domain.NewRun(time.Now(), "job-1", "server-1", nil, domain.TriggerSchedule)
	finished.Finish(time.Now(), domain.RunSuccess)
	if err := repo.Create(ctx, finished); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	ids, err := repo.InterruptRunningRuns(ctx, time.Now())
	if err != nil {
		t.Fatalf("InterruptRunningRuns() error = %v", err)
	}
	if len(ids) != 1 || ids[0] != running.ID {
		t.Fatalf("InterruptRunningRuns() = %v, want [%q]", ids, running.ID)
	}

	got, err := repo.Get(ctx, running.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Status != domain.RunInterrupted {
		t.Errorf("Status = %q, want INTERRUPTED", got.Status)
	}

	stillSuccess, err := repo.Get(ctx, finished.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if stillSuccess.Status != domain.RunSuccess {
		t.Errorf("a terminal run's status changed: got %q, want SUCCESS", stillSuccess.Status)
	}
}

func TestStepRepository_CreateUpdateListByRun(t *testing.T) {
	ctx := context.Background()
	db := openMigratedTestDB(t)
	runRepo := NewRunRepository(db)
	stepRepo := NewStepRepository(db)

	run, _ := domain.NewRun(time.Now(), "job-1", "server-1", nil, domain.TriggerManual)
	if err := runRepo.Create(ctx, run); err != nil {
		t.Fatalf("create run: %v", err)
	}

	step1, err := domain.NewRunStep(time.Now(), run.ID, domain.StepPreflight)
	if err != nil {
		t.Fatalf("domain.NewRunStep() error = %v", err)
	}
	if err := stepRepo.Create(ctx, step1, 0); err != nil {
		t.Fatalf("Create(step1) error = %v", err)
	}

	step2, err := domain.NewRunStep(time.Now(), run.ID, domain.StepArchive)
	if err != nil {
		t.Fatalf("domain.NewRunStep() error = %v", err)
	}
	exitCode := 1
	step2.ExitCode = &exitCode
	step2.Finish(time.Now(), domain.StepFailed)
	if err := stepRepo.Create(ctx, step2, 1); err != nil {
		t.Fatalf("Create(step2) error = %v", err)
	}

	step1.Finish(time.Now(), domain.StepSuccess)
	if err := stepRepo.Update(ctx, step1); err != nil {
		t.Fatalf("Update(step1) error = %v", err)
	}

	steps, err := stepRepo.ListByRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("ListByRun() error = %v", err)
	}
	if len(steps) != 2 {
		t.Fatalf("ListByRun() returned %d steps, want 2", len(steps))
	}
	if steps[0].Type != domain.StepPreflight || steps[0].Status != domain.StepSuccess {
		t.Errorf("steps[0] = %+v, want PREFLIGHT/SUCCESS", steps[0])
	}
	if steps[1].Type != domain.StepArchive || steps[1].Status != domain.StepFailed {
		t.Errorf("steps[1] = %+v, want ARCHIVE/FAILED", steps[1])
	}
	if steps[1].ExitCode == nil || *steps[1].ExitCode != 1 {
		t.Errorf("steps[1].ExitCode = %v, want 1", steps[1].ExitCode)
	}
}

func TestSettingsRepository_GetDefaultsAndUpdate(t *testing.T) {
	ctx := context.Background()
	db := openMigratedTestDB(t)
	repo := NewSettingsRepository(db)

	got, err := repo.Get(ctx)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	want := domain.DefaultSettings()
	if got != want {
		t.Errorf("Get() = %+v, want defaults %+v", got, want)
	}

	got.SchedulesPaused = true
	got.GlobalConcurrencyLimit = 5
	if err := repo.Update(ctx, got); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	after, err := repo.Get(ctx)
	if err != nil {
		t.Fatalf("Get() after update error = %v", err)
	}
	if !after.SchedulesPaused || after.GlobalConcurrencyLimit != 5 {
		t.Errorf("Get() after update = %+v, want paused=true, limit=5", after)
	}
}

func TestLockRepository_AcquireReleaseConflict(t *testing.T) {
	ctx := context.Background()
	db := openMigratedTestDB(t)
	lockRepo := NewLockRepository(db)
	_, j := createTestServerAndJob(t, db)

	if err := lockRepo.Acquire(ctx, j.ID, "run-1", formatTime(time.Now()), 1111); err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}

	err := lockRepo.Acquire(ctx, j.ID, "run-2", formatTime(time.Now()), 2222)
	code, ok := domain.CodeOf(err)
	if !ok || code != domain.ErrJobAlreadyRunning {
		t.Fatalf("second Acquire() error = %v, want JOB_ALREADY_RUNNING", err)
	}

	if err := lockRepo.Release(ctx, j.ID); err != nil {
		t.Fatalf("Release() error = %v", err)
	}
	if err := lockRepo.Acquire(ctx, j.ID, "run-3", formatTime(time.Now()), 3333); err != nil {
		t.Fatalf("Acquire() after release error = %v", err)
	}

	locks, err := lockRepo.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(locks) != 1 || locks[0].RunID != "run-3" {
		t.Errorf("List() = %v, want a single lock for run-3", locks)
	}
}
