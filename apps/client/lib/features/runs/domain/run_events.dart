import '../../../core/ipc/models.dart';

/// Typed events pushed by the service over the run event stream (ipc-api
/// specification, "Live event stream"). `run.logLine` has no case here: the
/// MVP service does not emit it (see protocol/README.md) — step output
/// arrives via [RunStepChangedEvent]'s step payload once a step finishes.
sealed class RunEvent {
  const RunEvent();
}

class RunStartedEvent extends RunEvent {
  const RunStartedEvent(this.run);
  final RunDto run;
}

class RunStepChangedEvent extends RunEvent {
  const RunStepChangedEvent(this.runId, this.step);
  final String runId;
  final StepDto step;
}

class RunFinishedEvent extends RunEvent {
  const RunFinishedEvent(this.run);
  final RunDto run;
}
