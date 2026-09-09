// Package applog sets up structured logging split into an
// application/service log and per-run backup logs, per the run-history
// specification's log-separation requirement. The application/service log
// is rotated by size and age so it cannot grow without bound; per-run logs
// are naturally bounded by that run's finite steps and the 10 MiB
// per-step output cap, so they are not separately rotated.
package applog

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"gopkg.in/natefinch/lumberjack.v2"

	"vpsbackupmanager/internal/secrets"
)

const (
	maxSizeMB    = 20 // rotate the app/service log after it reaches this size
	maxAgeDays   = 30 // delete rotated app/service log files older than this
	maxBackups   = 10
	runsSubdir   = "runs"
	serviceLogFN = "service.log"
)

// NewAppLogger opens (creating if needed) dataDir/service.log as a
// rotating, redacting structured logger for both application and Windows
// Service lifecycle messages, which in this single-binary design share
// one process and are treated as one log stream. Call the returned
// io.Closer during shutdown to flush the last write.
func NewAppLogger(dataDir string, redactor *secrets.Redactor, level slog.Level) (*slog.Logger, io.Closer, error) {
	logsDir := filepath.Join(dataDir, "logs")
	if err := os.MkdirAll(logsDir, 0o750); err != nil {
		return nil, nil, fmt.Errorf("create logs directory %q: %w", logsDir, err)
	}

	rotator := &lumberjack.Logger{
		Filename: filepath.Join(logsDir, serviceLogFN),
		MaxSize:  maxSizeMB, MaxAge: maxAgeDays, MaxBackups: maxBackups, Compress: true,
	}

	base := slog.NewJSONHandler(rotator, &slog.HandlerOptions{Level: level})
	var handler slog.Handler = base
	if redactor != nil {
		handler = secrets.NewSlogHandler(base, redactor)
	}
	return slog.New(handler), rotator, nil
}

// RunLogPath returns the per-run log file path for runID under dataDir.
func RunLogPath(dataDir, runID string) string {
	return filepath.Join(dataDir, "logs", runsSubdir, runID+".log")
}

// OpenRunLog creates (if needed) the runs log subdirectory and opens
// runID's log file for appending.
func OpenRunLog(dataDir, runID string) (*os.File, error) {
	dir := filepath.Join(dataDir, "logs", runsSubdir)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("create run logs directory %q: %w", dir, err)
	}
	path := RunLogPath(dataDir, runID)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600) //nolint:gosec // path is built from a server-generated run id, not external input
	if err != nil {
		return nil, fmt.Errorf("open run log %q: %w", path, err)
	}
	return f, nil
}

// noopCloser adapts a plain io.Writer that has no Close method.
type noopCloser struct{ io.Writer }

func (noopCloser) Close() error { return nil }

// NewDiscardLogger returns a logger that writes nowhere, for tests and
// contexts where NewAppLogger's file I/O is undesired.
func NewDiscardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(noopCloser{io.Discard}, nil))
}
