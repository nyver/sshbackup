package retention_test

import (
	"context"
	"testing"
	"time"

	"vpsbackupmanager/internal/backup"
	"vpsbackupmanager/internal/backup/backuptest"
	"vpsbackupmanager/internal/domain"
	"vpsbackupmanager/internal/retention"
)

type fakeLockChecker struct{ locked map[string]bool }

func (f fakeLockChecker) IsLocked(_ context.Context, jobID string) (bool, error) {
	return f.locked[jobID], nil
}

func testJobAndServer(t *testing.T) (*domain.Job, *domain.Server) {
	t.Helper()
	server, err := domain.NewServer(time.Now(), domain.NewServerParams{
		Name: "prod", Host: "1.2.3.4", Username: "deploy",
		AuthType: domain.AuthPrivateKey, PrivateKeyPath: "irrelevant",
	})
	if err != nil {
		t.Fatalf("domain.NewServer() error = %v", err)
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
	return job, server
}

func TestRemoteCleaner_Scan_ReportsFindingsWithoutDeleting(t *testing.T) {
	t.Parallel()
	job, server := testJobAndServer(t)

	transport := backuptest.New()
	transport.RunFunc = func(_ context.Context, _ string, _ time.Duration) (backup.RunResult, error) {
		return backup.RunResult{ExitCode: 0, Stdout: "/tmp/vps-backup-manager/" + job.ID + "/old1.tar.gz\n/tmp/vps-backup-manager/" + job.ID + "/old2.tar.gz\n"}, nil
	}

	cleaner := retention.NewRemoteCleaner(
		func(context.Context, backup.ConnectParams) (backup.Transport, error) { return transport, nil },
		fakeLockChecker{},
	)

	findings, err := cleaner.Scan(context.Background(), job, server, backup.ConnectParams{Server: server}, retention.DefaultStaleAge, false)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if len(findings) != 2 {
		t.Fatalf("findings = %v, want 2", findings)
	}
	for _, c := range transport.Commands() {
		if len(c) >= 2 && c[:2] == "rm" {
			t.Errorf("Scan() with autoDelete=false must never delete, but ran %q", c)
		}
	}
}

func TestRemoteCleaner_Scan_AutoDeleteRemovesFindings(t *testing.T) {
	t.Parallel()
	job, server := testJobAndServer(t)

	transport := backuptest.New()
	transport.RunFunc = func(_ context.Context, _ string, _ time.Duration) (backup.RunResult, error) {
		return backup.RunResult{ExitCode: 0, Stdout: "/tmp/vps-backup-manager/" + job.ID + "/old1.tar.gz\n"}, nil
	}

	cleaner := retention.NewRemoteCleaner(
		func(context.Context, backup.ConnectParams) (backup.Transport, error) { return transport, nil },
		fakeLockChecker{},
	)

	findings, err := cleaner.Scan(context.Background(), job, server, backup.ConnectParams{Server: server}, retention.DefaultStaleAge, true)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("findings = %v, want 1", findings)
	}

	var sawDelete bool
	for _, c := range transport.Commands() {
		if c == "rm -f '/tmp/vps-backup-manager/"+job.ID+"/old1.tar.gz'" {
			sawDelete = true
		}
	}
	if !sawDelete {
		t.Errorf("expected a delete command for the finding, got commands: %v", transport.Commands())
	}
}

func TestRemoteCleaner_Scan_NeverTouchesARunningJob(t *testing.T) {
	t.Parallel()
	job, server := testJobAndServer(t)

	transport := backuptest.New()
	transport.RunFunc = func(context.Context, string, time.Duration) (backup.RunResult, error) {
		t.Fatal("Scan must not even connect when the job has an active run")
		return backup.RunResult{}, nil
	}

	cleaner := retention.NewRemoteCleaner(
		func(context.Context, backup.ConnectParams) (backup.Transport, error) { return transport, nil },
		fakeLockChecker{locked: map[string]bool{job.ID: true}},
	)

	findings, err := cleaner.Scan(context.Background(), job, server, backup.ConnectParams{Server: server}, retention.DefaultStaleAge, true)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if findings != nil {
		t.Errorf("findings = %v, want nil for a running job", findings)
	}
}
