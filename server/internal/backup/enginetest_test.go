package backup_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"
	"testing"
	"time"

	"vpsbackupmanager/internal/backup"
	"vpsbackupmanager/internal/domain"
)

// memRunStore, memStepStore, and memLockStore are minimal in-memory
// RunStore/StepStore/LockStore implementations for engine tests. The
// repository implementations themselves (parameterized SQL, rows.Err,
// transactions) are covered by internal/store's own tests; these fakes
// exist purely so engine tests exercise engine logic, not persistence.
type memRunStore struct {
	mu   sync.Mutex
	runs map[string]*domain.Run
}

func newMemRunStore() *memRunStore { return &memRunStore{runs: make(map[string]*domain.Run)} }

func (s *memRunStore) Create(_ context.Context, run *domain.Run) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	clone := *run
	s.runs[run.ID] = &clone
	return nil
}

func (s *memRunStore) Update(_ context.Context, run *domain.Run) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	clone := *run
	s.runs[run.ID] = &clone
	return nil
}

func (s *memRunStore) Get(id string) *domain.Run {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.runs[id]
}

type memStepStore struct {
	mu    sync.Mutex
	steps map[string][]*domain.RunStep
}

func newMemStepStore() *memStepStore { return &memStepStore{steps: make(map[string][]*domain.RunStep)} }

func (s *memStepStore) Create(_ context.Context, step *domain.RunStep, _ int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	clone := *step
	s.steps[step.RunID] = append(s.steps[step.RunID], &clone)
	return nil
}

func (s *memStepStore) Update(_ context.Context, step *domain.RunStep) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.steps[step.RunID] {
		if existing.ID == step.ID {
			*existing = *step
			return nil
		}
	}
	return nil
}

func (s *memStepStore) StepsFor(runID string) []*domain.RunStep {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]*domain.RunStep(nil), s.steps[runID]...)
}

type memLockStore struct {
	mu     sync.Mutex
	locked map[string]bool
}

func newMemLockStore() *memLockStore { return &memLockStore{locked: make(map[string]bool)} }

func (s *memLockStore) Acquire(_ context.Context, jobID, _, _ string, _ int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.locked[jobID] {
		return domain.NewCodedError(domain.ErrJobAlreadyRunning, "Job already running", nil)
	}
	s.locked[jobID] = true
	return nil
}

func (s *memLockStore) Release(_ context.Context, jobID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.locked, jobID)
	return nil
}

// testClock ticks by a fixed step on every Now() call so ordering-sensitive
// assertions (StartedAt < FinishedAt) hold, and Sleep returns immediately
// (or ctx.Err() if already cancelled) so tests never wait in real time.
type testClock struct {
	mu   sync.Mutex
	next time.Time
}

func newTestClock() *testClock {
	return &testClock{next: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := c.next
	c.next = c.next.Add(time.Second)
	return t
}

func (c *testClock) Sleep(ctx context.Context, _ time.Duration) error {
	return ctx.Err()
}

type noopRetention struct{ applied bool }

func (r *noopRetention) Apply(context.Context, *domain.Job) (int, []string, error) {
	r.applied = true
	return 0, nil, nil
}

func testJob(t *testing.T, serverID string) *domain.Job {
	t.Helper()
	j, err := domain.NewJob(time.Now(), domain.NewJobParams{
		Name:             "beresta",
		ServerID:         serverID,
		Enabled:          true,
		Sources:          []domain.Source{{RemotePath: "/docker/volumes"}},
		Schedule:         domain.Schedule{Type: domain.ScheduleManual},
		LocalDestination: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("domain.NewJob() error = %v", err)
	}
	return j
}

func testServer(t *testing.T) *domain.Server {
	t.Helper()
	s, err := domain.NewServer(time.Now(), domain.NewServerParams{
		Name: "prod", Host: "203.0.113.5", Username: "deploy",
		AuthType: domain.AuthPrivateKey, PrivateKeyPath: "irrelevant-in-tests",
	})
	if err != nil {
		t.Fatalf("domain.NewServer() error = %v", err)
	}
	return s
}

// alwaysOKRun is a RunFunc that succeeds every command except the ones
// overridden in overrides, matched by substring.
func alwaysOKRun(overrides map[string]func() (backup.RunResult, error)) func(ctx context.Context, cmd string, timeout time.Duration) (backup.RunResult, error) {
	return func(_ context.Context, cmd string, _ time.Duration) (backup.RunResult, error) {
		for substr, fn := range overrides {
			if strings.Contains(cmd, substr) {
				return fn()
			}
		}
		return backup.RunResult{ExitCode: 0}, nil
	}
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
