package ipc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

// maxLineBytes bounds one JSON message, so a malformed or hostile client
// cannot exhaust memory with an unbounded line.
const maxLineBytes = 16 * 1024 * 1024

// Codec reads and writes newline-delimited JSON Envelopes over conn. Reads
// happen from a single goroutine (the connection's read loop); writes may
// happen concurrently from several goroutines (request replies and
// pushed events), so Write is internally serialized.
type Codec struct {
	scanner *bufio.Scanner

	writeMu sync.Mutex
	writer  io.Writer
}

// NewCodec wraps rw for envelope-at-a-time reads and writes.
func NewCodec(rw io.ReadWriter) *Codec {
	scanner := bufio.NewScanner(rw)
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineBytes)
	return &Codec{scanner: scanner, writer: rw}
}

// ReadEnvelope reads and decodes the next line as an Envelope. It returns
// io.EOF when the peer closed the connection.
func (c *Codec) ReadEnvelope() (Envelope, error) {
	if !c.scanner.Scan() {
		if err := c.scanner.Err(); err != nil {
			return Envelope{}, fmt.Errorf("read envelope: %w", err)
		}
		return Envelope{}, io.EOF
	}
	var env Envelope
	if err := json.Unmarshal(c.scanner.Bytes(), &env); err != nil {
		return Envelope{}, fmt.Errorf("decode envelope: %w", err)
	}
	return env, nil
}

// WriteEnvelope encodes env as one JSON line. Safe for concurrent use.
func (c *Codec) WriteEnvelope(env Envelope) error {
	data, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("encode envelope: %w", err)
	}
	data = append(data, '\n')

	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if _, err := c.writer.Write(data); err != nil {
		return fmt.Errorf("write envelope: %w", err)
	}
	return nil
}

// MarshalPayload encodes v as a json.RawMessage for an Envelope's Payload
// field.
func MarshalPayload(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		// v is always one of this package's own DTOs; a marshal failure
		// here means a programming error (e.g. an unmarshalable type),
		// not a runtime condition callers can meaningfully recover from.
		panic(fmt.Sprintf("ipc: marshal payload: %v", err))
	}
	return data
}

// UnmarshalPayload decodes a request/event's raw Payload into v.
func UnmarshalPayload(payload json.RawMessage, v any) error {
	if len(payload) == 0 {
		return fmt.Errorf("empty payload")
	}
	if err := json.Unmarshal(payload, v); err != nil {
		return fmt.Errorf("decode payload: %w", err)
	}
	return nil
}

func boolPtr(b bool) *bool { return &b }

// NewRequest builds a request Envelope.
func NewRequest(id, command string, payload any) Envelope {
	return Envelope{
		Type: MessageRequest, ID: id, ProtocolVersion: ProtocolVersion,
		Command: command, Payload: MarshalPayload(payload),
	}
}

// NewSuccessResponse builds a response Envelope for a successful request.
func NewSuccessResponse(id string, payload any) Envelope {
	return Envelope{Type: MessageResponse, ID: id, Success: boolPtr(true), Payload: MarshalPayload(payload)}
}

// NewErrorResponse builds a response Envelope for a failed request.
func NewErrorResponse(id, code, message string) Envelope {
	return Envelope{Type: MessageResponse, ID: id, Success: boolPtr(false), Error: &ErrorPayload{Code: code, Message: message}}
}

// NewEvent builds an event Envelope.
func NewEvent(name string, payload any) Envelope {
	return Envelope{Type: MessageEvent, Event: name, Payload: MarshalPayload(payload)}
}
