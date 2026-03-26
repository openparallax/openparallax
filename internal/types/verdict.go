package types

import "time"

// VerdictDecision is the security evaluation outcome.
type VerdictDecision string

const (
	// VerdictAllow permits the action to execute.
	VerdictAllow VerdictDecision = "ALLOW"
	// VerdictBlock prevents the action from executing.
	VerdictBlock VerdictDecision = "BLOCK"
	// VerdictEscalate requires the action to be evaluated at a higher tier.
	VerdictEscalate VerdictDecision = "ESCALATE"
)

// Verdict is the complete evaluation result from Shield.
type Verdict struct {
	// Decision is ALLOW, BLOCK, or ESCALATE.
	Decision VerdictDecision `json:"decision"`

	// Tier is which evaluation tier produced this verdict (0, 1, or 2).
	Tier int `json:"tier"`

	// Confidence is the classifier's confidence score (0.0-1.0).
	// Only meaningful for Tier 1 and Tier 2.
	Confidence float64 `json:"confidence"`

	// Reasoning is a human-readable explanation.
	Reasoning string `json:"reasoning"`

	// ActionHash is the SHA-256 of the ActionRequest that was evaluated.
	// The execution layer verifies this matches before executing.
	ActionHash string `json:"action_hash"`

	// EvaluatedAt is when the verdict was produced.
	EvaluatedAt time.Time `json:"evaluated_at"`

	// ExpiresAt is when the verdict expires (verdict TTL).
	// Default: 60 seconds after EvaluatedAt.
	ExpiresAt time.Time `json:"expires_at"`
}

// IsExpired returns true if the verdict has passed its TTL.
func (v *Verdict) IsExpired() bool {
	return time.Now().After(v.ExpiresAt)
}
