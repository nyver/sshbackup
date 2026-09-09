import 'dart:io' show Platform;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../l10n/gen/app_localizations.dart';
import '../../../shared/widgets/error_view.dart';
import '../data/settings_providers.dart';

/// The service always resolves its data directory to `%ProgramData%\
/// VPSBackupManager` (design.md, "Storage") — not user-configurable in
/// this version, and not exposed over IPC, so this reads the same
/// well-known environment variable the service itself uses rather than
/// inventing a round trip for it.
String _dataDirectoryPath() {
  final base = Platform.environment['ProgramData'] ?? r'C:\ProgramData';
  return '$base\\VPSBackupManager';
}

class SettingsScreen extends ConsumerWidget {
  const SettingsScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final settingsAsync = ref.watch(settingsProvider);

    return Scaffold(
      appBar: AppBar(title: Text(l10n.navSettings)),
      body: settingsAsync.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (error, _) => ErrorView(
          error: error,
          onRetry: () => ref.invalidate(settingsProvider),
        ),
        data: (settings) => ListView(
          padding: const EdgeInsets.all(16),
          children: [
            Card(
              child: Column(
                children: [
                  SwitchListTile(
                    value: settings.schedulesPaused,
                    onChanged: (v) => ref
                        .read(settingsProvider.notifier)
                        .updateSettings(settings.copyWith(schedulesPaused: v)),
                    title: Text(l10n.settingsPauseSchedules),
                    subtitle: Text(l10n.settingsPauseSchedulesHint),
                  ),
                  const Divider(height: 1),
                  ListTile(
                    title: Text(l10n.settingsConcurrencyLimit),
                    subtitle: Text(l10n.settingsConcurrencyLimitHint),
                    trailing: SizedBox(
                      width: 72,
                      child: TextFormField(
                        key: ValueKey(settings.globalConcurrencyLimit),
                        initialValue: settings.globalConcurrencyLimit
                            .toString(),
                        keyboardType: TextInputType.number,
                        textAlign: TextAlign.end,
                        onFieldSubmitted: (v) {
                          final limit = int.tryParse(v);
                          if (limit != null && limit > 0) {
                            ref
                                .read(settingsProvider.notifier)
                                .updateSettings(
                                  settings.copyWith(
                                    globalConcurrencyLimit: limit,
                                  ),
                                );
                          }
                        },
                      ),
                    ),
                  ),
                ],
              ),
            ),
            const SizedBox(height: 16),
            Card(
              child: Column(
                children: [
                  SwitchListTile(
                    value: settings.notifySuccess,
                    onChanged: (v) => ref
                        .read(settingsProvider.notifier)
                        .updateSettings(settings.copyWith(notifySuccess: v)),
                    title: Text(l10n.settingsNotifySuccess),
                  ),
                  const Divider(height: 1),
                  SwitchListTile(
                    value: settings.notifyFailure,
                    onChanged: (v) => ref
                        .read(settingsProvider.notifier)
                        .updateSettings(settings.copyWith(notifyFailure: v)),
                    title: Text(l10n.settingsNotifyFailure),
                  ),
                ],
              ),
            ),
            const SizedBox(height: 16),
            Card(
              child: Column(
                children: [
                  SwitchListTile(
                    value: settings.staleRemoteCleanupEnabled,
                    onChanged: (v) => ref
                        .read(settingsProvider.notifier)
                        .updateSettings(
                          settings.copyWith(staleRemoteCleanupEnabled: v),
                        ),
                    title: Text(l10n.settingsStaleCleanup),
                    subtitle: Text(
                      l10n.settingsStaleCleanupHint(
                        settings.staleRemoteCleanupAgeHours,
                      ),
                    ),
                  ),
                ],
              ),
            ),
            const SizedBox(height: 16),
            Card(
              child: ListTile(
                title: Text(l10n.settingsDataLocation),
                subtitle: SelectableText(_dataDirectoryPath()),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
