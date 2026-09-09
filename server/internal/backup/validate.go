package backup

import (
	"context"

	"vpsbackupmanager/internal/domain"
)

// CheckOutcome is one named check's pass/fail result from Validate Job.
type CheckOutcome struct {
	Name    string
	Passed  bool
	Message string
}

// ValidateResult is the full outcome of Validate Job: every check's
// result plus the measured sizes, so the caller can show them even when
// every check passed.
type ValidateResult struct {
	Success bool
	Checks  []CheckOutcome

	SourceSizeBytes int64
	SourceSizeKnown bool
	RemoteFreeBytes int64
	RemoteFreeKnown bool
	LocalFreeBytes  int64
	LocalFreeKnown  bool
}

// Validator runs Validate Job: the same checks as the run engine's
// PREFLIGHT stage, but never fail-fast (every check runs and is reported)
// and never executing a user script, per the backup-jobs specification.
type Validator struct {
	Connect Connector
}

// NewValidator constructs a Validator using connect to reach the server.
func NewValidator(connect Connector) *Validator {
	return &Validator{Connect: connect}
}

// Validate checks job against server: SSH connectivity and
// authentication, source existence/readability, tooling availability,
// remote temp writability, remote and local free space against the
// estimated source size, and local destination writability.
func (v *Validator) Validate(ctx context.Context, job *domain.Job, server *domain.Server, p ConnectParams, destDir string) *ValidateResult {
	result := &ValidateResult{Success: true}
	add := func(name string, r checkResult) {
		result.Checks = append(result.Checks, CheckOutcome{Name: name, Passed: r.ok, Message: r.message})
		if !r.ok {
			result.Success = false
		}
	}

	if err := job.Validate(); err != nil {
		add("configuration", failedCheck(domain.ErrInvalidConfig, err.Error()))
		return result
	}
	add("configuration", passed("job configuration is valid"))

	transport, err := v.Connect(ctx, p)
	if err != nil {
		code, ok := domain.CodeOf(err)
		if !ok {
			code = domain.ErrSSHConnectionFailed
		}
		add("ssh_connection", failedCheck(code, err.Error()))
		return result
	}
	defer func() { _ = transport.Close() }()
	add("ssh_connection", passed("connected and authenticated"))

	timeout := server.CommandTimeoutSeconds

	for _, src := range job.Sources {
		add("source:"+src.RemotePath, checkSourceReadable(ctx, transport, timeout, src.RemotePath))
	}
	for _, tool := range []string{"tar", "gzip", "sha256sum"} {
		add("tool:"+tool, checkToolAvailable(ctx, transport, timeout, tool))
	}
	add("remote_temp_writable", checkRemoteTempWritable(ctx, transport, timeout, job.RemoteJobDirectory()))

	result.SourceSizeBytes, result.SourceSizeKnown = estimateSourceSize(ctx, transport, timeout, job.Sources)

	result.RemoteFreeBytes, result.RemoteFreeKnown = measureRemoteFreeSpace(ctx, transport, timeout, job.RemoteTempDirectory)
	add("remote_free_space", checkRemoteFreeSpace(result.RemoteFreeBytes, result.RemoteFreeKnown, result.SourceSizeBytes, result.SourceSizeKnown))

	add("local_destination_writable", checkLocalDestinationWritable(destDir))

	result.LocalFreeBytes, result.LocalFreeKnown = measureLocalFreeSpace(destDir)
	add("local_free_space", checkLocalFreeSpace(destDir, result.LocalFreeBytes, result.LocalFreeKnown, result.SourceSizeBytes, result.SourceSizeKnown))

	return result
}
