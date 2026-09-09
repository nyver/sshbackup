package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Microsoft/go-winio"

	"vpsbackupmanager/internal/ipc"
)

func sha256HexForTest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func containsForTest(s, substr string) bool {
	return strings.Contains(s, substr)
}

// writeFixtureKey writes a placeholder private key file. Its content is
// never parsed as a real key: the end-to-end test's fakeConnector ignores
// ConnectParams entirely, but the IPC handler still reads the file from
// disk before calling it, so the path must exist.
func writeFixtureKey(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "id_ed25519")
	if err := os.WriteFile(path, []byte("placeholder-not-a-real-key"), 0o600); err != nil {
		t.Fatalf("write fixture key: %v", err)
	}
	return path
}

func waitForPipe(t *testing.T, pipeName string) (net.Conn, error) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		conn, err := winio.DialPipeContext(context.Background(), pipeName)
		if err == nil {
			return conn, nil
		}
		lastErr = err
		time.Sleep(10 * time.Millisecond)
	}
	return nil, lastErr
}

func sendRequest(t *testing.T, codec *ipc.Codec, id, command string, payload any) ipc.Envelope {
	t.Helper()
	if err := codec.WriteEnvelope(ipc.NewRequest(id, command, payload)); err != nil {
		t.Fatalf("send %s request: %v", command, err)
	}
	for {
		env, err := codec.ReadEnvelope()
		if err != nil {
			t.Fatalf("read response to %s: %v", command, err)
		}
		if env.Type == ipc.MessageResponse && env.ID == id {
			if env.Success == nil || !*env.Success {
				t.Fatalf("%s failed: %+v", command, env.Error)
			}
			return env
		}
		// Any other envelope (e.g. a live event) arriving before our
		// response is expected and ignored here.
	}
}

func mustDecode(t *testing.T, env ipc.Envelope, v any) {
	t.Helper()
	if err := ipc.UnmarshalPayload(env.Payload, v); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
}

func waitForRunFinished(t *testing.T, codec *ipc.Codec, runID string) ipc.RunFinishedEvent {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		env, err := codec.ReadEnvelope()
		if err != nil {
			t.Fatalf("read event while waiting for run.finished: %v", err)
		}
		if env.Type != ipc.MessageEvent || env.Event != ipc.EventRunFinished {
			continue
		}
		var payload ipc.RunFinishedEvent
		if err := json.Unmarshal(env.Payload, &payload); err != nil {
			t.Fatalf("decode run.finished payload: %v", err)
		}
		if payload.Run.ID == runID {
			return payload
		}
	}
	t.Fatalf("timed out waiting for run.finished for run %q", runID)
	return ipc.RunFinishedEvent{}
}
