package domain

import (
	"errors"
	"fmt"
	"testing"
)

func TestCodedError_ErrorsAs(t *testing.T) {
	t.Parallel()
	base := errors.New("connection refused")
	wrapped := fmt.Errorf("dial server: %w", NewCodedError(ErrSSHConnectionFailed, "could not reach host", base))

	var ce *CodedError
	if !errors.As(wrapped, &ce) {
		t.Fatal("expected errors.As to find the CodedError in the chain")
	}
	if ce.Code != ErrSSHConnectionFailed {
		t.Errorf("Code = %q, want %q", ce.Code, ErrSSHConnectionFailed)
	}
	if !errors.Is(wrapped, base) {
		t.Error("expected the original cause to still be reachable via errors.Is")
	}
}

func TestCodedError_Is(t *testing.T) {
	t.Parallel()
	err := NewCodedError(ErrSSHAuthFailed, "bad passphrase", nil)
	sentinel := &CodedError{Code: ErrSSHAuthFailed}
	if !errors.Is(err, sentinel) {
		t.Error("expected errors.Is to match on Code")
	}

	other := &CodedError{Code: ErrDownloadFailed}
	if errors.Is(err, other) {
		t.Error("expected errors.Is to not match a different Code")
	}
}

func TestCodeOf(t *testing.T) {
	t.Parallel()

	code, ok := CodeOf(NewCodedError(ErrChecksumMismatch, "mismatch", nil))
	if !ok || code != ErrChecksumMismatch {
		t.Errorf("CodeOf() = (%q, %v), want (%q, true)", code, ok, ErrChecksumMismatch)
	}

	if _, ok := CodeOf(errors.New("plain error")); ok {
		t.Error("expected CodeOf to report false for a plain error")
	}
}
