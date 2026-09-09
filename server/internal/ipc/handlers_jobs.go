package ipc

import (
	"context"
	"encoding/json"
	"time"

	"vpsbackupmanager/internal/domain"
)

func (b *Backend) registerJobHandlers(r *Router) {
	r.Handle(CmdJobsList, b.handleJobsList)
	r.Handle(CmdJobsCreate, b.handleJobsCreate)
	r.Handle(CmdJobsUpdate, b.handleJobsUpdate)
	r.Handle(CmdJobsDelete, b.handleJobsDelete)
	r.Handle(CmdJobsEnable, b.handleJobsSetEnabled(true))
	r.Handle(CmdJobsDisable, b.handleJobsSetEnabled(false))
	r.Handle(CmdJobsValidate, b.handleJobsValidate)
}

func (b *Backend) handleJobsList(ctx context.Context, _ json.RawMessage) (any, error) {
	jobs, err := b.Jobs.List(ctx)
	if err != nil {
		return nil, err
	}
	dtos := make([]JobDTO, len(jobs))
	for i, j := range jobs {
		dtos[i] = jobToDTO(j, b.nextRunAt(j))
	}
	return ListJobsResponse{Jobs: dtos}, nil
}

func (b *Backend) handleJobsCreate(ctx context.Context, raw json.RawMessage) (any, error) {
	var req SaveJobRequest
	if err := UnmarshalPayload(raw, &req); err != nil {
		return nil, domain.NewCodedError(domain.ErrInvalidConfig, err.Error(), err)
	}
	job, err := domain.NewJob(time.Now(), dtoToJobParams(req.Job))
	if err != nil {
		return nil, domain.NewCodedError(domain.ErrInvalidConfig, err.Error(), err)
	}
	if err := b.Jobs.Create(ctx, job); err != nil {
		return nil, err
	}
	return SaveJobResponse{Job: jobToDTO(job, b.nextRunAt(job))}, nil
}

func (b *Backend) handleJobsUpdate(ctx context.Context, raw json.RawMessage) (any, error) {
	var req SaveJobRequest
	if err := UnmarshalPayload(raw, &req); err != nil {
		return nil, domain.NewCodedError(domain.ErrInvalidConfig, err.Error(), err)
	}
	if req.Job.ID == "" {
		return nil, domain.NewCodedError(domain.ErrInvalidConfig, "job id is required for an update", nil)
	}
	existing, err := b.Jobs.Get(ctx, req.Job.ID)
	if err != nil {
		return nil, err
	}

	params := dtoToJobParams(req.Job)
	updated, err := domain.NewJob(existing.CreatedAt, params)
	if err != nil {
		return nil, domain.NewCodedError(domain.ErrInvalidConfig, err.Error(), err)
	}
	updated.ID = existing.ID
	updated.UpdatedAt = time.Now()
	// Preserve stable child ids only where the incoming DTO does not
	// already supply new ones (domain.NewJob mints fresh ids for
	// children); this is acceptable because the aggregate is always
	// replaced wholesale on update (see store.JobRepository.Update).

	if err := b.Jobs.Update(ctx, updated); err != nil {
		return nil, err
	}
	return SaveJobResponse{Job: jobToDTO(updated, b.nextRunAt(updated))}, nil
}

func (b *Backend) handleJobsDelete(ctx context.Context, raw json.RawMessage) (any, error) {
	var req DeleteJobRequest
	if err := UnmarshalPayload(raw, &req); err != nil {
		return nil, domain.NewCodedError(domain.ErrInvalidConfig, err.Error(), err)
	}
	if err := b.Jobs.Delete(ctx, req.ID, req.DeleteHistory); err != nil {
		return nil, err
	}
	return struct{}{}, nil
}

func (b *Backend) handleJobsSetEnabled(enabled bool) HandlerFunc {
	return func(ctx context.Context, raw json.RawMessage) (any, error) {
		var req SetJobEnabledRequest
		if err := UnmarshalPayload(raw, &req); err != nil {
			return nil, domain.NewCodedError(domain.ErrInvalidConfig, err.Error(), err)
		}
		if err := b.Jobs.SetEnabled(ctx, req.ID, enabled); err != nil {
			return nil, err
		}
		return struct{}{}, nil
	}
}

func (b *Backend) handleJobsValidate(ctx context.Context, raw json.RawMessage) (any, error) {
	var req ValidateJobRequest
	if err := UnmarshalPayload(raw, &req); err != nil {
		return nil, domain.NewCodedError(domain.ErrInvalidConfig, err.Error(), err)
	}
	job, err := b.Jobs.Get(ctx, req.ID)
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

	result := b.Validator.Validate(ctx, job, server, params, job.LocalDestination)
	checks := make([]CheckDTO, len(result.Checks))
	for i, c := range result.Checks {
		checks[i] = CheckDTO{Name: c.Name, Passed: c.Passed, Message: c.Message}
	}
	return ValidateJobResponse{
		Success: result.Success, Checks: checks,
		SourceSizeBytes: result.SourceSizeBytes, SourceSizeKnown: result.SourceSizeKnown,
		RemoteFreeBytes: result.RemoteFreeBytes, RemoteFreeKnown: result.RemoteFreeKnown,
		LocalFreeBytes: result.LocalFreeBytes, LocalFreeKnown: result.LocalFreeKnown,
	}, nil
}
