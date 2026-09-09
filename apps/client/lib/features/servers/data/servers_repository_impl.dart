import '../../../core/ipc/ipc_client.dart';
import '../../../core/ipc/models.dart';
import '../domain/servers_repository.dart';

class IpcServersRepository implements ServersRepository {
  IpcServersRepository(this._client);

  final IpcClient _client;

  @override
  Future<List<ServerDto>> list() async {
    final res = await _client.request('servers.list');
    return (res['servers'] as List)
        .map((e) => ServerDto.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  @override
  Future<ServerDto> create(SaveServerInput input) async {
    final res = await _client.request('servers.create', _payload(input));
    return ServerDto.fromJson(res['server'] as Map<String, dynamic>);
  }

  @override
  Future<ServerDto> update(String id, SaveServerInput input) async {
    final res = await _client.request('servers.update', {
      ..._payload(input),
      'id': id,
    });
    return ServerDto.fromJson(res['server'] as Map<String, dynamic>);
  }

  @override
  Future<void> delete(String id) async {
    await _client.request('servers.delete', {'id': id});
  }

  @override
  Future<TestConnectionResult> testConnection(String serverId) async {
    final res = await _client.request('servers.testConnection', {
      'server_id': serverId,
    });
    return TestConnectionResult.fromJson(res);
  }

  @override
  Future<ServerDto> confirmHostKey({
    required String serverId,
    required String fingerprint,
    required String algorithm,
  }) async {
    final res = await _client.request('servers.confirmHostKey', {
      'server_id': serverId,
      'fingerprint': fingerprint,
      'algorithm': algorithm,
    });
    return ServerDto.fromJson(res['server'] as Map<String, dynamic>);
  }

  Map<String, dynamic> _payload(SaveServerInput input) => {
    'name': input.name,
    'host': input.host,
    'port': input.port,
    'username': input.username,
    'auth_type': input.authType,
    if (input.privateKeyPath.isNotEmpty)
      'private_key_path': input.privateKeyPath,
    if (input.passphrase.isNotEmpty) 'passphrase': input.passphrase,
    if (input.connectionTimeoutSeconds > 0)
      'connection_timeout_seconds': input.connectionTimeoutSeconds,
    if (input.commandTimeoutSeconds > 0)
      'command_timeout_seconds': input.commandTimeoutSeconds,
  };
}
