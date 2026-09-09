import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/ipc/models.dart';
import '../../../l10n/gen/app_localizations.dart';
import '../../../shared/format.dart';
import '../../../shared/widgets/empty_state.dart';
import '../../../shared/widgets/error_view.dart';
import '../../../shared/widgets/status_badge.dart';
import '../../jobs/data/jobs_providers.dart';
import '../../runs/data/runs_providers.dart';
import '../../runs/ui/run_details_screen.dart';
import '../../servers/data/servers_providers.dart';

class DashboardScreen extends ConsumerWidget {
  const DashboardScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final servers = ref.watch(serversListProvider);
    final jobs = ref.watch(jobsListProvider);
    final recentRuns = ref.watch(recentRunsProvider);
    final activeRuns = ref.watch(activeRunsProvider);
    final jobNameById = {
      for (final j in jobs.value ?? const <JobDto>[]) j.id: j.name,
    };

    return Scaffold(
      appBar: AppBar(title: Text(l10n.navDashboard)),
      body: servers.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (error, _) => ErrorView(error: error),
        data: (serverList) {
          if (serverList.isEmpty) {
            return EmptyState(
              icon: Icons.rocket_launch_outlined,
              message: l10n.dashboardEmptyState,
            );
          }
          return ListView(
            padding: const EdgeInsets.all(16),
            children: [
              if (activeRuns.isNotEmpty) ...[
                Text(
                  l10n.dashboardActiveRuns,
                  style: Theme.of(context).textTheme.titleMedium,
                ),
                const SizedBox(height: 8),
                for (final run in activeRuns)
                  _ActiveRunCard(
                    run: run,
                    jobName: jobNameById[run.jobId] ?? run.jobId,
                  ),
                const SizedBox(height: 24),
              ],
              Text(
                l10n.dashboardUpcoming,
                style: Theme.of(context).textTheme.titleMedium,
              ),
              const SizedBox(height: 8),
              jobs.when(
                loading: () => const Center(child: CircularProgressIndicator()),
                error: (error, _) => ErrorView(error: error),
                data: (jobList) {
                  final upcoming =
                      jobList
                          .where((j) => j.enabled && j.nextRunAt.isNotEmpty)
                          .toList()
                        ..sort((a, b) => a.nextRunAt.compareTo(b.nextRunAt));
                  if (upcoming.isEmpty) {
                    return Padding(
                      padding: const EdgeInsets.symmetric(vertical: 8),
                      child: Text(
                        l10n.dashboardNoUpcoming,
                        style: Theme.of(context).textTheme.bodyMedium,
                      ),
                    );
                  }
                  return Card(
                    child: Column(
                      children: [
                        for (final job in upcoming.take(5))
                          ListTile(
                            leading: const Icon(Icons.schedule),
                            title: Text(job.name),
                            trailing: Text(formatTimestamp(job.nextRunAt)),
                          ),
                      ],
                    ),
                  );
                },
              ),
              const SizedBox(height: 24),
              Text(
                l10n.dashboardRecent,
                style: Theme.of(context).textTheme.titleMedium,
              ),
              const SizedBox(height: 8),
              recentRuns.when(
                loading: () => const Center(child: CircularProgressIndicator()),
                error: (error, _) => ErrorView(error: error),
                data: (runs) {
                  if (runs.isEmpty) {
                    return Padding(
                      padding: const EdgeInsets.symmetric(vertical: 8),
                      child: Text(
                        l10n.dashboardNoRecent,
                        style: Theme.of(context).textTheme.bodyMedium,
                      ),
                    );
                  }
                  return Card(
                    child: Column(
                      children: [
                        for (final run in runs.take(10))
                          ListTile(
                            leading: StatusBadge(status: run.status),
                            title: Text(jobNameById[run.jobId] ?? run.jobId),
                            subtitle: Text(
                              run.errorMessage.isNotEmpty
                                  ? '${formatTimestamp(run.startedAt)} — ${run.errorMessage}'
                                  : formatTimestamp(run.startedAt),
                            ),
                            trailing: run.archiveSize > 0
                                ? Text(formatBytes(run.archiveSize))
                                : null,
                            onTap: () => Navigator.of(context).push(
                              MaterialPageRoute<void>(
                                builder: (_) => RunDetailsScreen(runId: run.id),
                              ),
                            ),
                          ),
                      ],
                    ),
                  );
                },
              ),
            ],
          );
        },
      ),
    );
  }
}

class _ActiveRunCard extends ConsumerWidget {
  const _ActiveRunCard({required this.run, required this.jobName});

  final RunDto run;
  final String jobName;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final detailsAsync = ref.watch(runDetailsProvider(run.id));
    final currentStage = detailsAsync.value?.steps.isNotEmpty ?? false
        ? detailsAsync.value!.steps.last.type
        : null;

    return Card(
      child: ListTile(
        leading: const Icon(Icons.play_circle_outline),
        title: Text(jobName),
        subtitle: Text(l10n.dashboardActiveRunStage(currentStage ?? '—')),
        trailing: StatusBadge(status: run.status),
        onTap: () => Navigator.of(context).push(
          MaterialPageRoute<void>(
            builder: (_) => RunDetailsScreen(runId: run.id),
          ),
        ),
      ),
    );
  }
}
