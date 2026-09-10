package main

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"vpsbackupmanager/internal/backup"
	"vpsbackupmanager/internal/backup/backuptest"
	"vpsbackupmanager/internal/domain"
	"vpsbackupmanager/internal/ipc"
	"vpsbackupmanager/internal/secrets"
	"vpsbackupmanager/internal/store"
)

// TestBuildApp_IPCAvailableWhileMissedRunCatchUpIsStillRunning is a
// regression test: the service must report itself started, and accept IPC
// connections, without waiting for missed-run catch-up backups to finish.
// It previously ran DetectMissedRuns synchronously before starting the IPC
// server, so a slow catch-up run (or, on a host that had been offline for
// a while, several of them run back to back) left the Windows service
// stuck in StartPending and the Flutter client unable to connect for as
// long as catch-up took.
func TestBuildApp_IPCAvailableWhileMissedRunCatchUpIsStillRunning(t *testing.T) {
	dataDir := t.TempDir()
	db := openTestDB(t)
	ctx := context.Background()

	// A real (fixture) key file, unlike seedServerAndJob's placeholder
	// "irrelevant" path: the catch-up run must get far enough to call
	// Connect, not fail immediately trying to read a nonexistent key.
	server, err := domain.NewServer(time.Now(), domain.NewServerParams{
		Name: "prod", Host: "1.2.3.4", Username: "deploy",
		AuthType: domain.AuthPrivateKey, PrivateKeyPath: writeFixtureKey(t),
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
		Schedule:         domain.Schedule{Type: domain.ScheduleDaily, Hour: 3, Minute: 0, MissedRunPolicy: domain.MissedRunAsSoonAsPossible},
		LocalDestination: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("domain.NewJob() error = %v", err)
	}
	if err := store.NewJobRepository(db).Create(ctx, job); err != nil {
		t.Fatalf("create job: %v", err)
	}
	runs := store.NewRunRepository(db)
	lastRunAt := time.Now().Add(-48 * time.Hour)
	lastRun, err := domain.NewRun(lastRunAt, job.ID, server.ID, nil, domain.TriggerSchedule)
	if err != nil {
		t.Fatalf("domain.NewRun() error = %v", err)
	}
	lastRun.Finish(lastRunAt, domain.RunSuccess)
	if err := runs.Create(ctx, lastRun); err != nil {
		t.Fatalf("seed last run: %v", err)
	}

	// The catch-up run's connector blocks until the test releases it,
	// simulating a slow backup: a regression that blocks startup on
	// DetectMissedRuns before starting IPC would make waitForPipe below
	// time out. Released explicitly (not via t.Cleanup) before the test
	// ends: App.Shutdown waits for in-flight runs, so it must not race a
	// t.Cleanup-ordered close of this channel.
	blockCatchUp := make(chan struct{})
	releaseCatchUp := sync.OnceFunc(func() { close(blockCatchUp) })
	t.Cleanup(releaseCatchUp)
	fakeConnector := func(context.Context, backup.ConnectParams) (backup.Transport, error) {
		<-blockCatchUp
		return backuptest.New(), nil
	}

	pipeName := fmt.Sprintf(`\\.\pipe\vpsbackupmanager-startup-test-%d`, time.Now().UnixNano())

	app, err := buildApp(dataDir, db, "test", fakeConnector, noopNotifier{}, pipeName,
		secrets.NewRedactor(), silentLogger(), noopCloser{},
		store.NewServerRepository(db), store.NewJobRepository(db), runs,
		store.NewStepRepository(db), store.NewSettingsRepository(db), store.NewLockRepository(db),
		secrets.NewStore(t.TempDir()),
	)
	if err != nil {
		t.Fatalf("buildApp() error = %v", err)
	}
	t.Cleanup(app.Shutdown)

	conn, err := waitForPipe(t, pipeName)
	if err != nil {
		t.Fatalf("IPC pipe did not become available while a missed-run catch-up was still in flight: %v", err)
	}
	defer func() { _ = conn.Close() }()
	codec := ipc.NewCodec(conn)
	sendRequest(t, codec, "1", ipc.CmdServiceStatus, nil)

	// Let the catch-up run finish before app.Shutdown (a later t.Cleanup)
	// waits for it.
	releaseCatchUp()
}
