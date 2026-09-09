package remote

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"vpsbackupmanager/internal/domain"
)

// retryDelays are applied between connection attempts, per the
// server-management specification: 3 attempts total, with 5s/15s/30s
// delays between them.
var retryDelays = []time.Duration{5 * time.Second, 15 * time.Second, 30 * time.Second}

// ConnectionParams carries everything needed to dial a server. Secret
// material (Passphrase) must already be resolved by the caller (via
// internal/secrets) — this package never touches the credential store.
type ConnectionParams struct {
	Host     string
	Port     int
	Username string

	PrivateKeyPEM []byte
	Passphrase    string // empty when the key has no passphrase

	// TrustedFingerprint is the SHA-256 fingerprint confirmed for this
	// server. Dial refuses to proceed when it is empty: unattended runs
	// never perform trust-on-first-use (see TestConnection for that flow).
	TrustedFingerprint string

	ConnectTimeout time.Duration
	CommandTimeout time.Duration // default per-command timeout when Run's timeout is zero
}

func (p ConnectionParams) addr() string {
	return net.JoinHostPort(p.Host, strconv.Itoa(p.Port))
}

// buildAuthMethod parses the private key, decrypting it with Passphrase
// when set.
func buildAuthMethod(p ConnectionParams) (ssh.AuthMethod, error) {
	var (
		signer ssh.Signer
		err    error
	)
	if p.Passphrase != "" {
		signer, err = ssh.ParsePrivateKeyWithPassphrase(p.PrivateKeyPEM, []byte(p.Passphrase))
	} else {
		signer, err = ssh.ParsePrivateKey(p.PrivateKeyPEM)
	}
	if err != nil {
		return nil, domain.NewCodedError(domain.ErrSSHAuthFailed, "could not parse the configured private key", err)
	}
	return ssh.PublicKeys(signer), nil
}

// Dial establishes an SSH+SFTP connection for backup-engine use, retrying
// transport-level failures up to 3 times with 5s/15s/30s delays. Auth
// failures and host key errors are never retried. p.TrustedFingerprint
// must already hold a confirmed fingerprint; Dial performs no
// trust-on-first-use.
func Dial(ctx context.Context, p ConnectionParams, clock Clock) (*Client, error) {
	authMethod, err := buildAuthMethod(p)
	if err != nil {
		return nil, err
	}
	if p.TrustedFingerprint == "" {
		return nil, domain.NewCodedError(domain.ErrSSHHostKeyUnverified,
			"no trusted host key fingerprint is stored for this server", nil)
	}

	config := &ssh.ClientConfig{
		User:            p.Username,
		Auth:            []ssh.AuthMethod{authMethod},
		HostKeyCallback: newHostKeyCallback(p.TrustedFingerprint, nil),
		Timeout:         p.ConnectTimeout,
	}
	addr := p.addr()

	var lastErr error
	for attempt := 0; attempt <= len(retryDelays); attempt++ {
		if attempt > 0 {
			if err := clock.Sleep(ctx, retryDelays[attempt-1]); err != nil {
				return nil, err
			}
		}

		sshClient, err := dialOnce(ctx, addr, p.ConnectTimeout, config)
		if err == nil {
			sftpClient, err := sftp.NewClient(sshClient)
			if err != nil {
				_ = sshClient.Close()
				return nil, fmt.Errorf("open sftp session to %s: %w", addr, err)
			}
			return &Client{ssh: sshClient, sftp: sftpClient, defaultTimeout: p.CommandTimeout}, nil
		}

		lastErr = err
		if !isRetryable(err) {
			return nil, err
		}
	}
	return nil, fmt.Errorf("connect to %s after %d attempts: %w", addr, len(retryDelays)+1, lastErr)
}

// dialOnce performs one TCP dial plus SSH handshake attempt. A TCP-level
// failure is always retryable; a handshake-level failure is retryable only
// when it is not our own host-key CodedError or an authentication
// rejection (see isRetryable).
func dialOnce(ctx context.Context, addr string, timeout time.Duration, config *ssh.ClientConfig) (*ssh.Client, error) {
	dialer := &net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, domain.NewCodedError(domain.ErrSSHConnectionFailed, fmt.Sprintf("could not reach %s", addr), err)
	}

	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		if code, ok := domain.CodeOf(err); ok && (code == domain.ErrSSHHostKeyUnverified || code == domain.ErrSSHHostKeyChanged) {
			return nil, err
		}
		return nil, domain.NewCodedError(domain.ErrSSHAuthFailed, "SSH authentication failed", err)
	}
	return ssh.NewClient(sshConn, chans, reqs), nil
}

// isRetryable reports whether a Dial attempt failure should be retried:
// transport-level failures are, authentication and host key failures are
// not.
func isRetryable(err error) bool {
	code, ok := domain.CodeOf(err)
	if !ok {
		return true
	}
	switch code {
	case domain.ErrSSHAuthFailed, domain.ErrSSHHostKeyChanged, domain.ErrSSHHostKeyUnverified:
		return false
	default:
		return true
	}
}
