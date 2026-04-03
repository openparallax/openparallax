package shield

import "time"

// ActionType represents the category of action.
type ActionType string

// Action type constants used by Shield for routing and evaluation.
const (
	ActionReadFile     ActionType = "read_file"
	ActionWriteFile    ActionType = "write_file"
	ActionDeleteFile   ActionType = "delete_file"
	ActionMoveFile     ActionType = "move_file"
	ActionCopyFile     ActionType = "copy_file"
	ActionCreateDir    ActionType = "create_directory"
	ActionListDir      ActionType = "list_directory"
	ActionSearchFiles  ActionType = "search_files"
	ActionExecCommand  ActionType = "execute_command"
	ActionSendMessage  ActionType = "send_message"
	ActionSendEmail    ActionType = "send_email"
	ActionHTTPRequest  ActionType = "http_request"
	ActionMemoryWrite  ActionType = "memory_write"
	ActionMemorySearch ActionType = "memory_search"
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
type SensitivityLevel int

const (
	// SensitivityPublic is data with no access restrictions.
	SensitivityPublic SensitivityLevel = 0
	// SensitivityInternal is data restricted to internal use.
	SensitivityInternal SensitivityLevel = 1
	// SensitivityConfidential is data with limited distribution.
	SensitivityConfidential SensitivityLevel = 2
	// SensitivityRestricted is data requiring elevated access controls.
	SensitivityRestricted SensitivityLevel = 3
	// SensitivityCritical is data with the highest protection requirements.
	SensitivityCritical SensitivityLevel = 4
)

// DataClassification is the IFC tag attached to data flowing through the pipeline.
type DataClassification struct {
	Sensitivity SensitivityLevel `json:"sensitivity"`
	SourcePath  string           `json:"source_path,omitempty"`
	ContentType string           `json:"content_type,omitempty"`
}

// EvaluatorConfig configures the Tier 2 LLM evaluator.
type EvaluatorConfig struct {
	Provider  string `yaml:"provider" json:"provider"`
	Model     string `yaml:"model" json:"model"`
	APIKeyEnv string `yaml:"api_key_env" json:"api_key_env"`
	BaseURL   string `yaml:"base_url,omitempty" json:"base_url,omitempty"`
}
