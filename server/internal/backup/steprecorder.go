package backup

import (
	"context"
	"time"

	"vpsbackupmanager/internal/domain"
)

// stepRecorder persists each executed stage as a domain.RunStep, in
// workflow order, via the position counter it owns.
type stepRecorder struct {
	steps    StepStore
	runID    string
	position int

	// onChange, if set, is called after every persisted Finish or Skip so
	// Engine.Events can push a live step-changed event.
	onChange func(*domain.RunStep)
}

func newStepRecorder(steps StepStore, runID string) *stepRecorder {
	return &stepRecorder{steps: steps, runID: runID}
}

// Start creates and persists a new step in RUNNING status.
func (r *stepRecorder) Start(ctx context.Context, now time.Time, stepType domain.StepType) (*domain.RunStep, error) {
	step, err := domain.NewRunStep(now, r.runID, stepType)
	if err != nil {
		return nil, err
	}
	if err := r.steps.Create(ctx, step, r.position); err != nil {
		return nil, err
	}
	r.position++
	return step, nil
}

// Finish persists step's terminal state.
func (r *stepRecorder) Finish(ctx context.Context, step *domain.RunStep) error {
	if err := r.steps.Update(ctx, step); err != nil {
		return err
	}
	if r.onChange != nil {
		r.onChange(step)
	}
	return nil
}

// Skip creates and persists a step that never ran, per the
// run-history specification's skipped-step requirement.
func (r *stepRecorder) Skip(ctx context.Context, now time.Time, stepType domain.StepType, reason string) error {
	step, err := domain.NewRunStep(now, r.runID, stepType)
	if err != nil {
		return err
	}
	step.Skip(now, reason)
	if err := r.steps.Create(ctx, step, r.position); err != nil {
		return err
	}
	r.position++
	if r.onChange != nil {
		r.onChange(step)
	}
	return nil
}
