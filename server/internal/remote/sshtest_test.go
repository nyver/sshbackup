package remote

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"encoding/pem"
	"net"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// sessionHandler answers one "exec" request on a session channel.
type sessionHandler func(cmd string) (exitCode int, stdout, stderr string, sleep time.Duration)

// newInMemorySSHClient starts a minimal SSH server on a loopback TCP
// listener and returns a connected *ssh.Client, so Client.Run can be
// tested without a real VPS. A real (if local-only) socket is used rather
// than net.Pipe because the SSH version exchange has both sides write
// before either reads, which deadlocks on net.Pipe's unbuffered,
// synchronous Read/Write pairing. Only "exec" session requests are
// handled; handler controls the result.
func newInMemorySSHClient(t *testing.T, handler sessionHandler) *ssh.Client {
	t.Helper()

	srv := startTCPTestServer(t, &ssh.ServerConfig{NoClientAuth: true}, handler)

	clientConfig := &ssh.ClientConfig{
		User:            "test",
		Auth:            []ssh.AuthMethod{ssh.Password("unused")},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec // test-only loopback listener, not a real network connection
		Timeout:         5 * time.Second,
	}
	client, err := ssh.Dial("tcp", srv.Addr, clientConfig)
	if err != nil {
		t.Fatalf("client dial/handshake: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func serveSSHConn(t *testing.T, conn net.Conn, config *ssh.ServerConfig, handler sessionHandler) {
	t.Helper()
	sshConn, chans, reqs, err := ssh.NewServerConn(conn, config)
	if err != nil {
		return // client closed before/during handshake; nothing to serve
	}
	defer func() { _ = sshConn.Close() }()
	go ssh.DiscardRequests(reqs)

	for newChan := range chans {
		if newChan.ChannelType() != "session" {
			_ = newChan.Reject(ssh.UnknownChannelType, "unsupported channel type")
			continue
		}
		channel, requests, err := newChan.Accept()
		if err != nil {
			continue
		}
		go serveSession(channel, requests, handler)
	}
}

func serveSession(channel ssh.Channel, requests <-chan *ssh.Request, handler sessionHandler) {
	defer func() { _ = channel.Close() }()
	for req := range requests {
		if req.Type != "exec" {
			if req.WantReply {
				_ = req.Reply(false, nil)
			}
			continue
		}
		cmd := decodeExecPayload(req.Payload)
		if req.WantReply {
			_ = req.Reply(true, nil)
		}

		exitCode, stdout, stderr, sleep := 0, "", "", time.Duration(0)
		if handler != nil {
			exitCode, stdout, stderr, sleep = handler(cmd)
		}
		if sleep > 0 {
			time.Sleep(sleep)
		}
		_, _ = channel.Write([]byte(stdout))
		_, _ = channel.Stderr().Write([]byte(stderr))
		_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{uint32(exitCode)})) //nolint:gosec // exitCode is a small test-controlled int
		return
	}
}

// decodeExecPayload parses the SSH "exec" request payload: a single
// uint32-length-prefixed string carrying the command text.
func decodeExecPayload(payload []byte) string {
	if len(payload) < 4 {
		return ""
	}
	n := binary.BigEndian.Uint32(payload[:4])
	if int(n) > len(payload)-4 {
		return ""
	}
	return string(payload[4 : 4+n])
}

// tcpTestServer is a real loopback SSH server, for tests that exercise
// Dial itself (TCP dial + retry + handshake), as opposed to
// newInMemorySSHClient's net.Pipe shortcut used for session-level tests.
type tcpTestServer struct {
	Addr    string
	HostKey ssh.Signer

	acceptedConns int
	mu            sync.Mutex
}

// startTCPTestServer listens on 127.0.0.1:0 and accepts connections with
// config until the test ends. config.AddHostKey is called with a freshly
// generated key, whose fingerprint is exposed via HostKey.
func startTCPTestServer(t *testing.T, config *ssh.ServerConfig, handler sessionHandler) *tcpTestServer {
	t.Helper()
	hostKey := newEd25519Signer(t)
	config.AddHostKey(hostKey)

	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	srv := &tcpTestServer{Addr: ln.Addr().String(), HostKey: hostKey}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			srv.mu.Lock()
			srv.acceptedConns++
			srv.mu.Unlock()
			go serveSSHConn(t, conn, config, handler)
		}
	}()
	return srv
}

func (s *tcpTestServer) AcceptedConns() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.acceptedConns
}

// marshalPrivateKey PEM-encodes priv (an ed25519.PrivateKey) in OpenSSH
// format, for tests that need bytes to hand to ConnectionParams.PrivateKeyPEM.
func marshalPrivateKey(t *testing.T, priv ed25519.PrivateKey) []byte {
	t.Helper()
	block, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		t.Fatalf("MarshalPrivateKey: %v", err)
	}
	return pem.EncodeToMemory(block)
}

// newEd25519KeyPair generates a fresh key pair and returns both the
// ssh.Signer (for server-side host keys or authorized-key checks) and the
// PEM-encoded private key (for ConnectionParams.PrivateKeyPEM).
func newEd25519KeyPair(t *testing.T) (ssh.Signer, []byte) {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate ed25519 key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("build signer: %v", err)
	}
	return signer, marshalPrivateKey(t, priv)
}

func newEd25519Signer(t *testing.T) ssh.Signer {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate ed25519 key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("build signer: %v", err)
	}
	return signer
}
