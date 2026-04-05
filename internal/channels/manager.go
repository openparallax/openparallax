package channels

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/openparallax/openparallax/crypto"
	"github.com/openparallax/openparallax/internal/commands"
	"github.com/openparallax/openparallax/internal/engine"
	"github.com/openparallax/openparallax/internal/logging"
	"github.com/openparallax/openparallax/internal/types"
)

// Manager manages channel adapter lifecycle and message routing.
type Manager struct {
	engine    *engine.Engine
	log       *logging.Logger
	mu        sync.RWMutex
	adapters  []ChannelAdapter
	cancels   map[string]context.CancelFunc // adapter name → cancel
	sessions  sync.Map                      // chatKey → sessionID
	commands  *commands.Registry
	cmdEngine *commands.EngineAdapter
}

// NewManager creates a channel manager.
func NewManager(eng *engine.Engine, log *logging.Logger) *Manager {
	return &Manager{
		engine:    eng,
		log:       log,
		cancels:   make(map[string]context.CancelFunc),
		commands:  commands.NewRegistry(),
		cmdEngine: &commands.EngineAdapter{Engine: eng},
	}
}

// Register adds an adapter to the manager (called before StartAll).
func (m *Manager) Register(adapter ChannelAdapter) {
	if adapter.IsConfigured() {
		m.mu.Lock()
		m.adapters = append(m.adapters, adapter)
		m.mu.Unlock()
		m.log.Info("channel_registered", "adapter", adapter.Name())
	}
}

// StartAll starts all registered adapters in goroutines with retry logic.
func (m *Manager) StartAll(ctx context.Context) {
	m.mu.RLock()
	snapshot := make([]ChannelAdapter, len(m.adapters))
	copy(snapshot, m.adapters)
	m.mu.RUnlock()

	for _, adapter := range snapshot {
		adapterCtx, cancel := context.WithCancel(ctx)
		m.mu.Lock()
		m.cancels[adapter.Name()] = cancel
		m.mu.Unlock()
		go m.runWithRetry(adapterCtx, adapter)
	}
}

// StopAll gracefully stops all adapters.
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for name, cancel := range m.cancels {
		cancel()
		delete(m.cancels, name)
	}
	for _, adapter := range m.adapters {
		if err := adapter.Stop(); err != nil {
			m.log.Warn("channel_stop_error", "adapter", adapter.Name(), "error", err)
		}
	}
}

// AdapterCount returns the number of registered adapters.
func (m *Manager) AdapterCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.adapters)
}

// AdapterNames returns the names of all registered adapters.
func (m *Manager) AdapterNames() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := make([]string, len(m.adapters))
	for i, a := range m.adapters {
		names[i] = a.Name()
	}
	return names
}

// Detach stops and removes a running adapter by name.
func (m *Manager) Detach(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	idx := -1
	for i, a := range m.adapters {
		if a.Name() == name {
			idx = i
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("channel %q is not attached", name)
	}

	if cancel, ok := m.cancels[name]; ok {
		cancel()
		delete(m.cancels, name)
	}
	if err := m.adapters[idx].Stop(); err != nil {
		m.log.Warn("channel_detach_stop_error", "adapter", name, "error", err)
	}

	m.adapters = append(m.adapters[:idx], m.adapters[idx+1:]...)
	m.log.Info("channel_detached", "adapter", name)
	return nil
}

func (m *Manager) runWithRetry(ctx context.Context, adapter ChannelAdapter) {
	maxRetries := 5
	retryDelay := 30 * time.Second

	for attempt := range maxRetries {
		m.log.Info("channel_starting", "adapter", adapter.Name(), "attempt", attempt+1)
		err := adapter.Start(ctx)
		if err == nil || ctx.Err() != nil {
			return // clean exit or context cancelled
		}
		m.log.Warn("channel_failed", "adapter", adapter.Name(), "error", err, "attempt", attempt+1)
		if attempt < maxRetries-1 {
			select {
			case <-time.After(retryDelay):
			case <-ctx.Done():
				return
			}
		}
	}
	m.log.Error("channel_stopped", "adapter", adapter.Name(), "reason", "max retries exceeded")
}

// HandleCommand checks if content is a slash command and executes it.
// Returns the response text and true if a command was handled, or empty and false.
func (m *Manager) HandleCommand(adapterName, chatID, content string, channel commands.Channel) (string, commands.Action, bool) {
	if !strings.HasPrefix(strings.TrimSpace(content), "/") {
		return "", commands.ActionNone, false
	}

	sessionID := ""
	if sid, ok := m.sessions.Load(adapterName + ":" + chatID); ok {
		sessionID, _ = sid.(string)
	}

	cmdCtx := &commands.Context{
		Channel:   channel,
		SessionID: sessionID,
		Engine:    m.cmdEngine,
	}

	result, handled := m.commands.Execute(content, cmdCtx)
	if !handled {
		return "", commands.ActionNone, false
	}

	// Handle side-effects.
	switch result.Action {
	case commands.ActionNewSession:
		m.ResetSession(adapterName, chatID)
	case commands.ActionNewOTR:
		m.ResetSession(adapterName, chatID)
	}

	m.log.Info("command_executed", "adapter", adapterName,
		"chat_id", chatID, "command", strings.Fields(content)[0])

	return result.Text, result.Action, true
}

// HandleMessage routes an incoming channel message to the engine pipeline.
// Returns the full response text.
func (m *Manager) HandleMessage(ctx context.Context, adapterName, chatID, content string, mode types.SessionMode) (string, error) {
	sessionID := m.getOrCreateSession(adapterName, chatID, mode)
	messageID := "msg-" + crypto.NewID()

	collector := &responseCollector{}
	err := m.engine.ProcessMessageForWeb(ctx, collector, sessionID, messageID, content, mode)
	if err != nil {
		return "", err
	}

	return collector.fullResponse(), nil
}

func (m *Manager) getOrCreateSession(adapterName, chatID string, mode types.SessionMode) string {
	key := adapterName + ":" + chatID
	if sid, ok := m.sessions.Load(key); ok {
		if s, castOK := sid.(string); castOK {
			return s
		}
	}
	sid := crypto.NewID()
	m.sessions.Store(key, sid)

	if err := m.engine.DB().InsertSession(&types.Session{
		ID:        sid,
		Mode:      mode,
		CreatedAt: time.Now(),
	}); err != nil {
		m.log.Warn("channel_session_create_failed", "adapter", adapterName,
			"chat_id", chatID, "session", sid, "error", err)
	} else {
		m.log.Info("channel_session_created", "adapter", adapterName,
			"chat_id", chatID, "session", sid, "mode", mode)
	}
	return sid
}

// NotifyApproval broadcasts a Tier 3 approval request to all connected
// adapters that implement ApprovalHandler. Adapters that do not support
// approval prompts are silently skipped.
func (m *Manager) NotifyApproval(actionID, toolName, reasoning string, timeoutSecs int) {
	m.mu.RLock()
	snapshot := make([]ChannelAdapter, len(m.adapters))
	copy(snapshot, m.adapters)
	m.mu.RUnlock()

	for _, adapter := range snapshot {
		handler, ok := adapter.(ApprovalHandler)
		if !ok {
			continue
		}
		if err := handler.RequestApproval(actionID, toolName, reasoning, timeoutSecs); err != nil {
			m.log.Warn("tier3_notify_failed", "adapter", adapter.Name(), "action_id", actionID, "error", err)
		}
	}
}

// HandleApprovalResponse routes an approval decision from a channel adapter
// back to the Tier 3 manager.
func (m *Manager) HandleApprovalResponse(actionID string, approved bool) error {
	return m.engine.Tier3().Decide(actionID, approved)
}

// ResetSession creates a new session for a chat (used by /new command).
func (m *Manager) ResetSession(adapterName, chatID string) {
	key := adapterName + ":" + chatID
	m.sessions.Delete(key)
}

// responseCollector is an EventSender that collects the final response.
type responseCollector struct {
	mu       sync.Mutex
	text     strings.Builder
	response string
	done     bool
}

func (r *responseCollector) SendEvent(event *engine.PipelineEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	switch event.Type {
	case engine.EventLLMToken:
		if event.LLMToken != nil {
			r.text.WriteString(event.LLMToken.Text)
		}
	case engine.EventResponseComplete:
		if event.ResponseComplete != nil {
			r.response = event.ResponseComplete.Content
			r.done = true
		}
	}
	return nil
}

func (r *responseCollector) fullResponse() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.done && r.response != "" {
		return r.response
	}
	return r.text.String()
}
