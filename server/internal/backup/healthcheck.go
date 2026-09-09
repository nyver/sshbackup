package backup

import (
	"fmt"
	"time"

	"vpsbackupmanager/internal/domain"
)

// healthCheck runs the optional post-backup health check, retrying up to
// its configured attempt count. Failure never invalidates an already
// verified archive: it only downgrades the run to WARNING.
func (rc *runContext) healthCheck() {
	if rc.job.HealthCheck == nil {
		return
	}
	hc := rc.job.HealthCheck

	step, err := rc.steps.Start(rc.ctx, rc.now(), domain.StepHealthCheck)
	if err != nil {
		rc.engine.logger().Error("start health check step", "run_id", rc.run.ID, "error", err)
		return
	}

	var lastErr error
	succeeded := false
	for attempt := 1; attempt <= hc.Attempts; attempt++ {
		if attempt > 1 {
			if err := rc.engine.Clock.Sleep(rc.ctx, time.Duration(hc.IntervalSeconds)*time.Second); err != nil {
				lastErr = err
				break
			}
		}
		res, runErr := rc.runRemote(hc.Command)
		if runErr == nil && res.ExitCode == 0 {
			succeeded = true
			break
		}
		if runErr != nil {
			lastErr = runErr
		} else {
			lastErr = fmt.Errorf("exit code %d: %s", res.ExitCode, rc.engine.redact(res.Stderr))
		}
	}

	if !succeeded {
		rc.warn(domain.ErrHealthCheckFailed, fmt.Sprintf("health check failed after %d attempt(s): %v", hc.Attempts, rc.engine.redact(fmt.Sprint(lastErr))))
		step.Error = rc.run.ErrorMessage
	}
	step.Finish(rc.now(), stepStatus(succeeded))
	if err := rc.steps.Finish(rc.ctx, step); err != nil {
		rc.engine.logger().Error("persist health check step", "run_id", rc.run.ID, "error", err)
	}
}
