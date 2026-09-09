package backup

import (
	"context"
	"time"

	"vpsbackupmanager/internal/domain"
)

// runContext carries the mutable state of one run across its stage
// functions. It is not persisted directly; each stage records its own
// domain.RunStep and updates fields here that feed the final domain.Run.
type runContext struct {
	engine   *Engine
	ctx      context.Context //nolint:containedctx // the run's own cancellable context, threaded through every stage function
	job      *domain.Job
	server   *domain.Server
	connectP ConnectParams
	destDir  string

	run           *domain.Run
	steps         *stepRecorder
	cleanup       *cleanupRegistry
	cleanupTokens []cleanupToken

	transport Transport

	failed    bool
	hasWarn   bool
	cancelled bool

	recoveryOut       domain.RecoveryOutcome
	recoveryAttempted bool

	remoteArchivePath string // set once the archive stage creates it, for guaranteed remote cleanup
	localPartPath     string // set once the download stage starts writing, for cleanup on cancel/failure
	archiveVerified   bool
}

// fail records the first failure only: later failures (e.g. a cleanup
// action failing after the real cause) never overwrite the original
// error code/message shown to the user.
func (rc *runContext) fail(code domain.ErrorCode, message string) {
	if rc.failed {
		return
	}
	rc.failed = true
	rc.run.ErrorCode = code
	rc.run.ErrorMessage = message
}

// warn records a non-fatal problem. It does not overwrite a prior
// warning or failure, so the first reported error remains the headline
// reason shown to the user.
func (rc *runContext) warn(code domain.ErrorCode, message string) {
	rc.hasWarn = true
	if !rc.failed && rc.run.ErrorCode == "" {
		rc.run.ErrorCode = code
		rc.run.ErrorMessage = message
	}
}

// recordRecovery folds one recovery (ALWAYS/critical_cleanup) action's
// outcome into the run's overall recovery status: SUCCESS only if every
// such action that ran succeeded, FAILED if any did not.
func (rc *runContext) recordRecovery(err error) {
	rc.recoveryAttempted = true
	if err != nil {
		rc.recoveryOut = domain.RecoveryFailed
		return
	}
	if rc.recoveryOut != domain.RecoveryFailed {
		rc.recoveryOut = domain.RecoverySuccess
	}
}

func (rc *runContext) finalStatus() domain.RunStatus {
	switch {
	case rc.cancelled:
		return domain.RunCancelled
	case rc.failed:
		return domain.RunFailed
	case rc.hasWarn:
		return domain.RunWarning
	default:
		return domain.RunSuccess
	}
}

// checkCancelled reports whether the run's context has been cancelled
// (user cancellation or shutdown) and, the first time it notices, marks
// the run cancelled and stops the current .part download in progress.
func (rc *runContext) checkCancelled() bool {
	if rc.ctx.Err() == nil {
		return false
	}
	rc.cancelled = true
	return true
}

// execute runs every workflow stage in order. Its deferred cleanup drain
// guarantees ALWAYS/critical_cleanup actions still run when the function
// returns early because the run was cancelled.
func (rc *runContext) execute() {
	defer rc.drainCleanup()

	if !rc.preflight() {
		return
	}
	defer func() {
		if rc.transport != nil {
			_ = rc.transport.Close()
		}
	}()

	rc.runScripts(domain.ScriptPreBackup)
	if rc.checkCancelled() {
		return
	}

	if !rc.failed {
		rc.archive()
	}
	if rc.checkCancelled() {
		return
	}

	if !rc.failed {
		rc.downloadAndVerify()
	}
	if rc.checkCancelled() {
		return
	}

	rc.runScripts(domain.ScriptPostBackup)
	if rc.checkCancelled() {
		return
	}

	if !rc.failed {
		rc.healthCheck()
	}

	rc.remoteCleanup()

	if !rc.failed && rc.archiveVerified && rc.engine.Retention != nil {
		rc.applyRetention()
	}
}

// drainCleanup runs every not-yet-done ALWAYS/critical_cleanup action on a
// context detached from the run's own (possibly cancelled) context, with
// its own bounded timeout, per design.md's recovery guarantee.
func (rc *runContext) drainCleanup() {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(rc.ctx), rc.engine.cleanupBudget())
	defer cancel()

	for _, outcome := range rc.cleanup.Drain(cleanupCtx) {
		rc.recordRecovery(outcome.Err)
		if outcome.Err != nil {
			rc.engine.logger().Warn("cleanup action failed",
				"run_id", rc.run.ID, "job_id", rc.job.ID, "action", outcome.Description,
				"error", rc.engine.redact(outcome.Err.Error()))
		}
	}
}

// now is a small convenience so stage functions don't reach into
// rc.engine.Clock directly everywhere.
func (rc *runContext) now() time.Time {
	return rc.engine.Clock.Now()
}

// runRemote runs cmd with the server's default command timeout.
func (rc *runContext) runRemote(cmd string) (RunResult, error) {
	return runOnTransport(rc.ctx, rc.transport, rc.server.CommandTimeoutSeconds, cmd)
}
