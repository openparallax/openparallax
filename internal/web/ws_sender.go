package web

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/coder/websocket"
	"github.com/openparallax/openparallax/internal/engine"
)

// wsEventSender implements engine.EventSender for WebSocket connections.
// Events are serialized as JSON and sent as text messages.
type wsEventSender struct {
	conn *websocket.Conn
	ctx  context.Context
	mu   sync.Mutex
}

func newWSEventSender(ctx context.Context, conn *websocket.Conn) engine.EventSender {
	return &wsEventSender{conn: conn, ctx: ctx}
}

func (w *wsEventSender) SendEvent(event *engine.PipelineEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.conn.Write(w.ctx, websocket.MessageText, data)
}
