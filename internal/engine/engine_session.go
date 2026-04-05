package engine

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/openparallax/openparallax/audit"
	"github.com/openparallax/openparallax/crypto"
	"github.com/openparallax/openparallax/internal/types"
	"github.com/openparallax/openparallax/llm"
)

func (e *Engine) storeMessage(sessionID, messageID, role, content string) {
	if messageID == "" {
		messageID = fmt.Sprintf("msg-%d", time.Now().UnixNano())
	}
	e.ensureSession(sessionID)
	if err := e.db.InsertMessage(&types.Message{
		ID: messageID, SessionID: sessionID,
		Role: role, Content: content, Timestamp: time.Now(),
	}); err != nil {
		e.log.Warn("store_message_failed", "session", sessionID, "role", role, "error", err)
	}
}

// storeAssistantMessage saves an assistant message with thoughts (reasoning + tool calls).
func (e *Engine) storeAssistantMessage(sessionID string, msg *types.Message) {
	if msg.ID == "" {
		msg.ID = fmt.Sprintf("msg-%d", time.Now().UnixNano())
	}
	msg.SessionID = sessionID
	e.ensureSession(sessionID)
	if err := e.db.InsertMessage(msg); err != nil {
		e.log.Warn("store_assistant_message_failed", "session", sessionID, "error", err)
	}
}

// ensureSession creates the session if it doesn't exist.
func (e *Engine) ensureSession(sessionID string) {
	if _, err := e.db.GetSession(sessionID); err != nil {
		if insertErr := e.db.InsertSession(&types.Session{
			ID:        sessionID,
			Mode:      types.SessionNormal,
			CreatedAt: time.Now(),
		}); insertErr != nil {
			e.log.Warn("ensure_session_failed", "session", sessionID, "error", insertErr)
		}
	}
}

// generateSessionTitle asks the LLM for a short headline summarizing the conversation.
func (e *Engine) generateSessionTitle(sessionID string, history []llm.ChatMessage) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var summary strings.Builder
	for _, m := range history {
		if len(summary.String()) > 400 {
			break
		}
		fmt.Fprintf(&summary, "%s: %s\n", m.Role, truncateForLog(m.Content))
	}

	prompt := fmt.Sprintf(
		"Generate a short title (max 6 words) summarizing this conversation's topic:\n\n%s\nRespond with ONLY the title, no quotes, no punctuation at the end.",
		summary.String(),
	)

	title, err := e.llm.Complete(ctx, prompt)
	if err != nil {
		e.log.Debug("session_title_generation_failed", "session", sessionID, "error", err)
		return
	}

	title = strings.TrimSpace(title)
	title = strings.Trim(title, "\"'")
	if len(title) > 60 {
		title = title[:60]
	}
	if title == "" {
		return
	}

	if titleErr := e.db.UpdateSessionTitle(sessionID, title); titleErr != nil {
		e.log.Warn("session_title_update_failed", "session", sessionID, "error", titleErr)
	}
	e.log.Debug("session_titled", "session", sessionID, "title", title)
}

func (e *Engine) getHistory(sessionID string) []llm.ChatMessage {
	messages, err := e.db.GetMessages(sessionID)
	if err != nil {
		return nil
	}
	result := make([]llm.ChatMessage, 0, len(messages))
	for _, m := range messages {
		result = append(result, llm.ChatMessage{Role: m.Role, Content: m.Content})
	}
	return result
}

// auditLog writes an audit entry and logs a warning if the write fails.
func (e *Engine) auditLog(entry audit.Entry) {
	if err := e.audit.Log(entry); err != nil {
		e.log.Warn("audit_write_failed", "event_type", entry.EventType,
			"session", entry.SessionID, "action", entry.ActionType, "error", err)
	}
}

func (e *Engine) sendErrorEvent(sender EventSender, sid, mid, code, message string) error {
	return sender.SendEvent(&PipelineEvent{
		SessionID: sid, MessageID: mid, Type: EventError,
		Error: &PipelineErrorEvent{Code: code, Message: message, Recoverable: true},
	})
}

// ProcessHeartbeatTask processes a scheduled task from HEARTBEAT.md. It uses a
// persistent internal session so the agent has continuity across scheduled runs.
// Events are discarded — heartbeat tasks run silently in the background.
func (e *Engine) ProcessHeartbeatTask(ctx context.Context, task string) {
	e.mu.Lock()
	sid := e.heartbeatSessionID
	if sid == "" {
		sid = crypto.NewID()
		e.heartbeatSessionID = sid
		e.mu.Unlock()
		if hbErr := e.db.InsertSession(&types.Session{
			ID:    sid,
			Mode:  types.SessionHeartbeat,
			Title: "Heartbeat",
		}); hbErr != nil {
			e.log.Warn("heartbeat_session_create_failed", "session", sid, "error", hbErr)
		}
	} else {
		e.mu.Unlock()
	}

	mid := "hb-" + crypto.NewID()
	sender := &noopEventSender{}

	taskCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	e.log.Info("heartbeat_process", "session", sid, "task", task)
	if err := e.ProcessMessageForWeb(taskCtx, sender, sid, mid, task, types.SessionNormal); err != nil {
		e.log.Error("heartbeat_failed", "task", task, "error", err)
	}
}
