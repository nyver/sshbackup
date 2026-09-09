import '../../../core/ipc/models.dart';

/// Consumer-side port for the `settings.*` IPC commands.
abstract class SettingsRepository {
  Future<SettingsDto> get();
  Future<SettingsDto> set(SettingsDto settings);
}
