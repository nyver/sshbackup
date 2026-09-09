package scheduler_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"vpsbackupmanager/internal/domain"
)

func mustParse(t *testing.T, layout, value string) time.Time {
	t.Helper()
	tm, err := time.Parse(layout, value)
	if err != nil {
		t.Fatalf("parse %q: %v", value, err)
	}
	return tm
}

type memJobStore struct {
	jobs []*domain.Job
}

func (s *memJobStore) List(context.Context) ([]*domain.Job, error) {
	return s.jobs, nil
}

type memServerStore struct {
	servers map[string]*domain.Server
}

func (s *memServerStore) Get(_ context.Context, id string) (*domain.Server, error) {
	srv, ok := s.servers[id]
	if !ok {
		return nil, domain.NewCodedError(domain.ErrNotFound, "server not found", nil)
	}
	return srv, nil
}

type memSettingsStore struct {
	mu       sync.Mutex
	settings domain.Settings
}

func newMemSettingsStore(s domain.Settings) *memSettingsStore {
	return &memSettingsStore{settings: s}
}

func (s *memSettingsStore) Get(context.Context) (domain.Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.settings, nil
}

func (s *memSettingsStore) Set(v domain.Settings) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.settings = v
}

type memSchedRunStore struct {
	mu      sync.Mutex
	last    map[string]*domain.Run
	created []*domain.Run
}

func newMemSchedRunStore() *memSchedRunStore {
	return &memSchedRunStore{last: make(map[string]*domain.Run)}
}

func (s *memSchedRunStore) LastRun(_ context.Context, jobID string) (*domain.Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.last[jobID], nil
}

func (s *memSchedRunStore) Create(_ context.Context, run *domain.Run) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	clone := *run
	s.created = append(s.created, &clone)
	s.last[run.JobID] = &clone
	return nil
}

func (s *memSchedRunStore) SeedLastRun(jobID string, startedAt time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.last[jobID] = &domain.Run{JobID: jobID, StartedAt: startedAt, Status: domain.RunSuccess}
}

func (s *memSchedRunStore) Created() []*domain.Run {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]*domain.Run(nil), s.created...)
}

// fakeRunner records every call and blocks on a channel when Block is set,
// so tests can hold a "run" open to exercise concurrency limits.
type fakeRunner struct {
	mu    sync.Mutex
	calls []string

	Block   <-chan struct{} // if non-nil, Run waits on this before returning
	RunFunc func(job *domain.Job, server *domain.Server, trigger domain.Trigger) error
}

func (r *fakeRunner) Run(ctx context.Context, job *domain.Job, server *domain.Server, trigger domain.Trigger) (*domain.Run, error) {
	r.mu.Lock()
	r.calls = append(r.calls, job.ID+":"+string(trigger))
	r.mu.Unlock()

	if r.Block != nil {
		select {
		case <-r.Block:
		case <-ctx.Done():
		}
	}
	var err error
	if r.RunFunc != nil {
		err = r.RunFunc(job, server, trigger)
	}
	return &domain.Run{JobID: job.ID, Status: domain.RunSuccess}, err
}

func (r *fakeRunner) Calls() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.calls...)
}

func (r *fakeRunner) CallCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.calls)
}

// fakeClock returns a fixed, externally settable time.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func newFakeClock(t time.Time) *fakeClock { return &fakeClock{now: t} }

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
}
