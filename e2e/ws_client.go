//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/coder/websocket"
)

// WSEvent is a parsed WebSocket event from the engine.
// Pipeline events have a nested "data" field. Direct responses (like
// command_result) have fields at the top level. The Raw map holds
// all top-level fields for either case.
type WSEvent struct {
	Type      string         `json:"type"`
	SessionID string         `json:"session_id"`
	Timestamp int64          `json:"timestamp"`
	Data      map[string]any `json:"data"`
	Raw       map[string]any `json:"-"`
}

// WSClient wraps a WebSocket connection to the engine.
type WSClient struct {
	conn *websocket.Conn
}

// NewWSClient connects to the engine WebSocket at the given URL.
func NewWSClient(url string) (*WSClient, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		return nil, fmt.Errorf("ws connect %s: %w", url, err)
	}
	conn.SetReadLimit(10 << 20) // 10 MB
	return &WSClient{conn: conn}, nil
}

// SendMessage sends a chat message to a session.
func (ws *WSClient) SendMessage(sessionID, content string) error {
	msg := map[string]string{
		"type":       "message",
		"session_id": sessionID,
		"content":    content,
		"mode":       "normal",
	}
	data, _ := json.Marshal(msg)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return ws.conn.Write(ctx, websocket.MessageText, data)
}

// SendOTRMessage sends a message in OTR mode.
func (ws *WSClient) SendOTRMessage(sessionID, content string) error {
	msg := map[string]string{
		"type":       "message",
		"session_id": sessionID,
		"content":    content,
		"mode":       "otr",
	}
	data, _ := json.Marshal(msg)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return ws.conn.Write(ctx, websocket.MessageText, data)
}

// SendCommand sends a slash command.
func (ws *WSClient) SendCommand(sessionID, command string) error {
	msg := map[string]string{
		"type":       "command",
		"session_id": sessionID,
		"content":    command,
	}
	data, _ := json.Marshal(msg)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return ws.conn.Write(ctx, websocket.MessageText, data)
}

// CollectUntil reads events until it sees the given event type or times out.
// Returns all events collected including the terminal event.
func (ws *WSClient) CollectUntil(eventType string, timeout time.Duration) ([]WSEvent, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var events []WSEvent
	for {
		_, data, err := ws.conn.Read(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return events, fmt.Errorf("timeout waiting for %q (got %d events)", eventType, len(events))
			}
			return events, fmt.Errorf("ws read: %w", err)
		}
		ev := parseWSEvent(data)
		if ev.Type == "" {
			continue
		}
		events = append(events, ev)
		if ev.Type == eventType {
			return events, nil
		}
	}
}

// CollectFor reads events for the given duration and returns all collected.
func (ws *WSClient) CollectFor(d time.Duration) []WSEvent {
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()

	var events []WSEvent
	for {
		_, data, err := ws.conn.Read(ctx)
		if err != nil {
			return events
		}
		ev := parseWSEvent(data)
		if ev.Type != "" {
			events = append(events, ev)
		}
	}
}

// Close closes the WebSocket connection.
func (ws *WSClient) Close() {
	_ = ws.conn.Close(websocket.StatusNormalClosure, "test done")
}

// FindEvent returns the first event of the given type, or nil.
func FindEvent(events []WSEvent, eventType string) *WSEvent {
	for i := range events {
		if events[i].Type == eventType {
			return &events[i]
		}
	}
	return nil
}

// CountEvents returns how many events of the given type exist.
func CountEvents(events []WSEvent, eventType string) int {
	n := 0
	for _, ev := range events {
		if ev.Type == eventType {
			n++
		}
	}
	return n
}

// parseWSEvent unmarshals a raw JSON WebSocket frame. Handles both
// pipeline events (with nested "data") and direct responses (with
// fields at the top level like command_result).
func parseWSEvent(data []byte) WSEvent {
	var raw map[string]any
	if json.Unmarshal(data, &raw) != nil {
		return WSEvent{}
	}
	var ev WSEvent
	ev.Type, _ = raw["type"].(string)
	ev.SessionID, _ = raw["session_id"].(string)
	if ts, ok := raw["timestamp"].(float64); ok {
		ev.Timestamp = int64(ts)
	}
	if d, ok := raw["data"].(map[string]any); ok {
		ev.Data = d
	} else {
		ev.Data = raw
	}
	ev.Raw = raw
	return ev
}
