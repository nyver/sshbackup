package store

import (
	"context"
	"fmt"

	"vpsbackupmanager/internal/domain"
)

// SettingsRepository persists the single global domain.Settings row.
type SettingsRepository struct {
	db *DB
}

// NewSettingsRepository constructs a SettingsRepository over db.
func NewSettingsRepository(db *DB) *SettingsRepository {
	return &SettingsRepository{db: db}
}

// Get returns the global settings row, seeded by migration 0001.
func (r *SettingsRepository) Get(ctx context.Context) (domain.Settings, error) {
	var (
		s                                                             domain.Settings
		schedulesPaused, notifySuccess, notifyFailure, cleanupEnabled int64
	)
	err := r.db.QueryRowContext(ctx, `
		SELECT schedules_paused, global_concurrency_limit, notify_success, notify_failure,
			stale_remote_cleanup_enabled, stale_remote_cleanup_age_hours
		FROM settings WHERE id = 1`).Scan(
		&schedulesPaused, &s.GlobalConcurrencyLimit, &notifySuccess, &notifyFailure,
		&cleanupEnabled, &s.StaleRemoteCleanupAgeHours,
	)
	if err != nil {
		return domain.Settings{}, fmt.Errorf("get settings: %w", err)
	}
	s.SchedulesPaused = intToBool(schedulesPaused)
	s.NotifySuccess = intToBool(notifySuccess)
	s.NotifyFailure = intToBool(notifyFailure)
	s.StaleRemoteCleanupEnabled = intToBool(cleanupEnabled)
	return s, nil
}

// Update replaces the global settings row.
func (r *SettingsRepository) Update(ctx context.Context, s domain.Settings) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE settings SET
			schedules_paused = ?, global_concurrency_limit = ?, notify_success = ?, notify_failure = ?,
			stale_remote_cleanup_enabled = ?, stale_remote_cleanup_age_hours = ?
		WHERE id = 1`,
		boolToInt(s.SchedulesPaused), s.GlobalConcurrencyLimit, boolToInt(s.NotifySuccess), boolToInt(s.NotifyFailure),
		boolToInt(s.StaleRemoteCleanupEnabled), s.StaleRemoteCleanupAgeHours,
	)
	if err != nil {
		return fmt.Errorf("update settings: %w", err)
	}
	return nil
}
