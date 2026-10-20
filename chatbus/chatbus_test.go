package chatbus

import (
	"testing"
	"time"
)

func TestPublishSubscribe(t *testing.T) {
	ch, cancel := Subscribe(42)
	defer cancel()

	Publish(42, "job_progress", map[string]any{"job_id": 1})
	select {
	case ev := <-ch:
		if ev.Name != "job_progress" {
			t.Fatalf("event = %q", ev.Name)
		}
		payload, _ := ev.Payload.(map[string]any)
		if payload["job_id"] != 1 {
			t.Fatalf("payload = %+v", payload)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestSessionsAreIsolated(t *testing.T) {
	chA, cancelA := Subscribe(1)
	defer cancelA()
	chB, cancelB := Subscribe(2)
	defer cancelB()

	Publish(1, "job_done", nil)
	select {
	case <-chA:
	case <-time.After(time.Second):
		t.Fatal("session 1 should receive its event")
	}
	select {
	case ev := <-chB:
		t.Fatalf("session 2 leaked event %+v", ev)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestUnsubscribeClosesChannel(t *testing.T) {
	ch, cancel := Subscribe(7)
	cancel()
	if _, open := <-ch; open {
		t.Fatal("channel should be closed after unsubscribe")
	}
}

func TestPublishWithoutSubscribersIsSafe(t *testing.T) {
	Publish(999, "job_progress", nil)
	Publish(0, "job_progress", nil)
}
