package main

import (
	"context"
	"sync"
	"time"

	"github.com/openparallax/openparallax/internal/types"
)

// ShieldResult captures the outcome of a Shield evaluation for a recorded action.
type ShieldResult struct {
	Decision   string  `json:"decision"`
	Tier       int     `json:"tier"`
	Confidence float64 `json:"confidence"`
	Reasoning  string  `json:"reasoning"`
	LatencyMs  int64   `json:"latency_ms"`
	Tier0Ms    float64 `json:"tier0_ms"`
	Tier1Ms    float64 `json:"tier1_ms"`
	Tier2Ms    float64 `json:"tier2_ms"`
}

// RecordedAction is a single action proposed by the LLM during a test case.
type RecordedAction struct {
	Type          string         `json:"type"`
	Payload       map[string]any `json:"payload"`
	Timestamp     time.Time      `json:"timestamp"`
	ShieldVerdict *ShieldResult  `json:"shield_verdict,omitempty"`
	WouldExecute  bool           `json:"would_execute"`
}

// RecordingExecutor captures all proposed actions without executing them.
type RecordingExecutor struct {
	mu      sync.Mutex
	Actions []RecordedAction
}

// Execute records the action and returns a fake success result.
func (r *RecordingExecutor) Execute(_ context.Context, action *types.ActionRequest) *types.ActionResult {
	r.mu.Lock()
	r.Actions = append(r.Actions, RecordedAction{
		Type:         string(action.Type),
		Payload:      action.Payload,
		Timestamp:    time.Now(),
		WouldExecute: true,
	})
	r.mu.Unlock()
	return &types.ActionResult{
		RequestID: action.RequestID,
		Success:   true,
		Output:    "[eval: dry-run, action recorded]",
		Summary:   "recorded",
	}
}

// MarkBlocked records a blocked action with Shield verdict details.
func (r *RecordingExecutor) MarkBlocked(actionType string, payload map[string]any, verdict *ShieldResult) {
	r.mu.Lock()
	r.Actions = append(r.Actions, RecordedAction{
		Type:          actionType,
		Payload:       payload,
		Timestamp:     time.Now(),
		ShieldVerdict: verdict,
		WouldExecute:  false,
	})
	r.mu.Unlock()
}

// Reset clears all recorded actions for the next test case.
func (r *RecordingExecutor) Reset() {
	r.mu.Lock()
	r.Actions = nil
	r.mu.Unlock()
}

// Snapshot returns a copy of the recorded actions.
func (r *RecordingExecutor) Snapshot() []RecordedAction {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]RecordedAction, len(r.Actions))
	copy(out, r.Actions)
	return out
}
