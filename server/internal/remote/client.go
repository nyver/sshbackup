// Package remote implements backup.Transport over SSH and SFTP
// (golang.org/x/crypto/ssh, github.com/pkg/sftp): dialing with retry, host
// key verification with trust-on-first-use, capped command output, and
// streaming download.
package remote

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"vpsbackupmanager/internal/backup"
	"vpsbackupmanager/internal/domain"
)

// downloadChunkSize bounds how much of a single Download call is held in
// memory at once.
const downloadChunkSize = 1024 * 1024 // 1 MiB

// Client is the production backup.Transport implementation over SSH and
// SFTP.
type Client struct {
	ssh            *ssh.Client
	sftp           *sftp.Client
	defaultTimeout time.Duration
}

var _ backup.Transport = (*Client)(nil)

// Run executes cmd in a new SSH session, honoring timeout (or the client's
// default when timeout is zero). Output is captured through a writer
// capped at domain.MaxStepOutputBytes per stream.
func (c *Client) Run(ctx context.Context, cmd string, timeout time.Duration) (backup.RunResult, error) {
	if timeout <= 0 {
		timeout = c.defaultTimeout
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	session, err := c.ssh.NewSession()
	if err != nil {
		return backup.RunResult{}, fmt.Errorf("open ssh session: %w", err)
	}
	defer func() { _ = session.Close() }()

	stdout := newStepCappedWriter()
	stderr := newStepCappedWriter()
	session.Stdout = stdout
	session.Stderr = stderr

	start := time.Now()
	if err := session.Start(cmd); err != nil {
		return backup.RunResult{}, fmt.Errorf("start command: %w", err)
	}

	done := make(chan error, 1)
	go func() { done <- session.Wait() }()

	var runErr error
	select {
	case <-runCtx.Done():
		_ = session.Signal(ssh.SIGKILL)
		_ = session.Close()
		<-done // release the goroutine
		return backup.RunResult{
				Stdout: stdout.String(), Stderr: stderr.String(),
				StdoutTruncated: stdout.truncated, StderrTruncated: stderr.truncated,
				Duration: time.Since(start),
			},
			fmt.Errorf("command timed out after %s: %w", timeout, runCtx.Err())
	case runErr = <-done:
	}

	result := backup.RunResult{
		Stdout: stdout.String(), Stderr: stderr.String(),
		StdoutTruncated: stdout.truncated, StderrTruncated: stderr.truncated,
		Duration: time.Since(start),
	}

	var exitErr *ssh.ExitError
	switch {
	case runErr == nil:
		result.ExitCode = 0
	case errors.As(runErr, &exitErr):
		result.ExitCode = exitErr.ExitStatus()
	default:
		return result, fmt.Errorf("run command: %w", runErr)
	}
	return result, nil
}

// Download streams remotePath from the server to w in bounded chunks,
// honoring ctx cancellation so a dropped connection or user cancellation
// stops the transfer promptly instead of buffering the whole file first.
func (c *Client) Download(ctx context.Context, remotePath string, w io.Writer) error {
	src, err := c.sftp.Open(remotePath)
	if err != nil {
		return domain.NewCodedError(domain.ErrDownloadFailed, fmt.Sprintf("open remote file %q", remotePath), err)
	}
	defer func() { _ = src.Close() }()

	buf := make([]byte, downloadChunkSize)
	for {
		select {
		case <-ctx.Done():
			return domain.NewCodedError(domain.ErrDownloadFailed, "download cancelled", ctx.Err())
		default:
		}

		n, readErr := src.Read(buf)
		if n > 0 {
			if _, writeErr := w.Write(buf[:n]); writeErr != nil {
				return domain.NewCodedError(domain.ErrDownloadFailed, "write downloaded data locally", writeErr)
			}
		}
		if readErr == io.EOF {
			return nil
		}
		if readErr != nil {
			return domain.NewCodedError(domain.ErrDownloadFailed, fmt.Sprintf("read remote file %q", remotePath), readErr)
		}
	}
}

// Close releases the SFTP and SSH connections.
func (c *Client) Close() error {
	sftpErr := c.sftp.Close()
	sshErr := c.ssh.Close()
	return errors.Join(sftpErr, sshErr)
}
