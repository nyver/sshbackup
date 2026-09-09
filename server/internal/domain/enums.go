// Package domain holds the entities, status enums, error vocabulary, and
// pure configuration-validation rules shared by the backup engine, the
// repositories, the IPC API, and the scheduler. It performs no I/O.
package domain

// RunStatus is the terminal or in-progress status of a backup run.
type RunStatus string

const (
	// RunPending is a run that has been recorded but has not started.
	RunPending RunStatus = "PENDING"
	// RunRunning is a run currently executing.
	RunRunning RunStatus = "RUNNING"
	// RunSuccess is a run where every stage succeeded.
	RunSuccess RunStatus = "SUCCESS"
	// RunWarning is a run whose archive was stored and verified but a
	// non-essential stage failed.
	RunWarning RunStatus = "WARNING"
	// RunFailed is a run where archiving, download, or checksum
	// verification failed.
	RunFailed RunStatus = "FAILED"
	// RunCancelled is a run stopped by an explicit user cancellation.
	RunCancelled RunStatus = "CANCELLED"
	// RunSkipped is a run that never started, e.g. because its job was
	// already running or a schedule was paused.
	RunSkipped RunStatus = "SKIPPED"
	// RunInterrupted is a run left RUNNING when the service stopped
	// unexpectedly, discovered and reclassified at the next startup.
	RunInterrupted RunStatus = "INTERRUPTED"
)

// IsTerminal reports whether the status is final and must not change again,
// with the sole exception of RUNNING being moved to INTERRUPTED at startup.
func (s RunStatus) IsTerminal() bool {
	switch s {
	case RunSuccess, RunWarning, RunFailed, RunCancelled, RunSkipped, RunInterrupted:
		return true
	default:
		return false
	}
}

// StepStatus is the outcome of one executed run step.
type StepStatus string

const (
	// StepPending is a step that has been created but has not started.
	StepPending StepStatus = "PENDING"
	// StepRunning is a step currently executing.
	StepRunning StepStatus = "RUNNING"
	// StepSuccess is a step that completed successfully.
	StepSuccess StepStatus = "SUCCESS"
	// StepFailed is a step that exited non-zero, timed out, or errored.
	StepFailed StepStatus = "FAILED"
	// StepSkipped is a step never executed because its run condition was
	// not met.
	StepSkipped StepStatus = "SKIPPED"
	// StepCancelled is a step stopped by cancellation.
	StepCancelled StepStatus = "CANCELLED"
)

// StepType identifies which stage of the run workflow a step recorded.
type StepType string

const (
	// StepPreflight covers SSH connect, host key verification, and the
	// remaining pre-flight checks (tooling, paths, free space).
	StepPreflight StepType = "PREFLIGHT"
	// StepPreBackupScript is one executed PRE_BACKUP script.
	StepPreBackupScript StepType = "PRE_BACKUP_SCRIPT"
	// StepArchive is remote archive creation.
	StepArchive StepType = "ARCHIVE"
	// StepRemoteChecksum is computing the SHA-256 of the remote archive.
	StepRemoteChecksum StepType = "REMOTE_CHECKSUM"
	// StepDownload is the SFTP download to a .part file.
	StepDownload StepType = "DOWNLOAD"
	// StepLocalChecksum is computing the SHA-256 of the downloaded file.
	StepLocalChecksum StepType = "LOCAL_CHECKSUM"
	// StepVerifyChecksum is comparing the remote and local checksums and,
	// on a match, the atomic rename to the final archive name.
	StepVerifyChecksum StepType = "VERIFY_CHECKSUM"
	// StepPostBackupScript is one executed POST_BACKUP script.
	StepPostBackupScript StepType = "POST_BACKUP_SCRIPT"
	// StepHealthCheck is the optional post-backup health check.
	StepHealthCheck StepType = "HEALTH_CHECK"
	// StepRemoteCleanup is deletion of the remote temporary archive.
	StepRemoteCleanup StepType = "REMOTE_CLEANUP"
	// StepRetention is application of the job's local retention policy.
	StepRetention StepType = "RETENTION"
)

// ScriptType distinguishes pre- and post-backup scripts.
type ScriptType string

const (
	// ScriptPreBackup runs before the archive is created.
	ScriptPreBackup ScriptType = "PRE_BACKUP"
	// ScriptPostBackup runs after the archive stage, per its run condition.
	ScriptPostBackup ScriptType = "POST_BACKUP"
)

// RunCondition controls whether a script executes given the run's outcome
// so far.
type RunCondition string

const (
	// RunConditionOnSuccess runs the script only when every preceding
	// stage succeeded.
	RunConditionOnSuccess RunCondition = "ON_SUCCESS"
	// RunConditionOnFailure runs the script only when a preceding stage
	// failed.
	RunConditionOnFailure RunCondition = "ON_FAILURE"
	// RunConditionAlways runs the script regardless of preceding outcome.
	RunConditionAlways RunCondition = "ALWAYS"
)

// Trigger identifies what started a run.
type Trigger string

const (
	// TriggerSchedule is a run started by the scheduler at its due time.
	TriggerSchedule Trigger = "SCHEDULE"
	// TriggerManual is a run started by the user via Run now.
	TriggerManual Trigger = "MANUAL"
	// TriggerMissedSchedule is a catch-up run for an occurrence missed
	// while the service was not running.
	TriggerMissedSchedule Trigger = "MISSED_SCHEDULE"
)

// ScheduleType is the kind of schedule a job uses.
type ScheduleType string

const (
	// ScheduleManual is never dispatched automatically; only Run now
	// starts the job.
	ScheduleManual ScheduleType = "MANUAL"
	// ScheduleDaily runs once per day at a configured time.
	ScheduleDaily ScheduleType = "DAILY"
	// ScheduleWeekly runs on configured weekdays at a configured time.
	ScheduleWeekly ScheduleType = "WEEKLY"
	// ScheduleMonthly runs on a configured day of month at a configured
	// time.
	ScheduleMonthly ScheduleType = "MONTHLY"
	// ScheduleCron runs on a standard five-field cron expression.
	ScheduleCron ScheduleType = "CRON"
)

// MissedRunPolicy controls what happens to a schedule occurrence that
// passed while the service was not running.
type MissedRunPolicy string

const (
	// MissedRunAsSoonAsPossible starts one catch-up run shortly after the
	// service becomes ready, regardless of how many occurrences were
	// missed.
	MissedRunAsSoonAsPossible MissedRunPolicy = "RUN_AS_SOON_AS_POSSIBLE"
	// MissedRunSkip records a SKIPPED run instead of catching up.
	MissedRunSkip MissedRunPolicy = "SKIP"
)

// AuthenticationType is how the service authenticates to a server over SSH.
type AuthenticationType string

const (
	// AuthPrivateKey authenticates with an SSH private key, with or
	// without a passphrase.
	AuthPrivateKey AuthenticationType = "PRIVATE_KEY"
	// AuthPassword authenticates with a password.
	AuthPassword AuthenticationType = "PASSWORD"
)

// RecoveryOutcome reports whether ALWAYS/critical_cleanup scripts, run after
// a failure, actually restored the remote side.
type RecoveryOutcome string

const (
	// RecoveryNotApplicable means no recovery action was required.
	RecoveryNotApplicable RecoveryOutcome = "NOT_APPLICABLE"
	// RecoverySuccess means every executed recovery action succeeded.
	RecoverySuccess RecoveryOutcome = "SUCCESS"
	// RecoveryFailed means at least one recovery action failed; the
	// remote side may still be in a bad state.
	RecoveryFailed RecoveryOutcome = "FAILED"
)

// ArchiveFormat is the compression format used for an archive. Only tar.gz
// is supported in this version; the type exists so the schema and API do
// not need a breaking change to add another format later.
type ArchiveFormat string

const (
	// ArchiveTarGz is the only supported archive format in this version.
	ArchiveTarGz ArchiveFormat = "tar.gz"
)
