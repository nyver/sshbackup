package ipc

import (
	"time"

	"vpsbackupmanager/internal/domain"
)

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func formatTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return formatTime(*t)
}

func serverToDTO(s *domain.Server) ServerDTO {
	return ServerDTO{
		ID: s.ID, Name: s.Name, Host: s.Host, Port: s.Port, Username: s.Username,
		AuthType:                 string(s.AuthType),
		HasCredential:            s.CredentialReference != "",
		HostKeyAlgorithm:         s.HostKeyAlgorithm,
		HostKeyFingerprint:       s.HostKeyFingerprint,
		ConnectionTimeoutSeconds: s.ConnectionTimeoutSeconds,
		CommandTimeoutSeconds:    s.CommandTimeoutSeconds,
		CreatedAt:                formatTime(s.CreatedAt),
		UpdatedAt:                formatTime(s.UpdatedAt),
	}
}

func jobToDTO(j *domain.Job, nextRunAt string) JobDTO {
	sources := make([]SourceDTO, len(j.Sources))
	for i, s := range j.Sources {
		sources[i] = SourceDTO{RemotePath: s.RemotePath, Position: s.Position, Include: s.Include, Exclude: s.Exclude}
	}
	scripts := make([]ScriptDTO, len(j.Scripts))
	for i, sc := range j.Scripts {
		scripts[i] = ScriptDTO{
			Type: string(sc.Type), Command: sc.Command, Position: sc.Position,
			TimeoutSeconds: sc.TimeoutSeconds, RunCondition: string(sc.RunCondition),
			CriticalCleanup: sc.CriticalCleanup, RetryOnFailure: sc.RetryOnFailure,
		}
	}
	weekdays := make([]int, len(j.Schedule.Weekdays))
	for i, w := range j.Schedule.Weekdays {
		weekdays[i] = int(w)
	}
	var healthCheck *HealthCheckDTO
	if j.HealthCheck != nil {
		healthCheck = &HealthCheckDTO{
			Command: j.HealthCheck.Command, Attempts: j.HealthCheck.Attempts,
			IntervalSeconds: j.HealthCheck.IntervalSeconds,
		}
	}

	return JobDTO{
		ID: j.ID, Name: j.Name, ServerID: j.ServerID, Enabled: j.Enabled,
		Sources: sources, Scripts: scripts,
		Schedule: ScheduleDTO{
			Type: string(j.Schedule.Type), Hour: j.Schedule.Hour, Minute: j.Schedule.Minute,
			Weekdays: weekdays, DayOfMonth: j.Schedule.DayOfMonth, CronExpression: j.Schedule.CronExpression,
			MissedRunPolicy: string(j.Schedule.MissedRunPolicy),
		},
		RetentionPolicy: RetentionPolicyDTO{KeepLast: j.RetentionPolicy.KeepLast, MaxAgeDays: j.RetentionPolicy.MaxAgeDays},
		HealthCheck:     healthCheck,
		ArchiveFormat:   string(j.ArchiveFormat), RemoteTempDirectory: j.RemoteTempDirectory,
		LocalDestination: j.LocalDestination, ArchiveTimeoutSeconds: int(j.ArchiveTimeout.Seconds()),
		NextRunAt: nextRunAt,
		CreatedAt: formatTime(j.CreatedAt), UpdatedAt: formatTime(j.UpdatedAt),
	}
}

func dtoToJobParams(dto JobDTO) domain.NewJobParams {
	sources := make([]domain.Source, len(dto.Sources))
	for i, s := range dto.Sources {
		sources[i] = domain.Source{RemotePath: s.RemotePath, Position: s.Position, Include: s.Include, Exclude: s.Exclude}
	}
	scripts := make([]domain.Script, len(dto.Scripts))
	for i, sc := range dto.Scripts {
		scripts[i] = domain.Script{
			Type: domain.ScriptType(sc.Type), Command: sc.Command, Position: sc.Position,
			TimeoutSeconds: sc.TimeoutSeconds, RunCondition: domain.RunCondition(sc.RunCondition),
			CriticalCleanup: sc.CriticalCleanup, RetryOnFailure: sc.RetryOnFailure,
		}
	}
	weekdays := make([]time.Weekday, len(dto.Schedule.Weekdays))
	for i, w := range dto.Schedule.Weekdays {
		weekdays[i] = time.Weekday(w)
	}
	var healthCheck *domain.HealthCheck
	if dto.HealthCheck != nil {
		healthCheck = &domain.HealthCheck{
			Command: dto.HealthCheck.Command, Attempts: dto.HealthCheck.Attempts,
			IntervalSeconds: dto.HealthCheck.IntervalSeconds,
		}
	}

	return domain.NewJobParams{
		Name: dto.Name, ServerID: dto.ServerID, Enabled: dto.Enabled,
		Sources: sources, Scripts: scripts,
		Schedule: domain.Schedule{
			Type: domain.ScheduleType(dto.Schedule.Type), Hour: dto.Schedule.Hour, Minute: dto.Schedule.Minute,
			Weekdays: weekdays, DayOfMonth: dto.Schedule.DayOfMonth, CronExpression: dto.Schedule.CronExpression,
			MissedRunPolicy: domain.MissedRunPolicy(dto.Schedule.MissedRunPolicy),
		},
		RetentionPolicy:     domain.RetentionPolicy{KeepLast: dto.RetentionPolicy.KeepLast, MaxAgeDays: dto.RetentionPolicy.MaxAgeDays},
		HealthCheck:         healthCheck,
		RemoteTempDirectory: dto.RemoteTempDirectory, LocalDestination: dto.LocalDestination,
		ArchiveTimeout: time.Duration(dto.ArchiveTimeoutSeconds) * time.Second,
	}
}

func runToDTO(r *domain.Run) RunDTO {
	return RunDTO{
		ID: r.ID, JobID: r.JobID, ServerID: r.ServerID, SourcePaths: r.SourcePaths, Trigger: string(r.Trigger),
		StartedAt: formatTime(r.StartedAt), FinishedAt: formatTimePtr(r.FinishedAt), Status: string(r.Status),
		ArchiveName: r.ArchiveName, ArchiveSize: r.ArchiveSize, Checksum: r.Checksum,
		ErrorCode: string(r.ErrorCode), ErrorMessage: r.ErrorMessage, RecoveryOutcome: string(r.RecoveryOutcome),
	}
}

func stepToDTO(s *domain.RunStep) StepDTO {
	return StepDTO{
		ID: s.ID, Type: string(s.Type), StartedAt: formatTime(s.StartedAt), FinishedAt: formatTimePtr(s.FinishedAt),
		Status: string(s.Status), Output: s.Output, Truncated: s.Truncated, Error: s.Error,
		ExitCode: s.ExitCode, DurationMs: s.Duration.Milliseconds(),
	}
}

func settingsToDTO(s domain.Settings) SettingsDTO {
	return SettingsDTO{
		SchedulesPaused: s.SchedulesPaused, GlobalConcurrencyLimit: s.GlobalConcurrencyLimit,
		NotifySuccess: s.NotifySuccess, NotifyFailure: s.NotifyFailure,
		StaleRemoteCleanupEnabled: s.StaleRemoteCleanupEnabled, StaleRemoteCleanupAgeHours: s.StaleRemoteCleanupAgeHours,
	}
}

func dtoToSettings(dto SettingsDTO) domain.Settings {
	return domain.Settings{
		SchedulesPaused: dto.SchedulesPaused, GlobalConcurrencyLimit: dto.GlobalConcurrencyLimit,
		NotifySuccess: dto.NotifySuccess, NotifyFailure: dto.NotifyFailure,
		StaleRemoteCleanupEnabled: dto.StaleRemoteCleanupEnabled, StaleRemoteCleanupAgeHours: dto.StaleRemoteCleanupAgeHours,
	}
}
