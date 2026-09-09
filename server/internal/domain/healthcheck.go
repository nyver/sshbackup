package domain

import "errors"

// HealthCheck is an optional post-backup verification: a remote shell
// command retried up to Attempts times, Interval apart. Its failure never
// invalidates an already-verified archive.
type HealthCheck struct {
	Command         string
	Attempts        int
	IntervalSeconds int
}

// Validate checks the invariants required by the backup-execution
// specification.
func (h *HealthCheck) Validate() error {
	var errs []error
	if h.Command == "" {
		errs = append(errs, errors.New("health check command must not be empty"))
	}
	if h.Attempts < 1 {
		errs = append(errs, errors.New("health check attempts must be at least 1"))
	}
	if h.IntervalSeconds < 1 {
		errs = append(errs, errors.New("health check interval must be at least 1 second"))
	}
	return errors.Join(errs...)
}
