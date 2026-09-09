package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"vpsbackupmanager/internal/domain"
)

// ServerRepository persists domain.Server records.
type ServerRepository struct {
	db *DB
}

// NewServerRepository constructs a ServerRepository over db.
func NewServerRepository(db *DB) *ServerRepository {
	return &ServerRepository{db: db}
}

// Create inserts a new server record.
func (r *ServerRepository) Create(ctx context.Context, s *domain.Server) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO servers (
			id, name, host, port, username, authentication_type,
			credential_reference, private_key_path, host_key_algorithm, host_key_fingerprint,
			connection_timeout_seconds, command_timeout_seconds, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.ID, s.Name, s.Host, s.Port, s.Username, string(s.AuthType),
		s.CredentialReference, s.PrivateKeyPath, s.HostKeyAlgorithm, s.HostKeyFingerprint,
		s.ConnectionTimeoutSeconds, s.CommandTimeoutSeconds, formatTime(s.CreatedAt), formatTime(s.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("insert server %q: %w", s.ID, err)
	}
	return nil
}

// Update replaces an existing server record in place.
func (r *ServerRepository) Update(ctx context.Context, s *domain.Server) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE servers SET
			name = ?, host = ?, port = ?, username = ?, authentication_type = ?,
			credential_reference = ?, private_key_path = ?, host_key_algorithm = ?, host_key_fingerprint = ?,
			connection_timeout_seconds = ?, command_timeout_seconds = ?, updated_at = ?
		WHERE id = ?`,
		s.Name, s.Host, s.Port, s.Username, string(s.AuthType),
		s.CredentialReference, s.PrivateKeyPath, s.HostKeyAlgorithm, s.HostKeyFingerprint,
		s.ConnectionTimeoutSeconds, s.CommandTimeoutSeconds, formatTime(s.UpdatedAt), s.ID,
	)
	if err != nil {
		return fmt.Errorf("update server %q: %w", s.ID, err)
	}
	return checkRowsAffected(res, "server", s.ID)
}

// Get fetches a server by id.
func (r *ServerRepository) Get(ctx context.Context, id string) (*domain.Server, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, name, host, port, username, authentication_type,
			credential_reference, private_key_path, host_key_algorithm, host_key_fingerprint,
			connection_timeout_seconds, command_timeout_seconds, created_at, updated_at
		FROM servers WHERE id = ?`, id)
	s, err := scanServer(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.NewCodedError(domain.ErrNotFound, fmt.Sprintf("server %q not found", id), nil)
	}
	if err != nil {
		return nil, fmt.Errorf("get server %q: %w", id, err)
	}
	return s, nil
}

// List returns every server ordered by name.
func (r *ServerRepository) List(ctx context.Context) ([]*domain.Server, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, host, port, username, authentication_type,
			credential_reference, private_key_path, host_key_algorithm, host_key_fingerprint,
			connection_timeout_seconds, command_timeout_seconds, created_at, updated_at
		FROM servers ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list servers: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var servers []*domain.Server
	for rows.Next() {
		s, err := scanServer(rows)
		if err != nil {
			return nil, fmt.Errorf("scan server row: %w", err)
		}
		servers = append(servers, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate servers: %w", err)
	}
	return servers, nil
}

// JobsReferencing returns the names of jobs that reference the given
// server, for a friendly refusal message on delete.
func (r *ServerRepository) JobsReferencing(ctx context.Context, serverID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT name FROM backup_jobs WHERE server_id = ? ORDER BY name", serverID)
	if err != nil {
		return nil, fmt.Errorf("list jobs referencing server %q: %w", serverID, err)
	}
	defer func() { _ = rows.Close() }()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan job name: %w", err)
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate jobs referencing server %q: %w", serverID, err)
	}
	return names, nil
}

// Delete removes a server by id. It refuses when jobs still reference it,
// per the server-management specification.
func (r *ServerRepository) Delete(ctx context.Context, id string) error {
	jobs, err := r.JobsReferencing(ctx, id)
	if err != nil {
		return err
	}
	if len(jobs) > 0 {
		return domain.NewCodedError(domain.ErrInvalidConfig,
			fmt.Sprintf("server is referenced by %d job(s): %v", len(jobs), jobs), nil)
	}

	res, err := r.db.ExecContext(ctx, "DELETE FROM servers WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete server %q: %w", id, err)
	}
	return checkRowsAffected(res, "server", id)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanServer(row rowScanner) (*domain.Server, error) {
	var (
		s                    domain.Server
		authType             string
		createdAt, updatedAt string
	)
	if err := row.Scan(
		&s.ID, &s.Name, &s.Host, &s.Port, &s.Username, &authType,
		&s.CredentialReference, &s.PrivateKeyPath, &s.HostKeyAlgorithm, &s.HostKeyFingerprint,
		&s.ConnectionTimeoutSeconds, &s.CommandTimeoutSeconds, &createdAt, &updatedAt,
	); err != nil {
		return nil, err
	}
	s.AuthType = domain.AuthenticationType(authType)

	var err error
	if s.CreatedAt, err = parseTime(createdAt); err != nil {
		return nil, err
	}
	if s.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return nil, err
	}
	return &s, nil
}

func checkRowsAffected(res sql.Result, kind, id string) error {
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("read rows affected: %w", err)
	}
	if n == 0 {
		return domain.NewCodedError(domain.ErrNotFound, fmt.Sprintf("%s %q not found", kind, id), nil)
	}
	return nil
}
