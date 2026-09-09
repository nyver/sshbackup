import 'dart:async';
import 'dart:convert';
import 'dart:ffi';
import 'dart:io' show sleep;
import 'dart:isolate';
import 'dart:typed_data';

import 'package:async/async.dart';
import 'package:ffi/ffi.dart';
import 'package:win32/win32.dart';

/// Thrown when the named pipe cannot be opened at all (the service is not
/// running), or a connected pipe breaks (the service stopped).
class PipeUnavailableException implements Exception {
  PipeUnavailableException(this.message);
  final String message;

  @override
  String toString() => 'PipeUnavailableException: $message';
}

/// A newline-delimited line transport over a Windows named pipe.
///
/// All actual pipe I/O — connect, read, and write — happens in one
/// dedicated background isolate ("the pipe worker"), never the UI
/// isolate. Reads are polled with `PeekNamedPipe` rather than a blocking
/// `ReadFile` call: a synchronous named pipe handle in this Win32/Dart FFI
/// combination serializes a blocking read against a concurrent write from
/// another isolate (empirically reproducible), so a *second* isolate
/// blocked in `ReadFile` on the same handle is deliberately avoided —
/// everything happens on the one worker isolate's event loop instead.
class PipeTransport {
  PipeTransport._(
    this._workerIsolate,
    this._toWorker,
    this._linesController,
    this._readyPort,
  );

  final Isolate _workerIsolate;
  final SendPort _toWorker;
  final StreamController<String> _linesController;
  final ReceivePort _readyPort;
  bool _closed = false;

  /// Decoded lines received from the service, in order.
  Stream<String> get lines => _linesController.stream;

  /// Connects to pipeName, retrying while the service is starting up
  /// (ERROR_PIPE_BUSY) but giving up after timeout.
  static Future<PipeTransport> connect(
    String pipeName, {
    Duration timeout = const Duration(seconds: 5),
  }) async {
    final readyPort = ReceivePort();
    final workerIsolate = await Isolate.spawn(_workerMain, [
      readyPort.sendPort,
      pipeName,
      timeout.inMilliseconds,
    ]);

    final events = StreamQueue(readyPort);
    final first = await events.next;
    if (first is String && first.startsWith('error:')) {
      readyPort.close();
      workerIsolate.kill(priority: Isolate.immediate);
      throw PipeUnavailableException(first.substring('error:'.length));
    }
    final toWorker = first as SendPort;

    // ignore: close_sinks - stored on the returned PipeTransport and closed by its close() method
    final linesController = StreamController<String>.broadcast();
    final transport = PipeTransport._(
      workerIsolate,
      toWorker,
      linesController,
      readyPort,
    );
    unawaited(_pump(events, transport));
    return transport;
  }

  static Future<void> _pump(
    StreamQueue<dynamic> events,
    PipeTransport transport,
  ) async {
    await for (final message in events.rest) {
      transport._onWorkerMessage(message);
    }
  }

  void _onWorkerMessage(dynamic message) {
    if (_closed) return;
    final map = message as Map;
    switch (map['type']) {
      case 'line':
        _linesController.add(map['data'] as String);
      case 'closed':
      case 'error':
        _linesController.addError(
          PipeUnavailableException(
            map['message'] as String? ?? 'connection closed',
          ),
        );
        _linesController.close();
    }
  }

  /// Writes line followed by a newline. Fire-and-forget from the caller's
  /// perspective: the actual WriteFile call happens on the worker
  /// isolate; a failure surfaces as a 'closed'/'error' message on [lines].
  Future<void> writeLine(String line) async {
    if (_closed) {
      throw PipeUnavailableException('transport is closed');
    }
    _toWorker.send({'type': 'write', 'data': line});
  }

  Future<void> close() async {
    if (_closed) return;
    _closed = true;
    _toWorker.send({'type': 'close'});
    // Give the worker a moment to close the handle gracefully, then kill
    // it unconditionally so a lingering Timer/ReceivePort never keeps the
    // process (or a reconnect cycle) alive.
    await Future<void>.delayed(const Duration(milliseconds: 50));
    _workerIsolate.kill(priority: Isolate.immediate);
    _readyPort.close();
    await _linesController.close();
  }

  /// Entry point for the dedicated pipe worker isolate: owns the handle
  /// for its entire lifetime, polls for incoming data with
  /// `PeekNamedPipe` + `ReadFile`, and performs writes on request — all on
  /// this one isolate, so the handle never sees I/O from two isolates at
  /// once.
  static void _workerMain(List<Object?> args) {
    final readyPort = args[0] as SendPort;
    final pipeName = args[1] as String;
    final timeoutMs = args[2] as int;

    final HANDLE handle;
    try {
      handle = _openPipeBlocking(pipeName, Duration(milliseconds: timeoutMs));
    } catch (e) {
      readyPort.send('error:$e');
      return;
    }

    final commandPort = ReceivePort();
    readyPort.send(commandPort.sendPort);

    final pending = BytesBuilder();
    final peekAvail = calloc<Uint32>();
    final readBuf = calloc<Uint8>(_readChunkSize);
    final readN = calloc<Uint32>();
    Timer? pollTimer;

    void stop() {
      pollTimer?.cancel();
      calloc.free(peekAvail);
      calloc.free(readBuf);
      calloc.free(readN);
      CloseHandle(handle);
      commandPort.close();
    }

    void poll(Timer _) {
      final peek = PeekNamedPipe(
        handle,
        nullptr,
        0,
        nullptr,
        peekAvail,
        nullptr,
      );
      if (!peek.value) {
        readyPort.send({
          'type': 'closed',
          'message': 'PeekNamedPipe failed: Win32 error ${peek.error.code}',
        });
        stop();
        return;
      }
      if (peekAvail.value == 0) return;

      final toRead = peekAvail.value < _readChunkSize
          ? peekAvail.value
          : _readChunkSize;
      final read = ReadFile(handle, readBuf, toRead, readN, nullptr);
      if (!read.value || readN.value == 0) {
        readyPort.send({
          'type': 'closed',
          'message': 'ReadFile failed: Win32 error ${read.error.code}',
        });
        stop();
        return;
      }
      pending.add(readBuf.asTypedList(readN.value));
      _emitCompleteLines(pending, readyPort);
    }

    pollTimer = Timer.periodic(const Duration(milliseconds: 30), poll);

    commandPort.listen((message) {
      final map = message as Map;
      switch (map['type']) {
        case 'write':
          _writeLineBlocking(handle, map['data'] as String, readyPort);
        case 'close':
          stop();
      }
    });
  }

  static void _writeLineBlocking(
    HANDLE handle,
    String line,
    SendPort readyPort,
  ) {
    final bytes = utf8.encode('$line\n');
    final buf = calloc<Uint8>(bytes.length);
    final written = calloc<Uint32>();
    try {
      buf.asTypedList(bytes.length).setAll(0, bytes);
      final result = WriteFile(handle, buf, bytes.length, written, nullptr);
      if (!result.value) {
        readyPort.send({
          'type': 'closed',
          'message': 'WriteFile failed: Win32 error ${result.error.code}',
        });
      }
    } finally {
      calloc.free(buf);
      calloc.free(written);
    }
  }

  static HANDLE _openPipeBlocking(String pipeName, Duration timeout) {
    final namePtr = pipeName.toNativeUtf16();
    final deadline = DateTime.now().add(timeout);
    try {
      while (true) {
        final result = CreateFile(
          PCWSTR(namePtr),
          GENERIC_READ | GENERIC_WRITE,
          FILE_SHARE_NONE,
          null,
          OPEN_EXISTING,
          const FILE_FLAGS_AND_ATTRIBUTES(0),
          null,
        );
        if (result.value != INVALID_HANDLE_VALUE) {
          return result.value;
        }
        if (result.error != ERROR_PIPE_BUSY ||
            DateTime.now().isAfter(deadline)) {
          throw PipeUnavailableException(
            'could not open $pipeName: Win32 error ${result.error.code}',
          );
        }
        sleep(const Duration(milliseconds: 100));
      }
    } finally {
      calloc.free(namePtr);
    }
  }

  static void _emitCompleteLines(BytesBuilder pending, SendPort readyPort) {
    final data = pending.toBytes();
    var start = 0;
    for (var i = 0; i < data.length; i++) {
      if (data[i] == 0x0A) {
        final line = utf8.decode(data.sublist(start, i));
        readyPort.send({'type': 'line', 'data': line});
        start = i + 1;
      }
    }
    pending.clear();
    if (start < data.length) {
      pending.add(data.sublist(start));
    }
  }
}

const _readChunkSize = 64 * 1024;
