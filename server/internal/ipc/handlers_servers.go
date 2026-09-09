package ipc

import (
	"context"
	"encoding/json"
	"time"

	"vpsbackupmanager/internal/domain"
	"vpsbackupmanager/internal/remote"
)

func (b *Backend) registerServerHandlers(r *Router) {
	r.Handle(CmdServersList, b.handleServersList)
	r.Handle(CmdServersCreate, b.handleServersCreate)
	r.Handle(CmdServersUpdate, b.handleServersUpdate)
	r.Handle(CmdServersDelete, b.handleServersDelete)
	r.Handle(CmdServersTestConnection, b.handleTestConnection)
	r.Handle(CmdServersConfirmHostKey, b.handleConfirmHostKey)
}

func (b *Backend) handleServersList(ctx context.Context, _ json.RawMessage) (any, error) {
	servers, err := b.Servers.List(ctx)
	if err != nil {
		return nil, err
	}
	dtos := make([]ServerDTO, len(servers))
	for i, s := range servers {
		dtos[i] = serverToDTO(s)
	}
	return ListServersResponse{Servers: dtos}, nil
}

func (b *Backend) handleServersCreate(ctx context.Context, raw json.RawMessage) (any, error) {
	var req SaveServerRequest
	if err := UnmarshalPayload(raw, &req); err != nil {
		return nil, domain.NewCodedError(domain.ErrInvalidConfig, err.Error(), err)
	}

	credRef, err := b.storeCredential(req.Passphrase, "")
	if err != nil {
		return nil, err
	}
	s, err := domain.NewServer(time.Now(), domain.NewServerParams{
		Name: req.Name, Host: req.Host, Port: req.Port, Username: req.Username,
		AuthType: domain.AuthenticationType(req.AuthType), CredentialReference: credRef,
		PrivateKeyPath:           req.PrivateKeyPath,
		ConnectionTimeoutSeconds: req.ConnectionTimeoutSeconds, CommandTimeoutSeconds: req.CommandTimeoutSeconds,
	})
	if err != nil {
		return nil, domain.NewCodedError(domain.ErrInvalidConfig, err.Error(), err)
	}
	if err := b.Servers.Create(ctx, s); err != nil {
		return nil, err
	}
	return SaveServerResponse{Server: serverToDTO(s)}, nil
}

func (b *Backend) handleServersUpdate(ctx context.Context, raw json.RawMessage) (any, error) {
	var req SaveServerRequest
	if err := UnmarshalPayload(raw, &req); err != nil {
		return nil, domain.NewCodedError(domain.ErrInvalidConfig, err.Error(), err)
	}
	existing, err := b.Servers.Get(ctx, req.ID)
	if err != nil {
		return nil, err
	}

	credRef, err := b.storeCredential(req.Passphrase, existing.CredentialReference)
	if err != nil {
		return nil, err
	}

	existing.Name, existing.Host, existing.Port, existing.Username = req.Name, req.Host, req.Port, req.Username
	existing.AuthType = domain.AuthenticationType(req.AuthType)
	existing.CredentialReference = credRef
	if req.PrivateKeyPath != "" {
		existing.PrivateKeyPath = req.PrivateKeyPath
	}
	if req.ConnectionTimeoutSeconds > 0 {
		existing.ConnectionTimeoutSeconds = req.ConnectionTimeoutSeconds
	}
	if req.CommandTimeoutSeconds > 0 {
		existing.CommandTimeoutSeconds = req.CommandTimeoutSeconds
	}
	existing.UpdatedAt = time.Now()
	if err := existing.Validate(); err != nil {
		return nil, domain.NewCodedError(domain.ErrInvalidConfig, err.Error(), err)
	}
	if err := b.Servers.Update(ctx, existing); err != nil {
		return nil, err
	}
	return SaveServerResponse{Server: serverToDTO(existing)}, nil
}

// storeCredential saves passphrase under a new or existing reference. An
// empty passphrase with no existing reference means "no credential";
// an empty passphrase with an existing reference leaves it untouched
// (the UI never re-sends an unchanged passphrase).
func (b *Backend) storeCredential(passphrase, existingRef string) (string, error) {
	if passphrase == "" {
		return existingRef, nil
	}
	if existingRef != "" {
		if err := b.Secrets.Replace(existingRef, []byte(passphrase)); err != nil {
			return "", err
		}
		return existingRef, nil
	}
	return b.Secrets.Save([]byte(passphrase))
}

func (b *Backend) handleServersDelete(ctx context.Context, raw json.RawMessage) (any, error) {
	var req DeleteServerRequest
	if err := UnmarshalPayload(raw, &req); err != nil {
		return nil, domain.NewCodedError(domain.ErrInvalidConfig, err.Error(), err)
	}
	server, err := b.Servers.Get(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	if err := b.Servers.Delete(ctx, req.ID); err != nil {
		return nil, err
	}
	if server.CredentialReference != "" {
		if err := b.Secrets.Delete(server.CredentialReference); err != nil {
			return nil, err
		}
	}
	return struct{}{}, nil
}

func (b *Backend) handleTestConnection(ctx context.Context, raw json.RawMessage) (any, error) {
	var req TestConnectionRequest
	if err := UnmarshalPayload(raw, &req); err != nil {
		return nil, domain.NewCodedError(domain.ErrInvalidConfig, err.Error(), err)
	}
	server, err := b.Servers.Get(ctx, req.ServerID)
	if err != nil {
		return nil, err
	}

	params, err := b.connectionParams(server, server.HostKeyFingerprint)
	if err != nil {
		return nil, err
	}
	result := remote.TestConnection(ctx, params)

	resp := TestConnectionResponse{
		Success: result.Success, RemoteOSInfo: result.RemoteOSInfo, SSHServerVersion: result.SSHServerVersion,
	}
	if result.FailedStage != "" {
		resp.FailedStage = string(result.FailedStage)
	}
	if result.Err != nil {
		resp.Message = result.Err.Error()
	}
	if result.PresentedHostKey != nil {
		resp.HostKeyAlgorithm = result.PresentedHostKey.Algorithm
		resp.HostKeyFingerprint = result.PresentedHostKey.Fingerprint
		resp.FingerprintChanged = server.HasTrustedHostKey() && result.PresentedHostKey.Fingerprint != server.HostKeyFingerprint
	}
	return resp, nil
}

func (b *Backend) handleConfirmHostKey(ctx context.Context, raw json.RawMessage) (any, error) {
	var req ConfirmHostKeyRequest
	if err := UnmarshalPayload(raw, &req); err != nil {
		return nil, domain.NewCodedError(domain.ErrInvalidConfig, err.Error(), err)
	}
	server, err := b.Servers.Get(ctx, req.ServerID)
	if err != nil {
		return nil, err
	}

	// Re-verify against the confirmed fingerprint in one more connection
	// attempt, so a confirmation can never be recorded without actually
	// having seen that exact key.
	params, err := b.connectionParams(server, req.Fingerprint)
	if err != nil {
		return nil, err
	}
	result := remote.TestConnection(ctx, params)
	if result.PresentedHostKey == nil || result.PresentedHostKey.Fingerprint != req.Fingerprint {
		return nil, domain.NewCodedError(domain.ErrSSHHostKeyChanged,
			"the presented host key no longer matches the fingerprint being confirmed", nil)
	}

	server.HostKeyFingerprint = req.Fingerprint
	server.HostKeyAlgorithm = req.Algorithm
	server.UpdatedAt = time.Now()
	if err := b.Servers.Update(ctx, server); err != nil {
		return nil, err
	}
	return SaveServerResponse{Server: serverToDTO(server)}, nil
}
