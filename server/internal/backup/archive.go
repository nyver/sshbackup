package backup

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"vpsbackupmanager/internal/domain"
)

// archive runs the ARCHIVE step: build a single tar.gz on the remote host
// containing every source path, honoring exclude patterns and the job's
// archive timeout.
func (rc *runContext) archive() {
	step, err := rc.steps.Start(rc.ctx, rc.now(), domain.StepArchive)
	if err != nil {
		rc.fail(domain.ErrInternal, "could not record archive step")
		return
	}
	ok := rc.runArchive(step)
	step.Finish(rc.now(), stepStatus(ok))
	if err := rc.steps.Finish(rc.ctx, step); err != nil {
		rc.engine.logger().Error("persist archive step", "run_id", rc.run.ID, "error", err)
	}
}

func (rc *runContext) runArchive(step *domain.RunStep) bool {
	rc.run.ArchiveName = domain.ArchiveFileName(rc.job.Name, rc.run.StartedAt)
	rc.remoteArchivePath = rc.job.RemoteJobDirectory() + "/" + rc.run.ArchiveName

	cmd := rc.archiveCommand()
	result, err := rc.transport.Run(rc.ctx, cmd, rc.job.ArchiveTimeout)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			rc.fail(domain.ErrArchiveTimeout, "archive creation exceeded its timeout")
		} else {
			rc.fail(domain.ErrArchiveFailed, rc.engine.redact(err.Error()))
		}
		step.Error = rc.run.ErrorMessage
		rc.removePartialRemoteArchive()
		return false
	}
	if result.ExitCode != 0 {
		rc.fail(domain.ErrArchiveFailed, fmt.Sprintf("tar exited with code %d: %s", result.ExitCode, rc.engine.redact(result.Stderr)))
		step.Error = rc.run.ErrorMessage
		rc.removePartialRemoteArchive()
		return false
	}
	return true
}

func (rc *runContext) archiveCommand() string {
	var excludeArgs []string
	var sourceArgs []string
	hasInclude := false
	for _, s := range rc.job.Sources {
		for _, pattern := range s.Exclude {
			excludeArgs = append(excludeArgs, "--exclude="+shellQuote(pattern))
		}
		sourceArgs = append(sourceArgs, shellQuote(s.RemotePath))
		if len(s.Include) > 0 {
			hasInclude = true
		}
	}

	mkdir := "mkdir -p " + shellQuote(rc.job.RemoteJobDirectory())
	if hasInclude {
		return mkdir + " && " + rc.includeArchiveCommand(excludeArgs)
	}

	parts := []string{mkdir, "&&", "tar"}
	parts = append(parts, excludeArgs...)
	parts = append(parts, "-czf", shellQuote(rc.remoteArchivePath), "--")
	parts = append(parts, sourceArgs...)
	return strings.Join(parts, " ")
}

// includeArchiveCommand builds the tar invocation for jobs where at least one
// source has include patterns. tar cannot filter by include pattern while
// recursing, so the entries to archive are listed with find into a
// NUL-separated file that tar reads with -T.
//
// NOTE: an include pattern is matched (find -name glob) against the base name
// of every entry below the source path; a matching directory is archived with
// its whole content, and a trailing "/" is ignored. Sources without include
// patterns are archived whole. Exclude patterns still apply on top. This needs
// GNU tar (--null) and find (-print0) on the remote host; the list file is
// used instead of a pipe so that a failing find (e.g. a missing source path)
// aborts the run under plain POSIX sh, which has no pipefail.
func (rc *runContext) includeArchiveCommand(excludeArgs []string) string {
	list := shellQuote(rc.remoteArchivePath + ".list")

	var listCmds []string
	for _, s := range rc.job.Sources {
		if len(s.Include) == 0 {
			listCmds = append(listCmds, `printf '%s\000' `+shellQuote(s.RemotePath))
			continue
		}
		names := make([]string, 0, len(s.Include))
		for _, pattern := range s.Include {
			names = append(names, "-name "+shellQuote(strings.TrimRight(pattern, "/")))
		}
		listCmds = append(listCmds, "find "+shellQuote(s.RemotePath)+
			` -mindepth 1 \( `+strings.Join(names, " -o ")+` \) -prune -print0`)
	}

	tar := []string{"tar", "--null", "-T", list}
	tar = append(tar, excludeArgs...)
	tar = append(tar, "-czf", shellQuote(rc.remoteArchivePath))
	return "{ " + strings.Join(listCmds, " && ") + "; } > " + list +
		" && " + strings.Join(tar, " ") +
		"; status=$?; rm -f " + list + "; exit $status"
}

// removePartialRemoteArchive best-effort deletes a remote archive left
// behind by a failed archive stage, so retries don't collide with debris.
func (rc *runContext) removePartialRemoteArchive() {
	if rc.remoteArchivePath == "" {
		return
	}
	if _, err := rc.transport.Run(context.WithoutCancel(rc.ctx), "rm -f "+shellQuote(rc.remoteArchivePath)+" "+shellQuote(rc.remoteArchivePath+".list"), 0); err != nil {
		rc.engine.logger().Warn("remove partial remote archive", "run_id", rc.run.ID, "path", rc.remoteArchivePath, "error", err)
	}
}
