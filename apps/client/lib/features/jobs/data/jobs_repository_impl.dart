import '../../../core/ipc/ipc_client.dart';
import '../../../core/ipc/models.dart';
import '../domain/jobs_repository.dart';

class IpcJobsRepository implements JobsRepository {
  IpcJobsRepository(this._client);

  final IpcClient _client;

  @override
  Future<List<JobDto>> list() async {
    final res = await _client.request('jobs.list');
    return (res['jobs'] as List)
        .map((e) => JobDto.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  @override
  Future<JobDto> create(JobDto job) async {
    final res = await _client.request('jobs.create', {'job': job.toJson()});
    return JobDto.fromJson(res['job'] as Map<String, dynamic>);
  }

  @override
  Future<JobDto> update(JobDto job) async {
    final res = await _client.request('jobs.update', {'job': job.toJson()});
    return JobDto.fromJson(res['job'] as Map<String, dynamic>);
  }

  @override
  Future<void> delete(String id, {bool deleteHistory = false}) async {
    await _client.request('jobs.delete', {
      'id': id,
      'delete_history': deleteHistory,
    });
  }

  @override
  Future<void> setEnabled(String id, {required bool enabled}) async {
    await _client.request(enabled ? 'jobs.enable' : 'jobs.disable', {'id': id});
  }

  @override
  Future<ValidateJobResult> validate(String id) async {
    final res = await _client.request('jobs.validate', {'id': id});
    return ValidateJobResult.fromJson(res);
  }
}
