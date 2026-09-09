import '../../../core/ipc/models.dart';

/// Input for `servers.create` / `servers.update`. Passphrase, when
/// non-empty, replaces any existing credential; leaving it empty on update
/// keeps the credential already stored (write-only — never echoed back by
/// the service, per the credential-storage specification).
class SaveServerInput {
  const SaveServerInput({
    required this.name,
    required this.host,
    required this.port,
    required this.username,
    required this.authType,
    this.privateKeyPath = '',
    this.passphrase = '',
    this.connectionTimeoutSeconds = 0,
    this.commandTimeoutSeconds = 0,
  });

  final String name;
  final String host;
  final int port;
  final String username;
  final String authType;
  final String privateKeyPath;
  final String passphrase;
  final int connectionTimeoutSeconds;
  final int commandTimeoutSeconds;
}

/// Consumer-side port for the `servers.*` IPC commands (server-management
/// specification). Notifiers and widgets depend on this interface rather
/// than on `IpcClient` directly, so they stay testable with a fake.
abstract class ServersRepository {
  Future<List<ServerDto>> list();
  Future<ServerDto> create(SaveServerInput input);
  Future<ServerDto> update(String id, SaveServerInput input);
  Future<void> delete(String id);
  Future<TestConnectionResult> testConnection(String serverId);
  Future<ServerDto> confirmHostKey({
    required String serverId,
    required String fingerprint,
    required String algorithm,
  });
}
