import 'dart:async';

import 'package:vps_backup_manager/core/ipc/envelope.dart';
import 'package:vps_backup_manager/core/ipc/ipc_client.dart';

/// A controllable [IpcClient] double for tests: performs no real pipe I/O,
/// records every request sent, and lets tests script responses per command
/// and push server events on demand.
class FakeIpcClient extends IpcClient {
  final List<(String command, Object? payload)> sentRequests = [];

  /// Maps a command name to a function producing its response payload.
  /// Throw an [IpcException] from a handler to simulate a service error.
  final Map<String, Map<String, dynamic> Function(Object? payload)> handlers =
      {};

  final _eventsController = StreamController<Envelope>.broadcast();
  final _stateController = StreamController<IpcConnectionState>.broadcast();
  IpcConnectionState _fakeState = IpcConnectionState.connected;

  @override
  Stream<Envelope> get events => _eventsController.stream;

  @override
  Stream<IpcConnectionState> get connectionState => _stateController.stream;

  @override
  IpcConnectionState get state => _fakeState;

  void setConnectionState(IpcConnectionState next) {
    _fakeState = next;
    _stateController.add(next);
  }

  void pushEvent(String event, Map<String, dynamic> payload) {
    _eventsController.add(
      Envelope(type: 'event', event: event, payload: payload),
    );
  }

  @override
  Future<void> start() async {}

  @override
  Future<void> retryNow() async {}

  @override
  Future<void> dispose() async {
    await _eventsController.close();
    await _stateController.close();
  }

  @override
  Future<Map<String, dynamic>> request(
    String command, [
    Object? payload,
  ]) async {
    sentRequests.add((command, payload));
    final handler = handlers[command];
    if (handler == null) {
      throw IpcException(
        'UNSUPPORTED_COMMAND',
        'no fake handler registered for $command',
      );
    }
    return handler(payload);
  }
}
