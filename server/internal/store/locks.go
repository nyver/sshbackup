package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	"vpsbackupmanager/internal/domain"
)

// JobLock is the row recorded while a job has an active run: which run
// holds it, when it started, and which OS process owns it, so a stale lock
// left by a crashed process can be detected at startup.
type JobLock struct {
	JobID     string
	RunID     string
	StartedAt string // formatted timestamp; kept opaque to callers that only re-display it
	ProcessID int
}

// LockRepository persists job locks.
type LockRepository struct {
	db *DB
}

// NewLockRepository constructs a LockRepository over db.
func NewLockRepository(db *DB) *LockRepository {
	return &LockRepository{db: db}
}

// Acquire records a lock for jobID. It fails with domain.ErrJobAlreadyRunning
// when the job is already locked, per the job-lock specification.
func (r *LockRepository) Acquire(ctx context.Context, jobID, runID string, startedAt string, processID int) error {
	_, err := r.db.ExecContext(ctx,
		"INSERT INTO job_locks (job_id, run_id, started_at, process_id) VALUES (?, ?, ?, ?)",
		jobID, runID, startedAt, processID,
	)
	if err != nil {
		if isUniqueConstraintErr(err) {
			return domain.NewCodedError(domain.ErrJobAlreadyRunning, "Job already running", err)
		}
		return fmt.Errorf("acquire lock for job %q: %w", jobID, err)
	}
	return nil
}

// Release removes the lock for jobID, if any. Releasing an unlocked job is
// not an error, so callers can release unconditionally on every run exit
// path.
func (r *LockRepository) Release(ctx context.Context, jobID string) error {
	if _, err := r.db.ExecContext(ctx, "DELETE FROM job_locks WHERE job_id = ?", jobID); err != nil {
		return fmt.Errorf("release lock for job %q: %w", jobID, err)
	}
	return nil
}

// Get returns the lock for jobID, or (nil, nil) when the job is not
// locked.
func (r *LockRepository) Get(ctx context.Context, jobID string) (*JobLock, error) {
	var l JobLock
	err := r.db.QueryRowContext(ctx,
		"SELECT job_id, run_id, started_at, process_id FROM job_locks WHERE job_id = ?", jobID,
	).Scan(&l.JobID, &l.RunID, &l.StartedAt, &l.ProcessID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil //nolint:nilnil // absence of a lock is a valid, non-error outcome
	}
	if err != nil {
		return nil, fmt.Errorf("get lock for job %q: %w", jobID, err)
	}
	return &l, nil
}

// List returns every currently held lock, used at startup to detect stale
// locks left by a process that no longer exists.
func (r *LockRepository) List(ctx context.Context) ([]*JobLock, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT job_id, run_id, started_at, process_id FROM job_locks")
	if err != nil {
		return nil, fmt.Errorf("list locks: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var locks []*JobLock
	for rows.Next() {
		var l JobLock
		if err := rows.Scan(&l.JobID, &l.RunID, &l.StartedAt, &l.ProcessID); err != nil {
			return nil, fmt.Errorf("scan lock row: %w", err)
		}
		locks = append(locks, &l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate locks: %w", err)
	}
	return locks, nil
}

// isUniqueConstraintErr reports whether err is a SQLite constraint
// violation (the job_locks primary key, in practice), using the driver's
// typed error rather than matching its message text.
func isUniqueConstraintErr(err error) bool {
	var sqliteErr *sqlite.Error
	if !errors.As(err, &sqliteErr) {
		return false
	}
	return sqliteErr.Code()&0xff == sqlite3.SQLITE_CONSTRAINT
}
