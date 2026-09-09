import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/ipc/envelope.dart';
import '../../../core/ipc/models.dart';
import '../../../core/ipc_providers.dart';
import '../domain/run_events.dart';

/// Typed run events parsed from the service's raw event stream (ipc-api
/// specification, "Live event stream").
final runEventsProvider = StreamProvider<RunEvent>((ref) {
  final client = ref.watch(ipcClientProvider);
  return client.events
      .map(parseRunEvent)
      .where((event) => event != null)
      .cast<RunEvent>();
});

/// Parses one raw envelope into a [RunEvent], or null if it is not a run
/// event (e.g. `settings.changed`).
RunEvent? parseRunEvent(Envelope env) {
  final payload = env.payload;
  if (payload is! Map<String, dynamic>) return null;
  return switch (env.event) {
    'run.started' => RunStartedEvent(
      RunDto.fromJson(payload['run'] as Map<String, dynamic>),
    ),
    'run.stepChanged' => RunStepChangedEvent(
      payload['run_id'] as String,
      StepDto.fromJson(payload['step'] as Map<String, dynamic>),
    ),
    'run.finished' => RunFinishedEvent(
      RunDto.fromJson(payload['run'] as Map<String, dynamic>),
    ),
    _ => null,
  };
}
