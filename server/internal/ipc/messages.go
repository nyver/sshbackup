package ipc

// ServerDTO is a server record as exposed over IPC. It never carries
// secret material: HasCredential tells the UI only whether a passphrase
// is configured, per the credential-storage specification.
type ServerDTO struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	AuthType string `json:"auth_type"`

	HasCredential      bool   `json:"has_credential"`
	HostKeyAlgorithm   string `json:"host_key_algorithm,omitempty"`
	HostKeyFingerprint string `json:"host_key_fingerprint,omitempty"`

	ConnectionTimeoutSeconds int `json:"connection_timeout_seconds"`
	CommandTimeoutSeconds    int `json:"command_timeout_seconds"`

	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// ListServersResponse answers servers.list.
type ListServersResponse struct {
	Servers []ServerDTO `json:"servers"`
}

// SaveServerRequest is the payload for both servers.create and
// servers.update. Passphrase carries the one secret a server's
// credential_reference stores: the private key's passphrase when
// AuthType is PRIVATE_KEY, or the login password when AuthType is
// PASSWORD. If set, it replaces any existing credential; it is
// write-only and never echoed back. Leaving it empty on an update keeps
// the credential already stored.
type SaveServerRequest struct {
	ID                       string `json:"id,omitempty"` // empty on create
	Name                     string `json:"name"`
	Host                     string `json:"host"`
	Port                     int    `json:"port"`
	Username                 string `json:"username"`
	AuthType                 string `json:"auth_type"`
	PrivateKeyPath           string `json:"private_key_path,omitempty"`
	Passphrase               string `json:"passphrase,omitempty"`
	ConnectionTimeoutSeconds int    `json:"connection_timeout_seconds,omitempty"`
	CommandTimeoutSeconds    int    `json:"command_timeout_seconds,omitempty"`
}

// SaveServerResponse answers servers.create / servers.update.
type SaveServerResponse struct {
	Server ServerDTO `json:"server"`
}

// DeleteServerRequest is the payload for servers.delete.
type DeleteServerRequest struct {
	ID string `json:"id"`
}

// TestConnectionRequest is the payload for servers.testConnection: the
// server must already be saved.
type TestConnectionRequest struct {
	ServerID string `json:"server_id"`
}

// TestConnectionResponse reports which stage failed, or success details.
type TestConnectionResponse struct {
	Success            bool   `json:"success"`
	FailedStage        string `json:"failed_stage,omitempty"`
	Message            string `json:"message,omitempty"`
	HostKeyAlgorithm   string `json:"host_key_algorithm,omitempty"`
	HostKeyFingerprint string `json:"host_key_fingerprint,omitempty"`
	FingerprintChanged bool   `json:"fingerprint_changed,omitempty"`
	RemoteOSInfo       string `json:"remote_os_info,omitempty"`
	SSHServerVersion   string `json:"ssh_server_version,omitempty"`
}

// ConfirmHostKeyRequest is the payload for servers.confirmHostKey: the
// explicit second call of the trust-on-first-use flow, carrying the
// fingerprint the user confirmed.
type ConfirmHostKeyRequest struct {
	ServerID    string `json:"server_id"`
	Fingerprint string `json:"fingerprint"`
	Algorithm   string `json:"algorithm"`
}

// SourceDTO is one backup source.
type SourceDTO struct {
	RemotePath string   `json:"remote_path"`
	Position   int      `json:"position"`
	Include    []string `json:"include,omitempty"`
	Exclude    []string `json:"exclude,omitempty"`
}

// ScriptDTO is one job script.
type ScriptDTO struct {
	Type            string `json:"type"`
	Command         string `json:"command"`
	Position        int    `json:"position"`
	TimeoutSeconds  int    `json:"timeout_seconds"`
	RunCondition    string `json:"run_condition"`
	CriticalCleanup bool   `json:"critical_cleanup"`
	RetryOnFailure  bool   `json:"retry_on_failure"`
}

// ScheduleDTO is a job's schedule.
type ScheduleDTO struct {
	Type            string `json:"type"`
	Hour            int    `json:"hour,omitempty"`
	Minute          int    `json:"minute,omitempty"`
	Weekdays        []int  `json:"weekdays,omitempty"`
	DayOfMonth      int    `json:"day_of_month,omitempty"`
	CronExpression  string `json:"cron_expression,omitempty"`
	MissedRunPolicy string `json:"missed_run_policy"`
}

// RetentionPolicyDTO is a job's local retention policy.
type RetentionPolicyDTO struct {
	KeepLast   *int `json:"keep_last,omitempty"`
	MaxAgeDays *int `json:"max_age_days,omitempty"`
}

// HealthCheckDTO is a job's optional post-backup health check.
type HealthCheckDTO struct {
	Command         string `json:"command"`
	Attempts        int    `json:"attempts"`
	IntervalSeconds int    `json:"interval_seconds"`
}

// JobDTO is a backup job as exposed over IPC.
type JobDTO struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	ServerID string `json:"server_id"`
	Enabled  bool   `json:"enabled"`

	Sources []SourceDTO `json:"sources"`
	Scripts []ScriptDTO `json:"scripts,omitempty"`

	Schedule        ScheduleDTO        `json:"schedule"`
	RetentionPolicy RetentionPolicyDTO `json:"retention_policy"`
	HealthCheck     *HealthCheckDTO    `json:"health_check,omitempty"`

	ArchiveFormat         string `json:"archive_format"`
	RemoteTempDirectory   string `json:"remote_temp_directory"`
	LocalDestination      string `json:"local_destination"`
	ArchiveTimeoutSeconds int    `json:"archive_timeout_seconds"`

	NextRunAt string `json:"next_run_at,omitempty"`

	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// ListJobsResponse answers jobs.list.
type ListJobsResponse struct {
	Jobs []JobDTO `json:"jobs"`
}

// SaveJobRequest is the payload for both jobs.create and jobs.update.
type SaveJobRequest struct {
	Job JobDTO `json:"job"`
}

// SaveJobResponse answers jobs.create / jobs.update.
type SaveJobResponse struct {
	Job JobDTO `json:"job"`
}

// DeleteJobRequest is the payload for jobs.delete.
type DeleteJobRequest struct {
	ID            string `json:"id"`
	DeleteHistory bool   `json:"delete_history"`
}

// SetJobEnabledRequest is the payload for jobs.enable / jobs.disable.
type SetJobEnabledRequest struct {
	ID string `json:"id"`
}

// ValidateJobRequest is the payload for jobs.validate.
type ValidateJobRequest struct {
	ID string `json:"id"`
}

// CheckDTO is one named Validate Job check outcome.
type CheckDTO struct {
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Message string `json:"message"`
}

// ValidateJobResponse answers jobs.validate.
type ValidateJobResponse struct {
	Success bool       `json:"success"`
	Checks  []CheckDTO `json:"checks"`

	SourceSizeBytes int64 `json:"source_size_bytes,omitempty"`
	SourceSizeKnown bool  `json:"source_size_known"`
	RemoteFreeBytes int64 `json:"remote_free_bytes,omitempty"`
	RemoteFreeKnown bool  `json:"remote_free_known"`
	LocalFreeBytes  int64 `json:"local_free_bytes,omitempty"`
	LocalFreeKnown  bool  `json:"local_free_known"`
}

// RunNowRequest is the payload for runs.start.
type RunNowRequest struct {
	JobID string `json:"job_id"`
}

// RunNowResponse answers runs.start.
type RunNowResponse struct {
	RunID  string `json:"run_id"`
	Status string `json:"status"`
}

// CancelRunRequest is the payload for runs.cancel.
type CancelRunRequest struct {
	RunID string `json:"run_id"`
}

// ListRunsRequest is the payload for runs.list; zero values mean
// "no filter" for that field.
type ListRunsRequest struct {
	JobID  string `json:"job_id,omitempty"`
	Status string `json:"status,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

// RunDTO is a run record as exposed over IPC.
type RunDTO struct {
	ID          string   `json:"id"`
	JobID       string   `json:"job_id"`
	ServerID    string   `json:"server_id"`
	SourcePaths []string `json:"source_paths,omitempty"`
	Trigger     string   `json:"trigger"`

	StartedAt  string `json:"started_at"`
	FinishedAt string `json:"finished_at,omitempty"`
	Status     string `json:"status"`

	ArchiveName string `json:"archive_name,omitempty"`
	ArchiveSize int64  `json:"archive_size,omitempty"`
	Checksum    string `json:"checksum,omitempty"`

	ErrorCode    string `json:"error_code,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`

	RecoveryOutcome string `json:"recovery_outcome"`
}

// ListRunsResponse answers runs.list.
type ListRunsResponse struct {
	Runs []RunDTO `json:"runs"`
}

// GetRunRequest is the payload for runs.get.
type GetRunRequest struct {
	ID string `json:"id"`
}

// StepDTO is one run step as exposed over IPC.
type StepDTO struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	StartedAt  string `json:"started_at"`
	FinishedAt string `json:"finished_at,omitempty"`
	Status     string `json:"status"`
	Output     string `json:"output,omitempty"`
	Truncated  bool   `json:"truncated"`
	Error      string `json:"error,omitempty"`
	ExitCode   *int   `json:"exit_code,omitempty"`
	DurationMs int64  `json:"duration_ms,omitempty"`
}

// GetRunResponse answers runs.get: run details with steps and logs.
type GetRunResponse struct {
	Run   RunDTO    `json:"run"`
	Steps []StepDTO `json:"steps"`
}

// SettingsDTO is the global settings row as exposed over IPC.
type SettingsDTO struct {
	SchedulesPaused            bool `json:"schedules_paused"`
	GlobalConcurrencyLimit     int  `json:"global_concurrency_limit"`
	NotifySuccess              bool `json:"notify_success"`
	NotifyFailure              bool `json:"notify_failure"`
	StaleRemoteCleanupEnabled  bool `json:"stale_remote_cleanup_enabled"`
	StaleRemoteCleanupAgeHours int  `json:"stale_remote_cleanup_age_hours"`
}

// GetSettingsResponse answers settings.get.
type GetSettingsResponse struct {
	Settings SettingsDTO `json:"settings"`
}

// SetSettingsRequest is the payload for settings.set.
type SetSettingsRequest struct {
	Settings SettingsDTO `json:"settings"`
}

// ServiceStatusResponse answers service.status.
type ServiceStatusResponse struct {
	Version         string `json:"version"`
	ProtocolVersion int    `json:"protocol_version"`
	StartedAt       string `json:"started_at"`
}

// RunStartedEvent is pushed when a run begins.
type RunStartedEvent struct {
	Run RunDTO `json:"run"`
}

// RunStepChangedEvent is pushed whenever a run step's status changes.
type RunStepChangedEvent struct {
	RunID string  `json:"run_id"`
	Step  StepDTO `json:"step"`
}

// RunLogLineEvent is pushed for each new line appended to an active run's
// log.
type RunLogLineEvent struct {
	RunID     string `json:"run_id"`
	StepID    string `json:"step_id,omitempty"`
	Line      string `json:"line"`
	Timestamp string `json:"timestamp"`
}

// RunFinishedEvent is pushed when a run reaches a terminal status.
type RunFinishedEvent struct {
	Run RunDTO `json:"run"`
}

// SettingsChangedEvent is pushed whenever global settings change.
type SettingsChangedEvent struct {
	Settings SettingsDTO `json:"settings"`
}
