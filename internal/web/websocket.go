package web

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/openparallax/openparallax/crypto"
	"github.com/openparallax/openparallax/internal/commands"
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

	// Read loop: process client messages. Message handling runs in a
	// goroutine so the read loop stays responsive for pings and control
	// frames. Without this, long-running pipeline calls block the read
	// loop and the WebSocket connection can go stale.
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
			go s.handleWSMessage(ctx, conn, msg)
		case "command":
			s.handleWSCommand(ctx, conn, msg)
		case "cancel":
			s.handleCancel(msg.SessionID)
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
		// OTR sessions never touch the database.
		if mode != types.SessionOTR {
			if dbErr := s.engine.DB().InsertSession(&types.Session{
				ID:   sid,
				Mode: mode,
			}); dbErr != nil {
				s.log.Warn("ws_session_create_failed", "session", sid, "error", dbErr)
			}
		}
		s.log.Info("ws_session_auto_created", "session", sid, "mode", mode)
		s.writeWSJSON(ctx, conn, map[string]any{
			"type":    "session_created",
			"session": map[string]any{"id": sid, "mode": string(mode), "created_at": time.Now()},
		})
	}

	sender := s.getOrCreateSender(conn, ctx)

	// Create a cancellable context for this message so the stop button works.
	msgCtx, msgCancel := context.WithCancel(ctx)
	s.sessionCancels.Store(sid, msgCancel)
	defer func() {
		s.sessionCancels.Delete(sid)
		msgCancel()
	}()

	s.log.Info("ws_message_received", "session", sid, "message_id", mid, "content_len", len(msg.Content))
	if err := s.engine.ProcessMessageForWeb(msgCtx, sender, sid, mid, msg.Content, mode); err != nil {
		if msgCtx.Err() != nil {
			s.log.Info("ws_message_cancelled", "session", sid)
		} else {
			s.log.Error("ws_process_failed", "session", sid, "error", err)
			s.writeWSError(ctx, conn, sid, mid, "PIPELINE_FAILED", err.Error())
		}
	}
	s.log.Info("ws_message_done", "session", sid, "message_id", mid)
}

// handleWSCommand executes a slash command via the centralized registry.
func (s *Server) handleWSCommand(ctx context.Context, conn *websocket.Conn, msg wsClientMessage) {
	cmdCtx := &commands.Context{
		Channel:   commands.ChannelWeb,
		SessionID: msg.SessionID,
		Engine:    s.cmdEngine,
	}

	result, handled := s.cmdRegistry.Execute(msg.Content, cmdCtx)
	if !handled {
		return
	}

	s.log.Info("ws_command_executed", "session", msg.SessionID,
		"command", strings.Fields(msg.Content)[0], "action", result.Action)

	resp := map[string]any{
		"type":       "command_result",
		"session_id": msg.SessionID,
		"text":       result.Text,
		"action":     int(result.Action),
	}
	s.writeWSJSON(ctx, conn, resp)
}

// handleCancel cancels an active message processing for a session.
func (s *Server) handleCancel(sessionID string) {
	if fn, ok := s.sessionCancels.LoadAndDelete(sessionID); ok {
		if cancel, castOK := fn.(context.CancelFunc); castOK {
			s.log.Info("ws_cancel_requested", "session", sessionID)
			cancel()
		}
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
