package remote

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"vpsbackupmanager/internal/domain"
)

func TestIsRetryable(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"connection failed", domain.NewCodedError(domain.ErrSSHConnectionFailed, "unreachable", nil), true},
		{"auth failed", domain.NewCodedError(domain.ErrSSHAuthFailed, "bad passphrase", nil), false},
		{"host key unverified", domain.NewCodedError(domain.ErrSSHHostKeyUnverified, "no fingerprint", nil), false},
		{"host key changed", domain.NewCodedError(domain.ErrSSHHostKeyChanged, "mismatch", nil), false},
		{"uncoded error", errors.New("some plain network error"), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := isRetryable(tt.err); got != tt.want {
				t.Errorf("isRetryable(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// fakeClock records every requested Sleep duration and returns
// immediately, so retry-backoff tests run in milliseconds instead of
// 5+15+30 real seconds.
type fakeClock struct {
	now    time.Time
	sleeps []time.Duration
}

func (c *fakeClock) Now() time.Time { return c.now }

func (c *fakeClock) Sleep(ctx context.Context, d time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.sleeps = append(c.sleeps, d)
	return nil
}

func TestDial_AuthFailureIsNotRetried(t *testing.T) {
	t.Parallel()
	_, clientKeyPEM := newEd25519KeyPair(t)
	srv := startTCPTestServer(t, &ssh.ServerConfig{
		PublicKeyCallback: func(ssh.ConnMetadata, ssh.PublicKey) (*ssh.Permissions, error) {
			return nil, fmt.Errorf("access denied")
		},
	}, nil)

	host, portStr, err := net.SplitHostPort(srv.Addr)
	if err != nil {
		t.Fatalf("split host/port: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}

	clock := &fakeClock{}
	_, err = Dial(context.Background(), ConnectionParams{
		Host: host, Port: port, Username: "test",
		PrivateKeyPEM:      clientKeyPEM,
		TrustedFingerprint: ssh.FingerprintSHA256(srv.HostKey.PublicKey()),
		ConnectTimeout:     2 * time.Second,
	}, clock)

	code, ok := domain.CodeOf(err)
	if !ok || code != domain.ErrSSHAuthFailed {
		t.Fatalf("Dial() error = %v, want SSH_AUTH_FAILED", err)
	}
	if len(clock.sleeps) != 0 {
		t.Errorf("expected no retry backoff for an auth failure, got %v", clock.sleeps)
	}
	if got := srv.AcceptedConns(); got != 1 {
		t.Errorf("server accepted %d connections, want exactly 1 (no retry)", got)
	}
}

func TestDial_HostKeyUnverifiedIsNotRetried(t *testing.T) {
	t.Parallel()
	srv := startTCPTestServer(t, &ssh.ServerConfig{NoClientAuth: true}, nil)
	host, portStr, err := net.SplitHostPort(srv.Addr)
	if err != nil {
		t.Fatalf("split host/port: %v", err)
	}
	port, _ := strconv.Atoi(portStr)
	_, clientKeyPEM := newEd25519KeyPair(t)

	clock := &fakeClock{}
	_, err = Dial(context.Background(), ConnectionParams{
		Host: host, Port: port, Username: "test",
		PrivateKeyPEM:  clientKeyPEM,
		ConnectTimeout: 2 * time.Second,
		// TrustedFingerprint intentionally left empty.
	}, clock)

	code, ok := domain.CodeOf(err)
	if !ok || code != domain.ErrSSHHostKeyUnverified {
		t.Fatalf("Dial() error = %v, want SSH_HOST_KEY_UNVERIFIED", err)
	}
	if len(clock.sleeps) != 0 {
		t.Errorf("expected no retry backoff for an unverified host key, got %v", clock.sleeps)
	}
}

func TestDial_RetriesTransportFailure(t *testing.T) {
	t.Parallel()
	// Nothing is listening on this loopback port, so every attempt fails
	// at the TCP dial step, which must be retried per the specification.
	_, clientKeyPEM := newEd25519KeyPair(t)
	clock := &fakeClock{}
	_, err := Dial(context.Background(), ConnectionParams{
		Host: "127.0.0.1", Port: 1, Username: "test",
		PrivateKeyPEM:      clientKeyPEM,
		TrustedFingerprint: "SHA256:irrelevant",
		ConnectTimeout:     200 * time.Millisecond,
	}, clock)

	code, ok := domain.CodeOf(err)
	if !ok || code != domain.ErrSSHConnectionFailed {
		t.Fatalf("Dial() error = %v, want SSH_CONNECTION_FAILED", err)
	}
	if len(clock.sleeps) != len(retryDelays) {
		t.Fatalf("recorded sleeps = %v, want %d entries matching retryDelays", clock.sleeps, len(retryDelays))
	}
	for i, want := range retryDelays {
		if clock.sleeps[i] != want {
			t.Errorf("sleeps[%d] = %v, want %v", i, clock.sleeps[i], want)
		}
	}
}

func TestDial_RetryDelaysMatchSpecification(t *testing.T) {
	t.Parallel()
	want := []time.Duration{5 * time.Second, 15 * time.Second, 30 * time.Second}
	if len(retryDelays) != len(want) {
		t.Fatalf("retryDelays = %v, want %v", retryDelays, want)
	}
	for i, d := range want {
		if retryDelays[i] != d {
			t.Errorf("retryDelays[%d] = %v, want %v", i, retryDelays[i], d)
		}
	}
}

func TestDial_ConnectionParams_Addr(t *testing.T) {
	t.Parallel()
	p := ConnectionParams{Host: "203.0.113.5", Port: 2222}
	if got, want := p.addr(), "203.0.113.5:2222"; got != want {
		t.Errorf("addr() = %q, want %q", got, want)
	}
}

func TestDial_PasswordAuthFailureIsNotRetried(t *testing.T) {
	t.Parallel()
	srv := startTCPTestServer(t, &ssh.ServerConfig{
		PasswordCallback: func(ssh.ConnMetadata, []byte) (*ssh.Permissions, error) {
			return nil, fmt.Errorf("access denied")
		},
	}, nil)

	host, portStr, err := net.SplitHostPort(srv.Addr)
	if err != nil {
		t.Fatalf("split host/port: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}

	clock := &fakeClock{}
	_, err = Dial(context.Background(), ConnectionParams{
		Host: host, Port: port, Username: "test",
		AuthType:           domain.AuthPassword,
		Password:           "wrong-password",
		TrustedFingerprint: ssh.FingerprintSHA256(srv.HostKey.PublicKey()),
		ConnectTimeout:     2 * time.Second,
	}, clock)

	code, ok := domain.CodeOf(err)
	if !ok || code != domain.ErrSSHAuthFailed {
		t.Fatalf("Dial() error = %v, want SSH_AUTH_FAILED", err)
	}
	if len(clock.sleeps) != 0 {
		t.Errorf("expected no retry backoff for a password auth failure, got %v", clock.sleeps)
	}
}

func TestBuildAuthMethod_PasswordAuthRequiresPassword(t *testing.T) {
	t.Parallel()
	_, err := buildAuthMethod(ConnectionParams{AuthType: domain.AuthPassword})
	code, ok := domain.CodeOf(err)
	if !ok || code != domain.ErrSSHAuthFailed {
		t.Fatalf("buildAuthMethod() error = %v, want SSH_AUTH_FAILED", err)
	}
}
