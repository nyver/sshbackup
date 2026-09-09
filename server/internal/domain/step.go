package domain

import "time"

// MaxStepOutputBytes caps captured stdout/stderr per step, per the
// run-history specification.
const MaxStepOutputBytes = 10 * 1024 * 1024 // 10 MB

// RunStep is one recorded stage of a run's workflow.
type RunStep struct {
	ID    string
	RunID string
	Type  StepType

	StartedAt  time.Time
	FinishedAt *time.Time
	Status     StepStatus

	Output    string
	Truncated bool
	Error     string

	// ExitCode and Duration are populated for script steps only.
	ExitCode *int
	Duration time.Duration
}

// NewRunStep constructs a step in RUNNING status.
func NewRunStep(now time.Time, runID string, stepType StepType) (*RunStep, error) {
	id, err := NewID()
	if err != nil {
		return nil, err
	}
	return &RunStep{
		ID:        id,
		RunID:     runID,
		Type:      stepType,
		StartedAt: now,
		Status:    StepRunning,
	}, nil
}

// Finish marks the step terminal with the given status.
func (s *RunStep) Finish(now time.Time, status StepStatus) {
	s.Status = status
	s.FinishedAt = &now
}

// Skip marks the step SKIPPED with a reason, without ever having started.
func (s *RunStep) Skip(now time.Time, reason string) {
	s.Status = StepSkipped
	s.Error = reason
	s.FinishedAt = &now
}
