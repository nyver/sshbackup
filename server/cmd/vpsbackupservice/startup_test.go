package main

import (
	"context"
	"log/slog"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"vpsbackupmanager/internal/domain"
	"vpsbackupmanager/internal/ipc"
	"vpsbackupmanager/internal/scheduler"
	"vpsbackupmanager/internal/store"
)

type emptyJobStore struct{}

func (emptyJobStore) List(context.Context) ([]*domain.Job, error) { return nil, nil }

type noopRunner struct{}

func (noopRunner) Run(context.Context, *domain.Job, *domain.Server, domain.Trigger) (*domain.Run, error) {
	return nil, nil
}

func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(discardWriter{}, nil))
}

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }

func openTestDB(t *testing.T) *store.DB {
	t.Helper()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "backup.db")
	db, err := store.Open(ctx, path)
	if err != nil {
		t.Fatalf("store.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(ctx, db); err != nil {
		t.Fatalf("store.Migrate() error = %v", err)
	}
	return db
}

func seedServerAndJob(t *testing.T, db *store.DB) (*domain.Server, *domain.Job) {
	t.Helper()
	ctx := context.Background()
	s, err := domain.NewServer(time.Now(), domain.NewServerParams{
		Name: "prod", Host: "1.2.3.4", Username: "deploy",
		AuthType: domain.AuthPrivateKey, PrivateKeyPath: "irrelevant",
	})
	if err != nil {
		t.Fatalf("domain.NewServer() error = %v", err)
	}
	if err := store.NewServerRepository(db).Create(ctx, s); err != nil {
		t.Fatalf("create server: %v", err)
	}
	j, err := domain.NewJob(time.Now(), domain.NewJobParams{
		Name: "beresta", ServerID: s.ID, Enabled: true,
		Sources:          []domain.Source{{RemotePath: "/data"}},
		Schedule:         domain.Schedule{Type: domain.ScheduleManual},
		LocalDestination: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("domain.NewJob() error = %v", err)
	}
	if err := store.NewJobRepository(db).Create(ctx, j); err != nil {
		t.Fatalf("create job: %v", err)
	}
	return s, j
}

func TestReleaseStaleLocks_ReleasesEveryLockFoundAtStartup(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	ctx := context.Background()
	_, job := seedServerAndJob(t, db)

	locks := store.NewLockRepository(db)
	if err := locks.Acquire(ctx, job.ID, "stale-run", time.Now().UTC().Format(time.RFC3339Nano), 999999); err != nil {
		t.Fatalf("seed stale lock: %v", err)
	}

	releaseStaleLocks(ctx, locks, silentLogger())

	got, err := locks.Get(ctx, job.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got != nil {
		t.Errorf("Get() = %+v, want the stale lock released", got)
	}

	// A fresh acquire must now succeed: nothing was left dangling.
	if err := locks.Acquire(ctx, job.ID, "new-run", time.Now().UTC().Format(time.RFC3339Nano), 1234); err != nil {
		t.Errorf("Acquire() after release error = %v", err)
	}
}

func TestRecoverInterruptedRuns_MarksRunningAsInterrupted(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	ctx := context.Background()
	_, job := seedServerAndJob(t, db)
	runs := store.NewRunRepository(db)

	running, err := domain.NewRun(time.Now(), job.ID, job.ServerID, nil, domain.TriggerSchedule)
	if err != nil {
		t.Fatalf("domain.NewRun() error = %v", err)
	}
	running.Status = domain.RunRunning
	if err := runs.Create(ctx, running); err != nil {
		t.Fatalf("create running run: %v", err)
	}

	finished, err := domain.NewRun(time.Now(), job.ID, job.ServerID, nil, domain.TriggerSchedule)
	if err != nil {
		t.Fatalf("domain.NewRun() error = %v", err)
	}
	finished.Finish(time.Now(), domain.RunSuccess)
	if err := runs.Create(ctx, finished); err != nil {
		t.Fatalf("create finished run: %v", err)
	}

	recoverInterruptedRuns(ctx, runs, silentLogger())

	got, err := runs.Get(ctx, running.ID)
	if err != nil {
		t.Fatalf("Get(running) error = %v", err)
	}
	if got.Status != domain.RunInterrupted {
		t.Errorf("Status = %q, want INTERRUPTED", got.Status)
	}
	if got.RecoveryOutcome != domain.RecoveryFailed {
		t.Errorf("RecoveryOutcome = %q, want FAILED (recovery actions were not verified)", got.RecoveryOutcome)
	}
	if got.ErrorMessage == "" {
		t.Error("expected an explanatory ErrorMessage on the interrupted run")
	}

	stillDone, err := runs.Get(ctx, finished.ID)
	if err != nil {
		t.Fatalf("Get(finished) error = %v", err)
	}
	if stillDone.Status != domain.RunSuccess {
		t.Errorf("a terminal run's status changed: got %q, want SUCCESS (live lock / terminal run must not be touched)", stillDone.Status)
	}
}

func TestApp_Shutdown_RespectsBudget(t *testing.T) {
	old := shutdownBudget
	shutdownBudget = 100 * time.Millisecond
	t.Cleanup(func() { shutdownBudget = old })

	db := openTestDB(t)
	router := ipc.NewRouter(nil)
	events := ipc.NewEventHub()
	ipcServer := ipc.NewServer(router, events, silentLogger())

	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = ipcServer.Serve(ctx, `\\.\pipe\vpsbackupmanager-shutdown-test`) }()
	t.Cleanup(cancel)

	settings := store.NewSettingsRepository(db)
	dispatcher := scheduler.NewDispatcher(emptyJobStore{}, store.NewServerRepository(db), settings, &schedRunStore{runs: store.NewRunRepository(db)}, noopRunner{})
	dispatcher.Logger = silentLogger()
	dispatcher.PollInterval = time.Hour // never actually ticks during the test
	dispatcherCtx, dispatcherCancel := context.WithCancel(context.Background())
	dispatcher.Start(dispatcherCtx)
	t.Cleanup(dispatcherCancel)

	var runsWG sync.WaitGroup
	runsWG.Add(1) // a run that will never finish, to force the budget path

	app := &App{
		DB: db, IPCServer: ipcServer, Dispatcher: dispatcher, ActiveRuns: ipc.NewActiveRunRegistry(),
		Logger: silentLogger(), closeLog: noopCloser{}, runsWG: &runsWG, cancel: cancel,
	}

	start := time.Now()
	app.Shutdown()
	elapsed := time.Since(start)

	if elapsed > time.Second {
		t.Errorf("Shutdown() took %v, want it bounded by shutdownBudget (%v)", elapsed, shutdownBudget)
	}
	if elapsed < shutdownBudget {
		t.Errorf("Shutdown() returned in %v, faster than the budget (%v); it should have waited for the budget to expire", elapsed, shutdownBudget)
	}
}

type noopCloser struct{}

func (noopCloser) Close() error { return nil }
