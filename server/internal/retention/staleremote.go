package retention

import (
	"context"
	"fmt"
	"strings"
	"time"

	"vpsbackupmanager/internal/backup"
	"vpsbackupmanager/internal/domain"
)

// DefaultStaleAge matches the retention specification's default: a remote
// temporary archive is considered stale after 24 hours.
const DefaultStaleAge = 24 * time.Hour

// LockChecker reports whether a job currently has an active run, so stale
// cleanup never touches a running job's remote temporary directory.
type LockChecker interface {
	IsLocked(ctx context.Context, jobID string) (bool, error)
}

// StaleFinding is one remote temporary file older than the configured
// stale age.
type StaleFinding struct {
	JobID string
	Path  string
}

// RemoteCleaner detects (and, if enabled, deletes) stale temporary
// archives left in a job's remote temporary directory.
type RemoteCleaner struct {
	Connect backup.Connector
	Locks   LockChecker
	Clock   Clock
}

// NewRemoteCleaner constructs a RemoteCleaner.
func NewRemoteCleaner(connect backup.Connector, locks LockChecker) *RemoteCleaner {
	return &RemoteCleaner{Connect: connect, Locks: locks, Clock: SystemClock{}}
}

// Scan finds files in job's remote temporary directory older than maxAge,
// deleting them when autoDelete is true. It never touches the directory
// of a job with an active run.
func (c *RemoteCleaner) Scan(ctx context.Context, job *domain.Job, server *domain.Server, p backup.ConnectParams, maxAge time.Duration, autoDelete bool) ([]StaleFinding, error) {
	locked, err := c.Locks.IsLocked(ctx, job.ID)
	if err != nil {
		return nil, fmt.Errorf("check active lock for job %q: %w", job.ID, err)
	}
	if locked {
		return nil, nil
	}

	transport, err := c.Connect(ctx, p)
	if err != nil {
		return nil, err
	}
	defer func() { _ = transport.Close() }()

	dir := job.RemoteJobDirectory()
	minutes := int(maxAge.Minutes())
	cmd := fmt.Sprintf("find %s -maxdepth 1 -type f -mmin +%d 2>/dev/null", shellQuote(dir), minutes)
	res, err := transport.Run(ctx, cmd, time.Duration(server.CommandTimeoutSeconds)*time.Second)
	if err != nil || res.ExitCode != 0 {
		// The directory may simply not exist yet (job never ran); that is
		// not a stale-archive finding, and this is a best-effort sweep.
		return nil, nil
	}

	var findings []StaleFinding
	for _, path := range strings.Split(strings.TrimSpace(res.Stdout), "\n") {
		if path == "" {
			continue
		}
		findings = append(findings, StaleFinding{JobID: job.ID, Path: path})
	}

	if autoDelete {
		for _, f := range findings {
			if _, err := transport.Run(ctx, "rm -f "+shellQuote(f.Path), time.Duration(server.CommandTimeoutSeconds)*time.Second); err != nil {
				return findings, fmt.Errorf("delete stale remote archive %q: %w", f.Path, err)
			}
		}
	}
	return findings, nil
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
