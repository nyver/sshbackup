package main

import (
	"context"
	"fmt"
	"os"

	"vpsbackupmanager/internal/backup"
	"vpsbackupmanager/internal/domain"
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
	keyPEM, err := os.ReadFile(server.PrivateKeyPath) //nolint:gosec // path is an admin-configured server setting, not external input
	if err != nil {
		return nil, fmt.Errorf("read private key %q: %w", server.PrivateKeyPath, err)
	}
	var passphrase string
	if server.CredentialReference != "" {
		secretBytes, loadErr := r.secretsStore.Load(server.CredentialReference)
		if loadErr != nil {
			return nil, loadErr
		}
		passphrase = string(secretBytes)
	}
	return r.engine.Run(ctx, job, server, backup.ConnectParams{Server: server, PrivateKeyPEM: keyPEM, Passphrase: passphrase}, job.LocalDestination, trigger)
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
