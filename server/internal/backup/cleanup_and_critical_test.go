package backup_test

import (
	"context"
	"io"
	"testing"
	"time"

	"vpsbackupmanager/internal/backup"
	"vpsbackupmanager/internal/backup/backuptest"
	"vpsbackupmanager/internal/domain"
)

// TestEngine_Run_CancellationRunsAllGuaranteedCleanup covers: cancelling a
// run mid-download still attempts every ALWAYS/critical_cleanup script,
// and one of them failing does not prevent the next from running.
func TestEngine_Run_CancellationRunsAllGuaranteedCleanup(t *testing.T) {
	t.Parallel()
	server := testServer(t)
	job, err := domain.NewJob(time.Now(), domain.NewJobParams{
		Name:     "beresta",
		ServerID: server.ID,
		Enabled:  true,
		Sources:  []domain.Source{{RemotePath: "/docker/volumes"}},
		Scripts: []domain.Script{
			{Type: domain.ScriptPostBackup, Command: "cleanup-1-fails", Position: 0, RunCondition: domain.RunConditionAlways},
			{Type: domain.ScriptPostBackup, Command: "cleanup-2-succeeds", Position: 1, RunCondition: domain.RunConditionAlways},
		},
		Schedule:         domain.Schedule{Type: domain.ScheduleManual},
		LocalDestination: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("domain.NewJob() error = %v", err)
	}

	overrides := sizeAndFreeSpaceOverrides()
	overrides["sha256sum"] = func() (backup.RunResult, error) { return backup.RunResult{ExitCode: 0, Stdout: "irrelevant"}, nil }
	overrides["cleanup-1-fails"] = func() (backup.RunResult, error) { return backup.RunResult{ExitCode: 1, Stderr: "boom"}, nil }
	overrides["cleanup-2-succeeds"] = func() (backup.RunResult, error) { return backup.RunResult{ExitCode: 0}, nil }

	ctx, cancel := context.WithCancel(context.Background())
	transport := backuptest.New()
	transport.RunFunc = alwaysOKRun(overrides)
	transport.DownloadFunc = func(context.Context, string, io.Writer) error {
		// Simulate the connection dropping mid-download: cancel the run's
		// own context, then report the resulting failure the way
		// remote.Client.Download would.
		cancel()
		return domain.NewCodedError(domain.ErrDownloadFailed, "connection dropped", context.Canceled)
	}

	runs := newMemRunStore()
	steps := newMemStepStore()
	engine := newTestEngine(runs, steps, newMemLockStore(), func(context.Context, backup.ConnectParams) (backup.Transport, error) {
		return transport, nil
	})

	run, err := engine.Run(ctx, job, server, backup.ConnectParams{Server: server}, job.LocalDestination, domain.TriggerManual)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if run.Status != domain.RunCancelled {
		t.Fatalf("Status = %q, want CANCELLED", run.Status)
	}
	if run.RecoveryOutcome != domain.RecoveryFailed {
		t.Errorf("RecoveryOutcome = %q, want FAILED (one cleanup script failed)", run.RecoveryOutcome)
	}

	commands := transport.Commands()
	var sawFirst, sawSecond bool
	for _, c := range commands {
		if c == "cleanup-1-fails" {
			sawFirst = true
		}
		if c == "cleanup-2-succeeds" {
			sawSecond = true
		}
	}
	if !sawFirst || !sawSecond {
		t.Fatalf("commands = %v, want both cleanup scripts attempted despite the first failing", commands)
	}

	var postSteps []*domain.RunStep
	for _, s := range steps.StepsFor(run.ID) {
		if s.Type == domain.StepPostBackupScript {
			postSteps = append(postSteps, s)
		}
	}
	if len(postSteps) != 2 {
		t.Fatalf("recorded %d POST_BACKUP_SCRIPT steps, want 2 (one per guaranteed cleanup script): %+v", len(postSteps), postSteps)
	}
	if postSteps[0].Status != domain.StepFailed {
		t.Errorf("first cleanup step status = %q, want FAILED", postSteps[0].Status)
	}
	if postSteps[1].Status != domain.StepSuccess {
		t.Errorf("second cleanup step status = %q, want SUCCESS (must still run after the first failed)", postSteps[1].Status)
	}
}

// TestEngine_Run_CriticalFailureScenario covers specification §86: a
// pre-script stops a service, the archive stage then fails with no space
// left on device, and a POST_BACKUP ALWAYS script restarts the service —
// the run is FAILED/ARCHIVE_FAILED but recovery is reported SUCCESS.
func TestEngine_Run_CriticalFailureScenario(t *testing.T) {
	t.Parallel()
	server := testServer(t)
	job, err := domain.NewJob(time.Now(), domain.NewJobParams{
		Name:     "beresta",
		ServerID: server.ID,
		Enabled:  true,
		Sources:  []domain.Source{{RemotePath: "/docker/volumes"}},
		Scripts: []domain.Script{
			{Type: domain.ScriptPreBackup, Command: "docker compose down", Position: 0, RunCondition: domain.RunConditionAlways},
			{Type: domain.ScriptPostBackup, Command: "docker compose up -d", Position: 0, RunCondition: domain.RunConditionAlways, CriticalCleanup: true},
		},
		Schedule:         domain.Schedule{Type: domain.ScheduleManual},
		LocalDestination: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("domain.NewJob() error = %v", err)
	}

	overrides := sizeAndFreeSpaceOverrides()
	overrides["-czf"] = func() (backup.RunResult, error) {
		return backup.RunResult{ExitCode: 1, Stderr: "tar: no space left on device"}, nil
	}

	transport := backuptest.New()
	transport.RunFunc = alwaysOKRun(overrides)

	runs := newMemRunStore()
	steps := newMemStepStore()
	engine := newTestEngine(runs, steps, newMemLockStore(), func(context.Context, backup.ConnectParams) (backup.Transport, error) {
		return transport, nil
	})

	run, err := engine.Run(context.Background(), job, server, backup.ConnectParams{Server: server}, job.LocalDestination, domain.TriggerManual)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if run.Status != domain.RunFailed {
		t.Fatalf("Status = %q, want FAILED", run.Status)
	}
	if run.ErrorCode != domain.ErrArchiveFailed {
		t.Fatalf("ErrorCode = %q, want ARCHIVE_FAILED", run.ErrorCode)
	}
	if run.RecoveryOutcome != domain.RecoverySuccess {
		t.Fatalf("RecoveryOutcome = %q, want SUCCESS (the ALWAYS post-script restored the service)", run.RecoveryOutcome)
	}

	var sawPreScript, sawRecoveryScript bool
	for _, c := range transport.Commands() {
		if c == "docker compose down" {
			sawPreScript = true
		}
		if c == "docker compose up -d" {
			sawRecoveryScript = true
		}
	}
	if !sawPreScript {
		t.Error("expected the PRE_BACKUP script to have run before the archive stage")
	}
	if !sawRecoveryScript {
		t.Error("expected the ALWAYS POST_BACKUP recovery script to have run despite the archive failure")
	}
}
