package remote

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/pkg/sftp"
)

// pipeReadWriteCloser adapts a pair of io.Pipe halves into the single
// io.ReadWriteCloser sftp.NewServer/NewClientPipe expect, closing both
// halves so neither side is left blocked on a Read that will never
// complete.
type pipeReadWriteCloser struct {
	io.Reader
	io.WriteCloser
}

func (p pipeReadWriteCloser) Close() error {
	rc, _ := p.Reader.(io.Closer)
	writeErr := p.WriteCloser.Close()
	var readErr error
	if rc != nil {
		readErr = rc.Close()
	}
	if writeErr != nil {
		return writeErr
	}
	return readErr
}

// newInMemorySFTPClient starts a real *sftp.Server serving root over an
// in-memory pipe and returns a connected *sftp.Client, so Client.Download
// can be tested against the real wire protocol without a real SSH
// connection or network. pkg/sftp does not chroot, so callers must use
// paths relative to root (no leading "/").
func newInMemorySFTPClient(t *testing.T, root string) *sftp.Client {
	t.Helper()

	serverRead, clientWrite := io.Pipe()
	clientRead, serverWrite := io.Pipe()

	server, err := sftp.NewServer(
		pipeReadWriteCloser{serverRead, serverWrite},
		sftp.WithServerWorkingDirectory(root),
	)
	if err != nil {
		t.Fatalf("sftp.NewServer() error = %v", err)
	}
	serveDone := make(chan struct{})
	go func() {
		defer close(serveDone)
		_ = server.Serve()
	}()

	client, err := sftp.NewClientPipe(clientRead, clientWrite)
	if err != nil {
		t.Fatalf("sftp.NewClientPipe() error = %v", err)
	}

	// Cleanups run LIFO: close the server (and its half of the pipes)
	// first, so the client's blocked read unblocks with EOF instead of
	// the client waiting forever for a server that is waiting for it.
	t.Cleanup(func() { _ = client.Close() })
	t.Cleanup(func() {
		_ = server.Close()
		<-serveDone
	})
	return client
}

func TestClient_Download_Success(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	content := bytes.Repeat([]byte("archive-bytes-"), 1000)
	if err := os.WriteFile(filepath.Join(root, "backup.tar.gz"), content, 0o600); err != nil {
		t.Fatalf("write fixture file: %v", err)
	}

	sftpClient := newInMemorySFTPClient(t, root)
	c := &Client{sftp: sftpClient}

	var buf bytes.Buffer
	if err := c.Download(context.Background(), "backup.tar.gz", &buf); err != nil {
		t.Fatalf("Download() error = %v", err)
	}
	if !bytes.Equal(buf.Bytes(), content) {
		t.Errorf("downloaded %d bytes, want %d bytes matching the fixture", buf.Len(), len(content))
	}
}

func TestClient_Download_CancellationStopsTransfer(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// Large enough that the download loop, which checks ctx between 1 MiB
	// chunks, gets more than one chunk to work through.
	content := bytes.Repeat([]byte("x"), 5*downloadChunkSize)
	if err := os.WriteFile(filepath.Join(root, "big.tar.gz"), content, 0o600); err != nil {
		t.Fatalf("write fixture file: %v", err)
	}

	sftpClient := newInMemorySFTPClient(t, root)
	c := &Client{sftp: sftpClient}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled: the very first loop iteration must stop

	var buf bytes.Buffer
	err := c.Download(ctx, "big.tar.gz", &buf)
	if err == nil {
		t.Fatal("expected Download() to fail for an already-cancelled context")
	}
	if buf.Len() >= len(content) {
		t.Errorf("downloaded the full file despite cancellation: got %d bytes", buf.Len())
	}
}

func TestClient_Download_MissingFile(t *testing.T) {
	t.Parallel()
	sftpClient := newInMemorySFTPClient(t, t.TempDir())
	c := &Client{sftp: sftpClient}

	var buf bytes.Buffer
	err := c.Download(context.Background(), "does-not-exist.tar.gz", &buf)
	if err == nil {
		t.Fatal("expected an error for a missing remote file")
	}
}
