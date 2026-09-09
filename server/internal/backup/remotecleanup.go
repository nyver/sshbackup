package backup

import (
	"fmt"

	"vpsbackupmanager/internal/domain"
)

// remoteCleanup deletes the remote temporary archive, if one was created,
// regardless of whether the run otherwise succeeded: a run that failed
// after archiving still gets its remote temp file cleaned up, and a
// cleanup failure after an otherwise successful run downgrades the run to
// WARNING rather than FAILED.
func (rc *runContext) remoteCleanup() {
	if rc.remoteArchivePath == "" || rc.transport == nil {
		return
	}

	step, err := rc.steps.Start(rc.ctx, rc.now(), domain.StepRemoteCleanup)
	if err != nil {
		rc.engine.logger().Error("start remote cleanup step", "run_id", rc.run.ID, "error", err)
		return
	}

	res, runErr := rc.runRemote("rm -f " + shellQuote(rc.remoteArchivePath))
	ok := runErr == nil && res.ExitCode == 0
	if !ok {
		rc.warn(domain.ErrInternal, fmt.Sprintf("could not remove remote temporary archive %s", rc.remoteArchivePath))
		step.Error = rc.run.ErrorMessage
	}
	step.Finish(rc.now(), stepStatus(ok))
	if err := rc.steps.Finish(rc.ctx, step); err != nil {
		rc.engine.logger().Error("persist remote cleanup step", "run_id", rc.run.ID, "error", err)
	}
}
