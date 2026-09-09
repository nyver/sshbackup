package ipc

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"vpsbackupmanager/internal/domain"
)

func TestRouter_VersionMismatch(t *testing.T) {
	t.Parallel()
	r := NewRouter(nil)
	r.Handle("noop", func(context.Context, json.RawMessage) (any, error) { return struct{}{}, nil })

	resp := r.Dispatch(context.Background(), Envelope{
		Type: MessageRequest, ID: "1", ProtocolVersion: ProtocolVersion + 1, Command: "noop",
	})

	if resp.Success == nil || *resp.Success {
		t.Fatal("expected success = false")
	}
	if resp.Error == nil || resp.Error.Code != ErrVersionMismatch {
		t.Errorf("Error = %+v, want code %q", resp.Error, ErrVersionMismatch)
	}
	if resp.ID != "1" {
		t.Errorf("ID = %q, want it to echo the request", resp.ID)
	}
}

func TestRouter_UnknownCommand_KeepsConnectionUsable(t *testing.T) {
	t.Parallel()
	r := NewRouter(nil)
	r.Handle("known", func(context.Context, json.RawMessage) (any, error) { return struct{ OK bool }{true}, nil })

	unknown := r.Dispatch(context.Background(), Envelope{Type: MessageRequest, ID: "1", ProtocolVersion: ProtocolVersion, Command: "does.not.exist"})
	if unknown.Error == nil || unknown.Error.Code != ErrUnsupportedCommand {
		t.Fatalf("Error = %+v, want code %q", unknown.Error, ErrUnsupportedCommand)
	}

	// The same router (standing in for "the connection") must still
	// answer a subsequent, valid request normally.
	known := r.Dispatch(context.Background(), Envelope{Type: MessageRequest, ID: "2", ProtocolVersion: ProtocolVersion, Command: "known"})
	if known.Success == nil || !*known.Success {
		t.Fatalf("expected the next request to succeed after an unsupported command, got %+v", known)
	}
}

func TestRouter_CodedErrorBecomesStructuredResponse(t *testing.T) {
	t.Parallel()
	r := NewRouter(nil)
	r.Handle("get", func(context.Context, json.RawMessage) (any, error) {
		return nil, domain.NewCodedError(domain.ErrNotFound, `job "x" not found`, nil)
	})

	resp := r.Dispatch(context.Background(), Envelope{Type: MessageRequest, ID: "1", ProtocolVersion: ProtocolVersion, Command: "get"})
	if resp.Error == nil || resp.Error.Code != string(domain.ErrNotFound) {
		t.Fatalf("Error = %+v, want code %q", resp.Error, domain.ErrNotFound)
	}
	if resp.Error.Message != `job "x" not found` {
		t.Errorf("Error.Message = %q, want the coded error's own message", resp.Error.Message)
	}
}

func TestRouter_UncodedError_NeverLeaksRawMessage(t *testing.T) {
	t.Parallel()
	var logged error
	r := NewRouter(func(_ string, err error) { logged = err })
	r.Handle("boom", func(context.Context, json.RawMessage) (any, error) {
		return nil, errors.New("permission denied for user 'sa' on database 'internal_secrets'")
	})

	resp := r.Dispatch(context.Background(), Envelope{Type: MessageRequest, ID: "1", ProtocolVersion: ProtocolVersion, Command: "boom"})
	if resp.Error == nil || resp.Error.Code != string(domain.ErrInternal) {
		t.Fatalf("Error = %+v, want code %q", resp.Error, domain.ErrInternal)
	}
	if resp.Error.Message != "an internal error occurred" {
		t.Errorf("Error.Message = %q, an uncoded error's raw text must never reach the client", resp.Error.Message)
	}
	if logged == nil {
		t.Error("expected the raw error to still be reported server-side via onError")
	}
}
