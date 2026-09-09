package ipc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// fixturesDir points at the cross-language contract fixtures in
// /protocol/fixtures, shared with the Flutter client's own Dart tests.
func fixturesDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", "..", "..", "protocol", "fixtures"))
	if err != nil {
		t.Fatalf("resolve fixtures dir: %v", err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("fixtures dir %q not found: %v", dir, err)
	}
	return dir
}

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(fixturesDir(t), name)) //nolint:gosec // fixture path is built from a fixed test-local directory, not external input
	if err != nil {
		t.Fatalf("read fixture %q: %v", name, err)
	}
	return data
}

func TestFixtures_EnvelopeRoundTrip(t *testing.T) {
	t.Parallel()
	names := []string{
		"request_servers_list.json",
		"response_servers_list_success.json",
		"response_error_not_found.json",
		"response_error_version_mismatch.json",
		"request_runs_start.json",
		"response_runs_start_success.json",
		"event_run_started.json",
		"event_run_step_changed.json",
		"event_run_finished.json",
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			data := loadFixture(t, name)

			var env Envelope
			if err := json.Unmarshal(data, &env); err != nil {
				t.Fatalf("unmarshal fixture: %v", err)
			}
			if env.Type == "" {
				t.Error("fixture is missing a \"type\" field")
			}

			// Re-marshal and unmarshal again: the envelope shape must be
			// stable under a round trip, which is what both Go and Dart
			// implementations are validated against.
			reEncoded, err := json.Marshal(env)
			if err != nil {
				t.Fatalf("re-marshal envelope: %v", err)
			}
			var roundTripped Envelope
			if err := json.Unmarshal(reEncoded, &roundTripped); err != nil {
				t.Fatalf("unmarshal re-marshaled envelope: %v", err)
			}
			if roundTripped.Type != env.Type || roundTripped.ID != env.ID || roundTripped.Event != env.Event {
				t.Errorf("round trip changed envelope identity: got %+v, want %+v", roundTripped, env)
			}
		})
	}
}

func TestFixtures_RequestServersList_DecodesAsRequest(t *testing.T) {
	t.Parallel()
	var env Envelope
	if err := json.Unmarshal(loadFixture(t, "request_servers_list.json"), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env.Type != MessageRequest || env.Command != CmdServersList || env.ProtocolVersion != ProtocolVersion {
		t.Errorf("envelope = %+v, want a servers.list request at the current protocol version", env)
	}
}

func TestFixtures_ResponseServersListSuccess_PayloadDecodes(t *testing.T) {
	t.Parallel()
	var env Envelope
	if err := json.Unmarshal(loadFixture(t, "response_servers_list_success.json"), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env.Success == nil || !*env.Success {
		t.Fatal("expected success = true")
	}
	var resp ListServersResponse
	if err := UnmarshalPayload(env.Payload, &resp); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if len(resp.Servers) != 1 || resp.Servers[0].Name != "prod" {
		t.Errorf("Servers = %+v, want one server named %q", resp.Servers, "prod")
	}
	if resp.Servers[0].HasCredential != true {
		t.Error("expected has_credential = true")
	}
}

func TestFixtures_ErrorResponses_HaveStructuredError(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		fixture  string
		wantCode string
	}{
		{"response_error_not_found.json", "NOT_FOUND"},
		{"response_error_version_mismatch.json", "PROTOCOL_VERSION_MISMATCH"},
	} {
		t.Run(tt.fixture, func(t *testing.T) {
			t.Parallel()
			var env Envelope
			if err := json.Unmarshal(loadFixture(t, tt.fixture), &env); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if env.Success == nil || *env.Success {
				t.Fatal("expected success = false")
			}
			if env.Error == nil || env.Error.Code != tt.wantCode {
				t.Errorf("Error = %+v, want code %q", env.Error, tt.wantCode)
			}
		})
	}
}

func TestFixtures_RunEvents_DecodePayloads(t *testing.T) {
	t.Parallel()

	t.Run("run.started", func(t *testing.T) {
		t.Parallel()
		var env Envelope
		if err := json.Unmarshal(loadFixture(t, "event_run_started.json"), &env); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		var payload RunStartedEvent
		if err := UnmarshalPayload(env.Payload, &payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.Run.Status != "RUNNING" {
			t.Errorf("Run.Status = %q, want RUNNING", payload.Run.Status)
		}
	})

	t.Run("run.stepChanged", func(t *testing.T) {
		t.Parallel()
		var env Envelope
		if err := json.Unmarshal(loadFixture(t, "event_run_step_changed.json"), &env); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		var payload RunStepChangedEvent
		if err := UnmarshalPayload(env.Payload, &payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.Step.Type != "ARCHIVE" || payload.Step.Status != "SUCCESS" {
			t.Errorf("Step = %+v, want ARCHIVE/SUCCESS", payload.Step)
		}
	})

	t.Run("run.finished", func(t *testing.T) {
		t.Parallel()
		var env Envelope
		if err := json.Unmarshal(loadFixture(t, "event_run_finished.json"), &env); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		var payload RunFinishedEvent
		if err := UnmarshalPayload(env.Payload, &payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.Run.Status != "SUCCESS" || payload.Run.ArchiveSize == 0 {
			t.Errorf("Run = %+v, want a successful run with a non-zero archive size", payload.Run)
		}
	})
}
