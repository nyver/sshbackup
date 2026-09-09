package scheduler

import (
	"context"

	"vpsbackupmanager/internal/domain"
)

// JobStore is the persistence surface the scheduler needs for jobs.
type JobStore interface {
	List(ctx context.Context) ([]*domain.Job, error)
}

// ServerStore is the persistence surface the scheduler needs for servers.
type ServerStore interface {
	Get(ctx context.Context, id string) (*domain.Server, error)
}

// SettingsStore is the persistence surface the scheduler needs for global
// settings (pause state, concurrency limit).
type SettingsStore interface {
	Get(ctx context.Context) (domain.Settings, error)
}

// RunStore is the persistence surface the scheduler needs for runs: the
// most recent run of a job, for missed-run detection, and creating a
// SKIPPED bookkeeping run when a missed occurrence is deliberately not
// caught up.
type RunStore interface {
	LastRun(ctx context.Context, jobID string) (*domain.Run, error)
	Create(ctx context.Context, run *domain.Run) error
}

// Runner starts one run of job on server. Production wiring resolves
// credentials and calls backup.Engine.Run; tests inject a fake.
type Runner interface {
	Run(ctx context.Context, job *domain.Job, server *domain.Server, trigger domain.Trigger) (*domain.Run, error)
}
