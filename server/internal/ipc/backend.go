package ipc

import (
	"context"
	"fmt"
	"os"
	"time"

	"vpsbackupmanager/internal/backup"
	"vpsbackupmanager/internal/domain"
	"vpsbackupmanager/internal/remote"
	"vpsbackupmanager/internal/scheduler"
	"vpsbackupmanager/internal/store"
)

// ServerStore is the persistence surface handlers need for servers.
type ServerStore interface {
	Create(ctx context.Context, s *domain.Server) error
	Update(ctx context.Context, s *domain.Server) error
	Get(ctx context.Context, id string) (*domain.Server, error)
	List(ctx context.Context) ([]*domain.Server, error)
	Delete(ctx context.Context, id string) error
}

// JobStore is the persistence surface handlers need for jobs.
type JobStore interface {
	Create(ctx context.Context, j *domain.Job) error
	Update(ctx context.Context, j *domain.Job) error
	SetEnabled(ctx context.Context, id string, enabled bool) error
	Get(ctx context.Context, id string) (*domain.Job, error)
	List(ctx context.Context) ([]*domain.Job, error)
	Delete(ctx context.Context, id string, deleteHistory bool) error
}

// RunStore is the persistence surface handlers need for runs.
type RunStore interface {
	Get(ctx context.Context, id string) (*domain.Run, error)
	List(ctx context.Context, filter store.RunFilter) ([]*domain.Run, error)
}

// StepStore is the persistence surface handlers need for steps.
type StepStore interface {
	ListByRun(ctx context.Context, runID string) ([]*domain.RunStep, error)
}

// SettingsStore is the persistence surface handlers need for settings.
type SettingsStore interface {
	Get(ctx context.Context) (domain.Settings, error)
	Update(ctx context.Context, s domain.Settings) error
}

// SecretsStore resolves a credential_reference to its plaintext passphrase.
type SecretsStore interface {
	Save(plaintext []byte) (string, error)
	Replace(reference string, plaintext []byte) error
	Delete(reference string) error
	Load(reference string) ([]byte, error)
}

// Backend holds every dependency the IPC handlers need. It is the single
// place that turns store/backup/remote/secrets operations into the
// command handlers registered on a Router.
type Backend struct {
	Servers  ServerStore
	Jobs     JobStore
	Runs     RunStore
	Steps    StepStore
	Settings SettingsStore
	Secrets  SecretsStore

	Engine     *backup.Engine
	Validator  *backup.Validator
	ActiveRuns *ActiveRunRegistry
	Events     *EventHub

	Clock scheduler.Clock

	Version   string
	StartedAt time.Time
}

func (b *Backend) clock() scheduler.Clock {
	if b.Clock != nil {
		return b.Clock
	}
	return scheduler.SystemClock{}
}

// ResolveCredentials resolves whatever a server's AuthType needs: for
// PRIVATE_KEY, the key file plus its decrypted passphrase (if any); for
// PASSWORD, just the decrypted password (privateKeyPEM is nil). It never
// logs or returns the plaintext beyond the caller's immediate use.
//
// Exported so callers outside this package (the scheduler's job runner)
// resolve credentials the same way manual runs do, rather than duplicating
// (and risking diverging from) the AuthType branch.
func ResolveCredentials(secretsStore SecretsStore, server *domain.Server) (privateKeyPEM []byte, secret string, err error) {
	if server.CredentialReference != "" {
		secretBytes, loadErr := secretsStore.Load(server.CredentialReference)
		if loadErr != nil {
			return nil, "", loadErr
		}
		secret = string(secretBytes)
	}

	if server.AuthType == domain.AuthPassword {
		return nil, secret, nil
	}

	privateKeyPEM, err = os.ReadFile(server.PrivateKeyPath) //nolint:gosec // path is an admin-configured server setting, not external input
	if err != nil {
		return nil, "", domain.NewCodedError(domain.ErrSSHAuthFailed,
			fmt.Sprintf("could not read private key %q", server.PrivateKeyPath), err)
	}
	return privateKeyPEM, secret, nil
}

func (b *Backend) connectParams(server *domain.Server) (backup.ConnectParams, error) {
	key, secret, err := ResolveCredentials(b.Secrets, server)
	if err != nil {
		return backup.ConnectParams{}, err
	}
	if server.AuthType == domain.AuthPassword {
		return backup.ConnectParams{Server: server, Password: secret}, nil
	}
	return backup.ConnectParams{Server: server, PrivateKeyPEM: key, Passphrase: secret}, nil
}

func (b *Backend) connectionParams(server *domain.Server, trustedFingerprint string) (remote.ConnectionParams, error) {
	key, secret, err := ResolveCredentials(b.Secrets, server)
	if err != nil {
		return remote.ConnectionParams{}, err
	}
	params := remote.ConnectionParams{
		Host: server.Host, Port: server.Port, Username: server.Username,
		AuthType: server.AuthType, TrustedFingerprint: trustedFingerprint,
		ConnectTimeout: time.Duration(server.ConnectionTimeoutSeconds) * time.Second,
		CommandTimeout: time.Duration(server.CommandTimeoutSeconds) * time.Second,
	}
	if server.AuthType == domain.AuthPassword {
		params.Password = secret
	} else {
		params.PrivateKeyPEM = key
		params.Passphrase = secret
	}
	return params, nil
}

func (b *Backend) nextRunAt(job *domain.Job) string {
	next, ok, err := scheduler.NextRun(job.Schedule, b.clock().Now())
	if err != nil || !ok {
		return ""
	}
	return formatTime(next)
}
