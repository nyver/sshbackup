import 'dart:convert';

/// Protocol version this client speaks. Must match
/// server/internal/ipc.ProtocolVersion.
const kProtocolVersion = 1;

/// Mirrors the Go service's Envelope: the single JSON shape every line on
/// the pipe takes (see /protocol/README.md).
class Envelope {
  Envelope({
    required this.type,
    this.id,
    this.protocolVersion,
    this.command,
    this.event,
    this.success,
    this.payload,
    this.error,
  });

  factory Envelope.fromJsonString(String line) {
    final map = jsonDecode(line) as Map<String, dynamic>;
    return Envelope(
      type: map['type'] as String,
      id: map['id'] as String?,
      protocolVersion: map['protocol_version'] as int?,
      command: map['command'] as String?,
      event: map['event'] as String?,
      success: map['success'] as bool?,
      payload: map['payload'],
      error: map['error'] == null
          ? null
          : ErrorPayload.fromJson(map['error'] as Map<String, dynamic>),
    );
  }

  factory Envelope.request(String id, String command, Object? payload) {
    return Envelope(
      type: 'request',
      id: id,
      protocolVersion: kProtocolVersion,
      command: command,
      payload: payload,
    );
  }

  final String type;
  final String? id;
  final int? protocolVersion;
  final String? command;
  final String? event;
  final bool? success;
  final Object? payload;
  final ErrorPayload? error;

  bool get isRequest => type == 'request';
  bool get isResponse => type == 'response';
  bool get isEvent => type == 'event';

  String toJsonString() {
    return jsonEncode({
      'type': type,
      if (id != null) 'id': id,
      if (protocolVersion != null) 'protocol_version': protocolVersion,
      if (command != null) 'command': command,
      if (event != null) 'event': event,
      if (success != null) 'success': success,
      if (payload != null) 'payload': payload,
      if (error != null) 'error': error!.toJson(),
    });
  }
}

/// A stable, machine-readable error code plus a message safe to show
/// as-is, per the ipc-api specification's "errors are structured"
/// requirement.
class ErrorPayload {
  ErrorPayload({required this.code, required this.message});

  factory ErrorPayload.fromJson(Map<String, dynamic> json) {
    return ErrorPayload(
      code: json['code'] as String,
      message: json['message'] as String,
    );
  }

  final String code;
  final String message;

  Map<String, dynamic> toJson() => {'code': code, 'message': message};
}
