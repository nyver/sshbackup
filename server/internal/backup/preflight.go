package backup

import (
	"vpsbackupmanager/internal/domain"
)

// preflight runs the PREFLIGHT step: connect, host key trust, source
// checks, tooling checks, remote/local writability and free space. It
// returns false when the run should stop before any user script executes.
// Each check is a pure function shared with the non-fail-fast Validate Job
// operation (see checks.go, diskspace.go, and validate.go).
func (rc *runContext) preflight() bool {
	step, err := rc.steps.Start(rc.ctx, rc.now(), domain.StepPreflight)
	if err != nil {
		rc.fail(domain.ErrInternal, "could not record preflight step")
		return false
	}
	ok := rc.runPreflight()
	step.Error = rc.run.ErrorMessage
	step.Finish(rc.now(), stepStatus(ok))
	if err := rc.steps.Finish(rc.ctx, step); err != nil {
		rc.engine.logger().Error("persist preflight step", "run_id", rc.run.ID, "error", err)
	}
	return ok
}

func stepStatus(ok bool) domain.StepStatus {
	if ok {
		return domain.StepSuccess
	}
	return domain.StepFailed
}

func (rc *runContext) runPreflight() bool {
	if err := rc.job.Validate(); err != nil {
		rc.fail(domain.ErrInvalidConfig, err.Error())
		return false
	}

	transport, err := rc.engine.Connect(rc.ctx, rc.connectP)
	if err != nil {
		code, ok := domain.CodeOf(err)
		if !ok {
			code = domain.ErrSSHConnectionFailed
		}
		rc.fail(code, rc.engine.redact(err.Error()))
		return false
	}
	rc.transport = transport
	rc.registerCleanupScripts()

	timeout := rc.server.CommandTimeoutSeconds

	for _, src := range rc.job.Sources {
		if r := checkSourceReadable(rc.ctx, transport, timeout, src.RemotePath); !r.ok {
			rc.fail(r.code, r.message)
			return false
		}
	}
	for _, tool := range []string{"tar", "gzip", "sha256sum"} {
		if r := checkToolAvailable(rc.ctx, transport, timeout, tool); !r.ok {
			rc.fail(r.code, r.message)
			return false
		}
	}
	if r := checkRemoteTempWritable(rc.ctx, transport, timeout, rc.job.RemoteJobDirectory()); !r.ok {
		rc.fail(r.code, r.message)
		return false
	}

	sourceSize, sizeKnown := estimateSourceSize(rc.ctx, transport, timeout, rc.job.Sources)
	if !sizeKnown {
		rc.warn(domain.ErrInternal, "could not estimate source size (du unavailable); continuing without a size check")
	}

	remoteFree, remoteFreeKnown := measureRemoteFreeSpace(rc.ctx, transport, timeout, rc.job.RemoteTempDirectory)
	if r := checkRemoteFreeSpace(remoteFree, remoteFreeKnown, sourceSize, sizeKnown); !r.ok {
		rc.fail(r.code, r.message)
		return false
	} else if !remoteFreeKnown {
		rc.warn(domain.ErrInternal, "could not measure remote free space (df unavailable); continuing without a space check")
	}

	if r := checkLocalDestinationWritable(rc.destDir); !r.ok {
		rc.fail(r.code, r.message)
		return false
	}

	localFree, localFreeKnown := measureLocalFreeSpace(rc.destDir)
	if r := checkLocalFreeSpace(rc.destDir, localFree, localFreeKnown, sourceSize, sizeKnown); !r.ok {
		rc.fail(r.code, r.message)
		return false
	} else if !localFreeKnown {
		rc.warn(domain.ErrInternal, "could not measure local free space; continuing without a space check")
	}

	return true
}
