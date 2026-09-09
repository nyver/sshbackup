import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/ipc/models.dart';
import '../../../core/ipc_providers.dart';
import '../domain/settings_repository.dart';
import 'settings_repository_impl.dart';

final settingsRepositoryProvider = Provider<SettingsRepository>((ref) {
  return IpcSettingsRepository(ref.watch(ipcClientProvider));
});

/// The current global settings, kept in sync with the service's
/// `settings.changed` push event so a pause toggled from the tray (or a
/// second client) is reflected here without polling.
final settingsProvider = AsyncNotifierProvider<SettingsNotifier, SettingsDto>(
  SettingsNotifier.new,
);

class SettingsNotifier extends AsyncNotifier<SettingsDto> {
  @override
  Future<SettingsDto> build() {
    final client = ref.watch(ipcClientProvider);
    final subscription = client.events.listen((env) {
      if (env.event != 'settings.changed') return;
      final payload = env.payload;
      if (payload is! Map<String, dynamic>) return;
      final settingsJson = payload['settings'];
      if (settingsJson is! Map<String, dynamic>) return;
      state = AsyncData(SettingsDto.fromJson(settingsJson));
    });
    ref.onDispose(() => unawaited(subscription.cancel()));
    return ref.watch(settingsRepositoryProvider).get();
  }

  Future<void> updateSettings(SettingsDto settings) async {
    state = AsyncData(await ref.read(settingsRepositoryProvider).set(settings));
  }

  Future<void> setSchedulesPaused(bool paused) async {
    final current = state.value;
    if (current == null) return;
    await updateSettings(current.copyWith(schedulesPaused: paused));
  }
}
