package domain

import "fmt"

// RetentionPolicy bounds how many local archives a job keeps. A nil field
// means that limit is not applied; when both are set, an archive is deleted
// only once it falls outside both.
type RetentionPolicy struct {
	ID         string
	JobID      string
	KeepLast   *int
	MaxAgeDays *int
}

// Validate checks that any configured limit is positive.
func (r *RetentionPolicy) Validate() error {
	if r.KeepLast != nil && *r.KeepLast < 1 {
		return fmt.Errorf("keep_last must be at least 1, got %d", *r.KeepLast)
	}
	if r.MaxAgeDays != nil && *r.MaxAgeDays < 1 {
		return fmt.Errorf("max_age_days must be at least 1, got %d", *r.MaxAgeDays)
	}
	return nil
}

// HasLimit reports whether any retention limit is configured.
func (r *RetentionPolicy) HasLimit() bool {
	return r.KeepLast != nil || r.MaxAgeDays != nil
}
