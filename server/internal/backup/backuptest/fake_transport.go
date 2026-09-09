// Package backuptest provides a scripted fake of backup.Transport for
// tests: it can fail any command, block until context cancellation, or
// return oversized output, without touching a real network.
package backuptest

import (
	"context"
	"io"
	"sync"
	"time"

	"vpsbackupmanager/internal/backup"
)

// FakeTransport is a backup.Transport whose behavior is entirely
// controlled by the RunFunc/DownloadFunc/CloseFunc hooks a test sets. Any
// unset hook behaves as an immediate success.
type FakeTransport struct {
	RunFunc      func(ctx context.Context, cmd string, timeout time.Duration) (backup.RunResult, error)
	DownloadFunc func(ctx context.Context, remotePath string, w io.Writer) error
	CloseFunc    func() error

	mu       sync.Mutex
	commands []string
	closed   bool
}

// New returns a FakeTransport whose hooks default to trivial success; set
// RunFunc/DownloadFunc/CloseFunc to script specific behavior.
func New() *FakeTransport {
	return &FakeTransport{}
}

// Run records cmd and delegates to RunFunc.
func (f *FakeTransport) Run(ctx context.Context, cmd string, timeout time.Duration) (backup.RunResult, error) {
	f.mu.Lock()
	f.commands = append(f.commands, cmd)
	f.mu.Unlock()

	if f.RunFunc == nil {
		return backup.RunResult{ExitCode: 0}, nil
	}
	return f.RunFunc(ctx, cmd, timeout)
}

// Download delegates to DownloadFunc.
func (f *FakeTransport) Download(ctx context.Context, remotePath string, w io.Writer) error {
	if f.DownloadFunc == nil {
		return nil
	}
	return f.DownloadFunc(ctx, remotePath, w)
}

// Close delegates to CloseFunc and records that Close was called.
func (f *FakeTransport) Close() error {
	f.mu.Lock()
	f.closed = true
	f.mu.Unlock()

	if f.CloseFunc == nil {
		return nil
	}
	return f.CloseFunc()
}

// Commands returns every command passed to Run, in call order.
func (f *FakeTransport) Commands() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.commands...)
}

// Closed reports whether Close has been called.
func (f *FakeTransport) Closed() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.closed
}
