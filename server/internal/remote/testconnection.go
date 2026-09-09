package remote

import (
	"context"
	"fmt"
	"net"
	"strings"

	"golang.org/x/crypto/ssh"

	"vpsbackupmanager/internal/domain"
)

// Stage is the point in the connection sequence a Test Connection check
// reached or failed at.
type Stage string

const (
	// StageTCP is TCP reachability.
	StageTCP Stage = "TCP"
	// StageHostKey is the SSH handshake and host key verification.
	StageHostKey Stage = "HOST_KEY"
	// StageAuth is SSH authentication.
	StageAuth Stage = "AUTH"
	// StageCommand is execution of the harmless diagnostic command.
	StageCommand Stage = "COMMAND"
)

// TestResult is the outcome of TestConnection: either Success, or
// FailedStage/Err naming where it stopped. PresentedHostKey is filled
// whenever the handshake reached the host-key step, regardless of outcome,
// so the caller can drive trust-on-first-use confirmation or show a
// changed-key warning.
type TestResult struct {
	Success bool

	FailedStage Stage
	Err         error

	PresentedHostKey *HostKeyInfo
	RemoteOSInfo     string
	SSHServerVersion string
}

// TestConnection checks, in order, TCP reachability, the SSH handshake and
// host key verification, authentication, and execution of a harmless
// command (`uname -sr`), never mutating the remote host.
//
// When p.TrustedFingerprint is empty, this is the trust-on-first-use flow:
// the handshake deliberately aborts right after the host key is presented,
// before any authentication method runs, and the result carries the
// presented fingerprint for the UI to show a confirmation dialog. A caller
// confirms it by calling TestConnection again with TrustedFingerprint set
// to the value the user confirmed.
func TestConnection(ctx context.Context, p ConnectionParams) TestResult {
	authMethod, err := buildAuthMethod(p)
	if err != nil {
		return TestResult{FailedStage: StageAuth, Err: err}
	}

	var presented HostKeyInfo
	config := &ssh.ClientConfig{
		User:            p.Username,
		Auth:            []ssh.AuthMethod{authMethod},
		HostKeyCallback: newHostKeyCallback(p.TrustedFingerprint, &presented),
		Timeout:         p.ConnectTimeout,
	}
	addr := p.addr()

	dialer := &net.Dialer{Timeout: p.ConnectTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return TestResult{
			FailedStage: StageTCP,
			Err:         domain.NewCodedError(domain.ErrSSHConnectionFailed, fmt.Sprintf("could not reach %s", addr), err),
		}
	}

	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		if code, ok := domain.CodeOf(err); ok && (code == domain.ErrSSHHostKeyUnverified || code == domain.ErrSSHHostKeyChanged) {
			return TestResult{FailedStage: StageHostKey, Err: err, PresentedHostKey: &presented}
		}
		return TestResult{
			FailedStage:      StageAuth,
			Err:              domain.NewCodedError(domain.ErrSSHAuthFailed, "SSH authentication failed", err),
			PresentedHostKey: &presented,
		}
	}
	client := ssh.NewClient(sshConn, chans, reqs)
	defer func() { _ = client.Close() }()

	serverVersion := string(client.ServerVersion())

	session, err := client.NewSession()
	if err != nil {
		return TestResult{
			FailedStage: StageCommand, Err: fmt.Errorf("open diagnostic session: %w", err),
			PresentedHostKey: &presented, SSHServerVersion: serverVersion,
		}
	}
	defer func() { _ = session.Close() }()

	out, err := session.CombinedOutput("uname -sr")
	if err != nil {
		return TestResult{
			FailedStage: StageCommand, Err: fmt.Errorf("run diagnostic command: %w", err),
			PresentedHostKey: &presented, SSHServerVersion: serverVersion,
		}
	}

	return TestResult{
		Success:          true,
		PresentedHostKey: &presented,
		RemoteOSInfo:     strings.TrimSpace(string(out)),
		SSHServerVersion: serverVersion,
	}
}
