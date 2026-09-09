package backup

import (
	"context"
	"sync"
)

// cleanupEntry is one guaranteed recovery action: an ALWAYS or
// critical_cleanup script. done is set once the action has actually run,
// whether through the normal per-condition script loop or through Drain,
// so it never runs twice.
type cleanupEntry struct {
	description string
	fn          func(ctx context.Context) error
	done        bool
}

// cleanupRegistry collects ALWAYS/critical_cleanup actions as they are
// discovered and guarantees each runs exactly once, independent of
// whether the ones before it failed. Drain is called from a deferred
// finalizer on a context detached from the run's own cancellation, so
// cleanup still completes when the run itself was cancelled or the
// service is shutting down.
type cleanupRegistry struct {
	mu      sync.Mutex
	entries []*cleanupEntry
}

// Register adds a guaranteed action and returns a token to mark it done
// once the caller has run it directly (e.g. via the normal script loop).
func (r *cleanupRegistry) Register(description string, fn func(ctx context.Context) error) *cleanupEntry {
	r.mu.Lock()
	defer r.mu.Unlock()
	e := &cleanupEntry{description: description, fn: fn}
	r.entries = append(r.entries, e)
	return e
}

// MarkDone records that entry already ran, so Drain skips it.
func (r *cleanupRegistry) MarkDone(entry *cleanupEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry.done = true
}

// cleanupOutcome is the result of one drained (or directly run) action.
type cleanupOutcome struct {
	Description string
	Err         error
}

// Drain runs every not-yet-done entry on ctx, independently: one entry
// failing does not stop the rest, and every outcome is returned.
func (r *cleanupRegistry) Drain(ctx context.Context) []cleanupOutcome {
	r.mu.Lock()
	pending := make([]*cleanupEntry, 0, len(r.entries))
	for _, e := range r.entries {
		if !e.done {
			pending = append(pending, e)
		}
	}
	r.mu.Unlock()

	outcomes := make([]cleanupOutcome, 0, len(pending))
	for _, e := range pending {
		err := e.fn(ctx)
		r.MarkDone(e)
		outcomes = append(outcomes, cleanupOutcome{Description: e.description, Err: err})
	}
	return outcomes
}
