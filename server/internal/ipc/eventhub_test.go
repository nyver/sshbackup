package ipc

import (
	"testing"
	"time"
)

func TestEventHub_DeliversToMultipleSubscribers(t *testing.T) {
	t.Parallel()
	hub := NewEventHub()

	ch1, unsub1 := hub.Subscribe()
	defer unsub1()
	ch2, unsub2 := hub.Subscribe()
	defer unsub2()

	if got := hub.SubscriberCount(); got != 2 {
		t.Fatalf("SubscriberCount() = %d, want 2", got)
	}

	env := NewEvent(EventSettingsChanged, SettingsChangedEvent{Settings: SettingsDTO{GlobalConcurrencyLimit: 3}})
	hub.Publish(env)

	for i, ch := range []<-chan Envelope{ch1, ch2} {
		select {
		case got := <-ch:
			if got.Event != EventSettingsChanged {
				t.Errorf("subscriber %d received Event = %q, want %q", i, got.Event, EventSettingsChanged)
			}
		case <-time.After(time.Second):
			t.Errorf("subscriber %d did not receive the published event", i)
		}
	}
}

func TestEventHub_UnsubscribeStopsDelivery(t *testing.T) {
	t.Parallel()
	hub := NewEventHub()
	ch, unsub := hub.Subscribe()
	unsub()

	hub.Publish(NewEvent(EventRunStarted, RunStartedEvent{}))

	if _, ok := <-ch; ok {
		t.Error("expected the channel to be closed after unsubscribe")
	}
	if got := hub.SubscriberCount(); got != 0 {
		t.Errorf("SubscriberCount() = %d, want 0", got)
	}
}

func TestEventHub_SlowSubscriberDoesNotBlockOthers(t *testing.T) {
	t.Parallel()
	hub := NewEventHub()

	slow, unsubSlow := hub.Subscribe()
	defer unsubSlow()
	fast, unsubFast := hub.Subscribe()
	defer unsubFast()

	// Fill the slow subscriber's buffer without ever draining it.
	for i := 0; i < eventBufferSize+10; i++ {
		hub.Publish(NewEvent(EventRunStarted, RunStartedEvent{}))
	}

	select {
	case <-fast:
	case <-time.After(time.Second):
		t.Fatal("a slow, undrained subscriber must not block delivery to other subscribers")
	}
	_ = slow // deliberately never read from
}
