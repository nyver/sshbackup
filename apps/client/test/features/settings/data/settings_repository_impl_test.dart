import 'package:flutter_test/flutter_test.dart';
import 'package:vps_backup_manager/core/ipc/models.dart';
import 'package:vps_backup_manager/features/settings/data/settings_repository_impl.dart';

import '../../../test_helpers/fake_ipc_client.dart';
import '../../../test_helpers/fixtures.dart';

void main() {
  late FakeIpcClient client;
  late IpcSettingsRepository repository;

  setUp(() {
    client = FakeIpcClient();
    repository = IpcSettingsRepository(client);
  });

  test('get parses the settings response', () async {
    client.handlers['settings.get'] = (_) => {
      'settings': settingsJson(schedulesPaused: true),
    };

    final settings = await repository.get();

    expect(client.sentRequests.single.$1, 'settings.get');
    expect(settings.schedulesPaused, isTrue);
    expect(settings.globalConcurrencyLimit, 3);
  });

  test('set sends the full settings object', () async {
    client.handlers['settings.set'] = (payload) {
      final map = payload! as Map<String, dynamic>;
      expect(
        (map['settings'] as Map<String, dynamic>)['schedules_paused'],
        isTrue,
      );
      return {'settings': settingsJson(schedulesPaused: true)};
    };

    final result = await repository.set(
      SettingsDto.fromJson(settingsJson(schedulesPaused: true)),
    );

    expect(result.schedulesPaused, isTrue);
  });
}
