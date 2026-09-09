import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/ipc/ipc_client.dart';
import '../../../core/ipc/models.dart';
import '../../../l10n/gen/app_localizations.dart';
import '../../../shared/format.dart';
import '../../../shared/widgets/empty_state.dart';
import '../../../shared/widgets/error_view.dart';
import '../../runs/data/runs_providers.dart';
import '../data/jobs_providers.dart';
import 'job_editor_screen.dart';
import 'validate_job_dialog.dart';

class JobsScreen extends ConsumerWidget {
  const JobsScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final jobsAsync = ref.watch(jobsListProvider);

    return Scaffold(
      appBar: AppBar(title: Text(l10n.navJobs)),
      floatingActionButton: FloatingActionButton.extended(
        onPressed: () => Navigator.of(context).push(
          MaterialPageRoute<void>(builder: (_) => const JobEditorScreen()),
        ),
        icon: const Icon(Icons.add),
        label: Text(l10n.jobAddTitle),
      ),
      body: jobsAsync.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (error, _) => ErrorView(
          error: error,
          onRetry: () => ref.read(jobsListProvider.notifier).refresh(),
        ),
        data: (jobs) {
          if (jobs.isEmpty) {
            return EmptyState(
              icon: Icons.event_repeat_outlined,
              message: l10n.jobsEmptyState,
              action: FilledButton.icon(
                onPressed: () => Navigator.of(context).push(
                  MaterialPageRoute<void>(
                    builder: (_) => const JobEditorScreen(),
                  ),
                ),
                icon: const Icon(Icons.add),
                label: Text(l10n.jobAddTitle),
              ),
            );
          }
          return ListView.builder(
            padding: const EdgeInsets.fromLTRB(16, 16, 16, 96),
            itemCount: jobs.length,
            itemBuilder: (context, index) => _JobCard(job: jobs[index]),
          );
        },
      ),
    );
  }
}

class _JobCard extends ConsumerWidget {
  const _JobCard({required this.job});

  final JobDto job;

  String _scheduleSummary(AppLocalizations l10n) {
    switch (job.schedule.type) {
      case 'MANUAL':
        return l10n.scheduleManual;
      case 'DAILY':
        final hour = job.schedule.hour.toString().padLeft(2, '0');
        final minute = job.schedule.minute.toString().padLeft(2, '0');
        return l10n.scheduleSummaryDaily('$hour:$minute');
      case 'WEEKLY':
        return l10n.scheduleWeekly;
      case 'MONTHLY':
        return l10n.scheduleSummaryMonthly(job.schedule.dayOfMonth);
      case 'CRON':
        return job.schedule.cronExpression;
      default:
        return job.schedule.type;
    }
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    return Card(
      margin: const EdgeInsets.only(bottom: 12),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Row(
          children: [
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Text(
                        job.name,
                        style: Theme.of(context).textTheme.titleMedium,
                      ),
                      if (!job.enabled) ...[
                        const SizedBox(width: 8),
                        Chip(
                          label: Text(l10n.jobDisabledBadge),
                          visualDensity: VisualDensity.compact,
                        ),
                      ],
                    ],
                  ),
                  Text(
                    _scheduleSummary(l10n),
                    style: Theme.of(context).textTheme.bodySmall,
                  ),
                  if (job.nextRunAt.isNotEmpty)
                    Text(
                      l10n.jobNextRun(formatTimestamp(job.nextRunAt)),
                      style: Theme.of(context).textTheme.bodySmall,
                    ),
                ],
              ),
            ),
            Switch(
              value: job.enabled,
              onChanged: (v) => ref
                  .read(jobsListProvider.notifier)
                  .setEnabled(job.id, enabled: v),
            ),
            IconButton(
              tooltip: l10n.jobValidateTitle,
              icon: const Icon(Icons.fact_check_outlined),
              onPressed: () => showValidateJobDialog(
                context,
                repository: ref.read(jobsRepositoryProvider),
                jobId: job.id,
              ),
            ),
            IconButton(
              tooltip: l10n.jobRunNow,
              icon: const Icon(Icons.play_arrow),
              onPressed: () => _runNow(context, ref, job),
            ),
            IconButton(
              tooltip: l10n.edit,
              icon: const Icon(Icons.edit_outlined),
              onPressed: () => Navigator.of(context).push(
                MaterialPageRoute<void>(
                  builder: (_) => JobEditorScreen(existing: job),
                ),
              ),
            ),
            IconButton(
              tooltip: l10n.delete,
              icon: const Icon(Icons.delete_outline),
              onPressed: () => _confirmDelete(context, ref, job),
            ),
          ],
        ),
      ),
    );
  }

  Future<void> _runNow(BuildContext context, WidgetRef ref, JobDto job) async {
    final l10n = AppLocalizations.of(context)!;
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: Text(l10n.jobRunNowConfirmTitle),
        content: Text(l10n.jobRunNowConfirmBody(job.name)),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(context).pop(false),
            child: Text(l10n.cancel),
          ),
          FilledButton(
            onPressed: () => Navigator.of(context).pop(true),
            child: Text(l10n.jobRunNow),
          ),
        ],
      ),
    );
    if (confirmed != true || !context.mounted) return;

    try {
      await ref.read(runsRepositoryProvider).start(job.id);
    } on IpcException catch (e) {
      if (!context.mounted) return;
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text(e.message)));
    }
  }

  Future<void> _confirmDelete(
    BuildContext context,
    WidgetRef ref,
    JobDto job,
  ) async {
    final l10n = AppLocalizations.of(context)!;
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: Text(l10n.jobDeleteConfirmTitle),
        content: Text(l10n.jobDeleteConfirmBody(job.name)),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(context).pop(false),
            child: Text(l10n.cancel),
          ),
          FilledButton(
            onPressed: () => Navigator.of(context).pop(true),
            child: Text(l10n.delete),
          ),
        ],
      ),
    );
    if (confirmed ?? false) {
      await ref.read(jobsListProvider.notifier).delete(job.id);
    }
  }
}
