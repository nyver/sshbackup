package remote

import (
	"fmt"
	"net"

	"golang.org/x/crypto/ssh"

	"vpsbackupmanager/internal/domain"
)

// HostKeyInfo is the algorithm and SHA-256 fingerprint of a host key
// presented during an SSH handshake, in the same "SHA256:<base64>" form
// OpenSSH itself displays.
type HostKeyInfo struct {
	Algorithm   string
	Fingerprint string
}

// newHostKeyCallback builds an ssh.HostKeyCallback that never falls back to
// ssh.InsecureIgnoreHostKey. It always records the presented key into
// presented (when non-nil) so the caller can report it, regardless of the
// verification outcome.
//
//   - trustedFingerprint == "": the connection is unattended and no
//     fingerprint has ever been confirmed for this server, so the
//     handshake is aborted with SSH_HOST_KEY_UNVERIFIED before any
//     authentication method runs.
//   - trustedFingerprint set and matching: the handshake proceeds.
//   - trustedFingerprint set and mismatching: the handshake is aborted
//     with SSH_HOST_KEY_CHANGED before any authentication method runs.
func newHostKeyCallback(trustedFingerprint string, presented *HostKeyInfo) ssh.HostKeyCallback {
	return func(_ string, _ net.Addr, key ssh.PublicKey) error {
		return verifyHostKey(trustedFingerprint, presented, key)
	}
}

// verifyHostKey is the pure decision logic behind newHostKeyCallback,
// separated out so it can be unit tested without a real SSH handshake.
func verifyHostKey(trustedFingerprint string, presented *HostKeyInfo, key ssh.PublicKey) error {
	info := HostKeyInfo{Algorithm: key.Type(), Fingerprint: ssh.FingerprintSHA256(key)}
	if presented != nil {
		*presented = info
	}

	if trustedFingerprint == "" {
		return domain.NewCodedError(domain.ErrSSHHostKeyUnverified,
			"no trusted host key fingerprint is stored for this server; connect interactively to confirm it first", nil)
	}
	if info.Fingerprint != trustedFingerprint {
		return domain.NewCodedError(domain.ErrSSHHostKeyChanged,
			fmt.Sprintf("host key changed: expected %s, got %s (%s)", trustedFingerprint, info.Fingerprint, info.Algorithm), nil)
	}
	return nil
}
