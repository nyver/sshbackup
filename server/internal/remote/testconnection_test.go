package remote

import (
	"context"
	"errors"
	"net"
	"strconv"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"vpsbackupmanager/internal/domain"
)

var errWrongPassword = errors.New("wrong password")

func TestTestConnection_PasswordAuthSucceeds(t *testing.T) {
	t.Parallel()
	srv := startTCPTestServer(t, &ssh.ServerConfig{
		PasswordCallback: func(_ ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			if string(password) != "correct-horse" {
				return nil, errWrongPassword
			}
			return nil, nil
		},
	}, func(cmd string) (int, string, string, time.Duration) {
		if cmd == "uname -sr" {
			return 0, "Linux 6.1.0\n", "", 0
		}
		return 1, "", "unexpected command", 0
	})

	host, portStr, err := net.SplitHostPort(srv.Addr)
	if err != nil {
		t.Fatalf("split host/port: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}

	result := TestConnection(context.Background(), ConnectionParams{
		Host: host, Port: port, Username: "test",
		AuthType:           domain.AuthPassword,
		Password:           "correct-horse",
		TrustedFingerprint: ssh.FingerprintSHA256(srv.HostKey.PublicKey()),
		ConnectTimeout:     2 * time.Second,
	})

	if !result.Success {
		t.Fatalf("TestConnection() with correct password: Success = false, FailedStage = %v, Err = %v", result.FailedStage, result.Err)
	}
	if result.RemoteOSInfo != "Linux 6.1.0" {
		t.Errorf("RemoteOSInfo = %q, want %q", result.RemoteOSInfo, "Linux 6.1.0")
	}
}

func TestTestConnection_PasswordAuthFailure(t *testing.T) {
	t.Parallel()
	srv := startTCPTestServer(t, &ssh.ServerConfig{
		PasswordCallback: func(ssh.ConnMetadata, []byte) (*ssh.Permissions, error) {
			return nil, errWrongPassword
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

	result := TestConnection(context.Background(), ConnectionParams{
		Host: host, Port: port, Username: "test",
		AuthType:           domain.AuthPassword,
		Password:           "wrong-password",
		TrustedFingerprint: ssh.FingerprintSHA256(srv.HostKey.PublicKey()),
		ConnectTimeout:     2 * time.Second,
	})

	if result.Success {
		t.Fatal("TestConnection() with wrong password: Success = true, want false")
	}
	if result.FailedStage != StageAuth {
		t.Errorf("FailedStage = %v, want %v", result.FailedStage, StageAuth)
	}
}
