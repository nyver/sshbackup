import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/ipc/ipc_client.dart';
import '../../../core/ipc/models.dart';
import '../../../l10n/gen/app_localizations.dart';
import '../../../shared/widgets/empty_state.dart';
import '../../../shared/widgets/error_view.dart';
import '../../jobs/data/jobs_providers.dart';
import '../data/servers_providers.dart';
import 'host_key_dialog.dart';
import 'server_editor_dialog.dart';

/// In-memory only: the service does not persist a "last test result" per
/// server, so this reflects only tests run during the current UI session.
final _lastTestResultProvider =
    NotifierProvider<
      _LastTestResultNotifier,
      Map<String, TestConnectionResult>
    >(_LastTestResultNotifier.new);

class _LastTestResultNotifier
    extends Notifier<Map<String, TestConnectionResult>> {
  @override
  Map<String, TestConnectionResult> build() => const {};

  void set(String serverId, TestConnectionResult result) {
    state = {...state, serverId: result};
  }
}

class ServersScreen extends ConsumerWidget {
  const ServersScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final serversAsync = ref.watch(serversListProvider);

    return Scaffold(
      appBar: AppBar(title: Text(l10n.navServers)),
      floatingActionButton: FloatingActionButton.extended(
        onPressed: () => showServerEditorDialog(context),
        icon: const Icon(Icons.add),
        label: Text(l10n.serverAddTitle),
      ),
      body: serversAsync.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (error, _) => ErrorView(
          error: error,
          onRetry: () => ref.read(serversListProvider.notifier).refresh(),
        ),
        data: (servers) {
          if (servers.isEmpty) {
            return EmptyState(
              icon: Icons.dns_outlined,
              message: l10n.serversEmptyState,
              action: FilledButton.icon(
                onPressed: () => showServerEditorDialog(context),
                icon: const Icon(Icons.add),
                label: Text(l10n.serverAddTitle),
              ),
            );
          }
          return ListView.builder(
            padding: const EdgeInsets.fromLTRB(16, 16, 16, 96),
            itemCount: servers.length,
            itemBuilder: (context, index) =>
                _ServerCard(server: servers[index]),
          );
        },
      ),
    );
  }
}

class _ServerCard extends ConsumerWidget {
  const _ServerCard({required this.server});

  final ServerDto server;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final lastResult = ref.watch(_lastTestResultProvider)[server.id];

    return Card(
      margin: const EdgeInsets.only(bottom: 12),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Row(
          children: [
            CircleAvatar(
              child: Text(
                server.name.isEmpty ? '?' : server.name[0].toUpperCase(),
              ),
            ),
            const SizedBox(width: 16),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    server.name,
                    style: Theme.of(context).textTheme.titleMedium,
                  ),
                  Text(
                    '${server.username}@${server.host}:${server.port}',
                    style: Theme.of(context).textTheme.bodySmall,
                  ),
                  if (lastResult != null) ...[
                    const SizedBox(height: 4),
                    _LastResultLabel(result: lastResult),
                  ],
                ],
              ),
            ),
            IconButton(
              tooltip: l10n.serverTestConnection,
              icon: const Icon(Icons.wifi_tethering),
              onPressed: () => _runTestConnectionFlow(context, ref, server),
            ),
            IconButton(
              tooltip: l10n.edit,
              icon: const Icon(Icons.edit_outlined),
              onPressed: () =>
                  showServerEditorDialog(context, existing: server),
            ),
            IconButton(
              tooltip: l10n.delete,
              icon: const Icon(Icons.delete_outline),
              onPressed: () => _confirmDelete(context, ref, server),
            ),
          ],
        ),
      ),
    );
  }

  Future<void> _confirmDelete(
    BuildContext context,
    WidgetRef ref,
    ServerDto server,
  ) async {
    final l10n = AppLocalizations.of(context)!;
    final jobs = ref.read(jobsListProvider).value ?? const [];
    final referencingJobs = jobs.where((j) => j.serverId == server.id).length;
    if (referencingJobs > 0) {
      await showDialog<void>(
        context: context,
        builder: (context) => AlertDialog(
          title: Text(l10n.serverDeleteBlockedTitle),
          content: Text(
            l10n.serverDeleteBlockedBody(server.name, referencingJobs),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.of(context).pop(),
              child: Text(l10n.ok),
            ),
          ],
        ),
      );
      return;
    }

    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: Text(l10n.serverDeleteConfirmTitle),
        content: Text(l10n.serverDeleteConfirmBody(server.name)),
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
      await ref.read(serversListProvider.notifier).delete(server.id);
    }
  }
}

class _LastResultLabel extends StatelessWidget {
  const _LastResultLabel({required this.result});

  final TestConnectionResult result;

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final scheme = Theme.of(context).colorScheme;
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(
          result.success ? Icons.check_circle : Icons.error,
          size: 14,
          color: result.success ? Colors.green : scheme.error,
        ),
        const SizedBox(width: 4),
        Text(
          result.success
              ? l10n.serverConnectionSucceeded
              : (result.message.isEmpty
                    ? l10n.serverConnectionFailed
                    : result.message),
          style: Theme.of(context).textTheme.bodySmall,
        ),
      ],
    );
  }
}

Future<void> _runTestConnectionFlow(
  BuildContext context,
  WidgetRef ref,
  ServerDto server,
) async {
  final l10n = AppLocalizations.of(context)!;
  final repository = ref.read(serversRepositoryProvider);

  // The dialog is not user-dismissible, so its future only resolves with
  // the actual test result; the fallback below is defensive only.
  Future<TestConnectionResult> runTest() async {
    final result = await showDialog<TestConnectionResult>(
      context: context,
      barrierDismissible: false,
      builder: (context) => _TestConnectionProgressDialog(
        future: repository.testConnection(server.id),
      ),
    );
    return result ?? TestConnectionResult(success: false);
  }

  var result = await runTest();
  if (!context.mounted) return;

  // Trust-on-first-use: an unverified or changed host key surfaces as a
  // failed test carrying the presented fingerprint (server-management
  // specification).
  if (!result.success && result.hostKeyFingerprint.isNotEmpty) {
    final bool confirmed;
    if (result.fingerprintChanged) {
      confirmed = await showHostKeyChangedDialog(
        context: context,
        host: server.host,
        storedFingerprint: server.hostKeyFingerprint,
        presentedAlgorithm: result.hostKeyAlgorithm,
        presentedFingerprint: result.hostKeyFingerprint,
      );
    } else {
      confirmed = await showHostKeyConfirmDialog(
        context: context,
        host: server.host,
        algorithm: result.hostKeyAlgorithm,
        fingerprint: result.hostKeyFingerprint,
      );
    }
    if (confirmed) {
      try {
        await ref
            .read(serversListProvider.notifier)
            .confirmHostKey(
              serverId: server.id,
              fingerprint: result.hostKeyFingerprint,
              algorithm: result.hostKeyAlgorithm,
            );
        result = await runTest();
      } on IpcException catch (e) {
        result = TestConnectionResult(success: false, message: e.message);
      }
      if (!context.mounted) return;
    }
  }

  ref.read(_lastTestResultProvider.notifier).set(server.id, result);

  if (!context.mounted) return;
  ScaffoldMessenger.of(context).showSnackBar(
    SnackBar(
      content: Text(
        result.success
            ? l10n.serverConnectionSucceeded
            : (result.message.isEmpty
                  ? l10n.serverConnectionFailed
                  : result.message),
      ),
    ),
  );
}

class _TestConnectionProgressDialog extends StatefulWidget {
  const _TestConnectionProgressDialog({required this.future});

  final Future<TestConnectionResult> future;

  @override
  State<_TestConnectionProgressDialog> createState() =>
      _TestConnectionProgressDialogState();
}

class _TestConnectionProgressDialogState
    extends State<_TestConnectionProgressDialog> {
  @override
  void initState() {
    super.initState();
    unawaited(
      widget.future
          .then((result) {
            if (mounted) Navigator.of(context).pop(result);
          })
          .catchError((Object error) {
            if (mounted) {
              // Desktop-ui "Error presentation": never surface raw
              // exception text. An IpcException already carries a safe
              // message; anything else falls back to a generic one.
              final message = error is IpcException
                  ? error.message
                  : AppLocalizations.of(context)!.genericErrorMessage;
              Navigator.of(context)
                  .pop(TestConnectionResult(success: false, message: message));
            }
          }),
    );
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    return AlertDialog(
      content: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          const SizedBox(
            width: 20,
            height: 20,
            child: CircularProgressIndicator(strokeWidth: 2),
          ),
          const SizedBox(width: 16),
          Text(l10n.serverTestingConnection),
        ],
      ),
    );
  }
}
