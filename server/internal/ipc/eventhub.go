package ipc

import "sync"

// eventBufferSize bounds how many undelivered events a slow subscriber can
// accumulate before new events are dropped for it.
const eventBufferSize = 64

// EventHub broadcasts events to every currently connected session. A
// reconnecting client resynchronizes by re-reading current state through
// the normal request/response commands (e.g. runs.get), not by replaying
// missed events, so a dropped event for a slow consumer is not a
// correctness problem.
type EventHub struct {
	mu   sync.Mutex
	subs map[chan Envelope]struct{}
}

// NewEventHub constructs an empty EventHub.
func NewEventHub() *EventHub {
	return &EventHub{subs: make(map[chan Envelope]struct{})}
}

// Subscribe registers a new subscriber and returns its event channel and
// an unsubscribe function. The caller must call unsubscribe exactly once
// when done.
func (h *EventHub) Subscribe() (<-chan Envelope, func()) {
	ch := make(chan Envelope, eventBufferSize)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()

	unsubscribe := func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if _, ok := h.subs[ch]; ok {
			delete(h.subs, ch)
			close(ch)
		}
	}
	return ch, unsubscribe
}

// Publish sends env to every current subscriber. A subscriber whose
// buffer is full has the event dropped for it rather than blocking
// Publish or other subscribers.
func (h *EventHub) Publish(env Envelope) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs {
		select {
		case ch <- env:
		default:
		}
	}
}

// SubscriberCount reports how many sessions are currently subscribed,
// for tests and diagnostics.
func (h *EventHub) SubscriberCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.subs)
}
