package backup

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"vpsbackupmanager/internal/domain"
)

// downloadAndVerify runs the remote-checksum, download, local-checksum,
// and verify-checksum steps in order, stopping at the first failure.
func (rc *runContext) downloadAndVerify() {
	remoteSum, ok := rc.remoteChecksum()
	if !ok {
		return
	}
	localSum, ok := rc.downloadArchive()
	if !ok {
		return
	}
	rc.localChecksumStep(localSum)
	rc.verifyChecksums(remoteSum, localSum)
}

func (rc *runContext) remoteChecksum() (string, bool) {
	step, err := rc.steps.Start(rc.ctx, rc.now(), domain.StepRemoteChecksum)
	if err != nil {
		rc.fail(domain.ErrInternal, "could not record remote checksum step")
		return "", false
	}

	cmd := "sha256sum " + shellQuote(rc.remoteArchivePath) + " | awk '{print $1}'"
	res, runErr := rc.runRemote(cmd)
	sum := strings.TrimSpace(res.Stdout)
	ok := runErr == nil && res.ExitCode == 0 && sum != ""
	if !ok {
		rc.fail(domain.ErrArchiveFailed, "could not compute the remote archive checksum")
		step.Error = rc.run.ErrorMessage
	} else {
		step.Output = sum
	}
	step.Finish(rc.now(), stepStatus(ok))
	if err := rc.steps.Finish(rc.ctx, step); err != nil {
		rc.engine.logger().Error("persist remote checksum step", "run_id", rc.run.ID, "error", err)
	}
	return sum, ok
}

func (rc *runContext) downloadArchive() (localSum string, ok bool) {
	step, err := rc.steps.Start(rc.ctx, rc.now(), domain.StepDownload)
	if err != nil {
		rc.fail(domain.ErrInternal, "could not record download step")
		return "", false
	}

	rc.localPartPath = filepath.Join(rc.destDir, rc.run.ArchiveName+".part")
	sum, downloadErr := rc.streamDownload()
	if downloadErr != nil {
		code, codeOK := domain.CodeOf(downloadErr)
		if !codeOK {
			code = domain.ErrDownloadFailed
		}
		rc.fail(code, rc.engine.redact(downloadErr.Error()))
		step.Error = rc.run.ErrorMessage
		rc.deletePart()
		step.Finish(rc.now(), domain.StepFailed)
		if err := rc.steps.Finish(rc.ctx, step); err != nil {
			rc.engine.logger().Error("persist download step", "run_id", rc.run.ID, "error", err)
		}
		return "", false
	}

	step.Finish(rc.now(), domain.StepSuccess)
	if err := rc.steps.Finish(rc.ctx, step); err != nil {
		rc.engine.logger().Error("persist download step", "run_id", rc.run.ID, "error", err)
	}
	return sum, true
}

// streamDownload writes the remote archive to localPartPath while hashing
// the same stream, so memory use stays bounded regardless of archive size.
func (rc *runContext) streamDownload() (string, error) {
	file, err := os.Create(rc.localPartPath) //nolint:gosec // path is built from job.LocalDestination + a generated archive name, not external input
	if err != nil {
		return "", fmt.Errorf("create local part file: %w", err)
	}
	defer func() { _ = file.Close() }()

	hasher := sha256.New()
	if err := rc.transport.Download(rc.ctx, rc.remoteArchivePath, io.MultiWriter(file, hasher)); err != nil {
		return "", err
	}
	if err := file.Sync(); err != nil {
		return "", fmt.Errorf("flush local part file: %w", err)
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func (rc *runContext) localChecksumStep(localSum string) {
	step, err := rc.steps.Start(rc.ctx, rc.now(), domain.StepLocalChecksum)
	if err != nil {
		rc.engine.logger().Error("start local checksum step", "run_id", rc.run.ID, "error", err)
		return
	}
	step.Output = localSum
	step.Finish(rc.now(), domain.StepSuccess)
	if err := rc.steps.Finish(rc.ctx, step); err != nil {
		rc.engine.logger().Error("persist local checksum step", "run_id", rc.run.ID, "error", err)
	}
}

func (rc *runContext) verifyChecksums(remoteSum, localSum string) {
	step, err := rc.steps.Start(rc.ctx, rc.now(), domain.StepVerifyChecksum)
	if err != nil {
		rc.fail(domain.ErrInternal, "could not record verify checksum step")
		return
	}

	if remoteSum != localSum {
		rc.fail(domain.ErrChecksumMismatch, fmt.Sprintf("checksum mismatch: remote %s, local %s", remoteSum, localSum))
		step.Error = rc.run.ErrorMessage
		rc.deletePart()
		step.Finish(rc.now(), domain.StepFailed)
		if err := rc.steps.Finish(rc.ctx, step); err != nil {
			rc.engine.logger().Error("persist verify checksum step", "run_id", rc.run.ID, "error", err)
		}
		return
	}

	finalPath := filepath.Join(rc.destDir, rc.run.ArchiveName)
	if err := os.Rename(rc.localPartPath, finalPath); err != nil {
		rc.fail(domain.ErrDownloadFailed, fmt.Sprintf("could not finalize downloaded archive: %v", err))
		step.Error = rc.run.ErrorMessage
		step.Finish(rc.now(), domain.StepFailed)
		if err := rc.steps.Finish(rc.ctx, step); err != nil {
			rc.engine.logger().Error("persist verify checksum step", "run_id", rc.run.ID, "error", err)
		}
		return
	}

	rc.archiveVerified = true
	rc.run.Checksum = localSum
	info, statErr := os.Stat(finalPath)
	if statErr == nil {
		rc.run.ArchiveSize = info.Size()
	}
	step.Finish(rc.now(), domain.StepSuccess)
	if err := rc.steps.Finish(rc.ctx, step); err != nil {
		rc.engine.logger().Error("persist verify checksum step", "run_id", rc.run.ID, "error", err)
	}
}

// deletePart removes a partially downloaded or checksum-mismatched .part
// file so it is never mistaken for a valid archive.
func (rc *runContext) deletePart() {
	if rc.localPartPath == "" {
		return
	}
	if err := os.Remove(rc.localPartPath); err != nil && !os.IsNotExist(err) {
		rc.engine.logger().Warn("remove .part file", "run_id", rc.run.ID, "path", rc.localPartPath, "error", err)
	}
}
