package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"

	"vpsbackupmanager/internal/applog"
	"vpsbackupmanager/internal/backup"
	"vpsbackupmanager/internal/config"
	"vpsbackupmanager/internal/domain"
	"vpsbackupmanager/internal/ipc"
	"vpsbackupmanager/internal/notify"
	"vpsbackupmanager/internal/remote"
	"vpsbackupmanager/internal/retention"
	"vpsbackupmanager/internal/scheduler"
	"vpsbackupmanager/internal/secrets"
	"vpsbackupmanager/internal/store"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

// shutdownBudget bounds how long graceful shutdown waits for active runs'
// guaranteed cleanup actions before giving up and exiting anyway. A var,
// not a const, so tests can shrink it instead of waiting minutes.
var shutdownBudget = 2 * time.Minute

// App is the fully wired service: every subsystem plus the means to stop
// them in order. Build it with startup and always call Shutdown.
type App struct {
	DB         *store.DB
	IPCServer  *ipc.Server
	Dispatcher *scheduler.Dispatcher
	ActiveRuns *ipc.ActiveRunRegistry
	Logger     *slog.Logger

	closeLog io.Closer
	runsWG   *sync.WaitGroup
	cancel   context.CancelFunc
}

// startup implements the specified startup sequence: open and migrate the
// database, load enabled jobs, initialize the scheduler, recover
// interrupted runs and stale locks, then start IPC and dispatch. It
// returns an actionable error, without starting anything further, when
// the database cannot be opened or migrated.
func startup(ctx context.Context, mode string) (*App, error) { //nolint:contextcheck // this function deliberately builds runCtx, the service's own long-lived lifecycle context, independent of ctx (see the comment where runCtx is created)
	dataDir, err := config.DataDir()
	if err != nil {
		return nil, fmt.Errorf("resolve application data directory: %w", err)
	}

	redactor := secrets.NewRedactor()
	logger, closeLog, err := applog.NewAppLogger(dataDir, redactor, slog.LevelInfo)
	if err != nil {
		return nil, fmt.Errorf("set up logging: %w", err)
	}
	logger.Info("starting", "mode", mode, "version", version, "data_dir", dataDir)

	dbPath := config.DatabasePath(dataDir)
	db, err := store.Open(ctx, dbPath)
	if err != nil {
		_ = closeLog.Close()
		return nil, fmt.Errorf("open database %q: %w (the service cannot start until this is resolved)", dbPath, err)
	}
	if err := store.Migrate(ctx, db); err != nil {
		_ = db.Close()
		_ = closeLog.Close()
		return nil, fmt.Errorf("migrate database %q: %w (the service cannot start until this is resolved)", dbPath, err)
	}

	servers := store.NewServerRepository(db)
	jobs := store.NewJobRepository(db)
	runs := store.NewRunRepository(db)
	steps := store.NewStepRepository(db)
	settingsRepo := store.NewSettingsRepository(db)
	locks := store.NewLockRepository(db)

	secretsDir, err := config.SecretsDir(dataDir)
	if err != nil {
		_ = db.Close()
		_ = closeLog.Close()
		return nil, fmt.Errorf("create secrets directory: %w", err)
	}
	secretsStore := secrets.NewStore(secretsDir)

	connector := remote.NewConnector(remote.SystemClock{})
	notifier := &notify.WindowsToastNotifier{}

	return buildApp(dataDir, db, mode, connector, notifier, ipc.PipeName, redactor, logger, closeLog, //nolint:contextcheck // buildApp intentionally builds its own long-lived runCtx rather than taking one; see its doc comment
		servers, jobs, runs, steps, settingsRepo, locks, secretsStore)
}

// buildApp wires every subsystem given already-open database repositories
// and the dependencies production and tests differ on (connector,
// notifier, pipeName), then starts the scheduler and IPC server. Extracted
// from startup so an end-to-end test can drive the exact same wiring
// against a fake Connector and a test-only pipe name instead of a real
// SSH connection and the production pipe.
func buildApp(
	dataDir string, db *store.DB, mode string,
	connector backup.Connector, notifier notify.Notifier, pipeName string,
	redactor *secrets.Redactor, logger *slog.Logger, closeLog io.Closer,
	servers *store.ServerRepository, jobs *store.JobRepository, runs *store.RunRepository,
	steps *store.StepRepository, settingsRepo *store.SettingsRepository, locks *store.LockRepository,
	secretsStore *secrets.Store,
) (*App, error) {
	ctx := context.Background()
	releaseStaleLocks(ctx, locks, logger)
	recoverInterruptedRuns(ctx, runs, logger)

	// runCtx is the service's own long-lived lifecycle context: the
	// dispatcher and IPC server must keep running for the App's whole
	// lifetime, stopping only via App.Shutdown's explicit cancel.
	runCtx, cancel := context.WithCancel(context.Background())

	activeRuns := ipc.NewActiveRunRegistry()
	events := ipc.NewEventHub()
	var runsWG sync.WaitGroup

	notifyService := &notify.Service{
		Notifier: notifier,
		Redact:   redactor.Redact,
		Logger:   logger,
	}
	retentionApplier := retention.NewApplier(runs)

	engine := &backup.Engine{
		Connect: connector, Runs: runs, Steps: steps, Locks: locks,
		Retention: retentionApplier, Clock: backup.SystemClock{}, Redact: redactor.Redact,
		Logger: logger, CleanupBudget: shutdownBudget,
		Events: newCompositeEventSink(
			ipc.NewEngineEventSink(events, activeRuns),
			newRunLogEventSink(dataDir, logger),
			newWaitGroupEventSink(&runsWG),
			newNotifyEventSink(notifyService, jobs, settingsRepo, logger),
		),
	}
	validator := backup.NewValidator(connector)

	backend := &ipc.Backend{
		Servers: servers, Jobs: jobs, Runs: runs, Steps: steps, Settings: settingsRepo, Secrets: secretsStore,
		Engine: engine, Validator: validator, ActiveRuns: activeRuns, Events: events,
		Version: version, StartedAt: time.Now(),
	}
	router := ipc.NewRouter(func(command string, err error) {
		logger.Error("ipc handler error", "command", command, "error", redactor.Redact(err.Error()))
	})
	backend.RegisterHandlers(router)
	ipcServer := ipc.NewServer(router, events, logger)

	schedRunner := &schedulerRunner{engine: engine, servers: servers, secretsStore: secretsStore}
	dispatcher := scheduler.NewDispatcher(jobs, servers, settingsRepo, &schedRunStore{runs: runs}, schedRunner)
	dispatcher.Logger = logger
	if err := dispatcher.DetectMissedRuns(runCtx); err != nil { //nolint:contextcheck // runCtx is the service's own lifecycle context, deliberately independent of ctx; see where it is created above
		logger.Error("detect missed runs", "error", err)
	}
	dispatcher.Start(runCtx) //nolint:contextcheck // see above

	go func() {
		if err := ipcServer.Serve(runCtx, pipeName); err != nil {
			logger.Error("ipc server stopped", "error", err)
		}
	}()

	logger.Info("started", "mode", mode)
	return &App{
		DB: db, IPCServer: ipcServer, Dispatcher: dispatcher, ActiveRuns: activeRuns, Logger: logger,
		closeLog: closeLog, runsWG: &runsWG, cancel: cancel,
	}, nil
}

// releaseStaleLocks releases every job lock found at startup. A lock can
// only legitimately belong to the process that is starting right now, and
// this process has not dispatched anything yet, so every lock found here
// predates it and is stale by definition — left by a crashed or replaced
// previous instance.
func releaseStaleLocks(ctx context.Context, locks *store.LockRepository, logger *slog.Logger) {
	existing, err := locks.List(ctx)
	if err != nil {
		logger.Error("list job locks at startup", "error", err)
		return
	}
	for _, l := range existing {
		if err := locks.Release(ctx, l.JobID); err != nil {
			logger.Error("release stale job lock", "job_id", l.JobID, "error", err)
			continue
		}
		logger.Warn("released stale job lock left by a previous service instance", "job_id", l.JobID, "run_id", l.RunID, "process_id", l.ProcessID)
	}
}

// recoverInterruptedRuns marks every run still RUNNING at startup as
// INTERRUPTED. Since any registered critical cleanup action for that run
// did not get a chance to run (the process that would have run it is
// gone), recovery is conservatively reported as not verified rather than
// assumed successful.
func recoverInterruptedRuns(ctx context.Context, runs *store.RunRepository, logger *slog.Logger) {
	ids, err := runs.InterruptRunningRuns(ctx, time.Now())
	if err != nil {
		logger.Error("recover interrupted runs at startup", "error", err)
		return
	}
	for _, id := range ids {
		run, err := runs.Get(ctx, id)
		if err != nil {
			logger.Error("load interrupted run", "run_id", id, "error", err)
			continue
		}
		run.RecoveryOutcome = domain.RecoveryFailed
		run.ErrorMessage = "the service stopped unexpectedly while this run was in progress; " +
			"any pending critical cleanup or recovery actions may not have completed"
		if err := runs.Update(ctx, run); err != nil {
			logger.Error("persist interrupted run", "run_id", id, "error", err)
			continue
		}
		logger.Warn("marked run as interrupted after an unexpected stop", "run_id", id, "job_id", run.JobID)
	}
}

// Shutdown stops accepting new runs, cancels every active run (which
// still runs its guaranteed cleanup on a detached, bounded context — see
// backup.Engine.CleanupBudget), waits up to shutdownBudget for them to
// finish, and closes the database. It never hangs indefinitely.
func (a *App) Shutdown() {
	a.Logger.Info("shutting down")

	a.Dispatcher.Stop()
	_ = a.IPCServer.Close()

	a.cancel()
	a.ActiveRuns.CancelAll()

	done := make(chan struct{})
	go func() {
		a.runsWG.Wait()
		close(done)
	}()
	select {
	case <-done:
		a.Logger.Info("all active runs finished cleanly during shutdown")
	case <-time.After(shutdownBudget):
		a.Logger.Warn("shutdown budget exceeded; exiting with cleanup possibly incomplete", "budget", shutdownBudget)
	}

	if err := a.DB.Close(); err != nil {
		a.Logger.Error("close database", "error", err)
	}
	if err := a.closeLog.Close(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "close log: %v\n", err)
	}
}
