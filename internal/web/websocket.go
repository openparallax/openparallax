package web

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/openparallax/openparallax/internal/crypto"
	"github.com/openparallax/openparallax/internal/types"
)

// wsClientMessage is a message from the WebSocket client.
type wsClientMessage struct {
	Type      string `json:"type"`
	SessionID string `json:"session_id"`
	Content   string `json:"content"`
	Mode      string `json:"mode"`
	ActionID  string `json:"action_id,omitempty"`
	Decision  string `json:"decision,omitempty"`
}

// handleWebSocket upgrades an HTTP connection to WebSocket and handles
// bidirectional communication for the chat interface.
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		s.log.Error("ws_accept_failed", "error", err)
		return
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	s.registerConn(conn, ctx)
	defer s.unregisterConn(conn)

	s.log.Info("ws_connected", "remote", r.RemoteAddr)

	// Read loop: process client messages.
	for {
		_, data, readErr := conn.Read(ctx)
		if readErr != nil {
			if websocket.CloseStatus(readErr) == websocket.StatusNormalClosure {
				s.log.Info("ws_closed_normal", "remote", r.RemoteAddr)
			} else {
				s.log.Debug("ws_read_error", "error", readErr)
			}
			return
		}

		var msg wsClientMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			s.writeWSError(ctx, conn, "", "", "INVALID_JSON", "failed to parse message")
			continue
		}

		switch msg.Type {
		case "message":
			s.handleWSMessage(ctx, conn, msg)
		case "tier3_decision":
			s.handleTier3Decision(msg)
		case "ping":
			s.writeWSJSON(ctx, conn, map[string]string{"type": "pong"})
		default:
			s.writeWSError(ctx, conn, "", "", "UNKNOWN_TYPE", "unknown message type: "+msg.Type)
		}
	}
}

// handleWSMessage processes a chat message through the engine pipeline.
func (s *Server) handleWSMessage(ctx context.Context, conn *websocket.Conn, msg wsClientMessage) {
	sid := msg.SessionID
	mid := "msg-" + crypto.NewID()

	mode := types.SessionNormal
	if msg.Mode == "otr" {
		mode = types.SessionOTR
	}

	// Create session if none provided.
	if sid == "" {
		sid = crypto.NewID()
		_ = s.engine.DB().InsertSession(&types.Session{
			ID:   sid,
			Mode: mode,
		})
		s.writeWSJSON(ctx, conn, map[string]any{
			"type":    "session_created",
			"session": map[string]any{"id": sid, "mode": string(mode), "created_at": time.Now()},
		})
	}

	sender := newWSEventSender(ctx, conn)

	if err := s.engine.ProcessMessageForWeb(ctx, sender, sid, mid, msg.Content, mode); err != nil {
		s.log.Error("ws_process_failed", "session", sid, "error", err)
		s.writeWSError(ctx, conn, sid, mid, "PIPELINE_FAILED", err.Error())
	}
}

// handleTier3Decision resolves a pending human-in-the-loop approval.
func (s *Server) handleTier3Decision(msg wsClientMessage) {
	approved := msg.Decision == "approve"
	if err := s.engine.Tier3().Decide(msg.ActionID, approved); err != nil {
		s.log.Warn("tier3_decision_failed", "action_id", msg.ActionID, "error", err)
		return
	}
	decision := "denied"
	if approved {
		decision = "approved"
	}
	s.log.Info("tier3_event", "action_id", msg.ActionID, "status", decision)
}

func (s *Server) writeWSJSON(ctx context.Context, conn *websocket.Conn, data any) {
	bytes, err := json.Marshal(data)
	if err != nil {
		return
	}
	_ = conn.Write(ctx, websocket.MessageText, bytes)
}

func (s *Server) writeWSError(ctx context.Context, conn *websocket.Conn, sid, mid, code, message string) {
	s.writeWSJSON(ctx, conn, map[string]any{
		"type":       "error",
		"session_id": sid,
		"message_id": mid,
		"error":      map[string]any{"code": code, "message": message},
	})
}
