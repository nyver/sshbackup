import '../../../core/ipc/ipc_client.dart';
import '../../../core/ipc/models.dart';
import '../domain/runs_repository.dart';

class IpcRunsRepository implements RunsRepository {
  IpcRunsRepository(this._client);

  final IpcClient _client;

  @override
  Future<RunStartResult> start(String jobId) async {
    final res = await _client.request('runs.start', {'job_id': jobId});
    return RunStartResult(
      runId: res['run_id'] as String,
      status: res['status'] as String,
    );
  }

  @override
  Future<void> cancel(String runId) async {
    await _client.request('runs.cancel', {'run_id': runId});
  }

  @override
  Future<List<RunDto>> list({String? jobId, String? status, int? limit}) async {
    final res = await _client.request('runs.list', {
      if (jobId != null && jobId.isNotEmpty) 'job_id': jobId,
      if (status != null && status.isNotEmpty) 'status': status,
      'limit': ?limit,
    });
    return (res['runs'] as List)
        .map((e) => RunDto.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  @override
  Future<RunDetails> get(String id) async {
    final res = await _client.request('runs.get', {'id': id});
    return RunDetails(
      run: RunDto.fromJson(res['run'] as Map<String, dynamic>),
      steps: (res['steps'] as List)
          .map((e) => StepDto.fromJson(e as Map<String, dynamic>))
          .toList(),
    );
  }
}
