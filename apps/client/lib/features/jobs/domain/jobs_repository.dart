import '../../../core/ipc/models.dart';

/// Consumer-side port for the `jobs.*` IPC commands (backup-jobs
/// specification). [create] and [update] both take a full [JobDto]; the
/// service ignores `id`/`created_at`/`updated_at` on create and mints its
/// own, and requires `id` on update — see `server/internal/ipc/handlers_jobs.go`.
abstract class JobsRepository {
  Future<List<JobDto>> list();
  Future<JobDto> create(JobDto job);
  Future<JobDto> update(JobDto job);
  Future<void> delete(String id, {bool deleteHistory = false});
  Future<void> setEnabled(String id, {required bool enabled});
  Future<ValidateJobResult> validate(String id);
}
