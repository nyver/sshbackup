// Package backup implements the backup run engine: the staged workflow
// (lock, pre-flight, scripts, archive, download, verify, cleanup,
// retention), the cleanup registry that guarantees recovery actions run,
// and the narrow Transport port the engine depends on for all remote work.
package backup

import (
	"context"
	"io"
	"time"
)

// Transport is the narrow SSH/SFTP surface the run engine depends on. It is
// implemented in production by internal/remote and, in tests, by the
// scripted fake in internal/backup/backuptest — which can fail any
// command, block until context cancellation, or return oversized output —
// so the engine's failure handling is fully testable without a real VPS.
type Transport interface {
	// Run executes cmd on the remote host, honoring timeout, and returns
	// its exit code, captured output, and duration.
	Run(ctx context.Context, cmd string, timeout time.Duration) (RunResult, error)
	// Download streams remotePath to w in bounded chunks and honors ctx
	// cancellation.
	Download(ctx context.Context, remotePath string, w io.Writer) error
	// Close releases the underlying connection.
	Close() error
}

// RunResult is the outcome of one Transport.Run call.
type RunResult struct {
	ExitCode        int
	Stdout          string
	Stderr          string
	Duration        time.Duration
	StdoutTruncated bool
	StderrTruncated bool
}
