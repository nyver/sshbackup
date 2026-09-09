package ipc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"

	"github.com/Microsoft/go-winio"
)

// PipeName is the well-known endpoint the Flutter UI connects to.
const PipeName = `\\.\pipe\VPSBackupManager`

// pipeSecurityDescriptor grants full control only to local administrators
// (well-known SID "BA") and interactively logged-on users (well-known SID
// "IU"), denying network and service-account access — the local-only,
// interactive-user-or-admin access control the ipc-api specification
// requires. Windows enforces this at the kernel level when a client calls
// CreateFile on the pipe, so an unauthorized connection attempt never
// reaches Accept: there is no separate application-level rejection path
// to log, only the OS's own security audit log.
const pipeSecurityDescriptor = "D:P(A;;GA;;;BA)(A;;GA;;;IU)"

// Server accepts named pipe connections and runs one Session per
// connection, all sharing the same Router and EventHub.
type Server struct {
	Router *Router
	Events *EventHub
	Logger *slog.Logger

	listener net.Listener
}

// NewServer constructs a Server. Call Serve to start accepting
// connections.
func NewServer(router *Router, events *EventHub, logger *slog.Logger) *Server {
	return &Server{Router: router, Events: events, Logger: logger}
}

func (s *Server) logger() *slog.Logger {
	if s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}

// Serve listens on pipeName and accepts connections until ctx is
// cancelled. It blocks until the listener stops.
func (s *Server) Serve(ctx context.Context, pipeName string) error {
	ln, err := winio.ListenPipe(pipeName, &winio.PipeConfig{
		SecurityDescriptor: pipeSecurityDescriptor,
		MessageMode:        false,
		InputBufferSize:    64 * 1024,
		OutputBufferSize:   64 * 1024,
	})
	if err != nil {
		return fmt.Errorf("listen on named pipe %q: %w", pipeName, err)
	}
	s.listener = ln

	stopped := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = ln.Close()
		case <-stopped:
		}
	}()
	defer close(stopped)

	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("accept pipe connection: %w", err)
		}
		go s.handleConn(ctx, conn)
	}
}

// Close stops the listener immediately, without waiting for ctx.
func (s *Server) Close() error {
	if s.listener == nil {
		return nil
	}
	return s.listener.Close()
}

func (s *Server) handleConn(ctx context.Context, conn net.Conn) {
	defer func() { _ = conn.Close() }()

	session := &Session{Codec: NewCodec(conn), Router: s.Router, Events: s.Events}
	if err := session.Serve(ctx); err != nil && !IsClosed(err) && !errors.Is(err, context.Canceled) {
		s.logger().Warn("ipc session ended unexpectedly", "error", err)
	}
}
