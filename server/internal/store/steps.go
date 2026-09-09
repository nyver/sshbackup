package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"vpsbackupmanager/internal/domain"
)

// StepRepository persists domain.RunStep records.
type StepRepository struct {
	db *DB
}

// NewStepRepository constructs a StepRepository over db.
func NewStepRepository(db *DB) *StepRepository {
	return &StepRepository{db: db}
}

// Create inserts a new step at the given position within its run.
func (r *StepRepository) Create(ctx context.Context, step *domain.RunStep, position int) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO run_steps (id, run_id, step_type, position, started_at, finished_at, status, output, truncated, error, exit_code, duration_ms)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		step.ID, step.RunID, string(step.Type), position, formatTime(step.StartedAt), formatTimePtr(step.FinishedAt),
		string(step.Status), truncateOutput(step.Output), boolToInt(step.Truncated), step.Error,
		nullInt(step.ExitCode), step.Duration.Milliseconds(),
	)
	if err != nil {
		return fmt.Errorf("insert step %q: %w", step.ID, err)
	}
	return nil
}

// Update persists the full current state of an existing step record.
func (r *StepRepository) Update(ctx context.Context, step *domain.RunStep) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE run_steps SET
			finished_at = ?, status = ?, output = ?, truncated = ?, error = ?, exit_code = ?, duration_ms = ?
		WHERE id = ?`,
		formatTimePtr(step.FinishedAt), string(step.Status), truncateOutput(step.Output), boolToInt(step.Truncated),
		step.Error, nullInt(step.ExitCode), step.Duration.Milliseconds(), step.ID,
	)
	if err != nil {
		return fmt.Errorf("update step %q: %w", step.ID, err)
	}
	return checkRowsAffected(res, "step", step.ID)
}

// ListByRun returns every step of a run, in execution order.
func (r *StepRepository) ListByRun(ctx context.Context, runID string) ([]*domain.RunStep, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, run_id, step_type, started_at, finished_at, status, output, truncated, error, exit_code, duration_ms
		FROM run_steps WHERE run_id = ? ORDER BY position`, runID)
	if err != nil {
		return nil, fmt.Errorf("list steps for run %q: %w", runID, err)
	}
	defer func() { _ = rows.Close() }()

	var steps []*domain.RunStep
	for rows.Next() {
		step, err := scanStep(rows)
		if err != nil {
			return nil, fmt.Errorf("scan step row: %w", err)
		}
		steps = append(steps, step)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate steps for run %q: %w", runID, err)
	}
	return steps, nil
}

// truncateOutput enforces domain.MaxStepOutputBytes as a final backstop;
// the run engine's capped writer is expected to have already bounded
// output before it reaches the repository.
func truncateOutput(output string) string {
	if len(output) <= domain.MaxStepOutputBytes {
		return output
	}
	return output[:domain.MaxStepOutputBytes]
}

func scanStep(row rowScanner) (*domain.RunStep, error) {
	var (
		step             domain.RunStep
		stepType, status string
		startedAt        string
		finishedAt       sql.NullString
		truncated        int64
		exitCode         sql.NullInt64
		durationMs       int64
	)
	if err := row.Scan(
		&step.ID, &step.RunID, &stepType, &startedAt, &finishedAt, &status,
		&step.Output, &truncated, &step.Error, &exitCode, &durationMs,
	); err != nil {
		return nil, err
	}
	step.Type = domain.StepType(stepType)
	step.Status = domain.StepStatus(status)
	step.Truncated = intToBool(truncated)
	step.ExitCode = intPtr(exitCode)
	step.Duration = time.Duration(durationMs) * time.Millisecond

	var err error
	if step.StartedAt, err = parseTime(startedAt); err != nil {
		return nil, err
	}
	if step.FinishedAt, err = parseTimePtr(finishedAt); err != nil {
		return nil, err
	}
	return &step, nil
}
