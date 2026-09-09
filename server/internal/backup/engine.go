package backup

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"vpsbackupmanager/internal/domain"
)

// ConnectParams carries what a Connector needs to open a Transport for one
// run. Credential material is already resolved by the caller (private key
// bytes read from disk, or the password/passphrase decrypted via
// internal/secrets) so the engine itself never touches the filesystem or
// DPAPI for secrets. Which of Passphrase/Password is used is determined by
// Server.AuthType.
type ConnectParams struct {
	Server        *domain.Server
	PrivateKeyPEM []byte
	Passphrase    string
	Password      string
}

// Connector opens a Transport for a run. Production wiring is
// internal/remote.NewConnector; tests inject a func that returns a
// backuptest.FakeTransport.
type Connector func(ctx context.Context, p ConnectParams) (Transport, error)

// RetentionApplier applies a job's local retention policy after a verified
// backup. Implemented by internal/retention; nil disables the retention
// stage (used by engine-focused tests that don't exercise it).
type RetentionApplier interface {
	Apply(ctx context.Context, job *domain.Job) (deleted int, warnings []string, err error)
}

// EventSink observes a run's lifecycle for live progress delivery (the
// ipc-api specification's event stream). All methods are called
// synchronously from the run's own goroutine; nil is a valid Engine.Events
// value and simply disables event delivery.
type EventSink interface {
	RunStarted(run *domain.Run)
	StepChanged(runID string, step *domain.RunStep)
	RunFinished(run *domain.Run)
}

// Engine runs backup jobs: locking, pre-flight checks, scripts, archive
// creation, checksum-verified download, guaranteed cleanup/recovery
// actions, and retention.
type Engine struct {
	Connect   Connector
	Runs      RunStore
	Steps     StepStore
	Locks     LockStore
	Retention RetentionApplier
	Clock     Clock
	Redact    func(string) string // nil-safe; identity when nil
	Events    EventSink           // nil-safe; disables event delivery when nil
	Logger    *slog.Logger

	// ArchiveTimeout bounds archive creation; DownloadDir is where
	// verified archives land. ShutdownBudget bounds cleanup during
	// cancellation/shutdown when the caller does not impose a tighter one
	// via ctx.
	CleanupBudget time.Duration
}

func (e *Engine) logger() *slog.Logger {
	if e.Logger != nil {
		return e.Logger
	}
	return slog.Default()
}

func (e *Engine) redact(s string) string {
	if e.Redact == nil {
		return s
	}
	return e.Redact(s)
}

func (e *Engine) cleanupBudget() time.Duration {
	if e.CleanupBudget > 0 {
		return e.CleanupBudget
	}
	return 2 * time.Minute
}

// Run executes one backup run for job against server and blocks until it
// finishes, streaming the archive to destDir on success. It always
// returns a *domain.Run (even a SKIPPED one when the job's lock is
// already held) unless persistence itself fails. Use Start instead when
// the caller must not block (e.g. answering an IPC "run now" request with
// the new run id immediately).
func (e *Engine) Run(ctx context.Context, job *domain.Job, server *domain.Server, p ConnectParams, destDir string, trigger domain.Trigger) (*domain.Run, error) {
	run, rc, skipped, err := e.beginRun(ctx, job, server, p, destDir, trigger)
	if err != nil || skipped {
		return run, err
	}
	e.finishRun(ctx, run, rc)
	return run, nil
}

// Start begins one backup run and returns as soon as its initial
// PENDING/RUNNING (or SKIPPED) record is persisted, running the actual
// workflow in a background goroutine. Engine.Events, if set, is notified
// when the run starts, at each step change, and when it finishes, so a
// caller that only needs the initial record (e.g. an IPC handler) does
// not need to poll or block.
func (e *Engine) Start(ctx context.Context, job *domain.Job, server *domain.Server, p ConnectParams, destDir string, trigger domain.Trigger) (*domain.Run, error) {
	run, rc, skipped, err := e.beginRun(ctx, job, server, p, destDir, trigger)
	if err != nil || skipped {
		return run, err
	}
	go e.finishRun(ctx, run, rc)
	return run, nil
}

// beginRun acquires the job lock and persists the run's initial state
// synchronously. skipped is true when the job was already locked, in
// which case the returned run is already terminal (SKIPPED) and rc is nil
// — there is nothing left for the caller to run.
func (e *Engine) beginRun(ctx context.Context, job *domain.Job, server *domain.Server, p ConnectParams, destDir string, trigger domain.Trigger) (run *domain.Run, rc *runContext, skipped bool, err error) {
	now := e.Clock.Now()
	sourcePaths := make([]string, len(job.Sources))
	for i, s := range job.Sources {
		sourcePaths[i] = s.RemotePath
	}

	run, err = domain.NewRun(now, job.ID, server.ID, sourcePaths, trigger)
	if err != nil {
		return nil, nil, false, err
	}

	lockErr := e.Locks.Acquire(ctx, job.ID, run.ID, now.UTC().Format(time.RFC3339Nano), os.Getpid())
	if lockErr != nil {
		if code, ok := domain.CodeOf(lockErr); ok && code == domain.ErrJobAlreadyRunning {
			run.Status = domain.RunSkipped
			run.ErrorMessage = "Job already running"
			finished := e.Clock.Now()
			run.FinishedAt = &finished
			if err := e.Runs.Create(ctx, run); err != nil {
				return nil, nil, false, fmt.Errorf("persist skipped run: %w", err)
			}
			return run, nil, true, nil
		}
		return nil, nil, false, lockErr
	}

	run.Status = domain.RunRunning
	if err := e.Runs.Create(ctx, run); err != nil {
		_ = e.Locks.Release(context.WithoutCancel(ctx), job.ID)
		return nil, nil, false, fmt.Errorf("persist run: %w", err)
	}
	if e.Events != nil {
		e.Events.RunStarted(run)
	}

	rc = &runContext{
		engine:      e,
		ctx:         ctx,
		job:         job,
		server:      server,
		connectP:    p,
		destDir:     destDir,
		run:         run,
		steps:       newStepRecorder(e.Steps, run.ID),
		cleanup:     &cleanupRegistry{},
		recoveryOut: domain.RecoveryNotApplicable,
	}
	if e.Events != nil {
		rc.steps.onChange = func(step *domain.RunStep) { e.Events.StepChanged(run.ID, step) }
	}
	return run, rc, false, nil
}

// finishRun runs rc's workflow to completion, persists the final run
// state, and releases the job lock. Called synchronously by Run and in a
// background goroutine by Start.
func (e *Engine) finishRun(ctx context.Context, run *domain.Run, rc *runContext) {
	defer func() {
		releaseCtx := context.WithoutCancel(ctx)
		if err := e.Locks.Release(releaseCtx, rc.job.ID); err != nil {
			e.logger().Error("release job lock", "job_id", rc.job.ID, "error", err)
		}
	}()

	rc.execute() //nolint:contextcheck // execute and its stage methods intentionally read ctx from runContext (see the containedctx justification on that field) rather than threading it through every stage signature

	finished := e.Clock.Now()
	run.Finish(finished, rc.finalStatus())
	run.RecoveryOutcome = rc.recoveryOut
	if err := e.Runs.Update(ctx, run); err != nil {
		e.logger().Error("persist finished run", "run_id", run.ID, "error", err)
	}
	if e.Events != nil {
		e.Events.RunFinished(run)
	}
}
