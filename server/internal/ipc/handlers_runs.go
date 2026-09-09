package ipc

import (
	"context"
	"encoding/json"

	"vpsbackupmanager/internal/domain"
	"vpsbackupmanager/internal/store"
)

func (b *Backend) registerRunHandlers(r *Router) {
	r.Handle(CmdRunsStart, b.handleRunsStart)
	r.Handle(CmdRunsCancel, b.handleRunsCancel)
	r.Handle(CmdRunsList, b.handleRunsList)
	r.Handle(CmdRunsGet, b.handleRunsGet)
}

func (b *Backend) handleRunsStart(ctx context.Context, raw json.RawMessage) (any, error) {
	var req RunNowRequest
	if err := UnmarshalPayload(raw, &req); err != nil {
		return nil, domain.NewCodedError(domain.ErrInvalidConfig, err.Error(), err)
	}
	job, err := b.Jobs.Get(ctx, req.JobID)
	if err != nil {
		return nil, err
	}
	server, err := b.Servers.Get(ctx, job.ServerID)
	if err != nil {
		return nil, err
	}
	params, err := b.connectParams(server)
	if err != nil {
		return nil, err
	}

	// The run's own context must outlive this request/response exchange —
	// it is cancelled only via runs.cancel or service shutdown — so it is
	// deliberately not derived from ctx (which ends when this handler
	// returns).
	runCtx, cancel := context.WithCancel(context.WithoutCancel(ctx)) //nolint:contextcheck // see comment above
	run, err := b.Engine.Start(runCtx, job, server, params, job.LocalDestination, domain.TriggerManual)
	if err != nil {
		cancel()
		return nil, err
	}
	if run.Status == domain.RunRunning {
		b.ActiveRuns.Register(run.ID, cancel)
	} else {
		cancel() // SKIPPED: nothing to cancel later
	}
	return RunNowResponse{RunID: run.ID, Status: string(run.Status)}, nil
}

func (b *Backend) handleRunsCancel(_ context.Context, raw json.RawMessage) (any, error) {
	var req CancelRunRequest
	if err := UnmarshalPayload(raw, &req); err != nil {
		return nil, domain.NewCodedError(domain.ErrInvalidConfig, err.Error(), err)
	}
	if !b.ActiveRuns.Cancel(req.RunID) {
		return nil, domain.NewCodedError(domain.ErrNotFound, "run is not currently active", nil)
	}
	return struct{}{}, nil
}

func (b *Backend) handleRunsList(ctx context.Context, raw json.RawMessage) (any, error) {
	var req ListRunsRequest
	if len(raw) > 0 {
		if err := UnmarshalPayload(raw, &req); err != nil {
			return nil, domain.NewCodedError(domain.ErrInvalidConfig, err.Error(), err)
		}
	}
	runs, err := b.Runs.List(ctx, store.RunFilter{JobID: req.JobID, Status: domain.RunStatus(req.Status), Limit: req.Limit})
	if err != nil {
		return nil, err
	}
	dtos := make([]RunDTO, len(runs))
	for i, r := range runs {
		dtos[i] = runToDTO(r)
	}
	return ListRunsResponse{Runs: dtos}, nil
}

func (b *Backend) handleRunsGet(ctx context.Context, raw json.RawMessage) (any, error) {
	var req GetRunRequest
	if err := UnmarshalPayload(raw, &req); err != nil {
		return nil, domain.NewCodedError(domain.ErrInvalidConfig, err.Error(), err)
	}
	run, err := b.Runs.Get(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	steps, err := b.Steps.ListByRun(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	stepDTOs := make([]StepDTO, len(steps))
	for i, s := range steps {
		stepDTOs[i] = stepToDTO(s)
	}
	return GetRunResponse{Run: runToDTO(run), Steps: stepDTOs}, nil
}
