import 'dart:async';

import 'envelope.dart';
import 'pipe_transport.dart';

/// The Named Pipe endpoint the service listens on. Must match
/// server/internal/ipc.PipeName.
const defaultPipeName = r'\\.\pipe\VPSBackupManager';

/// How the client currently stands with respect to the background
/// service — drives the "service unavailable" UI state (desktop-ui
/// specification).
enum IpcConnectionState { connecting, connected, disconnected }

/// A structured IPC failure: a stable code plus a message safe to show
/// as-is, mirroring the service's own error envelopes.
class IpcException implements Exception {
  IpcException(this.code, this.message);
  final String code;
  final String message;

  @override
  String toString() => 'IpcException($code): $message';
}

/// The Named Pipe client: connects, correlates request/response pairs by
/// id, exposes the server-pushed event stream, and reconnects
/// automatically when the connection is lost — the UI never performs
/// backup work itself, only requests it through this client.
class IpcClient {
  IpcClient({
    this.pipeName = defaultPipeName,
    this.reconnectDelay = const Duration(seconds: 3),
  });

  final String pipeName;
  final Duration reconnectDelay;

  PipeTransport? _transport;
  StreamSubscription<String>? _linesSub;
  final Map<String, Completer<Envelope>> _pending = {};
  final StreamController<Envelope> _eventsController =
      StreamController<Envelope>.broadcast();
  final StreamController<IpcConnectionState> _stateController =
      StreamController<IpcConnectionState>.broadcast();
  IpcConnectionState _state = IpcConnectionState.disconnected;
  int _nextId = 0;
  Timer? _reconnectTimer;
  bool _disposed = false;

  /// Events pushed by the service (run started/stepChanged/finished,
  /// settings changed).
  Stream<Envelope> get events => _eventsController.stream;

  /// Connection state changes, for the service-unavailable UI state.
  Stream<IpcConnectionState> get connectionState => _stateController.stream;

  IpcConnectionState get state => _state;

  /// Connects (or starts trying to) and keeps reconnecting until
  /// [dispose] is called.
  Future<void> start() async {
    _disposed = false;
    await _connect();
  }

  /// Forces an immediate reconnect attempt, for a user-initiated "Retry".
  Future<void> retryNow() async {
    _reconnectTimer?.cancel();
    _reconnectTimer = null;
    if (_state != IpcConnectionState.connecting) {
      await _connect();
    }
  }

  Future<void> _connect() async {
    if (_disposed) return;
    _setState(IpcConnectionState.connecting);
    try {
      final transport = await PipeTransport.connect(pipeName);
      _transport = transport;
      _linesSub = transport.lines.listen(
        _onLine,
        onError: _onTransportError,
        onDone: _onTransportDone,
      );
      _setState(IpcConnectionState.connected);
    } on Object {
      _setState(IpcConnectionState.disconnected);
      _scheduleReconnect();
    }
  }

  void _onTransportError(Object error) {
    _failAllPending(error);
    _setState(IpcConnectionState.disconnected);
    _scheduleReconnect();
  }

  void _onTransportDone() {
    _failAllPending(
      IpcException('SERVICE_UNAVAILABLE', 'connection to the service was lost'),
    );
    _setState(IpcConnectionState.disconnected);
    _scheduleReconnect();
  }

  void _scheduleReconnect() {
    if (_disposed || _reconnectTimer != null) return;
    _reconnectTimer = Timer(reconnectDelay, () {
      _reconnectTimer = null;
      unawaited(_connect());
    });
  }

  void _setState(IpcConnectionState s) {
    if (_disposed) return;
    _state = s;
    _stateController.add(s);
  }

  void _onLine(String line) {
    final Envelope env;
    try {
      env = Envelope.fromJsonString(line);
    } on Object {
      return; // malformed line from the wire; ignore rather than crash the client
    }
    if (env.isResponse && env.id != null) {
      _pending.remove(env.id)?.complete(env);
    } else if (env.isEvent) {
      _eventsController.add(env);
    }
  }

  void _failAllPending(Object error) {
    final pending = _pending.values.toList();
    _pending.clear();
    for (final c in pending) {
      if (!c.isCompleted) c.completeError(error);
    }
  }

  /// Sends command with payload and returns the decoded response payload.
  /// Throws [IpcException] on a structured service error, a timeout, or
  /// when the service is currently unreachable.
  Future<Map<String, dynamic>> request(
    String command, [
    Object? payload,
  ]) async {
    final transport = _transport;
    if (transport == null || _state != IpcConnectionState.connected) {
      throw IpcException(
        'SERVICE_UNAVAILABLE',
        'the background service is not reachable',
      );
    }
    final id = 'c${_nextId++}';
    final completer = Completer<Envelope>();
    _pending[id] = completer;

    final env = Envelope.request(id, command, payload);
    try {
      await transport.writeLine(env.toJsonString());
    } on Object catch (e) {
      _pending.remove(id);
      throw IpcException('SERVICE_UNAVAILABLE', 'could not send request: $e');
    }

    final Envelope response;
    try {
      response = await completer.future.timeout(
        const Duration(seconds: 30),
        onTimeout: () => throw IpcException(
          'TIMEOUT',
          'the service did not respond in time',
        ),
      );
    } finally {
      _pending.remove(id);
    }

    if (response.success == false) {
      final err = response.error;
      throw IpcException(
        err?.code ?? 'UNKNOWN',
        err?.message ?? 'request failed',
      );
    }
    return (response.payload as Map<String, dynamic>?) ?? const {};
  }

  Future<void> dispose() async {
    _disposed = true;
    _reconnectTimer?.cancel();
    await _linesSub?.cancel();
    await _transport?.close();
    _failAllPending(IpcException('DISPOSED', 'client disposed'));
    await _eventsController.close();
    await _stateController.close();
  }
}
