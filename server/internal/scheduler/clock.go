package scheduler

import "time"

// Clock abstracts wall-clock time so dispatch tests can control "now"
// instead of racing real time.
type Clock interface {
	Now() time.Time
}

// SystemClock is the production Clock backed by the real time package.
type SystemClock struct{}

// Now returns the current wall-clock time.
func (SystemClock) Now() time.Time { return time.Now() }
