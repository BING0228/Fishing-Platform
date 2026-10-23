// Package chatbus is a process-local pub/sub for chat session events. Async
// jobs (info gathering) publish progress here; the chat SSE endpoint streams it
// into the same conversation.
package chatbus

import "sync"

// Event is one SSE event destined for a chat session.
type Event struct {
	Name    string
	Payload any
}

type hub struct {
	mu   sync.RWMutex
	subs map[uint]map[uint]chan Event
	next uint
}

var defaultHub = &hub{subs: map[uint]map[uint]chan Event{}}

const subscriberBuffer = 64

// Publish fans an event out to every subscriber of the session. Slow
// subscribers drop events instead of blocking the publisher.
func Publish(sessionID uint, name string, payload any) {
	if sessionID == 0 {
		return
	}
	defaultHub.mu.RLock()
	defer defaultHub.mu.RUnlock()
	for _, ch := range defaultHub.subs[sessionID] {
		select {
		case ch <- Event{Name: name, Payload: payload}:
		default:
		}
	}
}

// Subscribe registers a listener and returns its channel plus an unsubscribe
// function. The channel is closed by unsubscribe.
func Subscribe(sessionID uint) (<-chan Event, func()) {
	defaultHub.mu.Lock()
	defer defaultHub.mu.Unlock()
	if defaultHub.subs[sessionID] == nil {
		defaultHub.subs[sessionID] = map[uint]chan Event{}
	}
	defaultHub.next++
	id := defaultHub.next
	ch := make(chan Event, subscriberBuffer)
	defaultHub.subs[sessionID][id] = ch
	return ch, func() {
		defaultHub.mu.Lock()
		defer defaultHub.mu.Unlock()
		if subs, ok := defaultHub.subs[sessionID]; ok {
			if existing, ok := subs[id]; ok {
				delete(subs, id)
				close(existing)
			}
			if len(subs) == 0 {
				delete(defaultHub.subs, sessionID)
			}
		}
	}
}
