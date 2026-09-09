// Package retention deletes local archives a job's retention policy no
// longer requires, and detects (and optionally deletes) stale temporary
// archives left behind on remote hosts.
package retention

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"vpsbackupmanager/internal/backup"
	"vpsbackupmanager/internal/domain"
)

// RunStore is the persistence surface retention needs: the job's own
// archived runs, and clearing a run's archive_name once its file is
// deleted so history reflects that no local file remains.
type RunStore interface {
	ListArchived(ctx context.Context, jobID string) ([]*domain.Run, error)
	Update(ctx context.Context, run *domain.Run) error
}

// Clock abstracts wall-clock time so max_age_days tests don't depend on
// real time.
type Clock interface {
	Now() time.Time
}

// SystemClock is the production Clock backed by the real time package.
type SystemClock struct{}

// Now returns the current wall-clock time.
func (SystemClock) Now() time.Time { return time.Now() }

// Applier applies a job's local retention policy. It satisfies
// backup.RetentionApplier structurally, so internal/backup never needs to
// import this package.
type Applier struct {
	Runs  RunStore
	Clock Clock
}

// NewApplier constructs an Applier over runs.
func NewApplier(runs RunStore) *Applier {
	return &Applier{Runs: runs, Clock: SystemClock{}}
}

var _ backup.RetentionApplier = (*Applier)(nil)

// Apply deletes archives of job that fall outside its retention policy,
// considering only archives recorded in the job's own run history. When
// neither keep_last nor max_age_days is set, nothing is deleted. When both
// are set, an archive is deleted only when it is outside both limits.
// Deletion failures are returned as warnings, never as err, and the run
// history row is kept (with its archive_name cleared) after a file is
// deleted.
func (a *Applier) Apply(ctx context.Context, job *domain.Job) (deleted int, warnings []string, err error) {
	if !job.RetentionPolicy.HasLimit() {
		return 0, nil, nil
	}

	runs, err := a.Runs.ListArchived(ctx, job.ID)
	if err != nil {
		return 0, nil, fmt.Errorf("list archived runs for job %q: %w", job.ID, err)
	}

	var cutoff time.Time
	hasCutoff := job.RetentionPolicy.MaxAgeDays != nil
	if hasCutoff {
		cutoff = a.Clock.Now().AddDate(0, 0, -*job.RetentionPolicy.MaxAgeDays)
	}

	for i, run := range runs {
		if !shouldDelete(job.RetentionPolicy, i, run.StartedAt, cutoff, hasCutoff) {
			continue
		}

		path := filepath.Join(job.LocalDestination, run.ArchiveName)
		if rmErr := os.Remove(path); rmErr != nil && !os.IsNotExist(rmErr) {
			warnings = append(warnings, fmt.Sprintf("could not delete archive %s: %v", run.ArchiveName, rmErr))
			continue
		}

		run.ArchiveName = ""
		if updErr := a.Runs.Update(ctx, run); updErr != nil {
			warnings = append(warnings, fmt.Sprintf("deleted %s but could not update its history record: %v", path, updErr))
			continue
		}
		deleted++
	}
	return deleted, warnings, nil
}

// shouldDelete implements the retention specification's combination rule:
// an archive is deleted only when it falls outside every limit that is
// actually configured. index is the archive's position in the
// most-recent-first list (0 = newest), used against keep_last.
func shouldDelete(policy domain.RetentionPolicy, index int, startedAt, cutoff time.Time, hasCutoff bool) bool {
	withinKeepLast := policy.KeepLast != nil && index < *policy.KeepLast
	withinMaxAge := hasCutoff && !startedAt.Before(cutoff)

	switch {
	case policy.KeepLast != nil && policy.MaxAgeDays != nil:
		return !withinKeepLast && !withinMaxAge
	case policy.KeepLast != nil:
		return !withinKeepLast
	case policy.MaxAgeDays != nil:
		return !withinMaxAge
	default:
		return false
	}
}
