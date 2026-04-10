package engine

import (
	"sync"
)

// EventBroadcaster manages fan-out of pipeline events to all connected clients.
// Clients subscribe to specific sessions and receive events for those sessions.
type EventBroadcaster struct {
	mu      sync.RWMutex
	clients map[string]EventSender         // clientID → sender
	subs    map[string]map[string]struct{} // sessionID → set of clientIDs
	global  map[string]struct{}            // clientIDs subscribed to all events
}

// NewEventBroadcaster creates an EventBroadcaster.
func NewEventBroadcaster() *EventBroadcaster {
	return &EventBroadcaster{
		clients: make(map[string]EventSender),
		subs:    make(map[string]map[string]struct{}),
		global:  make(map[string]struct{}),
	}
}

// Subscribe registers a client for events on a specific session.
func (b *EventBroadcaster) Subscribe(clientID, sessionID string, sender EventSender) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.clients[clientID] = sender
	if _, ok := b.subs[sessionID]; !ok {
		b.subs[sessionID] = make(map[string]struct{})
	}
	b.subs[sessionID][clientID] = struct{}{}
}

// SubscribeGlobal registers a client for all events (e.g., log entries).
func (b *EventBroadcaster) SubscribeGlobal(clientID string, sender EventSender) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.clients[clientID] = sender
	b.global[clientID] = struct{}{}
}

// Unsubscribe removes a client from all subscriptions.
func (b *EventBroadcaster) Unsubscribe(clientID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.clients, clientID)
	delete(b.global, clientID)
	for sid, subs := range b.subs {
		delete(subs, clientID)
		if len(subs) == 0 {
			delete(b.subs, sid)
		}
	}
}

// Broadcast sends an event to all clients subscribed to its session,
// plus all globally subscribed clients.
func (b *EventBroadcaster) Broadcast(event *PipelineEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	sent := make(map[string]struct{})

	// Session subscribers.
	if subs, ok := b.subs[event.SessionID]; ok {
		for clientID := range subs {
			if sender, exists := b.clients[clientID]; exists {
				_ = sender.SendEvent(event)
				sent[clientID] = struct{}{}
			}
		}
	}

	// Global subscribers (skip if already sent via session).
	for clientID := range b.global {
		if _, already := sent[clientID]; already {
			continue
		}
		if sender, exists := b.clients[clientID]; exists {
			_ = sender.SendEvent(event)
		}
	}
}
