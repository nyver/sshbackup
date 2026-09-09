package backup

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"vpsbackupmanager/internal/domain"
)

// checkResult is one pass/fail outcome, pure of any run/step bookkeeping,
// shared by the run engine's fail-fast preflight stage and the
// non-fail-fast Validate Job operation.
type checkResult struct {
	ok      bool
	code    domain.ErrorCode
	message string
}

func passed(message string) checkResult { return checkResult{ok: true, message: message} }

func failedCheck(code domain.ErrorCode, message string) checkResult {
	return checkResult{ok: false, code: code, message: message}
}

func runOnTransport(ctx context.Context, transport Transport, timeoutSeconds int, cmd string) (RunResult, error) {
	return transport.Run(ctx, cmd, time.Duration(timeoutSeconds)*time.Second)
}

func checkSourceExists(ctx context.Context, transport Transport, timeoutSeconds int, path string) checkResult {
	res, err := runOnTransport(ctx, transport, timeoutSeconds, "test -e "+shellQuote(path))
	if err != nil || res.ExitCode != 0 {
		return failedCheck(domain.ErrRemoteSourceNotFound, fmt.Sprintf("source path %s does not exist on the remote host", path))
	}
	return passed(fmt.Sprintf("source path %s exists", path))
}

func checkSourceReadable(ctx context.Context, transport Transport, timeoutSeconds int, path string) checkResult {
	if r := checkSourceExists(ctx, transport, timeoutSeconds, path); !r.ok {
		return r
	}
	res, err := runOnTransport(ctx, transport, timeoutSeconds, "test -r "+shellQuote(path))
	if err != nil || res.ExitCode != 0 {
		return failedCheck(domain.ErrRemotePermissionDenied, fmt.Sprintf("source path %s is not readable by the configured user", path))
	}
	return passed(fmt.Sprintf("source path %s is readable", path))
}

func checkToolAvailable(ctx context.Context, transport Transport, timeoutSeconds int, tool string) checkResult {
	res, err := runOnTransport(ctx, transport, timeoutSeconds, "command -v "+shellQuote(tool))
	if err != nil || res.ExitCode != 0 {
		return failedCheck(domain.ErrRemoteToolMissing, fmt.Sprintf("required remote utility %q is not available", tool))
	}
	return passed(fmt.Sprintf("%s is available", tool))
}

func checkRemoteTempWritable(ctx context.Context, transport Transport, timeoutSeconds int, dir string) checkResult {
	cmd := "mkdir -p " + shellQuote(dir) + " && test -w " + shellQuote(dir)
	res, err := runOnTransport(ctx, transport, timeoutSeconds, cmd)
	if err != nil || res.ExitCode != 0 {
		return failedCheck(domain.ErrRemotePermissionDenied, fmt.Sprintf("remote temporary directory %s is not writable", dir))
	}
	return passed(fmt.Sprintf("remote temporary directory %s is writable", dir))
}

// estimateSourceSize sums `du -sb` over every source path. A missing `du`
// or any other failure degrades to "unknown" rather than failing,
// matching the free-space-checks specification.
func estimateSourceSize(ctx context.Context, transport Transport, timeoutSeconds int, sources []domain.Source) (bytes int64, known bool) {
	quoted := make([]string, len(sources))
	for i, s := range sources {
		quoted[i] = shellQuote(s.RemotePath)
	}
	cmd := "du -sb " + strings.Join(quoted, " ") + " 2>/dev/null | awk '{s+=$1} END{print s}'"
	res, err := runOnTransport(ctx, transport, timeoutSeconds, cmd)
	if err != nil || res.ExitCode != 0 {
		return 0, false
	}
	n, parseErr := strconv.ParseInt(strings.TrimSpace(res.Stdout), 10, 64)
	if parseErr != nil {
		return 0, false
	}
	return n, true
}

func measureRemoteFreeSpace(ctx context.Context, transport Transport, timeoutSeconds int, remoteTempDir string) (bytes int64, known bool) {
	cmd := "df -B1 --output=avail " + shellQuote(remoteTempDir) + " | tail -n1"
	res, err := runOnTransport(ctx, transport, timeoutSeconds, cmd)
	if err != nil || res.ExitCode != 0 {
		return 0, false
	}
	free, parseErr := strconv.ParseInt(strings.TrimSpace(res.Stdout), 10, 64)
	if parseErr != nil {
		return 0, false
	}
	return free, true
}

func checkRemoteFreeSpace(remoteFree int64, remoteFreeKnown bool, sourceSize int64, sourceSizeKnown bool) checkResult {
	if !remoteFreeKnown {
		return passed("remote free space could not be measured; continuing without a space check")
	}
	if sourceSizeKnown && sourceSize > remoteFree {
		return failedCheck(domain.ErrRemoteNoSpace, fmt.Sprintf(
			"estimated source size %d bytes exceeds remote free space %d bytes", sourceSize, remoteFree))
	}
	return passed(fmt.Sprintf("remote free space: %d bytes", remoteFree))
}
