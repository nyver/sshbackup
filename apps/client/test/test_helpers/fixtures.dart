/// Minimal valid wire-shape JSON for each DTO, matching
/// `server/internal/ipc/messages.go`. Tests overlay `{...base, 'field': v}`
/// to vary only the fields they care about.
library;

Map<String, dynamic> serverJson({
  String id = 's1',
  String name = 'prod',
  String host = '203.0.113.10',
  int port = 22,
  bool hasCredential = false,
  String hostKeyFingerprint = '',
}) => {
  'id': id,
  'name': name,
  'host': host,
  'port': port,
  'username': 'deploy',
  'auth_type': 'PRIVATE_KEY',
  'has_credential': hasCredential,
  'host_key_algorithm': hostKeyFingerprint.isEmpty ? '' : 'ssh-ed25519',
  'host_key_fingerprint': hostKeyFingerprint,
  'connection_timeout_seconds': 10,
  'command_timeout_seconds': 30,
  'created_at': '2026-01-01T00:00:00Z',
  'updated_at': '2026-01-01T00:00:00Z',
};

Map<String, dynamic> jobJson({
  String id = 'j1',
  String name = 'nightly',
  String serverId = 's1',
  bool enabled = true,
  String nextRunAt = '',
}) => {
  'id': id,
  'name': name,
  'server_id': serverId,
  'enabled': enabled,
  'sources': [
    {'remote_path': '/var/www', 'position': 0},
  ],
  'scripts': <Map<String, dynamic>>[],
  'schedule': {
    'type': 'DAILY',
    'hour': 2,
    'missed_run_policy': 'RUN_AS_SOON_AS_POSSIBLE',
  },
  'retention_policy': <String, dynamic>{},
  'archive_format': 'tar.gz',
  'remote_temp_directory': '/tmp/vps-backup-manager',
  'local_destination': r'D:\Backups',
  'archive_timeout_seconds': 3600,
  'next_run_at': nextRunAt,
  'created_at': '2026-01-01T00:00:00Z',
  'updated_at': '2026-01-01T00:00:00Z',
};

Map<String, dynamic> runJson({
  String id = 'r1',
  String jobId = 'j1',
  String status = 'SUCCESS',
  String startedAt = '2026-01-02T02:00:00Z',
  String finishedAt = '2026-01-02T02:05:00Z',
}) => {
  'id': id,
  'job_id': jobId,
  'server_id': 's1',
  'trigger': 'SCHEDULE',
  'started_at': startedAt,
  'finished_at': finishedAt,
  'status': status,
  'archive_name': 'nightly-20260102-020000.tar.gz',
  'archive_size': 104857600,
  'checksum': 'a' * 64,
  'recovery_outcome': 'NOT_APPLICABLE',
};

Map<String, dynamic> stepJson({
  String id = 'st1',
  String type = 'ARCHIVE',
  String status = 'SUCCESS',
  String command = '',
}) => {
  'id': id,
  'type': type,
  'started_at': '2026-01-02T02:00:00Z',
  'finished_at': '2026-01-02T02:01:00Z',
  'status': status,
  'output': '',
  'truncated': false,
  'duration_ms': 60000,
  'command': command,
};

Map<String, dynamic> settingsJson({bool schedulesPaused = false}) => {
  'schedules_paused': schedulesPaused,
  'global_concurrency_limit': 3,
  'notify_success': true,
  'notify_failure': true,
  'stale_remote_cleanup_enabled': false,
  'stale_remote_cleanup_age_hours': 24,
};
