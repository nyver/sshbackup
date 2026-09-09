package backup_test

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"

	"vpsbackupmanager/internal/backup"
	"vpsbackupmanager/internal/backup/backuptest"
	"vpsbackupmanager/internal/domain"
)

type recordingEventSink struct {
	mu       sync.Mutex
	started  []*domain.Run
	changed  []*domain.RunStep
	finished []*domain.Run
}

func (s *recordingEventSink) RunStarted(run *domain.Run) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.started = append(s.started, run)
}

func (s *recordingEventSink) StepChanged(_ string, step *domain.RunStep) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.changed = append(s.changed, step)
}

func (s *recordingEventSink) RunFinished(run *domain.Run) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.finished = append(s.finished, run)
}

func (s *recordingEventSink) FinishedCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.finished)
}

func TestEngine_Start_ReturnsBeforeCompletionAndFiresEvents(t *testing.T) {
	t.Parallel()
	server := testServer(t)
	job := testJob(t, server.ID)

	overrides := sizeAndFreeSpaceOverrides()
	overrides["sha256sum"] = func() (backup.RunResult, error) { return backup.RunResult{ExitCode: 0, Stdout: sha256Hex(nil)}, nil }

	transport := backuptest.New()
	transport.RunFunc = alwaysOKRun(overrides)
	transport.DownloadFunc = func(context.Context, string, io.Writer) error {
		return nil
	}

	sink := &recordingEventSink{}
	runs := newMemRunStore()
	engine := newTestEngine(runs, newMemStepStore(), newMemLockStore(), func(context.Context, backup.ConnectParams) (backup.Transport, error) {
		return transport, nil
	})
	engine.Events = sink

	run, err := engine.Start(context.Background(), job, server, backup.ConnectParams{Server: server}, job.LocalDestination, domain.TriggerManual)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if run.Status != domain.RunRunning {
		t.Fatalf("Status immediately after Start() = %q, want RUNNING", run.Status)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && sink.FinishedCount() == 0 {
		time.Sleep(5 * time.Millisecond)
	}
	if sink.FinishedCount() != 1 {
		t.Fatal("expected RunFinished to be called once the background run completes")
	}

	sink.mu.Lock()
	defer sink.mu.Unlock()
	if len(sink.started) != 1 {
		t.Errorf("RunStarted called %d times, want 1", len(sink.started))
	}
	if len(sink.changed) == 0 {
		t.Error("expected at least one StepChanged event during the run")
	}
}

func TestEngine_Start_SkippedJobIsSynchronous(t *testing.T) {
	t.Parallel()
	server := testServer(t)
	job := testJob(t, server.ID)

	locks := newMemLockStore()
	if err := locks.Acquire(context.Background(), job.ID, "other-run", "", 1); err != nil {
		t.Fatalf("seed lock: %v", err)
	}

	sink := &recordingEventSink{}
	engine := newTestEngine(newMemRunStore(), newMemStepStore(), locks, func(context.Context, backup.ConnectParams) (backup.Transport, error) {
		t.Fatal("Connect should never be called for an already-locked job")
		return nil, nil
	})
	engine.Events = sink

	run, err := engine.Start(context.Background(), job, server, backup.ConnectParams{Server: server}, job.LocalDestination, domain.TriggerManual)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if run.Status != domain.RunSkipped {
		t.Fatalf("Status = %q, want SKIPPED", run.Status)
	}
	if sink.FinishedCount() != 0 {
		t.Error("a SKIPPED run should not fire RunFinished (it never really started)")
	}
}
