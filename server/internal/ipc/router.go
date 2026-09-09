package ipc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"vpsbackupmanager/internal/domain"
)

// HandlerFunc handles one decoded request payload and returns the
// response payload, or an error. A *domain.CodedError's Code and Message
// become the response's structured error; any other error is logged
// server-side and reported to the client as a generic internal error, so
// an unexpected internal error message can never leak implementation
// detail or secrets over the wire.
type HandlerFunc func(ctx context.Context, raw json.RawMessage) (any, error)

// Router dispatches requests by command name and turns handler errors
// into structured error responses.
type Router struct {
	handlers map[string]HandlerFunc
	onError  func(command string, err error)
}

// NewRouter constructs an empty Router. onError, if non-nil, is called
// with the raw handler error for server-side logging whenever a handler
// returns an error that is not a *domain.CodedError (so the operator can
// still see what actually happened).
func NewRouter(onError func(command string, err error)) *Router {
	return &Router{handlers: make(map[string]HandlerFunc), onError: onError}
}

// Handle registers fn for command, replacing any existing handler for it.
func (r *Router) Handle(command string, fn HandlerFunc) {
	r.handlers[command] = fn
}

// Dispatch runs the handler registered for env.Command and builds the
// response envelope. A protocol version mismatch or an unregistered
// command is answered without ever reaching a handler, and — per the
// specification — leaves the connection usable for further requests.
func (r *Router) Dispatch(ctx context.Context, env Envelope) Envelope {
	if env.ProtocolVersion != ProtocolVersion {
		return NewErrorResponse(env.ID, ErrVersionMismatch, fmt.Sprintf(
			"protocol version mismatch: service supports %d, client sent %d; please update the application",
			ProtocolVersion, env.ProtocolVersion))
	}

	fn, ok := r.handlers[env.Command]
	if !ok {
		return NewErrorResponse(env.ID, ErrUnsupportedCommand, fmt.Sprintf("unsupported command %q", env.Command))
	}

	result, err := fn(ctx, env.Payload)
	if err != nil {
		var coded *domain.CodedError
		if errors.As(err, &coded) {
			return NewErrorResponse(env.ID, string(coded.Code), coded.Message)
		}
		if r.onError != nil {
			r.onError(env.Command, err)
		}
		return NewErrorResponse(env.ID, string(domain.ErrInternal), "an internal error occurred")
	}
	return NewSuccessResponse(env.ID, result)
}
