package main

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"vpsbackupmanager/internal/backup"
	"vpsbackupmanager/internal/backup/backuptest"
	"vpsbackupmanager/internal/ipc"
	"vpsbackupmanager/internal/notify"
	"vpsbackupmanager/internal/secrets"
	"vpsbackupmanager/internal/store"
)

// noopNotifier discards every notification, for the end-to-end test where
// no real toast delivery is possible or desired.
type noopNotifier struct{}

func (noopNotifier) Notify(context.Context, notify.Notification) error { return nil }

// TestEndToEnd_CreateServerCreateJobRunNowHistory wires the full service
// in console-equivalent mode against a fake transport (no real SSH) and
// drives one complete run through the IPC API exactly as the Flutter
// client would: create a server, create a job, trigger Run now, then read
// the persisted result back from history.
func TestEndToEnd_CreateServerCreateJobRunNowHistory(t *testing.T) {
	dataDir := t.TempDir()
	db := openTestDB(t)

	secretsDir := t.TempDir()
	secretsStore := secrets.NewStore(secretsDir)
	redactor := secrets.NewRedactor()

	archiveContent := []byte("end-to-end-fake-archive-bytes")
	hash := sha256HexForTest(archiveContent)
	transport := backuptest.New()
	transport.RunFunc = func(_ context.Context, cmd string, _ time.Duration) (backup.RunResult, error) {
		switch {
		case containsForTest(cmd, "du -sb"):
			return backup.RunResult{ExitCode: 0, Stdout: "100"}, nil
		case containsForTest(cmd, "df -B1"):
			return backup.RunResult{ExitCode: 0, Stdout: "999999999"}, nil
		case containsForTest(cmd, "sha256sum"):
			return backup.RunResult{ExitCode: 0, Stdout: hash}, nil
		default:
			return backup.RunResult{ExitCode: 0}, nil
		}
	}
	transport.DownloadFunc = func(_ context.Context, _ string, w io.Writer) error {
		_, err := w.Write(archiveContent)
		return err
	}
	fakeConnector := func(context.Context, backup.ConnectParams) (backup.Transport, error) {
		return transport, nil
	}

	pipeName := fmt.Sprintf(`\\.\pipe\vpsbackupmanager-e2e-%d`, time.Now().UnixNano())

	app, err := buildApp(dataDir, db, "test", fakeConnector, noopNotifier{}, pipeName,
		redactor, silentLogger(), noopCloser{},
		store.NewServerRepository(db), store.NewJobRepository(db), store.NewRunRepository(db),
		store.NewStepRepository(db), store.NewSettingsRepository(db), store.NewLockRepository(db),
		secretsStore,
	)
	if err != nil {
		t.Fatalf("buildApp() error = %v", err)
	}
	t.Cleanup(app.Shutdown)

	conn, err := waitForPipe(t, pipeName)
	if err != nil {
		t.Fatalf("connect to ipc pipe: %v", err)
	}
	defer func() { _ = conn.Close() }()
	codec := ipc.NewCodec(conn)

	// 1. Create a server.
	createServerResp := sendRequest(t, codec, "1", ipc.CmdServersCreate, ipc.SaveServerRequest{
		Name: "prod", Host: "203.0.113.5", Port: 22, Username: "deploy",
		AuthType: "PRIVATE_KEY", PrivateKeyPath: writeFixtureKey(t),
	})
	var serverResp ipc.SaveServerResponse
	mustDecode(t, createServerResp, &serverResp)
	if serverResp.Server.ID == "" {
		t.Fatal("expected a non-empty server id")
	}

	// 2. Create a job against that server.
	createJobResp := sendRequest(t, codec, "2", ipc.CmdJobsCreate, ipc.SaveJobRequest{
		Job: ipc.JobDTO{
			Name: "beresta", ServerID: serverResp.Server.ID, Enabled: true,
			Sources:               []ipc.SourceDTO{{RemotePath: "/docker/volumes"}},
			Schedule:              ipc.ScheduleDTO{Type: "MANUAL", MissedRunPolicy: "RUN_AS_SOON_AS_POSSIBLE"},
			LocalDestination:      t.TempDir(),
			ArchiveTimeoutSeconds: 7200,
		},
	})
	var jobResp ipc.SaveJobResponse
	mustDecode(t, createJobResp, &jobResp)
	if jobResp.Job.ID == "" {
		t.Fatal("expected a non-empty job id")
	}

	// 3. Run now.
	runNowResp := sendRequest(t, codec, "3", ipc.CmdRunsStart, ipc.RunNowRequest{JobID: jobResp.Job.ID})
	var startResp ipc.RunNowResponse
	mustDecode(t, runNowResp, &startResp)
	if startResp.RunID == "" {
		t.Fatal("expected a non-empty run id")
	}
	if startResp.Status != "RUNNING" && startResp.Status != "PENDING" {
		t.Fatalf("Status = %q immediately after runs.start, want RUNNING or PENDING", startResp.Status)
	}

	// 4. Wait for the run.finished event.
	finishedEvent := waitForRunFinished(t, codec, startResp.RunID)
	if finishedEvent.Run.Status != "SUCCESS" {
		t.Fatalf("run.finished status = %q, want SUCCESS (error: %s / %s)", finishedEvent.Run.Status, finishedEvent.Run.ErrorCode, finishedEvent.Run.ErrorMessage)
	}

	// 5. Read history back via runs.get, asserting the persisted result.
	getRunResp := sendRequest(t, codec, "4", ipc.CmdRunsGet, ipc.GetRunRequest{ID: startResp.RunID})
	var runDetails ipc.GetRunResponse
	mustDecode(t, getRunResp, &runDetails)
	if runDetails.Run.Status != "SUCCESS" {
		t.Errorf("persisted run status = %q, want SUCCESS", runDetails.Run.Status)
	}
	if runDetails.Run.Checksum != hash {
		t.Errorf("persisted checksum = %q, want %q", runDetails.Run.Checksum, hash)
	}
	if len(runDetails.Steps) == 0 {
		t.Error("expected persisted steps for the run")
	}

	listRunsResp := sendRequest(t, codec, "5", ipc.CmdRunsList, ipc.ListRunsRequest{JobID: jobResp.Job.ID})
	var listResp ipc.ListRunsResponse
	mustDecode(t, listRunsResp, &listResp)
	if len(listResp.Runs) != 1 || listResp.Runs[0].ID != startResp.RunID {
		t.Errorf("runs.list = %+v, want exactly the one run just executed", listResp.Runs)
	}
}
