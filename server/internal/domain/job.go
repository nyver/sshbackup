package domain

import (
	"errors"
	"time"
)

const (
	// DefaultRemoteTempDirectory is the parent directory under which each
	// job gets its own <remote_temp>/<job-id>/ working directory.
	DefaultRemoteTempDirectory = "/tmp/vps-backup-manager"
	// DefaultArchiveTimeout is applied when a job does not set one.
	DefaultArchiveTimeout = 2 * time.Hour
)

// Job is the full backup job aggregate: configuration, sources, scripts,
// schedule, and retention policy, persisted and loaded as one unit.
type Job struct {
	ID       string
	Name     string
	ServerID string
	Enabled  bool

	Sources []Source
	Scripts []Script

	Schedule        Schedule
	RetentionPolicy RetentionPolicy
	HealthCheck     *HealthCheck

	ArchiveFormat       ArchiveFormat
	RemoteTempDirectory string
	LocalDestination    string
	ArchiveTimeout      time.Duration

	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewJobParams carries the user-supplied fields for creating a Job. ID,
// timestamps, and per-child IDs are assigned by NewJob.
type NewJobParams struct {
	Name                string
	ServerID            string
	Enabled             bool
	Sources             []Source
	Scripts             []Script
	Schedule            Schedule
	RetentionPolicy     RetentionPolicy
	HealthCheck         *HealthCheck
	RemoteTempDirectory string
	LocalDestination    string
	ArchiveTimeout      time.Duration
}

// NewJob validates params and constructs a Job aggregate with fresh IDs and
// timestamps for the job and every child entity.
func NewJob(now time.Time, p NewJobParams) (*Job, error) {
	if p.RemoteTempDirectory == "" {
		p.RemoteTempDirectory = DefaultRemoteTempDirectory
	}
	if p.ArchiveTimeout == 0 {
		p.ArchiveTimeout = DefaultArchiveTimeout
	}

	jobID, err := NewID()
	if err != nil {
		return nil, err
	}

	sources := make([]Source, len(p.Sources))
	copy(sources, p.Sources)
	for i := range sources {
		sources[i].JobID = jobID
		if sources[i].ID == "" {
			id, err := NewID()
			if err != nil {
				return nil, err
			}
			sources[i].ID = id
		}
	}

	scripts := make([]Script, len(p.Scripts))
	copy(scripts, p.Scripts)
	for i := range scripts {
		scripts[i].JobID = jobID
		if scripts[i].ID == "" {
			id, err := NewID()
			if err != nil {
				return nil, err
			}
			scripts[i].ID = id
		}
		if scripts[i].TimeoutSeconds == 0 {
			scripts[i].TimeoutSeconds = DefaultScriptTimeoutSeconds
		}
	}

	schedule := p.Schedule
	schedule.JobID = jobID
	if schedule.ID == "" {
		id, err := NewID()
		if err != nil {
			return nil, err
		}
		schedule.ID = id
	}
	if schedule.MissedRunPolicy == "" {
		schedule.MissedRunPolicy = MissedRunAsSoonAsPossible
	}

	retention := p.RetentionPolicy
	retention.JobID = jobID
	if retention.ID == "" {
		id, err := NewID()
		if err != nil {
			return nil, err
		}
		retention.ID = id
	}

	j := &Job{
		ID:                  jobID,
		Name:                p.Name,
		ServerID:            p.ServerID,
		Enabled:             p.Enabled,
		Sources:             sources,
		Scripts:             scripts,
		Schedule:            schedule,
		RetentionPolicy:     retention,
		HealthCheck:         p.HealthCheck,
		ArchiveFormat:       ArchiveTarGz,
		RemoteTempDirectory: p.RemoteTempDirectory,
		LocalDestination:    p.LocalDestination,
		ArchiveTimeout:      p.ArchiveTimeout,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	if err := j.Validate(); err != nil {
		return nil, err
	}
	return j, nil
}

// Validate checks the invariants required by the backup-jobs
// specification: a non-empty name, a server reference, at least one
// source, an absolute remote temp directory, a non-empty local
// destination, and valid child entities.
func (j *Job) Validate() error {
	var errs []error

	if err := ValidateName("job", j.Name); err != nil {
		errs = append(errs, err)
	}
	if j.ServerID == "" {
		errs = append(errs, errors.New("job must reference a server"))
	}
	if len(j.Sources) == 0 {
		errs = append(errs, errors.New("job must have at least one backup source"))
	}
	for i := range j.Sources {
		if err := j.Sources[i].Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	for i := range j.Scripts {
		if err := j.Scripts[i].Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if err := ValidateAbsoluteRemotePath(j.RemoteTempDirectory); err != nil {
		errs = append(errs, err)
	}
	if j.LocalDestination == "" {
		errs = append(errs, errors.New("job local destination must not be empty"))
	}
	if j.ArchiveTimeout <= 0 {
		errs = append(errs, errors.New("job archive timeout must be positive"))
	}
	if err := j.Schedule.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := j.RetentionPolicy.Validate(); err != nil {
		errs = append(errs, err)
	}
	if j.HealthCheck != nil {
		if err := j.HealthCheck.Validate(); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// RemoteJobDirectory returns the per-job working directory under the job's
// remote temporary directory.
func (j *Job) RemoteJobDirectory() string {
	return j.RemoteTempDirectory + "/" + j.ID
}
