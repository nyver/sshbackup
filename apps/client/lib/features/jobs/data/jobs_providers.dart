import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/ipc/models.dart';
import '../../../core/ipc_providers.dart';
import '../../runs/data/run_events_provider.dart';
import '../../runs/domain/run_events.dart';
import '../domain/jobs_repository.dart';
import 'jobs_repository_impl.dart';

final jobsRepositoryProvider = Provider<JobsRepository>((ref) {
  return IpcJobsRepository(ref.watch(ipcClientProvider));
});

/// The current job list, refetched after any mutation or when a run
/// finishes (next-run times and enabled state can only change server-side).
final jobsListProvider = AsyncNotifierProvider<JobsListNotifier, List<JobDto>>(
  JobsListNotifier.new,
);

class JobsListNotifier extends AsyncNotifier<List<JobDto>> {
  @override
  Future<List<JobDto>> build() {
    ref.listen(runEventsProvider, (_, next) {
      final event = next.value;
      if (event is RunFinishedEvent) unawaited(refresh());
    });
    return ref.watch(jobsRepositoryProvider).list();
  }

  Future<void> refresh() async {
    state = await AsyncValue.guard(
      () => ref.read(jobsRepositoryProvider).list(),
    );
  }

  Future<JobDto> create(JobDto job) async {
    final result = await ref.read(jobsRepositoryProvider).create(job);
    await refresh();
    return result;
  }

  Future<JobDto> updateJob(JobDto job) async {
    final result = await ref.read(jobsRepositoryProvider).update(job);
    await refresh();
    return result;
  }

  Future<void> delete(String id, {bool deleteHistory = false}) async {
    await ref
        .read(jobsRepositoryProvider)
        .delete(id, deleteHistory: deleteHistory);
    await refresh();
  }

  Future<void> setEnabled(String id, {required bool enabled}) async {
    await ref.read(jobsRepositoryProvider).setEnabled(id, enabled: enabled);
    await refresh();
  }
}
