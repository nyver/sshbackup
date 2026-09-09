import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/ipc/models.dart';
import '../../../core/ipc_providers.dart';
import '../domain/run_events.dart';
import '../domain/runs_repository.dart';
import 'run_events_provider.dart';
import 'runs_repository_impl.dart';

final runsRepositoryProvider = Provider<RunsRepository>((ref) {
  return IpcRunsRepository(ref.watch(ipcClientProvider));
});

const nonTerminalRunStatuses = {'PENDING', 'RUNNING'};

/// The most recent runs (dashboard: "recent runs" and "active run"),
/// refetched whenever a run starts or finishes.
final recentRunsProvider =
    AsyncNotifierProvider<RecentRunsNotifier, List<RunDto>>(
      RecentRunsNotifier.new,
    );

class RecentRunsNotifier extends AsyncNotifier<List<RunDto>> {
  @override
  Future<List<RunDto>> build() {
    ref.listen(runEventsProvider, (_, next) {
      final event = next.value;
      if (event is RunStartedEvent || event is RunFinishedEvent) {
        unawaited(refresh());
      }
    });
    return ref.watch(runsRepositoryProvider).list(limit: 20);
  }

  Future<void> refresh() async {
    state = await AsyncValue.guard(
      () => ref.read(runsRepositoryProvider).list(limit: 20),
    );
  }
}

/// Runs from [recentRunsProvider] that are still `PENDING`/`RUNNING`
/// (dashboard: "active run with its current stage").
final activeRunsProvider = Provider<List<RunDto>>((ref) {
  final recent = ref.watch(recentRunsProvider).value ?? const [];
  return recent
      .where((r) => nonTerminalRunStatuses.contains(r.status))
      .toList();
});

/// Filter applied to the history table (desktop-ui specification: "History
/// table ... filterable by job and status").
class HistoryFilter {
  const HistoryFilter({this.jobId = '', this.status = ''});
  final String jobId;
  final String status;

  HistoryFilter copyWith({String? jobId, String? status}) =>
      HistoryFilter(jobId: jobId ?? this.jobId, status: status ?? this.status);
}

final historyFilterProvider =
    NotifierProvider<HistoryFilterNotifier, HistoryFilter>(
      HistoryFilterNotifier.new,
    );

class HistoryFilterNotifier extends Notifier<HistoryFilter> {
  @override
  HistoryFilter build() => const HistoryFilter();

  void setJobId(String jobId) => state = state.copyWith(jobId: jobId);

  void setStatus(String status) => state = state.copyWith(status: status);
}

/// The history table's run list for the current [historyFilterProvider],
/// refetched whenever a run starts or finishes.
final historyRunsProvider =
    AsyncNotifierProvider<HistoryRunsNotifier, List<RunDto>>(
      HistoryRunsNotifier.new,
    );

class HistoryRunsNotifier extends AsyncNotifier<List<RunDto>> {
  @override
  Future<List<RunDto>> build() {
    final filter = ref.watch(historyFilterProvider);
    ref.listen(runEventsProvider, (_, next) {
      final event = next.value;
      if (event is RunStartedEvent || event is RunFinishedEvent) {
        unawaited(refresh());
      }
    });
    return ref
        .read(runsRepositoryProvider)
        .list(jobId: filter.jobId, status: filter.status, limit: 200);
  }

  Future<void> refresh() async {
    final filter = ref.read(historyFilterProvider);
    state = await AsyncValue.guard(
      () => ref
          .read(runsRepositoryProvider)
          .list(jobId: filter.jobId, status: filter.status, limit: 200),
    );
  }
}

/// Live details for one run: times, status, per-step results. Re-fetches
/// on any event naming this run so an open run-details view stays current
/// without polling (ipc-api specification, "Live progress"). Built as a
/// [StreamProvider.family] (rather than an [AsyncNotifierProvider.family],
/// whose manual — non-code-generated — family API threads the argument
/// through the notifier's constructor instead of `build`) so the argument
/// flows naturally into the fetch-and-refetch loop below.
final runDetailsProvider = StreamProvider.family<RunDetails, String>((
  ref,
  runId,
) {
  final repo = ref.watch(runsRepositoryProvider);
  final client = ref.watch(ipcClientProvider);
  final controller = StreamController<RunDetails>();

  Future<void> fetchAndEmit() async {
    try {
      final details = await repo.get(runId);
      if (!controller.isClosed) controller.add(details);
    } on Object catch (e, st) {
      if (!controller.isClosed) controller.addError(e, st);
    }
  }

  final subscription = client.events.listen((env) {
    final event = parseRunEvent(env);
    final matchesThisRun = switch (event) {
      RunStartedEvent(:final run) => run.id == runId,
      RunStepChangedEvent(runId: final eventRunId) => eventRunId == runId,
      RunFinishedEvent(:final run) => run.id == runId,
      null => false,
    };
    if (matchesThisRun) unawaited(fetchAndEmit());
  });

  ref.onDispose(() {
    unawaited(subscription.cancel());
    unawaited(controller.close());
  });

  unawaited(fetchAndEmit());
  return controller.stream;
});
