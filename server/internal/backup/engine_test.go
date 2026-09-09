package backup_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"vpsbackupmanager/internal/backup"
	"vpsbackupmanager/internal/backup/backuptest"
	"vpsbackupmanager/internal/domain"
)

func newTestEngine(runs backup.RunStore, steps backup.StepStore, locks backup.LockStore, connect backup.Connector) *backup.Engine {
	return &backup.Engine{
		Connect:   connect,
		Runs:      runs,
		Steps:     steps,
		Locks:     locks,
		Retention: &noopRetention{},
		Clock:     newTestClock(),
	}
}

// sizeAndFreeSpaceOverrides returns the du/df fake responses every
// preflight-passing test needs.
func sizeAndFreeSpaceOverrides() map[string]func() (backup.RunResult, error) {
	return map[string]func() (backup.RunResult, error){
		"du -sb": func() (backup.RunResult, error) { return backup.RunResult{ExitCode: 0, Stdout: "100"}, nil },
		"df -B1": func() (backup.RunResult, error) { return backup.RunResult{ExitCode: 0, Stdout: "999999999"}, nil },
	}
}

func TestEngine_Run_FullSuccess(t *testing.T) {
	t.Parallel()
	server := testServer(t)
	job := testJob(t, server.ID)

	archiveContent := []byte("fake-archive-bytes")
	hash := sha256Hex(archiveContent)

	overrides := sizeAndFreeSpaceOverrides()
	overrides["sha256sum"] = func() (backup.RunResult, error) { return backup.RunResult{ExitCode: 0, Stdout: hash}, nil }

	transport := backuptest.New()
	transport.RunFunc = alwaysOKRun(overrides)
	transport.DownloadFunc = func(_ context.Context, _ string, w io.Writer) error {
		_, err := w.Write(archiveContent)
		return err
	}

	runs := newMemRunStore()
	steps := newMemStepStore()
	engine := newTestEngine(runs, steps, newMemLockStore(), func(context.Context, backup.ConnectParams) (backup.Transport, error) {
		return transport, nil
	})

	run, err := engine.Run(context.Background(), job, server, backup.ConnectParams{Server: server}, job.LocalDestination, domain.TriggerManual)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if run.Status != domain.RunSuccess {
		t.Fatalf("Status = %q, want SUCCESS (error: %s / %s)", run.Status, run.ErrorCode, run.ErrorMessage)
	}
	if run.Checksum != hash {
		t.Errorf("Checksum = %q, want %q", run.Checksum, hash)
	}
	if run.ArchiveSize != int64(len(archiveContent)) {
		t.Errorf("ArchiveSize = %d, want %d", run.ArchiveSize, len(archiveContent))
	}
	if !transport.Closed() {
		t.Error("expected the transport to be closed at the end of a run")
	}

	finalPath := filepath.Join(job.LocalDestination, run.ArchiveName)
	got, err := os.ReadFile(finalPath) //nolint:gosec // test-controlled path under t.TempDir()
	if err != nil {
		t.Fatalf("read final archive: %v", err)
	}
	if string(got) != string(archiveContent) {
		t.Errorf("final archive content = %q, want %q", got, archiveContent)
	}

	wantOrder := []domain.StepType{
		domain.StepPreflight, domain.StepArchive, domain.StepRemoteChecksum,
		domain.StepDownload, domain.StepLocalChecksum, domain.StepVerifyChecksum,
		domain.StepRemoteCleanup, domain.StepRetention,
	}
	gotSteps := steps.StepsFor(run.ID)
	if len(gotSteps) != len(wantOrder) {
		t.Fatalf("recorded %d steps, want %d: %+v", len(gotSteps), len(wantOrder), gotSteps)
	}
	for i, want := range wantOrder {
		if gotSteps[i].Type != want {
			t.Errorf("step[%d].Type = %q, want %q", i, gotSteps[i].Type, want)
		}
		if gotSteps[i].Status != domain.StepSuccess {
			t.Errorf("step[%d] (%s) status = %q, want SUCCESS: %s", i, gotSteps[i].Type, gotSteps[i].Status, gotSteps[i].Error)
		}
	}
}

func TestEngine_Run_PreflightFailureStopsForwardProgress(t *testing.T) {
	t.Parallel()
	server := testServer(t)
	job := testJob(t, server.ID)

	transport := backuptest.New()
	transport.RunFunc = func(_ context.Context, cmd string, _ time.Duration) (backup.RunResult, error) {
		if cmd == "test -e '/docker/volumes'" {
			return backup.RunResult{ExitCode: 1}, nil
		}
		return backup.RunResult{ExitCode: 0}, nil
	}

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
	if run.ErrorCode != domain.ErrRemoteSourceNotFound {
		t.Errorf("ErrorCode = %q, want REMOTE_SOURCE_NOT_FOUND", run.ErrorCode)
	}

	gotSteps := steps.StepsFor(run.ID)
	if len(gotSteps) != 1 || gotSteps[0].Type != domain.StepPreflight {
		t.Fatalf("steps = %+v, want exactly one PREFLIGHT step (no later stage should have run)", gotSteps)
	}
}

func TestEngine_Run_ArchiveFailureStopsDownload(t *testing.T) {
	t.Parallel()
	server := testServer(t)
	job := testJob(t, server.ID)

	overrides := sizeAndFreeSpaceOverrides()
	overrides["-czf"] = func() (backup.RunResult, error) {
		return backup.RunResult{ExitCode: 2, Stderr: "no space left on device"}, nil
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
	if run.Status != domain.RunFailed || run.ErrorCode != domain.ErrArchiveFailed {
		t.Fatalf("run = %+v, want FAILED/ARCHIVE_FAILED", run)
	}

	for _, s := range steps.StepsFor(run.ID) {
		if s.Type == domain.StepDownload || s.Type == domain.StepRemoteChecksum || s.Type == domain.StepVerifyChecksum {
			t.Errorf("stage %q ran after archive creation failed; forward progress should have stopped", s.Type)
		}
	}
}

func TestEngine_Run_OnSuccessScriptSkippedAfterFailure(t *testing.T) {
	t.Parallel()
	server := testServer(t)

	job, err := domain.NewJob(time.Now(), domain.NewJobParams{
		Name:     "beresta",
		ServerID: server.ID,
		Enabled:  true,
		Sources:  []domain.Source{{RemotePath: "/docker/volumes"}},
		Scripts: []domain.Script{
			{Type: domain.ScriptPreBackup, Command: "pre-fails", Position: 0, RunCondition: domain.RunConditionOnSuccess},
			{Type: domain.ScriptPostBackup, Command: "post-on-success", Position: 0, RunCondition: domain.RunConditionOnSuccess},
		},
		Schedule:         domain.Schedule{Type: domain.ScheduleManual},
		LocalDestination: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("domain.NewJob() error = %v", err)
	}

	overrides := sizeAndFreeSpaceOverrides()
	overrides["pre-fails"] = func() (backup.RunResult, error) { return backup.RunResult{ExitCode: 1, Stderr: "boom"}, nil }

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
	if run.Status != domain.RunFailed || run.ErrorCode != domain.ErrPreScriptFailed {
		t.Fatalf("run = %+v, want FAILED/PRE_SCRIPT_FAILED", run)
	}

	var sawSkippedPost, sawFailedPre bool
	for _, s := range steps.StepsFor(run.ID) {
		switch s.Type {
		case domain.StepPostBackupScript:
			sawSkippedPost = true
			if s.Status != domain.StepSkipped {
				t.Errorf("POST_BACKUP step status = %q, want SKIPPED", s.Status)
			}
			if s.Command != "post-on-success" {
				t.Errorf("skipped POST_BACKUP step Command = %q, want %q (what would have run)", s.Command, "post-on-success")
			}
		case domain.StepPreBackupScript:
			sawFailedPre = true
			if s.Command != "pre-fails" {
				t.Errorf("failed PRE_BACKUP step Command = %q, want %q", s.Command, "pre-fails")
			}
		}
	}
	if !sawSkippedPost {
		t.Fatal("expected a POST_BACKUP script step to be recorded as skipped")
	}
	if !sawFailedPre {
		t.Fatal("expected a PRE_BACKUP script step to be recorded")
	}
}

func TestEngine_Run_JobAlreadyRunningIsSkipped(t *testing.T) {
	t.Parallel()
	server := testServer(t)
	job := testJob(t, server.ID)

	locks := newMemLockStore()
	if err := locks.Acquire(context.Background(), job.ID, "other-run", "", 1); err != nil {
		t.Fatalf("seed lock: %v", err)
	}

	runs := newMemRunStore()
	engine := newTestEngine(runs, newMemStepStore(), locks, func(context.Context, backup.ConnectParams) (backup.Transport, error) {
		t.Fatal("Connect should never be called when the job is already locked")
		return nil, nil
	})

	run, err := engine.Run(context.Background(), job, server, backup.ConnectParams{Server: server}, job.LocalDestination, domain.TriggerSchedule)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if run.Status != domain.RunSkipped {
		t.Fatalf("Status = %q, want SKIPPED", run.Status)
	}
	if run.ErrorMessage != "Job already running" {
		t.Errorf("ErrorMessage = %q, want %q", run.ErrorMessage, "Job already running")
	}
}
