package secrets

import (
	"context"
	"log/slog"
	"strings"
	"sync"
)

// RedactedPlaceholder replaces every occurrence of a registered secret
// value.
const RedactedPlaceholder = "***"

// Redactor holds the set of secret values currently in use (private key
// passphrases, passwords) so logs, captured command output, and
// notifications can replace them centrally instead of relying on every
// call site to remember to do so. It is safe for concurrent use.
type Redactor struct {
	mu      sync.RWMutex
	secrets map[string]struct{}
}

// NewRedactor returns an empty Redactor.
func NewRedactor() *Redactor {
	return &Redactor{secrets: make(map[string]struct{})}
}

// Register adds secret to the set of values that Redact replaces. Empty
// strings are ignored so an unset passphrase never causes every string to
// be redacted.
func (r *Redactor) Register(secret string) {
	if secret == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.secrets[secret] = struct{}{}
}

// Unregister removes secret from the set, e.g. once the run that used it
// has finished, so the set does not grow without bound over the service's
// lifetime.
func (r *Redactor) Unregister(secret string) {
	if secret == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.secrets, secret)
}

// Redact returns s with every registered secret value replaced by
// RedactedPlaceholder.
func (r *Redactor) Redact(s string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for secret := range r.secrets {
		s = strings.ReplaceAll(s, secret, RedactedPlaceholder)
	}
	return s
}

// NewSlogHandler wraps next so every log message and string attribute
// passes through redactor.Redact before being handled, so a secret cannot
// reach a log file just because one call site forgot to redact it.
func NewSlogHandler(next slog.Handler, redactor *Redactor) slog.Handler {
	return &redactingHandler{next: next, redactor: redactor}
}

type redactingHandler struct {
	next     slog.Handler
	redactor *Redactor
}

func (h *redactingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *redactingHandler) Handle(ctx context.Context, record slog.Record) error {
	redacted := slog.NewRecord(record.Time, record.Level, h.redactor.Redact(record.Message), record.PC)
	record.Attrs(func(a slog.Attr) bool {
		redacted.AddAttrs(h.redactAttr(a))
		return true
	})
	return h.next.Handle(ctx, redacted)
}

func (h *redactingHandler) redactAttr(a slog.Attr) slog.Attr {
	a.Value = a.Value.Resolve()
	switch a.Value.Kind() {
	case slog.KindString:
		return slog.String(a.Key, h.redactor.Redact(a.Value.String()))
	case slog.KindGroup:
		group := a.Value.Group()
		redacted := make([]slog.Attr, len(group))
		for i, ga := range group {
			redacted[i] = h.redactAttr(ga)
		}
		return slog.Attr{Key: a.Key, Value: slog.GroupValue(redacted...)}
	default:
		return a
	}
}

func (h *redactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	redacted := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		redacted[i] = h.redactAttr(a)
	}
	return &redactingHandler{next: h.next.WithAttrs(redacted), redactor: h.redactor}
}

func (h *redactingHandler) WithGroup(name string) slog.Handler {
	return &redactingHandler{next: h.next.WithGroup(name), redactor: h.redactor}
}
