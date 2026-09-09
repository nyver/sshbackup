package remote

import (
	"testing"

	"golang.org/x/crypto/ssh"

	"vpsbackupmanager/internal/domain"
)

func testPublicKey(t *testing.T) ssh.PublicKey {
	t.Helper()
	signer := newEd25519Signer(t)
	return signer.PublicKey()
}

func TestVerifyHostKey_NoTrustedFingerprint(t *testing.T) {
	t.Parallel()
	key := testPublicKey(t)

	var presented HostKeyInfo
	err := verifyHostKey("", &presented, key)

	code, ok := domain.CodeOf(err)
	if !ok || code != domain.ErrSSHHostKeyUnverified {
		t.Fatalf("verifyHostKey() error = %v, want SSH_HOST_KEY_UNVERIFIED", err)
	}
	if presented.Fingerprint == "" {
		t.Error("expected the presented key to still be captured before rejecting")
	}
}

func TestVerifyHostKey_Matching(t *testing.T) {
	t.Parallel()
	key := testPublicKey(t)
	fingerprint := ssh.FingerprintSHA256(key)

	var presented HostKeyInfo
	err := verifyHostKey(fingerprint, &presented, key)
	if err != nil {
		t.Fatalf("verifyHostKey() error = %v, want nil for a matching fingerprint", err)
	}
	if presented.Fingerprint != fingerprint {
		t.Errorf("presented.Fingerprint = %q, want %q", presented.Fingerprint, fingerprint)
	}
}

func TestVerifyHostKey_Mismatch(t *testing.T) {
	t.Parallel()
	key := testPublicKey(t)

	var presented HostKeyInfo
	err := verifyHostKey("SHA256:not-the-real-fingerprint", &presented, key)

	code, ok := domain.CodeOf(err)
	if !ok || code != domain.ErrSSHHostKeyChanged {
		t.Fatalf("verifyHostKey() error = %v, want SSH_HOST_KEY_CHANGED", err)
	}
	// The mismatch must abort before authentication: verifyHostKey itself
	// never runs an auth step, so this test also documents that the
	// caller (Dial/TestConnection) has no path to Auth once this errors.
}

func TestVerifyHostKey_NeverAcceptsSilently(t *testing.T) {
	t.Parallel()
	// Regression guard: an empty trusted fingerprint must never be treated
	// as "accept anything" (the InsecureIgnoreHostKey mistake).
	key := testPublicKey(t)
	if err := verifyHostKey("", nil, key); err == nil {
		t.Fatal("verifyHostKey with no trusted fingerprint must always return an error")
	}
}
