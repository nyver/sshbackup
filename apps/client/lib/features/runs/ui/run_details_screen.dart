import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/ipc/ipc_client.dart';
import '../../../core/ipc/models.dart';
import '../../../l10n/gen/app_localizations.dart';
import '../../../shared/format.dart';
import '../../../shared/widgets/error_view.dart';
import '../../../shared/widgets/status_badge.dart';
import '../data/runs_providers.dart';
import '../domain/runs_repository.dart';

class RunDetailsScreen extends ConsumerWidget {
  const RunDetailsScreen({required this.runId, super.key});

  final String runId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final detailsAsync = ref.watch(runDetailsProvider(runId));

    return Scaffold(
      appBar: AppBar(title: Text(l10n.runDetailsTitle)),
      body: detailsAsync.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (error, _) => ErrorView(error: error),
        data: (details) => _RunDetailsBody(details: details),
      ),
    );
  }
}

class _RunDetailsBody extends ConsumerWidget {
  const _RunDetailsBody({required this.details});

  final RunDetails details;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final run = details.run;
    final isActive = nonTerminalRunStatuses.contains(run.status);
    final duration = durationBetween(run.startedAt, run.finishedAt);

    return ListView(
      padding: const EdgeInsets.all(16),
      children: [
        Card(
          child: Padding(
            padding: const EdgeInsets.all(16),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    StatusBadge(status: run.status),
                    const Spacer(),
                    if (isActive)
                      OutlinedButton.icon(
                        onPressed: () => _cancel(context, ref, run.id),
                        icon: const Icon(Icons.stop_circle_outlined),
                        label: Text(l10n.runCancel),
                      ),
                  ],
                ),
                const SizedBox(height: 12),
                _InfoRow(
                  label: l10n.runDetailsStarted,
                  value: formatTimestamp(run.startedAt),
                ),
                _InfoRow(
                  label: l10n.runDetailsFinished,
                  value: run.finishedAt.isEmpty
                      ? '—'
                      : formatTimestamp(run.finishedAt),
                ),
                if (duration != null)
                  _InfoRow(
                    label: l10n.runDetailsDuration,
                    value: formatDuration(duration),
                  ),
                _InfoRow(label: l10n.runDetailsTrigger, value: run.trigger),
                if (run.archiveName.isNotEmpty) ...[
                  _InfoRow(
                    label: l10n.runDetailsArchiveName,
                    value: run.archiveName,
                  ),
                  _InfoRow(
                    label: l10n.runDetailsArchiveSize,
                    value: formatBytes(run.archiveSize),
                  ),
                ],
                if (run.checksum.isNotEmpty)
                  _InfoRow(
                    label: l10n.runDetailsChecksum,
                    value: run.checksum,
                    monospace: true,
                  ),
                if (run.recoveryOutcome != 'NOT_APPLICABLE')
                  _InfoRow(
                    label: l10n.runDetailsRecoveryOutcome,
                    value: _recoveryOutcomeLabel(l10n, run.recoveryOutcome),
                  ),
                if (run.errorCode.isNotEmpty) ...[
                  const SizedBox(height: 8),
                  Container(
                    padding: const EdgeInsets.all(12),
                    decoration: BoxDecoration(
                      color: Theme.of(context).colorScheme.errorContainer,
                      borderRadius: BorderRadius.circular(8),
                    ),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          run.errorCode,
                          style: Theme.of(context).textTheme.labelMedium,
                        ),
                        if (run.errorMessage.isNotEmpty) Text(run.errorMessage),
                      ],
                    ),
                  ),
                ],
              ],
            ),
          ),
        ),
        const SizedBox(height: 16),
        Text(
          l10n.runDetailsSteps,
          style: Theme.of(context).textTheme.titleMedium,
        ),
        const SizedBox(height: 8),
        for (final step in details.steps) _StepTile(step: step),
      ],
    );
  }

  String _recoveryOutcomeLabel(AppLocalizations l10n, String outcome) =>
      switch (outcome) {
        'SUCCESS' => l10n.recoveryOutcomeSuccess,
        'FAILED' => l10n.recoveryOutcomeFailed,
        _ => outcome,
      };

  Future<void> _cancel(
    BuildContext context,
    WidgetRef ref,
    String runId,
  ) async {
    try {
      await ref.read(runsRepositoryProvider).cancel(runId);
    } on IpcException catch (e) {
      if (!context.mounted) return;
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text(e.message)));
    }
  }
}

class _InfoRow extends StatelessWidget {
  const _InfoRow({
    required this.label,
    required this.value,
    this.monospace = false,
  });

  final String label;
  final String value;
  final bool monospace;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 2),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 140,
            child: Text(label, style: Theme.of(context).textTheme.bodySmall),
          ),
          Expanded(
            child: SelectableText(
              value,
              style: monospace
                  ? const TextStyle(fontFamily: 'monospace')
                  : null,
            ),
          ),
        ],
      ),
    );
  }
}

class _StepTile extends StatelessWidget {
  const _StepTile({required this.step});

  final StepDto step;

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final duration = step.durationMs > 0
        ? formatDuration(Duration(milliseconds: step.durationMs))
        : null;
    return Card(
      margin: const EdgeInsets.only(bottom: 8),
      child: ExpansionTile(
        initiallyExpanded: step.status == 'FAILED',
        title: Row(
          children: [
            Expanded(child: Text(step.type)),
            if (duration != null) ...[
              Text(duration, style: Theme.of(context).textTheme.bodySmall),
              const SizedBox(width: 12),
            ],
            StatusBadge(status: step.status),
          ],
        ),
        // A collapsed script step otherwise shows only its type, and a
        // job commonly has several PRE_BACKUP_SCRIPT/POST_BACKUP_SCRIPT
        // steps — the command preview is what tells them apart at a
        // glance, before expanding.
        subtitle: step.command.isEmpty
            ? null
            : Text(
                step.command,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: Theme.of(context).textTheme.bodySmall
                    ?.copyWith(fontFamily: 'monospace'),
              ),
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 0, 16, 16),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                if (step.command.isNotEmpty) ...[
                  Text(
                    l10n.runDetailsCommand,
                    style: Theme.of(context).textTheme.labelSmall,
                  ),
                  const SizedBox(height: 4),
                  Container(
                    width: double.infinity,
                    padding: const EdgeInsets.all(8),
                    margin: const EdgeInsets.only(bottom: 8),
                    decoration: BoxDecoration(
                      color: Theme.of(context)
                          .colorScheme
                          .surfaceContainerHighest,
                      borderRadius: BorderRadius.circular(6),
                    ),
                    child: SelectableText(
                      step.command,
                      style: const TextStyle(
                        fontFamily: 'monospace',
                        fontSize: 12,
                      ),
                    ),
                  ),
                ],
                if (step.error.isNotEmpty)
                  Padding(
                    padding: const EdgeInsets.only(bottom: 8),
                    child: Text(
                      step.error,
                      style: TextStyle(
                        color: Theme.of(context).colorScheme.error,
                      ),
                    ),
                  ),
                if (step.output.isNotEmpty)
                  Container(
                    width: double.infinity,
                    padding: const EdgeInsets.all(8),
                    decoration: BoxDecoration(
                      color: Theme.of(context)
                          .colorScheme
                          .surfaceContainerHighest,
                      borderRadius: BorderRadius.circular(6),
                    ),
                    child: SelectableText(
                      step.truncated
                          ? '${step.output}\n${l10n.runDetailsOutputTruncated}'
                          : step.output,
                      style: const TextStyle(
                        fontFamily: 'monospace',
                        fontSize: 12,
                      ),
                    ),
                  )
                else
                  Text(
                    l10n.runDetailsNoOutput,
                    style: Theme.of(context).textTheme.bodySmall,
                  ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}
