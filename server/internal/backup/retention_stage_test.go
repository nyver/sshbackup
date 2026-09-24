package backup_test

import (
	"context"
	"io"
	"os"
	"testing"
	"time"

	"vpsbackupmanager/internal/backup"
	"vpsbackupmanager/internal/backup/backuptest"
	"vpsbackupmanager/internal/domain"
	"vpsbackupmanager/internal/retention"
)

// TestEngine_Run_RetentionCountsTheJustCreatedArchive is a regression test
// for a bug where retention ran (and queried run history) before the
// current run's own archive_name was persisted, so it never counted the
// archive it had just created against keep_last: every job settled at
// keep_last+1 archives instead of keep_last.
func TestEngine_Run_RetentionCountsTheJustCreatedArchive(t *testing.T) {
	t.Parallel()
	server := testServer(t)

	keepLast := 5
	job, err := domain.NewJob(time.Now(), domain.NewJobParams{
		Name:             "beresta",
		ServerID:         server.ID,
		Enabled:          true,
		Sources:          []domain.Source{{RemotePath: "/docker/volumes"}},
		Schedule:         domain.Schedule{Type: domain.ScheduleManual},
		LocalDestination: t.TempDir(),
		RetentionPolicy:  domain.RetentionPolicy{KeepLast: &keepLast},
	})
	if err != nil {
		t.Fatalf("domain.NewJob() error = %v", err)
	}

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
	engine := &backup.Engine{
		Connect:   func(context.Context, backup.ConnectParams) (backup.Transport, error) { return transport, nil },
		Runs:      runs,
		Steps:     newMemStepStore(),
		Locks:     newMemLockStore(),
		Retention: retention.NewApplier(runs),
		Clock:     newTestClock(),
	}

	const totalRuns = 7
	for i := 0; i < totalRuns; i++ {
		run, err := engine.Run(context.Background(), job, server, backup.ConnectParams{Server: server}, job.LocalDestination, domain.TriggerManual)
		if err != nil {
			t.Fatalf("run %d: Run() error = %v", i, err)
		}
		if run.Status != domain.RunSuccess {
			t.Fatalf("run %d: Status = %q, want SUCCESS (error: %s / %s)", i, run.Status, run.ErrorCode, run.ErrorMessage)
		}

		entries, err := os.ReadDir(job.LocalDestination)
		if err != nil {
			t.Fatalf("run %d: ReadDir() error = %v", i, err)
		}
		wantFiles := min(i+1, keepLast)
		if len(entries) != wantFiles {
			names := make([]string, len(entries))
			for j, e := range entries {
				names[j] = e.Name()
			}
			t.Fatalf("run %d: %d files on disk, want %d: %v", i, len(entries), wantFiles, names)
		}

		archived, err := runs.ListArchived(context.Background(), job.ID)
		if err != nil {
			t.Fatalf("run %d: ListArchived() error = %v", i, err)
		}
		if len(archived) != wantFiles {
			t.Fatalf("run %d: %d archived run records, want %d", i, len(archived), wantFiles)
		}
	}
}
