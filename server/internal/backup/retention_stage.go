package backup

import (
	"strings"

	"vpsbackupmanager/internal/domain"
)

// applyRetention runs the RETENTION step after a verified backup. A
// deletion failure is recorded as a warning and never turns a successful
// run into a failed one, per the retention specification.
func (rc *runContext) applyRetention() {
	step, err := rc.steps.Start(rc.ctx, rc.now(), domain.StepRetention)
	if err != nil {
		rc.engine.logger().Error("start retention step", "run_id", rc.run.ID, "error", err)
		return
	}

	_, warnings, applyErr := rc.engine.Retention.Apply(rc.ctx, rc.job)
	ok := applyErr == nil && len(warnings) == 0
	if applyErr != nil {
		rc.warn(domain.ErrInternal, "retention: "+applyErr.Error())
		step.Error = rc.run.ErrorMessage
	} else if len(warnings) > 0 {
		rc.warn(domain.ErrInternal, "retention: "+strings.Join(warnings, "; "))
		step.Error = rc.run.ErrorMessage
	}
	step.Finish(rc.now(), stepStatus(ok))
	if err := rc.steps.Finish(rc.ctx, step); err != nil {
		rc.engine.logger().Error("persist retention step", "run_id", rc.run.ID, "error", err)
	}
}
