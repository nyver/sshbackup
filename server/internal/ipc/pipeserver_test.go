package ipc

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/Microsoft/go-winio"
)

func startTestPipeServer(t *testing.T) (pipeName string, server *Server, router *Router, hub *EventHub) {
	t.Helper()
	pipeName = fmt.Sprintf(`\\.\pipe\vpsbackupmanager-test-%s`, t.Name()+randSuffix())
	router = NewRouter(nil)
	hub = NewEventHub()
	server = NewServer(router, hub, nil)

	ctx, cancel := context.WithCancel(context.Background())
	serveErrCh := make(chan error, 1)
	go func() { serveErrCh <- server.Serve(ctx, pipeName) }()
	t.Cleanup(func() {
		cancel()
		<-serveErrCh
	})

	// Give the listener a moment to be ready to accept.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := winio.DialPipeContext(context.Background(), pipeName)
		if err == nil {
			_ = conn.Close()
			return pipeName, server, router, hub
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("pipe %q never became ready to accept", pipeName)
	return "", nil, nil, nil
}

var randCounter int

// randSuffix keeps concurrent test pipe names unique without pulling in
// math/rand for what is purely a name-collision avoidance need.
func randSuffix() string {
	randCounter++
	return fmt.Sprintf("-%d-%d", time.Now().UnixNano(), randCounter)
}

func TestServer_RequestResponseRoundTrip(t *testing.T) {
	t.Parallel()
	pipeName, _, router, _ := startTestPipeServer(t)
	router.Handle("echo", func(_ context.Context, raw json.RawMessage) (any, error) {
		var payload struct {
			Message string `json:"message"`
		}
		if err := UnmarshalPayload(raw, &payload); err != nil {
			return nil, err
		}
		return payload, nil
	})

	conn, err := winio.DialPipeContext(context.Background(), pipeName)
	if err != nil {
		t.Fatalf("DialPipeContext() error = %v", err)
	}
	defer func() { _ = conn.Close() }()

	codec := NewCodec(conn)
	req := NewRequest("r1", "echo", struct {
		Message string `json:"message"`
	}{Message: "hello"})
	if err := codec.WriteEnvelope(req); err != nil {
		t.Fatalf("WriteEnvelope() error = %v", err)
	}

	resp, err := codec.ReadEnvelope()
	if err != nil {
		t.Fatalf("ReadEnvelope() error = %v", err)
	}
	if resp.ID != "r1" || resp.Success == nil || !*resp.Success {
		t.Fatalf("resp = %+v, want a successful response to r1", resp)
	}
}

func TestServer_EventDeliveryToMultipleClients(t *testing.T) {
	t.Parallel()
	pipeName, _, _, hub := startTestPipeServer(t)

	const clientCount = 3
	codecs := make([]*Codec, clientCount)
	for i := range codecs {
		conn, err := winio.DialPipeContext(context.Background(), pipeName)
		if err != nil {
			t.Fatalf("DialPipeContext() error = %v", err)
		}
		t.Cleanup(func() { _ = conn.Close() })
		codecs[i] = NewCodec(conn)
	}

	// Give each connection's Session a moment to subscribe before
	// publishing, since Subscribe happens at the start of Session.Serve.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && hub.SubscriberCount() < clientCount {
		time.Sleep(10 * time.Millisecond)
	}
	if got := hub.SubscriberCount(); got != clientCount {
		t.Fatalf("SubscriberCount() = %d, want %d", got, clientCount)
	}

	hub.Publish(NewEvent(EventSettingsChanged, SettingsChangedEvent{Settings: SettingsDTO{GlobalConcurrencyLimit: 5}}))

	for i, c := range codecs {
		env, err := c.ReadEnvelope()
		if err != nil {
			t.Fatalf("client %d ReadEnvelope() error = %v", i, err)
		}
		if env.Type != MessageEvent || env.Event != EventSettingsChanged {
			t.Errorf("client %d received %+v, want a settings.changed event", i, env)
		}
	}
}
