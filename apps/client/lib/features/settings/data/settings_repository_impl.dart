import '../../../core/ipc/ipc_client.dart';
import '../../../core/ipc/models.dart';
import '../domain/settings_repository.dart';

class IpcSettingsRepository implements SettingsRepository {
  IpcSettingsRepository(this._client);

  final IpcClient _client;

  @override
  Future<SettingsDto> get() async {
    final res = await _client.request('settings.get');
    return SettingsDto.fromJson(res['settings'] as Map<String, dynamic>);
  }

  @override
  Future<SettingsDto> set(SettingsDto settings) async {
    final res = await _client.request('settings.set', {
      'settings': settings.toJson(),
    });
    return SettingsDto.fromJson(res['settings'] as Map<String, dynamic>);
  }
}
