import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'ipc/ipc_client.dart';

/// The single [IpcClient] instance for the app's lifetime. The UI never
/// performs backup work itself — every screen goes through this client
/// (desktop-ui specification: "UI contains no backup logic").
final ipcClientProvider = Provider<IpcClient>((ref) {
  final client = IpcClient();
  unawaited(client.start());
  ref.onDispose(() => unawaited(client.dispose()));
  return client;
});

/// The client's current standing with the background service, seeded with
/// its value at subscription time so the service-unavailable state (ipc-api
/// specification) is correct immediately rather than after the first event.
final connectionStateProvider = StreamProvider<IpcConnectionState>((ref) {
  final client = ref.watch(ipcClientProvider);
  return Stream.multi((controller) {
    controller.add(client.state);
    final sub = client.connectionState.listen(controller.add);
    controller.onCancel = sub.cancel;
  });
});
