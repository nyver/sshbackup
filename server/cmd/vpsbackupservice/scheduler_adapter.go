package main

import (
	"context"

	"vpsbackupmanager/internal/backup"
	"vpsbackupmanager/internal/domain"
	"vpsbackupmanager/internal/ipc"
	"vpsbackupmanager/internal/secrets"
	"vpsbackupmanager/internal/store"
)

// schedulerRunner adapts backup.Engine to scheduler.Runner: it resolves
// the job's private key and passphrase, then runs the job to completion
// (scheduler.Dispatcher already calls Runner.Run from its own worker
// goroutine, so a blocking call here is correct).
type schedulerRunner struct {
	engine       *backup.Engine
	servers      *store.ServerRepository
	secretsStore *secrets.Store
}

func (r *schedulerRunner) Run(ctx context.Context, job *domain.Job, server *domain.Server, trigger domain.Trigger) (*domain.Run, error) {
	keyPEM, secret, err := ipc.ResolveCredentials(r.secretsStore, server)
	if err != nil {
		return nil, err
	}
	connectParams := backup.ConnectParams{Server: server, PrivateKeyPEM: keyPEM, Passphrase: secret}
	if server.AuthType == domain.AuthPassword {
		connectParams = backup.ConnectParams{Server: server, Password: secret}
	}
	return r.engine.Run(ctx, job, server, connectParams, job.LocalDestination, trigger)
}

// schedRunStore adapts store.RunRepository to scheduler.RunStore.
type schedRunStore struct {
	runs *store.RunRepository
}

func (s *schedRunStore) LastRun(ctx context.Context, jobID string) (*domain.Run, error) {
	list, err := s.runs.List(ctx, store.RunFilter{JobID: jobID, Limit: 1})
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, nil //nolint:nilnil // absence of a prior run is a valid, non-error outcome
	}
	return list[0], nil
}

func (s *schedRunStore) Create(ctx context.Context, run *domain.Run) error {
	return s.runs.Create(ctx, run)
}
