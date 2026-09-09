package domain

import "time"

// Run is one execution of a backup job.
type Run struct {
	ID    string
	JobID string

	// ServerID and SourcePaths snapshot the configuration actually used by
	// this run so that later edits to the job do not rewrite history.
	ServerID    string
	SourcePaths []string

	Trigger    Trigger
	StartedAt  time.Time
	FinishedAt *time.Time
	Status     RunStatus

	ArchiveName string
	ArchiveSize int64
	Checksum    string

	ErrorCode    ErrorCode
	ErrorMessage string

	RecoveryOutcome RecoveryOutcome
}

// NewRun constructs a run in PENDING status, ready to be persisted before
// execution starts.
func NewRun(now time.Time, jobID, serverID string, sourcePaths []string, trigger Trigger) (*Run, error) {
	id, err := NewID()
	if err != nil {
		return nil, err
	}
	return &Run{
		ID:              id,
		JobID:           jobID,
		ServerID:        serverID,
		SourcePaths:     append([]string(nil), sourcePaths...),
		Trigger:         trigger,
		StartedAt:       now,
		Status:          RunPending,
		RecoveryOutcome: RecoveryNotApplicable,
	}, nil
}

// Finish marks the run terminal with the given status and finish time. It
// does not overwrite an already-terminal status, matching the run-history
// requirement that terminal statuses are stable.
func (r *Run) Finish(now time.Time, status RunStatus) {
	if r.Status.IsTerminal() {
		return
	}
	r.Status = status
	r.FinishedAt = &now
}

// Duration returns the elapsed time of a finished run, or zero if it has
// not finished.
func (r *Run) Duration() time.Duration {
	if r.FinishedAt == nil {
		return 0
	}
	return r.FinishedAt.Sub(r.StartedAt)
}
