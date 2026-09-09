import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/ipc/ipc_client.dart';
import '../core/ipc_providers.dart';
import '../l10n/gen/app_localizations.dart';

/// Gates [child] behind the background service being reachable
/// (ipc-api specification, "Service unavailable"): while disconnected, no
/// screen renders — so no configuration screen can appear to accept
/// changes that were never sent — and the user sees a clear message plus a
/// retry action instead.
class ConnectionGate extends ConsumerWidget {
  const ConnectionGate({required this.child, super.key});

  final Widget child;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final connectionAsync = ref.watch(connectionStateProvider);
    final state = connectionAsync.value ?? IpcConnectionState.connecting;

    return switch (state) {
      IpcConnectionState.connected => child,
      IpcConnectionState.connecting => _StatusScreen(
        icon: Icons.sync,
        title: l10n.serviceConnecting,
        message: null,
        showRetry: false,
      ),
      IpcConnectionState.disconnected => _StatusScreen(
        icon: Icons.cloud_off,
        title: l10n.serviceUnavailableTitle,
        message: l10n.serviceUnavailableMessage,
        showRetry: true,
      ),
    };
  }
}

class _StatusScreen extends ConsumerWidget {
  const _StatusScreen({
    required this.icon,
    required this.title,
    required this.message,
    required this.showRetry,
  });

  final IconData icon;
  final String title;
  final String? message;
  final bool showRetry;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    return Scaffold(
      body: Center(
        child: Padding(
          padding: const EdgeInsets.all(32),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(
                icon,
                size: 56,
                color: Theme.of(context).colorScheme.onSurfaceVariant,
              ),
              const SizedBox(height: 16),
              Text(
                title,
                textAlign: TextAlign.center,
                style: Theme.of(context).textTheme.titleLarge,
              ),
              if (message != null) ...[
                const SizedBox(height: 8),
                ConstrainedBox(
                  constraints: const BoxConstraints(maxWidth: 420),
                  child: Text(
                    message!,
                    textAlign: TextAlign.center,
                    style: Theme.of(context).textTheme.bodyMedium,
                  ),
                ),
              ],
              if (showRetry) ...[
                const SizedBox(height: 20),
                FilledButton.icon(
                  onPressed: () =>
                      unawaited(ref.read(ipcClientProvider).retryNow()),
                  icon: const Icon(Icons.refresh),
                  label: Text(l10n.retry),
                ),
              ],
            ],
          ),
        ),
      ),
    );
  }
}
