package shield

import (
	"time"

	"github.com/openparallax/openparallax/ifc"
)

// ActionType is an alias for the public ifc.ActionType.
type ActionType = ifc.ActionType

// Action type constants — aliases to the ifc package.
const (
	ActionReadFile     = ifc.ActionReadFile
	ActionWriteFile    = ifc.ActionWriteFile
	ActionDeleteFile   = ifc.ActionDeleteFile
	ActionMoveFile     = ifc.ActionMoveFile
	ActionCopyFile     = ifc.ActionCopyFile
	ActionCreateDir    = ifc.ActionCreateDir
	ActionListDir      = ifc.ActionListDir
	ActionSearchFiles  = ifc.ActionSearchFiles
	ActionExecCommand  = ifc.ActionExecCommand
	ActionSendMessage  = ifc.ActionSendMessage
	ActionSendEmail    = ifc.ActionSendEmail
	ActionHTTPRequest  = ifc.ActionHTTPRequest
	ActionMemoryWrite  = ifc.ActionMemoryWrite
	ActionMemorySearch = ifc.ActionMemorySearch
)

// ActionRequest represents a proposed action from the agent.
type ActionRequest struct {
	RequestID          string              `json:"request_id"`
	Type               ActionType          `json:"type"`
	Payload            map[string]any      `json:"payload"`
	Hash               string              `json:"hash"`
	DataClassification *DataClassification `json:"data_classification,omitempty"`
	MinTier            int                 `json:"min_tier,omitempty"`
	Timestamp          time.Time           `json:"timestamp"`
}

// VerdictDecision is the security evaluation outcome.
type VerdictDecision string

const (
	// VerdictAllow permits the action to execute.
	VerdictAllow VerdictDecision = "ALLOW"
	// VerdictBlock prevents the action from executing.
	VerdictBlock VerdictDecision = "BLOCK"
	// VerdictEscalate requires evaluation at a higher tier.
	VerdictEscalate VerdictDecision = "ESCALATE"
)

// Verdict is the complete evaluation result from Shield.
type Verdict struct {
	Decision    VerdictDecision `json:"decision"`
	Tier        int             `json:"tier"`
	Confidence  float64         `json:"confidence"`
	Reasoning   string          `json:"reasoning"`
	ActionHash  string          `json:"action_hash"`
	EvaluatedAt time.Time       `json:"evaluated_at"`
	ExpiresAt   time.Time       `json:"expires_at"`
}

// IsExpired returns true if the verdict has passed its TTL.
func (v *Verdict) IsExpired() bool {
	return time.Now().After(v.ExpiresAt)
}

// SensitivityLevel is the data sensitivity classification.
// SensitivityLevel is an alias for the public ifc type.
type SensitivityLevel = ifc.SensitivityLevel

// DataClassification is an alias for the public ifc type.
type DataClassification = ifc.DataClassification

// Sensitivity level constants — aliases to the ifc package.
const (
	SensitivityPublic       = ifc.SensitivityPublic
	SensitivityInternal     = ifc.SensitivityInternal
	SensitivityConfidential = ifc.SensitivityConfidential
	SensitivityRestricted   = ifc.SensitivityRestricted
	SensitivityCritical     = ifc.SensitivityCritical
)

// EvaluatorConfig configures the Tier 2 LLM evaluator.
type EvaluatorConfig struct {
	Provider  string `yaml:"provider" json:"provider"`
	Model     string `yaml:"model" json:"model"`
	APIKeyEnv string `yaml:"api_key_env" json:"api_key_env"`
	BaseURL   string `yaml:"base_url,omitempty" json:"base_url,omitempty"`
}
