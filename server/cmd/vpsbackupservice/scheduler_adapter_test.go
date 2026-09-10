package main

import (
	"context"
	"io"
	"testing"
	"time"

	"vpsbackupmanager/internal/backup"
	"vpsbackupmanager/internal/backup/backuptest"
	"vpsbackupmanager/internal/domain"
	"vpsbackupmanager/internal/secrets"
	"vpsbackupmanager/internal/store"
)

// TestSchedulerRunner_Run_PasswordAuthServer is a regression test for a bug
// where every scheduled (as opposed to manual "Run now") run against a
// PASSWORD-authenticated server failed immediately with "read private key
// \"\": ... file not found": schedulerRunner.Run unconditionally read
// server.PrivateKeyPath, which is legitimately empty for PASSWORD auth.
func TestSchedulerRunner_Run_PasswordAuthServer(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	ctx := context.Background()

	secretsStore := secrets.NewStore(t.TempDir())
	ref, err := secretsStore.Save([]byte("s3cret"))
	if err != nil {
		t.Fatalf("secretsStore.Save() error = %v", err)
	}

	server, err := domain.NewServer(time.Now(), domain.NewServerParams{
		Name: "prod", Host: "1.2.3.4", Username: "deploy",
		AuthType: domain.AuthPassword, CredentialReference: ref,
	})
	if err != nil {
		t.Fatalf("domain.NewServer() error = %v", err)
	}
	if err := store.NewServerRepository(db).Create(ctx, server); err != nil {
		t.Fatalf("create server: %v", err)
	}

	job, err := domain.NewJob(time.Now(), domain.NewJobParams{
		Name: "beresta", ServerID: server.ID, Enabled: true,
		Sources:          []domain.Source{{RemotePath: "/data"}},
		Schedule:         domain.Schedule{Type: domain.ScheduleManual},
		LocalDestination: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("domain.NewJob() error = %v", err)
	}
	if err := store.NewJobRepository(db).Create(ctx, job); err != nil {
		t.Fatalf("create job: %v", err)
	}

	archiveContent := []byte("end-to-end-fake-archive-bytes")
	transport := backuptest.New()
	transport.RunFunc = func(_ context.Context, cmd string, _ time.Duration) (backup.RunResult, error) {
		switch {
		case containsForTest(cmd, "du -sb"):
			return backup.RunResult{ExitCode: 0, Stdout: "100"}, nil
		case containsForTest(cmd, "df -B1"):
			return backup.RunResult{ExitCode: 0, Stdout: "999999999"}, nil
		case containsForTest(cmd, "sha256sum"):
			return backup.RunResult{ExitCode: 0, Stdout: sha256HexForTest(archiveContent)}, nil
		default:
			return backup.RunResult{ExitCode: 0}, nil
		}
	}
	transport.DownloadFunc = func(_ context.Context, _ string, w io.Writer) error {
		_, err := w.Write(archiveContent)
		return err
	}

	var gotPassword string
	var gotHadKey bool
	engine := &backup.Engine{
		Connect: func(_ context.Context, p backup.ConnectParams) (backup.Transport, error) {
			gotPassword = p.Password
			gotHadKey = p.PrivateKeyPEM != nil
			return transport, nil
		},
		Runs: store.NewRunRepository(db), Steps: store.NewStepRepository(db), Locks: store.NewLockRepository(db),
		Clock: backup.SystemClock{}, Logger: silentLogger(),
	}

	runner := &schedulerRunner{engine: engine, servers: store.NewServerRepository(db), secretsStore: secretsStore}
	run, err := runner.Run(ctx, job, server, domain.TriggerSchedule)
	if err != nil {
		t.Fatalf("schedulerRunner.Run() error = %v, want nil (password-auth servers must not read a private key file)", err)
	}
	if run.Status != domain.RunSuccess {
		t.Fatalf("run.Status = %q, want SUCCESS (error: %s / %s)", run.Status, run.ErrorCode, run.ErrorMessage)
	}
	if gotHadKey {
		t.Error("connector received a non-nil PrivateKeyPEM for a PASSWORD-auth server")
	}
	if gotPassword != "s3cret" {
		t.Errorf("connector received Password = %q, want %q", gotPassword, "s3cret")
	}
}
