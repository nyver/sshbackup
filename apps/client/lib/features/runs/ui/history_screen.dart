import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/ipc/models.dart';
import '../../../l10n/gen/app_localizations.dart';
import '../../../shared/format.dart';
import '../../../shared/widgets/empty_state.dart';
import '../../../shared/widgets/error_view.dart';
import '../../../shared/widgets/status_badge.dart';
import '../../jobs/data/jobs_providers.dart';
import '../data/runs_providers.dart';
import 'run_details_screen.dart';

const _allStatuses = [
  'PENDING',
  'RUNNING',
  'SUCCESS',
  'WARNING',
  'FAILED',
  'CANCELLED',
  'SKIPPED',
  'INTERRUPTED',
];

class HistoryScreen extends ConsumerWidget {
  const HistoryScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final runsAsync = ref.watch(historyRunsProvider);
    final jobs = ref.watch(jobsListProvider).value ?? const [];
    final filter = ref.watch(historyFilterProvider);
    final jobNameById = {for (final j in jobs) j.id: j.name};

    return Scaffold(
      appBar: AppBar(title: Text(l10n.navHistory)),
      body: Column(
        children: [
          Padding(
            padding: const EdgeInsets.all(16),
            child: Row(
              children: [
                Expanded(
                  child: DropdownButtonFormField<String>(
                    initialValue: filter.jobId,
                    decoration: InputDecoration(
                      labelText: l10n.historyFilterJob,
                      isDense: true,
                    ),
                    items: [
                      DropdownMenuItem(
                        value: '',
                        child: Text(l10n.historyFilterAll),
                      ),
                      for (final job in jobs)
                        DropdownMenuItem(value: job.id, child: Text(job.name)),
                    ],
                    onChanged: (v) => ref
                        .read(historyFilterProvider.notifier)
                        .setJobId(v ?? ''),
                  ),
                ),
                const SizedBox(width: 16),
                Expanded(
                  child: DropdownButtonFormField<String>(
                    initialValue: filter.status,
                    decoration: InputDecoration(
                      labelText: l10n.historyFilterStatus,
                      isDense: true,
                    ),
                    items: [
                      DropdownMenuItem(
                        value: '',
                        child: Text(l10n.historyFilterAll),
                      ),
                      for (final status in _allStatuses)
                        DropdownMenuItem(value: status, child: Text(status)),
                    ],
                    onChanged: (v) => ref
                        .read(historyFilterProvider.notifier)
                        .setStatus(v ?? ''),
                  ),
                ),
              ],
            ),
          ),
          Expanded(
            child: runsAsync.when(
              loading: () => const Center(child: CircularProgressIndicator()),
              error: (error, _) => ErrorView(
                error: error,
                onRetry: () => ref.read(historyRunsProvider.notifier).refresh(),
              ),
              data: (runs) {
                if (runs.isEmpty) {
                  return EmptyState(
                    icon: Icons.history,
                    message: l10n.historyEmptyState,
                  );
                }
                return SingleChildScrollView(
                  scrollDirection: Axis.horizontal,
                  child: DataTable(
                    columns: [
                      DataColumn(label: Text(l10n.historyColumnDate)),
                      DataColumn(label: Text(l10n.historyColumnJob)),
                      DataColumn(label: Text(l10n.historyColumnStatus)),
                      DataColumn(label: Text(l10n.historyColumnSize)),
                      DataColumn(label: Text(l10n.historyColumnDuration)),
                      DataColumn(label: Text(l10n.historyColumnTrigger)),
                    ],
                    rows: [
                      for (final run in runs)
                        DataRow(
                          onSelectChanged: (_) => Navigator.of(context).push(
                            MaterialPageRoute<void>(
                              builder: (_) => RunDetailsScreen(runId: run.id),
                            ),
                          ),
                          cells: [
                            DataCell(Text(formatTimestamp(run.startedAt))),
                            DataCell(Text(jobNameById[run.jobId] ?? run.jobId)),
                            DataCell(StatusBadge(status: run.status)),
                            DataCell(
                              Text(
                                run.archiveSize > 0
                                    ? formatBytes(run.archiveSize)
                                    : '—',
                              ),
                            ),
                            DataCell(Text(_duration(run))),
                            DataCell(Text(run.trigger)),
                          ],
                        ),
                    ],
                  ),
                );
              },
            ),
          ),
        ],
      ),
    );
  }

  String _duration(RunDto run) {
    final d = durationBetween(run.startedAt, run.finishedAt);
    return d == null ? '—' : formatDuration(d);
  }
}
