package types

import "time"

// SessionMode controls what the agent can do in a session.
type SessionMode string

const (
	// SessionNormal is the default mode with full agent capabilities and persistence.
	SessionNormal SessionMode = "normal"

	// SessionOTR is Off the Record mode with read-only access and no memory persistence.
	SessionOTR SessionMode = "otr"

	// SessionHeartbeat is used for scheduled tasks from HEARTBEAT.md. These
	// sessions are hidden from the user-facing session list.
	SessionHeartbeat SessionMode = "heartbeat"
)

// Session represents a conversation session.
type Session struct {
	// ID is the unique session identifier.
	ID string `json:"id"`

	// Mode is the session operating mode.
	Mode SessionMode `json:"mode"`

	// Title is the session display name.
	Title string `json:"title,omitempty"`

	// CreatedAt is when the session was created.
	CreatedAt time.Time `json:"created_at"`

	// LastMsgAt is when the last message was sent.
	LastMsgAt *time.Time `json:"last_message_at,omitempty"`

	// MessageCount is the total number of messages in the session.
	MessageCount int `json:"message_count"`
}

// SessionMetadata is the lightweight version for listing sessions.
type SessionMetadata struct {
	// ID is the session identifier.
	ID string `json:"id"`

	// Title is the session display name.
	Title string `json:"title,omitempty"`

	// Mode is the session operating mode.
	Mode SessionMode `json:"mode"`

	// CreatedAt is the creation timestamp as an RFC 3339 string.
	CreatedAt string `json:"created_at"`

	// LastMsgAt is the last message timestamp as an RFC 3339 string.
	LastMsgAt string `json:"last_message_at,omitempty"`

	// Preview is a snippet of the most recent message.
	Preview string `json:"preview,omitempty"`
}

// Message is a single message in a session.
type Message struct {
	// ID is the message identifier.
	ID string `json:"id"`

	// SessionID is the parent session.
	SessionID string `json:"session_id"`

	// Role is "user" or "assistant".
	Role string `json:"role"`

	// Content is the message text.
	Content string `json:"content"`

	// Thoughts are pipeline stage observations recorded during processing.
	Thoughts []Thought `json:"thoughts,omitempty"`

	// Timestamp is when the message was created.
	Timestamp time.Time `json:"timestamp"`
}

// Thought is a single pipeline stage observation.
type Thought struct {
	// Stage is the pipeline stage name (e.g., "parsing", "evaluating", "executing").
	Stage string `json:"stage"`

	// Summary is a human-readable description of what happened.
	Summary string `json:"summary"`

	// Detail contains optional structured data about the stage.
	Detail map[string]any `json:"detail,omitempty"`
}
