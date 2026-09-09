// Package ipc implements the Named Pipes API between the Flutter UI and
// the backup service: a versioned, newline-delimited JSON request/response
// protocol plus a server-pushed event stream, restricted to the
// interactive user and local administrators.
package ipc

import "encoding/json"

// ProtocolVersion is bumped whenever a message shape changes
// incompatibly. A mismatch between client and server is rejected with
// ErrVersionMismatch rather than acted upon.
const ProtocolVersion = 1

// MessageType discriminates the three kinds of envelope on the wire.
type MessageType string

const (
	// MessageRequest is sent by the client and expects a MessageResponse
	// carrying the same ID.
	MessageRequest MessageType = "request"
	// MessageResponse answers a MessageRequest with the same ID.
	MessageResponse MessageType = "response"
	// MessageEvent is pushed by the server unsolicited; it carries no ID.
	MessageEvent MessageType = "event"
)

// Envelope is the single JSON shape every line on the pipe takes.
// Request: Type=request, ID, ProtocolVersion, Command, Payload.
// Response: Type=response, ID (echoing the request), Success, and either
// Payload (on success) or Error (on failure).
// Event: Type=event, Event, Payload. Events carry no ID or Success.
type Envelope struct {
	Type            MessageType     `json:"type"`
	ID              string          `json:"id,omitempty"`
	ProtocolVersion int             `json:"protocol_version,omitempty"`
	Command         string          `json:"command,omitempty"`
	Event           string          `json:"event,omitempty"`
	Success         *bool           `json:"success,omitempty"`
	Payload         json.RawMessage `json:"payload,omitempty"`
	Error           *ErrorPayload   `json:"error,omitempty"`
}

// ErrorPayload is a structured, stable machine-readable error: a code the
// UI can branch on, and a message it can show as-is, per the ipc-api
// specification's "errors are structured" requirement. It never carries
// secrets, key material, or stack traces.
type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Well-known error codes for protocol-level failures, distinct from the
// domain.ErrorCode vocabulary a handler's own failures use.
const (
	ErrVersionMismatch    = "PROTOCOL_VERSION_MISMATCH"
	ErrUnsupportedCommand = "UNSUPPORTED_COMMAND"
	ErrInvalidPayload     = "INVALID_PAYLOAD"
)

// Command names for the "supported operations" (ipc-api specification).
const (
	CmdServersList           = "servers.list"
	CmdServersCreate         = "servers.create"
	CmdServersUpdate         = "servers.update"
	CmdServersDelete         = "servers.delete"
	CmdServersTestConnection = "servers.testConnection"
	CmdServersConfirmHostKey = "servers.confirmHostKey"

	CmdJobsList     = "jobs.list"
	CmdJobsCreate   = "jobs.create"
	CmdJobsUpdate   = "jobs.update"
	CmdJobsDelete   = "jobs.delete"
	CmdJobsEnable   = "jobs.enable"
	CmdJobsDisable  = "jobs.disable"
	CmdJobsValidate = "jobs.validate"

	CmdRunsStart  = "runs.start"
	CmdRunsCancel = "runs.cancel"
	CmdRunsList   = "runs.list"
	CmdRunsGet    = "runs.get"

	CmdSettingsGet = "settings.get"
	CmdSettingsSet = "settings.set"

	CmdServiceStatus = "service.status"
)

// Event names pushed to connected clients (ipc-api specification's "live
// event stream").
const (
	EventRunStarted      = "run.started"
	EventRunStepChanged  = "run.stepChanged"
	EventRunLogLine      = "run.logLine"
	EventRunFinished     = "run.finished"
	EventSettingsChanged = "settings.changed"
)
