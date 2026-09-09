package remote

import (
	"context"
	"time"
)

// Clock abstracts wall-clock time and sleeping so retry backoff is
// testable without a test actually waiting 5, 15, and 30 seconds.
type Clock interface {
	Now() time.Time
	// Sleep waits for d or until ctx is cancelled, whichever comes first,
	// returning ctx.Err() in the latter case.
	Sleep(ctx context.Context, d time.Duration) error
}

// SystemClock is the production Clock backed by the real time package.
type SystemClock struct{}

// Now returns the current wall-clock time.
func (SystemClock) Now() time.Time { return time.Now() }

// Sleep waits for d or until ctx is cancelled.
func (SystemClock) Sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
