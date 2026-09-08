package presence

import (
	"testing"
	"time"
)

func waitFor(t *testing.T, ch <-chan Event) Event {
	t.Helper()
	select {
	case ev, ok := <-ch:
		if !ok {
			t.Fatal("stream closed before the event arrived")
		}
		return ev
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the event")
		return Event{}
	}
}

func TestPublishFansOutToEveryStream(t *testing.T) {
	b := NewBroker()
	a := b.Subscribe("p1")
	c := b.Subscribe("p1")
	if got := b.Count("p1"); got != 2 {
		t.Fatalf("count = %d, want 2", got)
	}
	b.Publish("p1", Event{Name: "player", Data: []byte(`{}`)})
	for _, s := range []Sub{a, c} {
		if ev := waitFor(t, s.C()); ev.Name != "player" {
			t.Fatalf("event name = %q, want player", ev.Name)
		}
	}
	// another player's stream stays untouched
	d := b.Subscribe("p2")
	select {
	case ev := <-d.C():
		t.Fatalf("stray event %+v on another player's stream", ev)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestUnsubscribeClosesAndStopsDelivery(t *testing.T) {
	b := NewBroker()
	s := b.Subscribe("p1")
	b.Unsubscribe("p1", s)
	if got := b.Count("p1"); got != 0 {
		t.Fatalf("count = %d, want 0", got)
	}
	b.Publish("p1", Event{Name: "player", Data: []byte(`{}`)})
	if _, ok := <-s.C(); ok {
		t.Fatal("unsubscribed stream should be closed and receive nothing")
	}
	// double unsubscribe is a no-op, not a panic on a closed channel
	b.Unsubscribe("p1", s)
}

func TestSlowStreamIsDroppedNotBlocked(t *testing.T) {
	b := NewBroker()
	s := b.Subscribe("p1")
	for i := 0; i < subBuffer; i++ {
		b.Publish("p1", Event{Name: "fill", Data: []byte(`{}`)})
	}
	// buffer is full: this publish must drop the stream instead of blocking
	done := make(chan struct{})
	go func() {
		b.Publish("p1", Event{Name: "overflow", Data: []byte(`{}`)})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("publish blocked on a full stream buffer")
	}
	if got := b.Count("p1"); got != 0 {
		t.Fatalf("count = %d, want 0 after the drop", got)
	}
	// the buffered events may still drain, but "overflow" is never
	// delivered and the channel eventually reports closed
	for i := 0; i < subBuffer+1; i++ {
		ev, ok := <-s.C()
		if !ok {
			return
		}
		if ev.Name == "overflow" {
			t.Fatal("overflow event delivered to a dropped stream")
		}
	}
	t.Fatal("dropped stream should report closed after draining")
}

func TestPublishWithoutSubscribersIsNoop(t *testing.T) {
	b := NewBroker()
	b.Publish("nobody", Event{Name: "player", Data: []byte(`{}`)})
}
