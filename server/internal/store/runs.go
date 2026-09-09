package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"vpsbackupmanager/internal/domain"
)

// RunRepository persists domain.Run records.
type RunRepository struct {
	db *DB
}

// NewRunRepository constructs a RunRepository over db.
func NewRunRepository(db *DB) *RunRepository {
	return &RunRepository{db: db}
}

// Create inserts a new run record.
func (r *RunRepository) Create(ctx context.Context, run *domain.Run) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO backup_runs (
			id, job_id, server_id, source_paths, trigger, started_at, finished_at, status,
			archive_name, archive_size, checksum, error_code, error_message, recovery_outcome
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		run.ID, run.JobID, run.ServerID, joinStrings(run.SourcePaths), string(run.Trigger),
		formatTime(run.StartedAt), formatTimePtr(run.FinishedAt), string(run.Status),
		run.ArchiveName, run.ArchiveSize, run.Checksum, string(run.ErrorCode), run.ErrorMessage, string(run.RecoveryOutcome),
	)
	if err != nil {
		return fmt.Errorf("insert run %q: %w", run.ID, err)
	}
	return nil
}

// Update persists the full current state of an existing run record.
func (r *RunRepository) Update(ctx context.Context, run *domain.Run) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE backup_runs SET
			finished_at = ?, status = ?, archive_name = ?, archive_size = ?, checksum = ?,
			error_code = ?, error_message = ?, recovery_outcome = ?
		WHERE id = ?`,
		formatTimePtr(run.FinishedAt), string(run.Status), run.ArchiveName, run.ArchiveSize, run.Checksum,
		string(run.ErrorCode), run.ErrorMessage, string(run.RecoveryOutcome), run.ID,
	)
	if err != nil {
		return fmt.Errorf("update run %q: %w", run.ID, err)
	}
	return checkRowsAffected(res, "run", run.ID)
}

// Get fetches a run by id.
func (r *RunRepository) Get(ctx context.Context, id string) (*domain.Run, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, job_id, server_id, source_paths, trigger, started_at, finished_at, status,
			archive_name, archive_size, checksum, error_code, error_message, recovery_outcome
		FROM backup_runs WHERE id = ?`, id)
	run, err := scanRun(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.NewCodedError(domain.ErrNotFound, fmt.Sprintf("run %q not found", id), nil)
	}
	if err != nil {
		return nil, fmt.Errorf("get run %q: %w", id, err)
	}
	return run, nil
}

// RunFilter narrows List results. Zero values mean "no filter" for that
// field.
type RunFilter struct {
	JobID  string
	Status domain.RunStatus
	Limit  int
}

// List returns runs matching filter, most recently started first.
func (r *RunRepository) List(ctx context.Context, filter RunFilter) ([]*domain.Run, error) {
	query := `
		SELECT id, job_id, server_id, source_paths, trigger, started_at, finished_at, status,
			archive_name, archive_size, checksum, error_code, error_message, recovery_outcome
		FROM backup_runs WHERE 1=1`
	var args []any
	if filter.JobID != "" {
		query += " AND job_id = ?"
		args = append(args, filter.JobID)
	}
	if filter.Status != "" {
		query += " AND status = ?"
		args = append(args, string(filter.Status))
	}
	query += " ORDER BY started_at DESC"
	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list runs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var runs []*domain.Run
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, fmt.Errorf("scan run row: %w", err)
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate runs: %w", err)
	}
	return runs, nil
}

// ListArchived returns every run of jobID that still has a local archive
// file recorded (archive_name set), most recently started first. Used by
// retention to consider only this job's own archives.
func (r *RunRepository) ListArchived(ctx context.Context, jobID string) ([]*domain.Run, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, job_id, server_id, source_paths, trigger, started_at, finished_at, status,
			archive_name, archive_size, checksum, error_code, error_message, recovery_outcome
		FROM backup_runs WHERE job_id = ? AND archive_name != '' ORDER BY started_at DESC`, jobID)
	if err != nil {
		return nil, fmt.Errorf("list archived runs for job %q: %w", jobID, err)
	}
	defer func() { _ = rows.Close() }()

	var runs []*domain.Run
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, fmt.Errorf("scan run row: %w", err)
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate archived runs: %w", err)
	}
	return runs, nil
}

// RunningJobIDs returns the job ids of every run currently in RUNNING
// status, used at startup to detect interrupted runs.
func (r *RunRepository) RunningJobIDs(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT DISTINCT job_id FROM backup_runs WHERE status = ?", string(domain.RunRunning))
	if err != nil {
		return nil, fmt.Errorf("query running job ids: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan job id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate running job ids: %w", err)
	}
	return ids, nil
}

// RunningIDs returns the ids of every run currently in RUNNING status.
func (r *RunRepository) RunningIDs(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id FROM backup_runs WHERE status = ?", string(domain.RunRunning))
	if err != nil {
		return nil, fmt.Errorf("query running runs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan running run id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate running runs: %w", err)
	}
	return ids, nil
}

// InterruptRunningRuns moves every run still in RUNNING status to
// INTERRUPTED with the given finish time, and returns their ids. Used once
// at startup to recover from an unexpected stop.
func (r *RunRepository) InterruptRunningRuns(ctx context.Context, finishedAt time.Time) ([]string, error) {
	ids, err := r.RunningIDs(ctx)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	_, err = r.db.ExecContext(ctx,
		"UPDATE backup_runs SET status = ?, finished_at = ?, error_message = ? WHERE status = ?",
		string(domain.RunInterrupted), formatTime(finishedAt), "service stopped unexpectedly while this run was in progress", string(domain.RunRunning),
	)
	if err != nil {
		return nil, fmt.Errorf("mark running runs interrupted: %w", err)
	}
	return ids, nil
}

func scanRun(row rowScanner) (*domain.Run, error) {
	var (
		run                        domain.Run
		sourcePaths                string
		trigger, status            string
		startedAt                  string
		finishedAt                 sql.NullString
		errorCode, recoveryOutcome string
	)
	if err := row.Scan(
		&run.ID, &run.JobID, &run.ServerID, &sourcePaths, &trigger, &startedAt, &finishedAt, &status,
		&run.ArchiveName, &run.ArchiveSize, &run.Checksum, &errorCode, &run.ErrorMessage, &recoveryOutcome,
	); err != nil {
		return nil, err
	}
	run.SourcePaths = splitStrings(sourcePaths)
	run.Trigger = domain.Trigger(trigger)
	run.Status = domain.RunStatus(status)
	run.ErrorCode = domain.ErrorCode(errorCode)
	run.RecoveryOutcome = domain.RecoveryOutcome(recoveryOutcome)

	var err error
	if run.StartedAt, err = parseTime(startedAt); err != nil {
		return nil, err
	}
	if run.FinishedAt, err = parseTimePtr(finishedAt); err != nil {
		return nil, err
	}
	return &run, nil
}
