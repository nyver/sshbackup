import 'package:flutter_test/flutter_test.dart';
import 'package:vps_backup_manager/features/servers/data/servers_repository_impl.dart';
import 'package:vps_backup_manager/features/servers/domain/servers_repository.dart';

import '../../../test_helpers/fake_ipc_client.dart';
import '../../../test_helpers/fixtures.dart';

void main() {
  late FakeIpcClient client;
  late IpcServersRepository repository;

  setUp(() {
    client = FakeIpcClient();
    repository = IpcServersRepository(client);
  });

  test('list sends servers.list and parses the response', () async {
    client.handlers['servers.list'] = (_) => {
      'servers': [serverJson()],
    };

    final servers = await repository.list();

    expect(client.sentRequests.single.$1, 'servers.list');
    expect(servers, hasLength(1));
    expect(servers.single.name, 'prod');
  });

  test('create omits an empty passphrase and optional timeouts', () async {
    client.handlers['servers.create'] = (payload) {
      final map = payload! as Map<String, dynamic>;
      expect(map['name'], 'staging');
      expect(map['auth_type'], 'PRIVATE_KEY');
      expect(map.containsKey('passphrase'), isFalse);
      expect(map.containsKey('connection_timeout_seconds'), isFalse);
      return {'server': serverJson(id: 'new-id', name: 'staging')};
    };

    final result = await repository.create(
      const SaveServerInput(
        name: 'staging',
        host: '203.0.113.20',
        port: 22,
        username: 'deploy',
        authType: 'PRIVATE_KEY',
        privateKeyPath: r'C:\keys\id_ed25519',
      ),
    );

    expect(result.id, 'new-id');
  });

  test('create includes a non-empty passphrase', () async {
    client.handlers['servers.create'] = (payload) {
      final map = payload! as Map<String, dynamic>;
      expect(map['passphrase'], 'hunter2');
      return {'server': serverJson()};
    };

    await repository.create(
      const SaveServerInput(
        name: 'prod',
        host: 'h',
        port: 22,
        username: 'u',
        authType: 'PRIVATE_KEY',
        privateKeyPath: r'C:\keys\id_ed25519',
        passphrase: 'hunter2',
      ),
    );
  });

  test(
    'create sends PASSWORD auth with the password under passphrase',
    () async {
      client.handlers['servers.create'] = (payload) {
        final map = payload! as Map<String, dynamic>;
        expect(map['auth_type'], 'PASSWORD');
        expect(map.containsKey('private_key_path'), isFalse);
        expect(map['passphrase'], 'correct-horse');
        return {
          'server': serverJson()
            ..['auth_type'] = 'PASSWORD'
            ..['has_credential'] = true,
        };
      };

      final result = await repository.create(
        const SaveServerInput(
          name: 'prod',
          host: 'h',
          port: 22,
          username: 'u',
          authType: 'PASSWORD',
          passphrase: 'correct-horse',
        ),
      );

      expect(result.authType, 'PASSWORD');
    },
  );

  test('testConnection surfaces an unverified host key', () async {
    client.handlers['servers.testConnection'] = (payload) {
      expect((payload! as Map<String, dynamic>)['server_id'], 's1');
      return {
        'success': false,
        'host_key_algorithm': 'ssh-ed25519',
        'host_key_fingerprint': 'SHA256:abc123',
      };
    };

    final result = await repository.testConnection('s1');

    expect(result.success, isFalse);
    expect(result.fingerprintChanged, isFalse);
    expect(result.hostKeyFingerprint, 'SHA256:abc123');
  });

  test('delete sends the server id and discards the response', () async {
    client.handlers['servers.delete'] = (payload) {
      expect((payload! as Map<String, dynamic>)['id'], 's1');
      return <String, dynamic>{};
    };

    await repository.delete('s1');
  });
}
