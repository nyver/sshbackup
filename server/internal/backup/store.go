package backup

import (
	"context"

	"vpsbackupmanager/internal/domain"
)

// RunStore is the persistence surface the engine needs for runs. It is
// satisfied by *store.RunRepository without either package importing the
// other's concrete types.
type RunStore interface {
	Create(ctx context.Context, run *domain.Run) error
	Update(ctx context.Context, run *domain.Run) error
}

// StepStore is the persistence surface the engine needs for steps.
type StepStore interface {
	Create(ctx context.Context, step *domain.RunStep, position int) error
	Update(ctx context.Context, step *domain.RunStep) error
}

// LockStore is the persistence surface the engine needs for job locks.
type LockStore interface {
	Acquire(ctx context.Context, jobID, runID, startedAt string, processID int) error
	Release(ctx context.Context, jobID string) error
}
