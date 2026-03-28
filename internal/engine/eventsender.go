package engine

import "github.com/openparallax/openparallax/internal/types"

// EventType identifies a pipeline event.
type EventType string

const (
	EventLLMToken         EventType = "llm_token"
	EventActionStarted    EventType = "action_started"
	EventShieldVerdict    EventType = "shield_verdict"
	EventActionCompleted  EventType = "action_completed"
	EventActionArtifact   EventType = "action_artifact"
	EventResponseComplete EventType = "response_complete"
	EventOTRBlocked       EventType = "otr_blocked"
	EventError            EventType = "error"
)

// PipelineEvent is a transport-neutral event emitted during message processing.
// Exactly one of the payload fields is set per event, determined by Type.
type PipelineEvent struct {
	SessionID string    `json:"session_id"`
	MessageID string    `json:"message_id"`
	Type      EventType `json:"type"`

	// Payload — one per event type.
	LLMToken         *LLMTokenEvent         `json:"text,omitempty"`
	ActionStarted    *ActionStartedEvent    `json:"action_started,omitempty"`
	ShieldVerdict    *ShieldVerdictEvent    `json:"shield_verdict,omitempty"`
	ActionCompleted  *ActionCompletedEvent  `json:"action_completed,omitempty"`
	ActionArtifact   *ActionArtifactEvent   `json:"action_artifact,omitempty"`
	ResponseComplete *ResponseCompleteEvent `json:"response_complete,omitempty"`
	OTRBlocked       *OTRBlockedEvent       `json:"otr_blocked,omitempty"`
	Error            *PipelineErrorEvent    `json:"error,omitempty"`
}

// LLMTokenEvent is a single streamed token from the LLM.
type LLMTokenEvent struct {
	Text string `json:"text"`
}

// ActionStartedEvent signals that a tool call is beginning.
type ActionStartedEvent struct {
	ToolName string `json:"tool_name"`
	Summary  string `json:"summary"`
}

// ShieldVerdictEvent carries the Shield security evaluation result.
type ShieldVerdictEvent struct {
	ToolName   string  `json:"tool_name"`
	Decision   string  `json:"decision"`
	Tier       int     `json:"tier"`
	Confidence float64 `json:"confidence"`
	Reasoning  string  `json:"reasoning"`
}

// ActionCompletedEvent signals that a tool call finished.
type ActionCompletedEvent struct {
	ToolName string `json:"tool_name"`
	Success  bool   `json:"success"`
	Summary  string `json:"summary"`
}

// ActionArtifactEvent carries a generated artifact.
type ActionArtifactEvent struct {
	Artifact *types.Artifact `json:"artifact"`
}

// ResponseCompleteEvent carries the full assistant response text.
type ResponseCompleteEvent struct {
	Content string `json:"content"`
}

// OTRBlockedEvent signals an action blocked by OTR mode.
type OTRBlockedEvent struct {
	Reason string `json:"reason"`
}

// PipelineErrorEvent carries an error.
type PipelineErrorEvent struct {
	Code        string `json:"code"`
	Message     string `json:"message"`
	Recoverable bool   `json:"recoverable"`
}

// EventSender is the transport-neutral interface for emitting pipeline events.
// Implemented by grpcEventSender (protobuf over gRPC) and wsEventSender (JSON over WebSocket).
type EventSender interface {
	SendEvent(event *PipelineEvent) error
}
