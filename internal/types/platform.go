package types

// MessagingPlatform identifies a messaging channel.
type MessagingPlatform string

const (
	// PlatformCLI is the interactive terminal channel.
	PlatformCLI MessagingPlatform = "cli"
	// PlatformWhatsApp is the WhatsApp channel.
	PlatformWhatsApp MessagingPlatform = "whatsapp"
	// PlatformTelegram is the Telegram channel.
	PlatformTelegram MessagingPlatform = "telegram"
	// PlatformDiscord is the Discord channel.
	PlatformDiscord MessagingPlatform = "discord"
	// PlatformSlack is the Slack channel.
	PlatformSlack MessagingPlatform = "slack"
	// PlatformSignal is the Signal channel.
	PlatformSignal MessagingPlatform = "signal"
	// PlatformIMessage is the iMessage channel (macOS only).
	PlatformIMessage MessagingPlatform = "imessage"
	// PlatformTeams is the Microsoft Teams channel.
	PlatformTeams MessagingPlatform = "teams"
	// PlatformWeb is the Web UI channel.
	PlatformWeb MessagingPlatform = "web"
)

// AllMessagingPlatforms contains every defined messaging platform.
var AllMessagingPlatforms = []MessagingPlatform{
	PlatformCLI, PlatformWhatsApp, PlatformTelegram, PlatformDiscord,
	PlatformSlack, PlatformSignal, PlatformIMessage, PlatformTeams, PlatformWeb,
}

// AuditEventType identifies the category of audit event.
// This is the native Go type paralleling the protobuf AuditEventType enum.
type AuditEventType int

const (
	// AuditActionProposed records that an action was proposed by the agent.
	AuditActionProposed AuditEventType = 1
	// AuditActionEvaluated records that an action was evaluated by Shield.
	AuditActionEvaluated AuditEventType = 2
	// AuditActionApproved records that an action was approved.
	AuditActionApproved AuditEventType = 3
	// AuditActionBlocked records that an action was blocked by Shield.
	AuditActionBlocked AuditEventType = 4
	// AuditActionExecuted records that an action was executed.
	AuditActionExecuted AuditEventType = 5
	// AuditActionFailed records that an action execution failed.
	AuditActionFailed AuditEventType = 6
	// AuditShieldError records a Shield evaluation error.
	AuditShieldError AuditEventType = 7
	// AuditCanaryVerified records successful canary token verification.
	AuditCanaryVerified AuditEventType = 8
	// AuditCanaryMissing records a missing canary token.
	AuditCanaryMissing AuditEventType = 9
	// AuditRateLimitHit records a rate limit event.
	AuditRateLimitHit AuditEventType = 10
	// AuditBudgetExhausted records that the daily evaluation budget was exhausted.
	AuditBudgetExhausted AuditEventType = 11
	// AuditSelfProtection records a self-protection rule trigger.
	AuditSelfProtection AuditEventType = 12
	// AuditTransactionBegin records the start of a transaction.
	AuditTransactionBegin AuditEventType = 13
	// AuditTransactionCommit records a transaction commit.
	AuditTransactionCommit AuditEventType = 14
	// AuditTransactionRollback records a transaction rollback.
	AuditTransactionRollback AuditEventType = 15
	// AuditIntegrityViolation records an integrity chain violation.
	AuditIntegrityViolation AuditEventType = 16
	// AuditSessionStarted records the start of a session.
	AuditSessionStarted AuditEventType = 17
	// AuditSessionEnded records the end of a session.
	AuditSessionEnded AuditEventType = 18
)

// AuditEntry is the native Go representation of an audit log entry.
type AuditEntry struct {
	// ID is the unique entry identifier.
	ID string `json:"id"`

	// EventType categorizes the audit event.
	EventType AuditEventType `json:"event_type"`

	// Timestamp is the event time in Unix milliseconds.
	Timestamp int64 `json:"timestamp"`

	// SessionID is the session this event belongs to.
	SessionID string `json:"session_id,omitempty"`

	// ActionType is the action type string, if applicable.
	ActionType string `json:"action_type,omitempty"`

	// DetailsJSON is a JSON string with event-specific details.
	DetailsJSON string `json:"details_json,omitempty"`

	// PreviousHash is the hash of the preceding audit entry.
	PreviousHash string `json:"previous_hash"`

	// Hash is the integrity hash of this entry.
	Hash string `json:"hash"`

	// OTR indicates whether the event occurred in OTR mode.
	OTR bool `json:"otr"`

	// Source identifies where the event originated.
	Source string `json:"source,omitempty"`
}
