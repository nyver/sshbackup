package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"vpsbackupmanager/internal/domain"
)

// JobRepository persists the domain.Job aggregate: the job row plus its
// sources, scripts, schedule, and retention policy, as one unit.
type JobRepository struct {
	db *DB
}

// NewJobRepository constructs a JobRepository over db.
func NewJobRepository(db *DB) *JobRepository {
	return &JobRepository{db: db}
}

// Create persists a new job aggregate in one transaction.
func (r *JobRepository) Create(ctx context.Context, j *domain.Job) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := insertJobRow(ctx, tx, j); err != nil {
		return err
	}
	if err := replaceJobChildren(ctx, tx, j); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit create job %q: %w", j.ID, err)
	}
	return nil
}

// Update replaces an existing job aggregate in place: the job row is
// updated, and every child (sources, scripts, schedule, retention) is
// deleted and re-inserted so the aggregate stays internally consistent.
func (r *JobRepository) Update(ctx context.Context, j *domain.Job) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx, `
		UPDATE backup_jobs SET
			name = ?, server_id = ?, enabled = ?, archive_format = ?,
			remote_temp_directory = ?, local_destination = ?, archive_timeout_seconds = ?,
			health_check_command = ?, health_check_attempts = ?, health_check_interval_seconds = ?,
			updated_at = ?
		WHERE id = ?`,
		j.Name, j.ServerID, boolToInt(j.Enabled), string(j.ArchiveFormat),
		j.RemoteTempDirectory, j.LocalDestination, int(j.ArchiveTimeout.Seconds()),
		healthCheckCommand(j.HealthCheck), healthCheckAttempts(j.HealthCheck), healthCheckIntervalSeconds(j.HealthCheck),
		formatTime(j.UpdatedAt), j.ID,
	)
	if err != nil {
		return fmt.Errorf("update job %q: %w", j.ID, err)
	}
	if err := checkRowsAffected(res, "job", j.ID); err != nil {
		return err
	}

	for _, stmt := range []string{
		"DELETE FROM backup_sources WHERE job_id = ?",
		"DELETE FROM job_scripts WHERE job_id = ?",
	} {
		if _, err := tx.ExecContext(ctx, stmt, j.ID); err != nil {
			return fmt.Errorf("clear job children for %q: %w", j.ID, err)
		}
	}
	if err := replaceJobChildren(ctx, tx, j); err != nil {
		return err
	}
	if err := upsertSchedule(ctx, tx, j.ID, &j.Schedule); err != nil {
		return err
	}
	if err := upsertRetention(ctx, tx, j.ID, &j.RetentionPolicy); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit update job %q: %w", j.ID, err)
	}
	return nil
}

// SetEnabled toggles a job's enabled flag without touching the rest of the
// aggregate.
func (r *JobRepository) SetEnabled(ctx context.Context, id string, enabled bool) error {
	res, err := r.db.ExecContext(ctx, "UPDATE backup_jobs SET enabled = ? WHERE id = ?", boolToInt(enabled), id)
	if err != nil {
		return fmt.Errorf("set job %q enabled=%v: %w", id, enabled, err)
	}
	return checkRowsAffected(res, "job", id)
}

// Get loads a job aggregate by id.
func (r *JobRepository) Get(ctx context.Context, id string) (*domain.Job, error) {
	jobs, err := r.loadJobs(ctx, "WHERE id = ?", id)
	if err != nil {
		return nil, err
	}
	if len(jobs) == 0 {
		return nil, domain.NewCodedError(domain.ErrNotFound, fmt.Sprintf("job %q not found", id), nil)
	}
	return jobs[0], nil
}

// List loads every job aggregate, ordered by name, using a fixed number of
// batched queries regardless of job count.
func (r *JobRepository) List(ctx context.Context) ([]*domain.Job, error) {
	return r.loadJobs(ctx, "ORDER BY name")
}

// Delete removes a job and its configuration. When deleteHistory is true,
// its run history and steps are removed too; otherwise history rows are
// left in place with an orphaned job_id, per the safe-deletion
// specification. Deletion is refused while the job has an active run.
func (r *JobRepository) Delete(ctx context.Context, id string, deleteHistory bool) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var lockedJobID string
	err = tx.QueryRowContext(ctx, "SELECT job_id FROM job_locks WHERE job_id = ?", id).Scan(&lockedJobID)
	if err == nil {
		return domain.NewCodedError(domain.ErrJobAlreadyRunning,
			"cannot delete a job with an active run; wait for it to finish or cancel it", nil)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("check active lock for job %q: %w", id, err)
	}

	if deleteHistory {
		if _, err := tx.ExecContext(ctx, "DELETE FROM backup_runs WHERE job_id = ?", id); err != nil {
			return fmt.Errorf("delete run history for job %q: %w", id, err)
		}
	}

	res, err := tx.ExecContext(ctx, "DELETE FROM backup_jobs WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete job %q: %w", id, err)
	}
	if err := checkRowsAffected(res, "job", id); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete job %q: %w", id, err)
	}
	return nil
}

func insertJobRow(ctx context.Context, tx *sql.Tx, j *domain.Job) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO backup_jobs (
			id, name, server_id, enabled, archive_format, remote_temp_directory, local_destination,
			archive_timeout_seconds, health_check_command, health_check_attempts, health_check_interval_seconds,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		j.ID, j.Name, j.ServerID, boolToInt(j.Enabled), string(j.ArchiveFormat), j.RemoteTempDirectory, j.LocalDestination,
		int(j.ArchiveTimeout.Seconds()), healthCheckCommand(j.HealthCheck), healthCheckAttempts(j.HealthCheck), healthCheckIntervalSeconds(j.HealthCheck),
		formatTime(j.CreatedAt), formatTime(j.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("insert job %q: %w", j.ID, err)
	}
	return nil
}

// replaceJobChildren inserts sources and scripts, and upserts the schedule
// and retention policy, for a job whose own row already exists in tx.
func replaceJobChildren(ctx context.Context, tx *sql.Tx, j *domain.Job) error {
	for _, s := range j.Sources {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO backup_sources (id, job_id, remote_path, position, include_patterns, exclude_patterns)
			VALUES (?, ?, ?, ?, ?, ?)`,
			s.ID, j.ID, s.RemotePath, s.Position, joinStrings(s.Include), joinStrings(s.Exclude),
		); err != nil {
			return fmt.Errorf("insert source %q for job %q: %w", s.ID, j.ID, err)
		}
	}
	for _, sc := range j.Scripts {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO job_scripts (id, job_id, script_type, command, position, timeout_seconds, run_condition, critical_cleanup, retry_on_failure)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			sc.ID, j.ID, string(sc.Type), sc.Command, sc.Position, sc.TimeoutSeconds, string(sc.RunCondition),
			boolToInt(sc.CriticalCleanup), boolToInt(sc.RetryOnFailure),
		); err != nil {
			return fmt.Errorf("insert script %q for job %q: %w", sc.ID, j.ID, err)
		}
	}
	if err := upsertSchedule(ctx, tx, j.ID, &j.Schedule); err != nil {
		return err
	}
	if err := upsertRetention(ctx, tx, j.ID, &j.RetentionPolicy); err != nil {
		return err
	}
	return nil
}

func upsertSchedule(ctx context.Context, tx *sql.Tx, jobID string, s *domain.Schedule) error {
	weekdays := make([]int, len(s.Weekdays))
	for i, w := range s.Weekdays {
		weekdays[i] = int(w)
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO schedules (id, job_id, schedule_type, hour, minute, weekdays, day_of_month, cron_expression, missed_run_policy)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(job_id) DO UPDATE SET
			schedule_type = excluded.schedule_type, hour = excluded.hour, minute = excluded.minute,
			weekdays = excluded.weekdays, day_of_month = excluded.day_of_month,
			cron_expression = excluded.cron_expression, missed_run_policy = excluded.missed_run_policy`,
		s.ID, jobID, string(s.Type), s.Hour, s.Minute, joinInts(weekdays), s.DayOfMonth, s.CronExpression, string(s.MissedRunPolicy),
	)
	if err != nil {
		return fmt.Errorf("upsert schedule for job %q: %w", jobID, err)
	}
	return nil
}

func upsertRetention(ctx context.Context, tx *sql.Tx, jobID string, p *domain.RetentionPolicy) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO retention_policies (id, job_id, keep_last, max_age_days)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(job_id) DO UPDATE SET keep_last = excluded.keep_last, max_age_days = excluded.max_age_days`,
		p.ID, jobID, nullInt(p.KeepLast), nullInt(p.MaxAgeDays),
	)
	if err != nil {
		return fmt.Errorf("upsert retention policy for job %q: %w", jobID, err)
	}
	return nil
}

func healthCheckCommand(h *domain.HealthCheck) string {
	if h == nil {
		return ""
	}
	return h.Command
}

func healthCheckAttempts(h *domain.HealthCheck) int {
	if h == nil {
		return 0
	}
	return h.Attempts
}

func healthCheckIntervalSeconds(h *domain.HealthCheck) int {
	if h == nil {
		return 0
	}
	return h.IntervalSeconds
}

// loadJobs runs the job query with the given WHERE/ORDER BY clause and
// args, then batch-loads sources, scripts, schedules, and retention
// policies for the resulting jobs in four additional queries total.
func (r *JobRepository) loadJobs(ctx context.Context, clause string, args ...any) ([]*domain.Job, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, server_id, enabled, archive_format, remote_temp_directory, local_destination,
			archive_timeout_seconds, health_check_command, health_check_attempts, health_check_interval_seconds,
			created_at, updated_at
		FROM backup_jobs `+clause, args...)
	if err != nil {
		return nil, fmt.Errorf("query jobs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	byID := make(map[string]*domain.Job)
	var order []string
	for rows.Next() {
		j, err := scanJobRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan job row: %w", err)
		}
		byID[j.ID] = j
		order = append(order, j.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate jobs: %w", err)
	}
	if len(order) == 0 {
		return nil, nil
	}

	if err := attachSources(ctx, r.db, byID); err != nil {
		return nil, err
	}
	if err := attachScripts(ctx, r.db, byID); err != nil {
		return nil, err
	}
	if err := attachSchedules(ctx, r.db, byID); err != nil {
		return nil, err
	}
	if err := attachRetention(ctx, r.db, byID); err != nil {
		return nil, err
	}

	jobs := make([]*domain.Job, len(order))
	for i, id := range order {
		jobs[i] = byID[id]
	}
	return jobs, nil
}

func scanJobRow(row rowScanner) (*domain.Job, error) {
	var (
		j                          domain.Job
		enabled                    int64
		archiveFormat              string
		archiveTimeoutSeconds      int
		healthCheckCmd             string
		healthCheckAttempts        int
		healthCheckIntervalSeconds int
		createdAt, updatedAt       string
	)
	if err := row.Scan(
		&j.ID, &j.Name, &j.ServerID, &enabled, &archiveFormat, &j.RemoteTempDirectory, &j.LocalDestination,
		&archiveTimeoutSeconds, &healthCheckCmd, &healthCheckAttempts, &healthCheckIntervalSeconds,
		&createdAt, &updatedAt,
	); err != nil {
		return nil, err
	}
	j.Enabled = intToBool(enabled)
	j.ArchiveFormat = domain.ArchiveFormat(archiveFormat)
	j.ArchiveTimeout = secondsToDuration(archiveTimeoutSeconds)
	if healthCheckCmd != "" {
		j.HealthCheck = &domain.HealthCheck{
			Command: healthCheckCmd, Attempts: healthCheckAttempts, IntervalSeconds: healthCheckIntervalSeconds,
		}
	}

	var err error
	if j.CreatedAt, err = parseTime(createdAt); err != nil {
		return nil, err
	}
	if j.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return nil, err
	}
	return &j, nil
}
