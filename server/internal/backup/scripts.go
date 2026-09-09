package backup

import (
	"context"
	"fmt"
	"time"

	"vpsbackupmanager/internal/domain"
)

// registerCleanupScripts registers every ALWAYS/critical_cleanup script
// (from either phase) into the cleanup registry right after a successful
// connect, before any script runs, so the guarantee holds even if the run
// is cancelled or the service shuts down before reaching that script in
// the normal flow.
//
// NOTE: a critical_cleanup script keeps its own run_condition for the
// normal per-condition loop below, but critical_cleanup itself overrides
// that condition for the guarantee: if the normal loop skips it (e.g. an
// ON_SUCCESS critical_cleanup script after a failure), the drain still
// attempts it, because the specification requires critical_cleanup
// scripts to be attempted "even when earlier stages failed."
func (rc *runContext) registerCleanupScripts() {
	for i := range rc.job.Scripts {
		script := rc.job.Scripts[i]
		if script.RunCondition != domain.RunConditionAlways && !script.CriticalCleanup {
			continue
		}
		stepType := scriptStepType(script.Type)
		entry := rc.cleanup.Register(fmt.Sprintf("%s script (position %d)", script.Type, script.Position),
			func(ctx context.Context) error {
				return rc.runScriptRecorded(ctx, script, stepType)
			})
		rc.cleanupTokens = append(rc.cleanupTokens, cleanupToken{script: script, entry: entry})
	}
}

func scriptStepType(t domain.ScriptType) domain.StepType {
	if t == domain.ScriptPostBackup {
		return domain.StepPostBackupScript
	}
	return domain.StepPreBackupScript
}

// cleanupToken links a script to its cleanup-registry entry so the normal
// per-condition loop can mark it done once it actually runs there.
type cleanupToken struct {
	script domain.Script
	entry  *cleanupEntry
}

func (rc *runContext) findCleanupEntry(script domain.Script) *cleanupEntry {
	for _, t := range rc.cleanupTokens {
		if t.script.ID == script.ID {
			return t.entry
		}
	}
	return nil
}

// runScripts executes every script of scriptType in configured order,
// applying each one's run_condition against the run's failure state so
// far.
func (rc *runContext) runScripts(scriptType domain.ScriptType) {
	stepType := scriptStepType(scriptType)

	for _, script := range rc.job.Scripts {
		if script.Type != scriptType {
			continue
		}
		if rc.checkCancelled() {
			return
		}
		if !script.ShouldRun(rc.failed) {
			if err := rc.steps.Skip(rc.ctx, rc.now(), stepType, "preceding stage did not meet this script's run condition", script.Command); err != nil {
				rc.engine.logger().Error("persist skipped script step", "run_id", rc.run.ID, "error", err)
			}
			continue
		}

		runErr := rc.runScriptRecorded(rc.ctx, script, stepType)
		if entry := rc.findCleanupEntry(script); entry != nil {
			rc.cleanup.MarkDone(entry)
			rc.recordRecovery(runErr)
		}
		if runErr != nil {
			code := domain.ErrPostScriptFailed
			if scriptType == domain.ScriptPreBackup {
				code = domain.ErrPreScriptFailed
			}
			rc.fail(code, fmt.Sprintf("%s script at position %d failed: %v", scriptType, script.Position, rc.engine.redact(runErr.Error())))
		}
	}
}

// runScriptRecorded records a step around one script execution. It is
// shared by the normal per-condition loop and the cleanup-registry drain
// so every attempted script — guaranteed or not — is visible in run
// history.
func (rc *runContext) runScriptRecorded(ctx context.Context, script domain.Script, stepType domain.StepType) error {
	step, err := rc.steps.Start(ctx, rc.now(), stepType)
	if err != nil {
		rc.engine.logger().Error("start script step", "run_id", rc.run.ID, "error", err)
		return err
	}
	step.Command = script.Command

	runErr := rc.executeScript(ctx, script)
	status := domain.StepSuccess
	if runErr != nil {
		status = domain.StepFailed
		step.Error = rc.engine.redact(runErr.Error())
	}
	step.Finish(rc.now(), status)
	if err := rc.steps.Finish(ctx, step); err != nil {
		rc.engine.logger().Error("persist script step", "run_id", rc.run.ID, "error", err)
	}
	return runErr
}

// executeScript runs one script's command, honoring its timeout and
// opt-in retry.
func (rc *runContext) executeScript(ctx context.Context, script domain.Script) error {
	timeout := time.Duration(script.TimeoutSeconds) * time.Second
	attempts := 1
	if script.RetryOnFailure {
		attempts = 2
	}

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		result, err := rc.transport.Run(ctx, script.Command, timeout)
		if err != nil {
			lastErr = err
			continue
		}
		if result.ExitCode == 0 {
			return nil
		}
		lastErr = fmt.Errorf("exit code %d: %s", result.ExitCode, rc.engine.redact(result.Stderr))
	}
	return lastErr
}
