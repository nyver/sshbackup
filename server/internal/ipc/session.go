package ipc

import (
	"context"
	"errors"
	"io"
)

// Session owns one connected client's read loop (dispatching requests)
// and event delivery (forwarding published events to that client).
type Session struct {
	Codec  *Codec
	Router *Router
	Events *EventHub
}

// Serve runs the session until ctx is cancelled or the connection ends,
// returning the reason (io.EOF for a normal client disconnect).
func (s *Session) Serve(ctx context.Context) error {
	events, unsubscribe := s.Events.Subscribe()
	defer unsubscribe()

	writeErrCh := make(chan error, 1)
	go s.pushEvents(ctx, events, writeErrCh)

	for {
		env, err := s.Codec.ReadEnvelope()
		if err != nil {
			return err
		}
		if env.Type != MessageRequest {
			continue // stray non-request line; ignore rather than tear down the connection
		}

		resp := s.Router.Dispatch(ctx, env)
		if err := s.Codec.WriteEnvelope(resp); err != nil {
			return err
		}

		select {
		case err := <-writeErrCh:
			return err
		default:
		}
	}
}

func (s *Session) pushEvents(ctx context.Context, events <-chan Envelope, errCh chan<- error) {
	for {
		select {
		case <-ctx.Done():
			return
		case env, ok := <-events:
			if !ok {
				return
			}
			if err := s.Codec.WriteEnvelope(env); err != nil {
				select {
				case errCh <- err:
				default:
				}
				return
			}
		}
	}
}

// IsClosed reports whether err represents a normal connection close
// rather than an unexpected failure worth logging loudly.
func IsClosed(err error) bool {
	return errors.Is(err, io.EOF)
}
