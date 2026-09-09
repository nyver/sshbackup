package domain

import (
	"errors"
	"fmt"
)

// ErrorCode is the stable, machine-readable vocabulary shared by the run
// engine, the database, the IPC API, and the UI (specification §64, plus
// SSH_HOST_KEY_UNVERIFIED for the trust-on-first-use flow).
type ErrorCode string

const (
	// ErrSSHConnectionFailed is a TCP or SSH handshake failure.
	ErrSSHConnectionFailed ErrorCode = "SSH_CONNECTION_FAILED"
	// ErrSSHAuthFailed is an SSH authentication rejection, an unreadable
	// key file, or an unresolvable credential reference.
	ErrSSHAuthFailed ErrorCode = "SSH_AUTH_FAILED"
	// ErrSSHHostKeyUnverified means no fingerprint is stored yet and the
	// connection is unattended, so trust-on-first-use cannot run.
	ErrSSHHostKeyUnverified ErrorCode = "SSH_HOST_KEY_UNVERIFIED"
	// ErrSSHHostKeyChanged means the presented host key does not match the
	// stored fingerprint.
	ErrSSHHostKeyChanged ErrorCode = "SSH_HOST_KEY_CHANGED"

	// ErrRemoteSourceNotFound means a configured source path does not
	// exist on the remote host.
	ErrRemoteSourceNotFound ErrorCode = "REMOTE_SOURCE_NOT_FOUND"
	// ErrRemotePermissionDenied means a remote path exists but is not
	// readable (or the remote temp directory is not writable) by the SSH
	// user.
	ErrRemotePermissionDenied ErrorCode = "REMOTE_PERMISSION_DENIED"
	// ErrRemoteToolMissing means a required remote utility (tar, gzip,
	// sha256sum) is not available.
	ErrRemoteToolMissing ErrorCode = "REMOTE_TOOL_MISSING"
	// ErrRemoteNoSpace means the estimated source size exceeds remote
	// free space in the temporary directory's filesystem.
	ErrRemoteNoSpace ErrorCode = "REMOTE_NO_SPACE"

	// ErrArchiveFailed is any archive creation failure other than a
	// timeout.
	ErrArchiveFailed ErrorCode = "ARCHIVE_FAILED"
	// ErrArchiveTimeout means archive creation exceeded the job's archive
	// timeout.
	ErrArchiveTimeout ErrorCode = "ARCHIVE_TIMEOUT"

	// ErrDownloadFailed is an SFTP download failure, including a dropped
	// connection mid-transfer.
	ErrDownloadFailed ErrorCode = "DOWNLOAD_FAILED"
	// ErrChecksumMismatch means the downloaded file's SHA-256 does not
	// match the remote archive's SHA-256.
	ErrChecksumMismatch ErrorCode = "CHECKSUM_MISMATCH"
	// ErrPreScriptFailed means a PRE_BACKUP script exited non-zero or
	// timed out.
	ErrPreScriptFailed ErrorCode = "PRE_SCRIPT_FAILED"
	// ErrPostScriptFailed means a POST_BACKUP script exited non-zero or
	// timed out.
	ErrPostScriptFailed ErrorCode = "POST_SCRIPT_FAILED"
	// ErrHealthCheckFailed means every configured health check attempt
	// failed.
	ErrHealthCheckFailed ErrorCode = "HEALTH_CHECK_FAILED"

	// ErrLocalNoSpace means the estimated source size exceeds free space
	// on the local destination volume.
	ErrLocalNoSpace ErrorCode = "LOCAL_NO_SPACE"
	// ErrLocalPermissionDenied means the local destination directory
	// cannot be written by the service account.
	ErrLocalPermissionDenied ErrorCode = "LOCAL_PERMISSION_DENIED"

	// ErrJobAlreadyRunning means a run could not start because the job's
	// lock is held by another active run.
	ErrJobAlreadyRunning ErrorCode = "JOB_ALREADY_RUNNING"
	// ErrInvalidConfig means job or server configuration failed
	// validation.
	ErrInvalidConfig ErrorCode = "INVALID_CONFIGURATION"
	// ErrRunCancelled means the run was stopped by an explicit user
	// cancellation.
	ErrRunCancelled ErrorCode = "RUN_CANCELLED"
	// ErrNotFound means the requested entity does not exist.
	ErrNotFound ErrorCode = "NOT_FOUND"
	// ErrInternal is an unexpected internal failure with no more specific
	// code.
	ErrInternal ErrorCode = "INTERNAL_ERROR"
)

// CodedError wraps an error with a stable ErrorCode so callers across
// package boundaries can branch on the code with errors.As while the
// wrapped chain still carries the underlying cause for logs.
type CodedError struct {
	Code    ErrorCode
	Message string
	Err     error
}

// NewCodedError builds a CodedError. err may be nil when the code is the
// only information available (e.g. a validation failure with no underlying
// cause).
func NewCodedError(code ErrorCode, message string, err error) *CodedError {
	return &CodedError{Code: code, Message: message, Err: err}
}

func (e *CodedError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *CodedError) Unwrap() error {
	return e.Err
}

// Is reports whether target is a *CodedError with the same Code, so
// sentinel-style checks such as errors.Is(err, &CodedError{Code: ...}) work.
func (e *CodedError) Is(target error) bool {
	t, ok := target.(*CodedError)
	if !ok {
		return false
	}
	return e.Code == t.Code
}

// CodeOf extracts the ErrorCode from err, walking the Unwrap chain, and
// returns ("", false) when no CodedError is present.
func CodeOf(err error) (ErrorCode, bool) {
	var ce *CodedError
	if errors.As(err, &ce) {
		return ce.Code, true
	}
	return "", false
}
