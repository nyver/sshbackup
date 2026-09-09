package ipc

import "vpsbackupmanager/internal/domain"

// engineEventSink adapts backup.EventSink to publish through an EventHub,
// translating domain types to wire DTOs, and unregisters a finished run
// from ActiveRuns so its cancel function is not kept around forever.
type engineEventSink struct {
	hub        *EventHub
	activeRuns *ActiveRunRegistry
}

// NewEngineEventSink returns a backup.EventSink (satisfied structurally,
// so this package need not import internal/backup just for the
// interface) that publishes every run lifecycle event to hub and
// unregisters each run from activeRuns once it finishes.
func NewEngineEventSink(hub *EventHub, activeRuns *ActiveRunRegistry) *engineEventSink { //nolint:revive // returned only to be assigned to backup.EventSink; the concrete type need not be exported
	return &engineEventSink{hub: hub, activeRuns: activeRuns}
}

func (s *engineEventSink) RunStarted(run *domain.Run) {
	s.hub.Publish(NewEvent(EventRunStarted, RunStartedEvent{Run: runToDTO(run)}))
}

func (s *engineEventSink) StepChanged(runID string, step *domain.RunStep) {
	s.hub.Publish(NewEvent(EventRunStepChanged, RunStepChangedEvent{RunID: runID, Step: stepToDTO(step)}))
}

func (s *engineEventSink) RunFinished(run *domain.Run) {
	s.hub.Publish(NewEvent(EventRunFinished, RunFinishedEvent{Run: runToDTO(run)}))
	if s.activeRuns != nil {
		s.activeRuns.Unregister(run.ID)
	}
}
