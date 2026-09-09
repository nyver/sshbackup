package domain

import (
	"testing"
	"time"
)

func TestRun_FinishIsStableOnceTerminal(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	r, err := NewRun(start, "job-1", "server-1", []string{"/docker/volumes"}, TriggerManual)
	if err != nil {
		t.Fatalf("NewRun() error = %v", err)
	}

	firstFinish := start.Add(time.Minute)
	r.Finish(firstFinish, RunSuccess)
	if r.Status != RunSuccess {
		t.Fatalf("Status = %q, want %q", r.Status, RunSuccess)
	}

	secondFinish := start.Add(2 * time.Minute)
	r.Finish(secondFinish, RunFailed)
	if r.Status != RunSuccess {
		t.Errorf("terminal status changed: got %q, want it to remain %q", r.Status, RunSuccess)
	}
	if !r.FinishedAt.Equal(firstFinish) {
		t.Errorf("FinishedAt changed after the run was already terminal: got %v, want %v", r.FinishedAt, firstFinish)
	}
}

func TestRun_Duration(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	r, err := NewRun(start, "job-1", "server-1", nil, TriggerSchedule)
	if err != nil {
		t.Fatalf("NewRun() error = %v", err)
	}
	if got := r.Duration(); got != 0 {
		t.Errorf("Duration() before finish = %v, want 0", got)
	}
	r.Finish(start.Add(90*time.Second), RunSuccess)
	if got := r.Duration(); got != 90*time.Second {
		t.Errorf("Duration() = %v, want 90s", got)
	}
}

func TestRunStep_SkipRecordsReason(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	step, err := NewRunStep(now, "run-1", StepPostBackupScript)
	if err != nil {
		t.Fatalf("NewRunStep() error = %v", err)
	}
	step.Skip(now, "preceding stage failed")
	if step.Status != StepSkipped {
		t.Errorf("Status = %q, want %q", step.Status, StepSkipped)
	}
	if step.Error != "preceding stage failed" {
		t.Errorf("Error = %q, want the skip reason", step.Error)
	}
	if step.FinishedAt == nil {
		t.Error("expected FinishedAt to be set")
	}
}
