package remote

import "vpsbackupmanager/internal/domain"

// cappedWriter accumulates up to domain.MaxStepOutputBytes and silently
// discards anything past the cap, recording that it did so. It keeps
// memory bounded regardless of how much a remote command writes to
// stdout/stderr (specification: memory-bounded execution).
type cappedWriter struct {
	limit     int
	buf       []byte
	truncated bool
}

func newCappedWriter(limit int) *cappedWriter {
	return &cappedWriter{limit: limit}
}

func (w *cappedWriter) Write(p []byte) (int, error) {
	remaining := w.limit - len(w.buf)
	if remaining <= 0 {
		w.truncated = true
		return len(p), nil
	}
	if len(p) > remaining {
		w.buf = append(w.buf, p[:remaining]...)
		w.truncated = true
		return len(p), nil
	}
	w.buf = append(w.buf, p...)
	return len(p), nil
}

func (w *cappedWriter) String() string {
	return string(w.buf)
}

// newStepCappedWriter returns a cappedWriter bounded at
// domain.MaxStepOutputBytes, the limit shared with run-history storage.
func newStepCappedWriter() *cappedWriter {
	return newCappedWriter(domain.MaxStepOutputBytes)
}
