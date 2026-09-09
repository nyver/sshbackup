package remote

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestClient_Run_Success(t *testing.T) {
	t.Parallel()
	sshClient := newInMemorySSHClient(t, func(cmd string) (int, string, string, time.Duration) {
		return 0, "hello from " + cmd, "", 0
	})
	c := &Client{ssh: sshClient, defaultTimeout: 5 * time.Second}

	result, err := c.Run(context.Background(), "echo hi", 0)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}
	if result.Stdout != "hello from echo hi" {
		t.Errorf("Stdout = %q, want %q", result.Stdout, "hello from echo hi")
	}
}

func TestClient_Run_NonZeroExit(t *testing.T) {
	t.Parallel()
	sshClient := newInMemorySSHClient(t, func(_ string) (int, string, string, time.Duration) {
		return 1, "", "boom", 0
	})
	c := &Client{ssh: sshClient, defaultTimeout: 5 * time.Second}

	result, err := c.Run(context.Background(), "false", 0)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", result.ExitCode)
	}
	if result.Stderr != "boom" {
		t.Errorf("Stderr = %q, want %q", result.Stderr, "boom")
	}
}

func TestClient_Run_TimeoutTerminatesCommand(t *testing.T) {
	t.Parallel()
	sshClient := newInMemorySSHClient(t, func(_ string) (int, string, string, time.Duration) {
		return 0, "too slow", "", 2 * time.Second
	})
	c := &Client{ssh: sshClient, defaultTimeout: 5 * time.Second}

	start := time.Now()
	_, err := c.Run(context.Background(), "sleep 2", 100*time.Millisecond)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected a timeout error")
	}
	if elapsed > time.Second {
		t.Errorf("Run() took %v, expected it to return promptly after the 100ms timeout", elapsed)
	}
}

func TestClient_Run_OutputUnderCapRoundTrips(t *testing.T) {
	t.Parallel()
	// Truncation behavior itself is covered by TestCappedWriter; this
	// confirms Run() wires session output through newStepCappedWriter
	// (domain.MaxStepOutputBytes) rather than dropping or mangling it.
	oversized := strings.Repeat("a", 100)
	sshClient := newInMemorySSHClient(t, func(_ string) (int, string, string, time.Duration) {
		return 0, oversized, "", 0
	})
	c := &Client{ssh: sshClient, defaultTimeout: 5 * time.Second}

	result, err := c.Run(context.Background(), "echo", 0)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Stdout != oversized {
		t.Errorf("Stdout length = %d, want %d (under the real 10 MiB cap)", len(result.Stdout), len(oversized))
	}
	if result.StdoutTruncated {
		t.Error("StdoutTruncated = true, want false (well under the cap)")
	}
}
