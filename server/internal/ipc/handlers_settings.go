package ipc

import (
	"context"
	"encoding/json"

	"vpsbackupmanager/internal/domain"
)

func (b *Backend) registerSettingsHandlers(r *Router) {
	r.Handle(CmdSettingsGet, b.handleSettingsGet)
	r.Handle(CmdSettingsSet, b.handleSettingsSet)
	r.Handle(CmdServiceStatus, b.handleServiceStatus)
}

func (b *Backend) handleSettingsGet(ctx context.Context, _ json.RawMessage) (any, error) {
	s, err := b.Settings.Get(ctx)
	if err != nil {
		return nil, err
	}
	return GetSettingsResponse{Settings: settingsToDTO(s)}, nil
}

func (b *Backend) handleSettingsSet(ctx context.Context, raw json.RawMessage) (any, error) {
	var req SetSettingsRequest
	if err := UnmarshalPayload(raw, &req); err != nil {
		return nil, domain.NewCodedError(domain.ErrInvalidConfig, err.Error(), err)
	}
	settings := dtoToSettings(req.Settings)
	if err := settings.Validate(); err != nil {
		return nil, domain.NewCodedError(domain.ErrInvalidConfig, err.Error(), err)
	}
	if err := b.Settings.Update(ctx, settings); err != nil {
		return nil, err
	}
	if b.Events != nil {
		b.Events.Publish(NewEvent(EventSettingsChanged, SettingsChangedEvent{Settings: settingsToDTO(settings)}))
	}
	return GetSettingsResponse{Settings: settingsToDTO(settings)}, nil
}

func (b *Backend) handleServiceStatus(context.Context, json.RawMessage) (any, error) {
	return ServiceStatusResponse{
		Version: b.Version, ProtocolVersion: ProtocolVersion, StartedAt: formatTime(b.StartedAt),
	}, nil
}

// RegisterHandlers wires every command handler onto r.
func (b *Backend) RegisterHandlers(r *Router) {
	b.registerServerHandlers(r)
	b.registerJobHandlers(r)
	b.registerRunHandlers(r)
	b.registerSettingsHandlers(r)
}
