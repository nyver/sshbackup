package backup_test

import (
	"context"
	"testing"
	"time"

	"vpsbackupmanager/internal/backup"
	"vpsbackupmanager/internal/backup/backuptest"
	"vpsbackupmanager/internal/domain"
)

func TestValidator_Validate_Success(t *testing.T) {
	t.Parallel()
	server := testServer(t)
	job := testJob(t, server.ID)

	transport := backuptest.New()
	transport.RunFunc = alwaysOKRun(sizeAndFreeSpaceOverrides())

	v := backup.NewValidator(func(context.Context, backup.ConnectParams) (backup.Transport, error) {
		return transport, nil
	})
	result := v.Validate(context.Background(), job, server, backup.ConnectParams{Server: server}, job.LocalDestination)

	if !result.Success {
		t.Fatalf("Success = false, want true; checks: %+v", result.Checks)
	}
	if !result.SourceSizeKnown || result.SourceSizeBytes != 100 {
		t.Errorf("SourceSizeBytes = %d (known=%v), want 100 (known=true)", result.SourceSizeBytes, result.SourceSizeKnown)
	}
	if !result.RemoteFreeKnown {
		t.Error("expected RemoteFreeKnown = true")
	}
	if len(transport.Commands()) == 0 {
		t.Fatal("expected the validator to have run remote checks")
	}
	if !transport.Closed() {
		t.Error("expected the transport to be closed after validation")
	}
}

func TestValidator_Validate_NeverExecutesUserScripts(t *testing.T) {
	t.Parallel()
	server := testServer(t)
	job, err := domain.NewJob(time.Now(), domain.NewJobParams{
		Name:     "beresta",
		ServerID: server.ID,
		Enabled:  true,
		Sources:  []domain.Source{{RemotePath: "/docker/volumes"}},
		Scripts: []domain.Script{
			{Type: domain.ScriptPreBackup, Command: "docker compose down", Position: 0, RunCondition: domain.RunConditionAlways},
		},
		Schedule:         domain.Schedule{Type: domain.ScheduleManual},
		LocalDestination: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("domain.NewJob() error = %v", err)
	}

	transport := backuptest.New()
	transport.RunFunc = alwaysOKRun(sizeAndFreeSpaceOverrides())

	v := backup.NewValidator(func(context.Context, backup.ConnectParams) (backup.Transport, error) {
		return transport, nil
	})
	v.Validate(context.Background(), job, server, backup.ConnectParams{Server: server}, job.LocalDestination)

	for _, c := range transport.Commands() {
		if c == "docker compose down" {
			t.Fatal("Validate must never execute a user script, but the destructive pre-script ran")
		}
	}
}

func TestValidator_Validate_ReportsAllProblems(t *testing.T) {
	t.Parallel()
	server := testServer(t)
	job, err := domain.NewJob(time.Now(), domain.NewJobParams{
		Name:     "beresta",
		ServerID: server.ID,
		Enabled:  true,
		Sources: []domain.Source{
			{RemotePath: "/docker/volumes"},
			{RemotePath: "/missing/path"},
		},
		Schedule:         domain.Schedule{Type: domain.ScheduleManual},
		LocalDestination: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("domain.NewJob() error = %v", err)
	}

	overrides := sizeAndFreeSpaceOverrides()
	overrides["test -e '/missing/path'"] = func() (backup.RunResult, error) { return backup.RunResult{ExitCode: 1}, nil }

	transport := backuptest.New()
	transport.RunFunc = alwaysOKRun(overrides)

	v := backup.NewValidator(func(context.Context, backup.ConnectParams) (backup.Transport, error) {
		return transport, nil
	})
	result := v.Validate(context.Background(), job, server, backup.ConnectParams{Server: server}, job.LocalDestination)

	if result.Success {
		t.Fatal("Success = true, want false")
	}

	var firstSourcePassed, secondSourceFailed bool
	for _, c := range result.Checks {
		if c.Name == "source:/docker/volumes" && c.Passed {
			firstSourcePassed = true
		}
		if c.Name == "source:/missing/path" && !c.Passed {
			secondSourceFailed = true
		}
	}
	if !firstSourcePassed {
		t.Error("expected the first (valid) source to still be reported as passed")
	}
	if !secondSourceFailed {
		t.Error("expected the second (missing) source to be reported as failed")
	}

	// Every check after the failing source must still have run: Validate
	// Job reports every problem in one pass rather than stopping early.
	var sawToolCheck bool
	for _, c := range result.Checks {
		if c.Name == "tool:tar" {
			sawToolCheck = true
		}
	}
	if !sawToolCheck {
		t.Error("expected checks after the failing source to still have run")
	}
}
