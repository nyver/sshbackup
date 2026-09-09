package secrets

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
)

func TestRedactor_Redact(t *testing.T) {
	t.Parallel()
	r := NewRedactor()
	r.Register("hunter2")

	got := r.Redact(`connecting with password "hunter2" to host`)
	want := `connecting with password "***" to host`
	if got != want {
		t.Errorf("Redact() = %q, want %q", got, want)
	}
}

func TestRedactor_UnregisteredValuesUntouched(t *testing.T) {
	t.Parallel()
	r := NewRedactor()

	s := "authentication failed for user deploy"
	if got := r.Redact(s); got != s {
		t.Errorf("Redact() with no registered secrets = %q, want unchanged %q", got, s)
	}
}

func TestRedactor_Unregister(t *testing.T) {
	t.Parallel()
	r := NewRedactor()
	r.Register("hunter2")
	r.Unregister("hunter2")

	s := "password hunter2 accepted"
	if got := r.Redact(s); got != s {
		t.Errorf("Redact() after Unregister() = %q, want unchanged %q", got, s)
	}
}

func TestSlogHandler_RedactsMessageAndAttrs(t *testing.T) {
	t.Parallel()
	r := NewRedactor()
	r.Register("hunter2")

	var buf bytes.Buffer
	base := slog.NewJSONHandler(&buf, nil)
	logger := slog.New(NewSlogHandler(base, r))

	logger.Info("connected using password hunter2", "detail", "passphrase=hunter2")

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("unmarshal log entry: %v", err)
	}
	msg, _ := entry["msg"].(string)
	if msg != "connected using password ***" {
		t.Errorf("msg = %q, want redacted secret", msg)
	}
	detail, _ := entry["detail"].(string)
	if detail != "passphrase=***" {
		t.Errorf("detail attr = %q, want redacted secret", detail)
	}
	if bytes.Contains(buf.Bytes(), []byte("hunter2")) {
		t.Errorf("log output still contains the secret: %s", buf.String())
	}
}

func TestSlogHandler_WithAttrsRedactsBoundAttrs(t *testing.T) {
	t.Parallel()
	r := NewRedactor()
	r.Register("hunter2")

	var buf bytes.Buffer
	base := slog.NewJSONHandler(&buf, nil)
	logger := slog.New(NewSlogHandler(base, r)).With("bound", "token=hunter2")

	logger.Info("ready")

	if bytes.Contains(buf.Bytes(), []byte("hunter2")) {
		t.Errorf("bound attr leaked the secret: %s", buf.String())
	}
}

func TestSlogHandler_Enabled_DelegatesToNext(t *testing.T) {
	t.Parallel()
	r := NewRedactor()
	base := slog.NewJSONHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelWarn})
	h := NewSlogHandler(base, r)

	if h.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("expected Info to be disabled when the base handler is configured for Warn")
	}
	if !h.Enabled(context.Background(), slog.LevelWarn) {
		t.Error("expected Warn to be enabled")
	}
}
