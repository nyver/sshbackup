// Dart mirrors of server/internal/ipc/messages.go's DTOs. Field names and
// JSON keys must stay in sync with the Go source (the source of truth);
// both sides are checked against the shared fixtures in /protocol/fixtures.

class ServerDto {
  ServerDto({
    required this.id,
    required this.name,
    required this.host,
    required this.port,
    required this.username,
    required this.authType,
    required this.hasCredential,
    this.hostKeyAlgorithm = '',
    this.hostKeyFingerprint = '',
    required this.connectionTimeoutSeconds,
    required this.commandTimeoutSeconds,
    required this.createdAt,
    required this.updatedAt,
  });

  factory ServerDto.fromJson(Map<String, dynamic> j) => ServerDto(
    id: j['id'] as String,
    name: j['name'] as String,
    host: j['host'] as String,
    port: j['port'] as int,
    username: j['username'] as String,
    authType: j['auth_type'] as String,
    hasCredential: j['has_credential'] as bool? ?? false,
    hostKeyAlgorithm: j['host_key_algorithm'] as String? ?? '',
    hostKeyFingerprint: j['host_key_fingerprint'] as String? ?? '',
    connectionTimeoutSeconds: j['connection_timeout_seconds'] as int,
    commandTimeoutSeconds: j['command_timeout_seconds'] as int,
    createdAt: j['created_at'] as String,
    updatedAt: j['updated_at'] as String,
  );

  final String id;
  final String name;
  final String host;
  final int port;
  final String username;
  final String authType;
  final bool hasCredential;
  final String hostKeyAlgorithm;
  final String hostKeyFingerprint;
  final int connectionTimeoutSeconds;
  final int commandTimeoutSeconds;
  final String createdAt;
  final String updatedAt;

  bool get hasTrustedHostKey => hostKeyFingerprint.isNotEmpty;
}

class SourceDto {
  SourceDto({
    required this.remotePath,
    required this.position,
    this.include = const [],
    this.exclude = const [],
  });

  factory SourceDto.fromJson(Map<String, dynamic> j) => SourceDto(
    remotePath: j['remote_path'] as String,
    position: j['position'] as int? ?? 0,
    include: (j['include'] as List?)?.cast<String>() ?? const [],
    exclude: (j['exclude'] as List?)?.cast<String>() ?? const [],
  );

  final String remotePath;
  final int position;
  final List<String> include;
  final List<String> exclude;

  Map<String, dynamic> toJson() => {
    'remote_path': remotePath,
    'position': position,
    if (include.isNotEmpty) 'include': include,
    if (exclude.isNotEmpty) 'exclude': exclude,
  };
}

class ScriptDto {
  ScriptDto({
    required this.type,
    required this.command,
    required this.position,
    required this.timeoutSeconds,
    required this.runCondition,
    required this.criticalCleanup,
    required this.retryOnFailure,
  });

  factory ScriptDto.fromJson(Map<String, dynamic> j) => ScriptDto(
    type: j['type'] as String,
    command: j['command'] as String,
    position: j['position'] as int? ?? 0,
    timeoutSeconds: j['timeout_seconds'] as int? ?? 300,
    runCondition: j['run_condition'] as String,
    criticalCleanup: j['critical_cleanup'] as bool? ?? false,
    retryOnFailure: j['retry_on_failure'] as bool? ?? false,
  );

  final String type;
  final String command;
  final int position;
  final int timeoutSeconds;
  final String runCondition;
  final bool criticalCleanup;
  final bool retryOnFailure;

  Map<String, dynamic> toJson() => {
    'type': type,
    'command': command,
    'position': position,
    'timeout_seconds': timeoutSeconds,
    'run_condition': runCondition,
    'critical_cleanup': criticalCleanup,
    'retry_on_failure': retryOnFailure,
  };
}

class ScheduleDto {
  ScheduleDto({
    required this.type,
    this.hour = 0,
    this.minute = 0,
    this.weekdays = const [],
    this.dayOfMonth = 0,
    this.cronExpression = '',
    required this.missedRunPolicy,
  });

  factory ScheduleDto.fromJson(Map<String, dynamic> j) => ScheduleDto(
    type: j['type'] as String,
    hour: j['hour'] as int? ?? 0,
    minute: j['minute'] as int? ?? 0,
    weekdays: (j['weekdays'] as List?)?.cast<int>() ?? const [],
    dayOfMonth: j['day_of_month'] as int? ?? 0,
    cronExpression: j['cron_expression'] as String? ?? '',
    missedRunPolicy: j['missed_run_policy'] as String,
  );

  final String type;
  final int hour;
  final int minute;
  final List<int> weekdays;
  final int dayOfMonth;
  final String cronExpression;
  final String missedRunPolicy;

  Map<String, dynamic> toJson() => {
    'type': type,
    if (hour != 0) 'hour': hour,
    if (minute != 0) 'minute': minute,
    if (weekdays.isNotEmpty) 'weekdays': weekdays,
    if (dayOfMonth != 0) 'day_of_month': dayOfMonth,
    if (cronExpression.isNotEmpty) 'cron_expression': cronExpression,
    'missed_run_policy': missedRunPolicy,
  };
}

class RetentionPolicyDto {
  RetentionPolicyDto({this.keepLast, this.maxAgeDays});

  factory RetentionPolicyDto.fromJson(Map<String, dynamic> j) =>
      RetentionPolicyDto(
        keepLast: j['keep_last'] as int?,
        maxAgeDays: j['max_age_days'] as int?,
      );

  final int? keepLast;
  final int? maxAgeDays;

  Map<String, dynamic> toJson() => {
    if (keepLast != null) 'keep_last': keepLast,
    if (maxAgeDays != null) 'max_age_days': maxAgeDays,
  };
}

class HealthCheckDto {
  HealthCheckDto({
    required this.command,
    required this.attempts,
    required this.intervalSeconds,
  });

  factory HealthCheckDto.fromJson(Map<String, dynamic> j) => HealthCheckDto(
    command: j['command'] as String,
    attempts: j['attempts'] as int,
    intervalSeconds: j['interval_seconds'] as int,
  );

  final String command;
  final int attempts;
  final int intervalSeconds;

  Map<String, dynamic> toJson() => {
    'command': command,
    'attempts': attempts,
    'interval_seconds': intervalSeconds,
  };
}

class JobDto {
  JobDto({
    required this.id,
    required this.name,
    required this.serverId,
    required this.enabled,
    required this.sources,
    this.scripts = const [],
    required this.schedule,
    required this.retentionPolicy,
    this.healthCheck,
    required this.archiveFormat,
    required this.remoteTempDirectory,
    required this.localDestination,
    required this.archiveTimeoutSeconds,
    this.nextRunAt = '',
    required this.createdAt,
    required this.updatedAt,
  });

  factory JobDto.fromJson(Map<String, dynamic> j) => JobDto(
    id: j['id'] as String,
    name: j['name'] as String,
    serverId: j['server_id'] as String,
    enabled: j['enabled'] as bool,
    sources: (j['sources'] as List)
        .map((e) => SourceDto.fromJson(e as Map<String, dynamic>))
        .toList(),
    scripts:
        (j['scripts'] as List?)
            ?.map((e) => ScriptDto.fromJson(e as Map<String, dynamic>))
            .toList() ??
        const [],
    schedule: ScheduleDto.fromJson(j['schedule'] as Map<String, dynamic>),
    retentionPolicy: RetentionPolicyDto.fromJson(
      j['retention_policy'] as Map<String, dynamic>? ?? const {},
    ),
    healthCheck: j['health_check'] == null
        ? null
        : HealthCheckDto.fromJson(j['health_check'] as Map<String, dynamic>),
    archiveFormat: j['archive_format'] as String? ?? 'tar.gz',
    remoteTempDirectory: j['remote_temp_directory'] as String,
    localDestination: j['local_destination'] as String,
    archiveTimeoutSeconds: j['archive_timeout_seconds'] as int,
    nextRunAt: j['next_run_at'] as String? ?? '',
    createdAt: j['created_at'] as String,
    updatedAt: j['updated_at'] as String,
  );

  final String id;
  final String name;
  final String serverId;
  final bool enabled;
  final List<SourceDto> sources;
  final List<ScriptDto> scripts;
  final ScheduleDto schedule;
  final RetentionPolicyDto retentionPolicy;
  final HealthCheckDto? healthCheck;
  final String archiveFormat;
  final String remoteTempDirectory;
  final String localDestination;
  final int archiveTimeoutSeconds;
  final String nextRunAt;
  final String createdAt;
  final String updatedAt;

  Map<String, dynamic> toJson() => {
    'id': id,
    'name': name,
    'server_id': serverId,
    'enabled': enabled,
    'sources': sources.map((e) => e.toJson()).toList(),
    'scripts': scripts.map((e) => e.toJson()).toList(),
    'schedule': schedule.toJson(),
    'retention_policy': retentionPolicy.toJson(),
    if (healthCheck != null) 'health_check': healthCheck!.toJson(),
    'archive_format': archiveFormat,
    'remote_temp_directory': remoteTempDirectory,
    'local_destination': localDestination,
    'archive_timeout_seconds': archiveTimeoutSeconds,
  };
}

class RunDto {
  RunDto({
    required this.id,
    required this.jobId,
    required this.serverId,
    this.sourcePaths = const [],
    required this.trigger,
    required this.startedAt,
    this.finishedAt = '',
    required this.status,
    this.archiveName = '',
    this.archiveSize = 0,
    this.checksum = '',
    this.errorCode = '',
    this.errorMessage = '',
    required this.recoveryOutcome,
  });

  factory RunDto.fromJson(Map<String, dynamic> j) => RunDto(
    id: j['id'] as String,
    jobId: j['job_id'] as String,
    serverId: j['server_id'] as String,
    sourcePaths: (j['source_paths'] as List?)?.cast<String>() ?? const [],
    trigger: j['trigger'] as String,
    startedAt: j['started_at'] as String,
    finishedAt: j['finished_at'] as String? ?? '',
    status: j['status'] as String,
    archiveName: j['archive_name'] as String? ?? '',
    archiveSize: j['archive_size'] as int? ?? 0,
    checksum: j['checksum'] as String? ?? '',
    errorCode: j['error_code'] as String? ?? '',
    errorMessage: j['error_message'] as String? ?? '',
    recoveryOutcome: j['recovery_outcome'] as String? ?? 'NOT_APPLICABLE',
  );

  final String id;
  final String jobId;
  final String serverId;
  final List<String> sourcePaths;
  final String trigger;
  final String startedAt;
  final String finishedAt;
  final String status;
  final String archiveName;
  final int archiveSize;
  final String checksum;
  final String errorCode;
  final String errorMessage;
  final String recoveryOutcome;

  bool get isTerminal => const {
    'SUCCESS',
    'WARNING',
    'FAILED',
    'CANCELLED',
    'SKIPPED',
    'INTERRUPTED',
  }.contains(status);
}

class StepDto {
  StepDto({
    required this.id,
    required this.type,
    required this.startedAt,
    this.finishedAt = '',
    required this.status,
    this.output = '',
    this.truncated = false,
    this.error = '',
    this.exitCode,
    this.durationMs = 0,
    this.command = '',
  });

  factory StepDto.fromJson(Map<String, dynamic> j) => StepDto(
    id: j['id'] as String,
    type: j['type'] as String,
    startedAt: j['started_at'] as String,
    finishedAt: j['finished_at'] as String? ?? '',
    status: j['status'] as String,
    output: j['output'] as String? ?? '',
    truncated: j['truncated'] as bool? ?? false,
    error: j['error'] as String? ?? '',
    exitCode: j['exit_code'] as int?,
    durationMs: j['duration_ms'] as int? ?? 0,
    command: j['command'] as String? ?? '',
  );

  final String id;
  final String type;
  final String startedAt;
  final String finishedAt;
  final String status;
  final String output;
  final bool truncated;
  final String error;
  final int? exitCode;
  final int durationMs;

  /// The exact command text that ran, for PRE_BACKUP_SCRIPT/
  /// POST_BACKUP_SCRIPT steps only — empty for every other step type.
  final String command;
}

class SettingsDto {
  SettingsDto({
    required this.schedulesPaused,
    required this.globalConcurrencyLimit,
    required this.notifySuccess,
    required this.notifyFailure,
    required this.staleRemoteCleanupEnabled,
    required this.staleRemoteCleanupAgeHours,
  });

  factory SettingsDto.fromJson(Map<String, dynamic> j) => SettingsDto(
    schedulesPaused: j['schedules_paused'] as bool,
    globalConcurrencyLimit: j['global_concurrency_limit'] as int,
    notifySuccess: j['notify_success'] as bool,
    notifyFailure: j['notify_failure'] as bool,
    staleRemoteCleanupEnabled: j['stale_remote_cleanup_enabled'] as bool,
    staleRemoteCleanupAgeHours: j['stale_remote_cleanup_age_hours'] as int,
  );

  final bool schedulesPaused;
  final int globalConcurrencyLimit;
  final bool notifySuccess;
  final bool notifyFailure;
  final bool staleRemoteCleanupEnabled;
  final int staleRemoteCleanupAgeHours;

  Map<String, dynamic> toJson() => {
    'schedules_paused': schedulesPaused,
    'global_concurrency_limit': globalConcurrencyLimit,
    'notify_success': notifySuccess,
    'notify_failure': notifyFailure,
    'stale_remote_cleanup_enabled': staleRemoteCleanupEnabled,
    'stale_remote_cleanup_age_hours': staleRemoteCleanupAgeHours,
  };

  SettingsDto copyWith({
    bool? schedulesPaused,
    int? globalConcurrencyLimit,
    bool? notifySuccess,
    bool? notifyFailure,
    bool? staleRemoteCleanupEnabled,
    int? staleRemoteCleanupAgeHours,
  }) => SettingsDto(
    schedulesPaused: schedulesPaused ?? this.schedulesPaused,
    globalConcurrencyLimit:
        globalConcurrencyLimit ?? this.globalConcurrencyLimit,
    notifySuccess: notifySuccess ?? this.notifySuccess,
    notifyFailure: notifyFailure ?? this.notifyFailure,
    staleRemoteCleanupEnabled:
        staleRemoteCleanupEnabled ?? this.staleRemoteCleanupEnabled,
    staleRemoteCleanupAgeHours:
        staleRemoteCleanupAgeHours ?? this.staleRemoteCleanupAgeHours,
  );
}

class CheckDto {
  CheckDto({required this.name, required this.passed, required this.message});

  factory CheckDto.fromJson(Map<String, dynamic> j) => CheckDto(
    name: j['name'] as String,
    passed: j['passed'] as bool,
    message: j['message'] as String,
  );

  final String name;
  final bool passed;
  final String message;
}

class ValidateJobResult {
  ValidateJobResult({
    required this.success,
    required this.checks,
    this.sourceSizeBytes = 0,
    this.sourceSizeKnown = false,
    this.remoteFreeBytes = 0,
    this.remoteFreeKnown = false,
    this.localFreeBytes = 0,
    this.localFreeKnown = false,
  });

  factory ValidateJobResult.fromJson(Map<String, dynamic> j) =>
      ValidateJobResult(
        success: j['success'] as bool,
        checks: (j['checks'] as List)
            .map((e) => CheckDto.fromJson(e as Map<String, dynamic>))
            .toList(),
        sourceSizeBytes: j['source_size_bytes'] as int? ?? 0,
        sourceSizeKnown: j['source_size_known'] as bool? ?? false,
        remoteFreeBytes: j['remote_free_bytes'] as int? ?? 0,
        remoteFreeKnown: j['remote_free_known'] as bool? ?? false,
        localFreeBytes: j['local_free_bytes'] as int? ?? 0,
        localFreeKnown: j['local_free_known'] as bool? ?? false,
      );

  final bool success;
  final List<CheckDto> checks;
  final int sourceSizeBytes;
  final bool sourceSizeKnown;
  final int remoteFreeBytes;
  final bool remoteFreeKnown;
  final int localFreeBytes;
  final bool localFreeKnown;
}

class TestConnectionResult {
  TestConnectionResult({
    required this.success,
    this.failedStage = '',
    this.message = '',
    this.hostKeyAlgorithm = '',
    this.hostKeyFingerprint = '',
    this.fingerprintChanged = false,
    this.remoteOsInfo = '',
    this.sshServerVersion = '',
  });

  factory TestConnectionResult.fromJson(Map<String, dynamic> j) =>
      TestConnectionResult(
        success: j['success'] as bool,
        failedStage: j['failed_stage'] as String? ?? '',
        message: j['message'] as String? ?? '',
        hostKeyAlgorithm: j['host_key_algorithm'] as String? ?? '',
        hostKeyFingerprint: j['host_key_fingerprint'] as String? ?? '',
        fingerprintChanged: j['fingerprint_changed'] as bool? ?? false,
        remoteOsInfo: j['remote_os_info'] as String? ?? '',
        sshServerVersion: j['ssh_server_version'] as String? ?? '',
      );

  final bool success;
  final String failedStage;
  final String message;
  final String hostKeyAlgorithm;
  final String hostKeyFingerprint;
  final bool fingerprintChanged;
  final String remoteOsInfo;
  final String sshServerVersion;
}

class ServiceStatus {
  ServiceStatus({
    required this.version,
    required this.protocolVersion,
    required this.startedAt,
  });

  factory ServiceStatus.fromJson(Map<String, dynamic> j) => ServiceStatus(
    version: j['version'] as String,
    protocolVersion: j['protocol_version'] as int,
    startedAt: j['started_at'] as String,
  );

  final String version;
  final int protocolVersion;
  final String startedAt;
}
