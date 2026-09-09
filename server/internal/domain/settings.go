package domain

import "errors"

const (
	// DefaultGlobalConcurrencyLimit is the maximum number of concurrent
	// runs applied on first start.
	DefaultGlobalConcurrencyLimit = 3
	// DefaultStaleRemoteCleanupAgeHours is how old a remote temporary
	// archive must be before it is considered stale.
	DefaultStaleRemoteCleanupAgeHours = 24
)

// Settings is the single global settings row: pause state, concurrency
// limit, and notification/cleanup preferences.
type Settings struct {
	SchedulesPaused bool

	GlobalConcurrencyLimit int

	NotifySuccess bool
	NotifyFailure bool

	StaleRemoteCleanupEnabled  bool
	StaleRemoteCleanupAgeHours int
}

// DefaultSettings returns the settings applied on first start.
func DefaultSettings() Settings {
	return Settings{
		SchedulesPaused:            false,
		GlobalConcurrencyLimit:     DefaultGlobalConcurrencyLimit,
		NotifySuccess:              true,
		NotifyFailure:              true,
		StaleRemoteCleanupEnabled:  false,
		StaleRemoteCleanupAgeHours: DefaultStaleRemoteCleanupAgeHours,
	}
}

// Validate checks that the concurrency limit and cleanup age are positive.
func (s *Settings) Validate() error {
	if s.GlobalConcurrencyLimit < 1 {
		return errors.New("global concurrency limit must be at least 1")
	}
	if s.StaleRemoteCleanupAgeHours < 1 {
		return errors.New("stale remote cleanup age must be at least 1 hour")
	}
	return nil
}
