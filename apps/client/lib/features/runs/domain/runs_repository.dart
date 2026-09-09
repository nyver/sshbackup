import '../../../core/ipc/models.dart';

class RunStartResult {
  const RunStartResult({required this.runId, required this.status});
  final String runId;
  final String status;
}

class RunDetails {
  const RunDetails({required this.run, required this.steps});
  final RunDto run;
  final List<StepDto> steps;
}

/// Consumer-side port for the `runs.*` IPC commands (run-history and
/// backup-execution specifications).
abstract class RunsRepository {
  Future<RunStartResult> start(String jobId);
  Future<void> cancel(String runId);
  Future<List<RunDto>> list({String? jobId, String? status, int? limit});
  Future<RunDetails> get(String id);
}
