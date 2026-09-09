package ipc

import (
	"context"
	"sync"
)

// ActiveRunRegistry tracks the cancel function for every run currently in
// progress, so runs.cancel can stop one by id. Both scheduler-triggered
// and manually-triggered (Run now) runs register themselves here.
type ActiveRunRegistry struct {
	mu      sync.Mutex
	cancels map[string]context.CancelFunc
}

// NewActiveRunRegistry constructs an empty registry.
func NewActiveRunRegistry() *ActiveRunRegistry {
	return &ActiveRunRegistry{cancels: make(map[string]context.CancelFunc)}
}

// Register associates runID with cancel, so a later Cancel(runID) stops
// it. Callers must call Unregister once the run finishes.
func (r *ActiveRunRegistry) Register(runID string, cancel context.CancelFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cancels[runID] = cancel
}

// Unregister removes runID once its run has finished.
func (r *ActiveRunRegistry) Unregister(runID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.cancels, runID)
}

// Cancel invokes the registered cancel function for runID, reporting
// whether one was found (i.e. the run was actually active).
func (r *ActiveRunRegistry) Cancel(runID string) bool {
	r.mu.Lock()
	cancel, ok := r.cancels[runID]
	r.mu.Unlock()
	if ok {
		cancel()
	}
	return ok
}

// CancelAll invokes every registered cancel function, for graceful
// shutdown: runs started through this registry (e.g. "Run now" over IPC)
// deliberately use a context detached from any single request's
// lifetime, so shutdown must reach them this way rather than by
// cancelling one parent context.
func (r *ActiveRunRegistry) CancelAll() {
	r.mu.Lock()
	cancels := make([]context.CancelFunc, 0, len(r.cancels))
	for _, c := range r.cancels {
		cancels = append(cancels, c)
	}
	r.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
}
