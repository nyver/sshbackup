import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/ipc/models.dart';
import '../../../core/ipc_providers.dart';
import '../domain/servers_repository.dart';
import 'servers_repository_impl.dart';

final serversRepositoryProvider = Provider<ServersRepository>((ref) {
  return IpcServersRepository(ref.watch(ipcClientProvider));
});

/// The current server list, refetched after any create/update/delete.
/// There is no `servers.changed` push event (unlike runs/settings), so
/// mutations refresh explicitly rather than reacting to a stream.
final serversListProvider =
    AsyncNotifierProvider<ServersListNotifier, List<ServerDto>>(
      ServersListNotifier.new,
    );

class ServersListNotifier extends AsyncNotifier<List<ServerDto>> {
  @override
  Future<List<ServerDto>> build() {
    return ref.watch(serversRepositoryProvider).list();
  }

  Future<void> refresh() async {
    state = await AsyncValue.guard(
      () => ref.read(serversRepositoryProvider).list(),
    );
  }

  Future<ServerDto> create(SaveServerInput input) async {
    final server = await ref.read(serversRepositoryProvider).create(input);
    await refresh();
    return server;
  }

  Future<ServerDto> updateServer(String id, SaveServerInput input) async {
    final server = await ref.read(serversRepositoryProvider).update(id, input);
    await refresh();
    return server;
  }

  Future<void> delete(String id) async {
    await ref.read(serversRepositoryProvider).delete(id);
    await refresh();
  }

  Future<ServerDto> confirmHostKey({
    required String serverId,
    required String fingerprint,
    required String algorithm,
  }) async {
    final server = await ref
        .read(serversRepositoryProvider)
        .confirmHostKey(
          serverId: serverId,
          fingerprint: fingerprint,
          algorithm: algorithm,
        );
    await refresh();
    return server;
  }
}
