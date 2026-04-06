package engine

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/openparallax/openparallax/crypto"
	"github.com/openparallax/openparallax/internal/types"
)

// Tier3Decision represents a human approval or denial.
type Tier3Decision struct {
	Approved bool
}

// PendingAction is an action awaiting human approval.
type PendingAction struct {
	ID        string
	SessionID string
	Action    *types.ActionRequest
	Reasoning string
	CreatedAt time.Time
	Timeout   time.Duration
	ResultCh  chan Tier3Decision
}

// Tier3Manager handles human-in-the-loop approval for uncertain Shield verdicts.
type Tier3Manager struct {
	pending     map[string]*PendingAction
	mu          sync.Mutex
	maxPerHour  int
	timeout     time.Duration
	hourlyCount int
	hourReset   time.Time
}

// NewTier3Manager creates a Tier 3 manager with the given rate limit and timeout.
func NewTier3Manager(maxPerHour int, timeoutSecs int) *Tier3Manager {
	if maxPerHour <= 0 {
		maxPerHour = 10
	}
	if timeoutSecs <= 0 {
		timeoutSecs = 300
	}
	return &Tier3Manager{
		pending:    make(map[string]*PendingAction),
		maxPerHour: maxPerHour,
		timeout:    time.Duration(timeoutSecs) * time.Second,
		hourReset:  time.Now().Add(time.Hour),
	}
}

// RateLimitExceeded checks if the hourly Tier 3 budget is exhausted.
func (m *Tier3Manager) RateLimitExceeded() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resetIfNeeded()
	return m.hourlyCount >= m.maxPerHour
}

// Submit creates a pending action and blocks until the user responds or timeout.
// Returns true if approved, false if denied or timed out.
func (m *Tier3Manager) Submit(ctx context.Context, pa *PendingAction) (bool, error) {
	m.mu.Lock()
	m.resetIfNeeded()
	m.hourlyCount++
	pa.CreatedAt = time.Now()
	pa.Timeout = m.timeout
	pa.ResultCh = make(chan Tier3Decision, 1)
	m.pending[pa.ID] = pa
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		delete(m.pending, pa.ID)
		m.mu.Unlock()
	}()

	select {
	case decision := <-pa.ResultCh:
		return decision.Approved, nil
	case <-time.After(pa.Timeout):
		return false, nil
	case <-ctx.Done():
		return false, ctx.Err()
	}
}

// Decide resolves a pending action. Called when the user approves or denies.
func (m *Tier3Manager) Decide(actionID string, approved bool) error {
	m.mu.Lock()
	pa, ok := m.pending[actionID]
	m.mu.Unlock()

	if !ok {
		return fmt.Errorf("no pending action: %s", actionID)
	}

	pa.ResultCh <- Tier3Decision{Approved: approved}
	return nil
}

// DecideForSession resolves a pending action after validating that the caller's
// session matches the session that created it. Returns an error if the action
// does not exist or the session does not match.
func (m *Tier3Manager) DecideForSession(actionID, sessionID string, approved bool) error {
	m.mu.Lock()
	pa, ok := m.pending[actionID]
	m.mu.Unlock()

	if !ok {
		return fmt.Errorf("no pending action: %s", actionID)
	}

	if pa.SessionID != "" && pa.SessionID != sessionID {
		return fmt.Errorf("session mismatch for action %s", actionID)
	}

	pa.ResultCh <- Tier3Decision{Approved: approved}
	return nil
}

// Pending returns all pending actions for UI display.
func (m *Tier3Manager) Pending() []*PendingAction {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]*PendingAction, 0, len(m.pending))
	for _, pa := range m.pending {
		result = append(result, pa)
	}
	return result
}

// HourlyRemaining returns how many Tier 3 prompts remain this hour.
func (m *Tier3Manager) HourlyRemaining() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resetIfNeeded()
	remaining := m.maxPerHour - m.hourlyCount
	if remaining < 0 {
		return 0
	}
	return remaining
}

func (m *Tier3Manager) resetIfNeeded() {
	if time.Now().After(m.hourReset) {
		m.hourlyCount = 0
		m.hourReset = time.Now().Add(time.Hour)
	}
}

// ChannelController is implemented by the channel manager to support dynamic
// attach/detach of messaging adapters at runtime.
type ChannelController interface {
	AdapterNames() []string
	Detach(name string) error
}

// SetChannelController registers the channel manager for runtime control.
func (e *Engine) SetChannelController(c ChannelController) {
	e.channelController = c
}

// ChannelController returns the registered channel controller, or nil.
func (e *Engine) ChannelCtrl() ChannelController { return e.channelController }

// ApprovalNotifier is implemented by channel managers to forward Tier 3
// approval requests to connected messaging platforms (Telegram, Discord, etc.).
// The notifier sends a human-readable prompt and routes responses back via
// Tier3Manager.Decide. Implementations must be safe for concurrent use.
type ApprovalNotifier interface {
	NotifyApproval(actionID, toolName, reasoning string, timeoutSecs int)
}

// SetApprovalNotifier registers a channel-level notifier for Tier 3 approvals.
// Called by the process manager after channel adapters are initialized.
func (e *Engine) SetApprovalNotifier(n ApprovalNotifier) {
	e.approvalNotifier = n
}

// requestTier3Approval broadcasts an approval request to all connected clients
// (web UI via broadcaster, messaging channels via approvalNotifier) and blocks
// until the user responds or the timeout expires.
func (e *Engine) requestTier3Approval(ctx context.Context, sid, mid, toolName string, action *types.ActionRequest, reasoning string) (bool, error) {
	if e.tier3Manager.RateLimitExceeded() {
		e.log.Warn("tier3_rate_limit", "tool", toolName)
		return false, fmt.Errorf("tier 3 hourly rate limit exceeded")
	}

	pa := &PendingAction{
		ID:        crypto.NewID(),
		SessionID: sid,
		Action:    action,
		Reasoning: reasoning,
	}

	timeoutSecs := int(e.tier3Manager.timeout.Seconds())

	// Broadcast to web UI and gRPC subscribers.
	e.broadcaster.Broadcast(&PipelineEvent{
		SessionID: sid, MessageID: mid,
		Type: EventTier3ApprovalNeeded,
		Tier3Approval: &Tier3ApprovalEvent{
			ActionID:    pa.ID,
			ToolName:    toolName,
			Reasoning:   reasoning,
			TimeoutSecs: timeoutSecs,
		},
	})

	// Notify connected channel adapters (Telegram, Discord, etc.).
	if e.approvalNotifier != nil {
		e.approvalNotifier.NotifyApproval(pa.ID, toolName, reasoning, timeoutSecs)
	}

	e.log.Info("tier3_approval_requested", "action_id", pa.ID, "tool", toolName)

	return e.tier3Manager.Submit(ctx, pa)
}
