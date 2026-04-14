package types

import "github.com/openparallax/openparallax/audit"

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

// AuditEventType is an alias for the public audit.EventType.
type AuditEventType = audit.EventType

// AuditEntry is an alias for the public audit.LogEntry.
type AuditEntry = audit.LogEntry

// Audit event type constants — aliases to the public audit package.
const (
	AuditActionProposed             = audit.ActionProposed
	AuditActionEvaluated            = audit.ActionEvaluated
	AuditActionApproved             = audit.ActionApproved
	AuditActionBlocked              = audit.ActionBlocked
	AuditActionExecuted             = audit.ActionExecuted
	AuditActionFailed               = audit.ActionFailed
	AuditShieldError                = audit.ShieldError
	AuditCanaryVerified             = audit.CanaryVerified
	AuditCanaryMissing              = audit.CanaryMissing
	AuditRateLimitHit               = audit.RateLimitHit
	AuditBudgetExhausted            = audit.BudgetExhausted
	AuditSelfProtection             = audit.SelfProtection
	AuditTransactionBegin           = audit.TransactionBegin
	AuditTransactionCommit          = audit.TransactionCommit
	AuditTransactionRollback        = audit.TransactionRollback
	AuditIntegrityViolation         = audit.IntegrityViolation
	AuditSessionStarted             = audit.SessionStarted
	AuditSessionEnded               = audit.SessionEnded
	AuditConfigChanged              = audit.ConfigChanged
	AuditIFCClassified              = audit.IFCClassified
	AuditChronicleSnapshot          = audit.ChronicleSnapshot
	AuditChronicleSnapshotFailed    = audit.ChronicleSnapshotFailed
	AuditSandboxCanaryResult        = audit.SandboxCanaryResult
	AuditSecurityModeChanged        = audit.SecurityModeChanged
	AuditSealedConfigTamperDetected = audit.SealedConfigTamperDetected
	AuditProtectionBypassAttempted  = audit.ProtectionBypassAttempted
	AuditIFCBlocked                 = audit.IFCBlocked
	AuditIFCAuditWouldBlock         = audit.IFCAuditWouldBlock
	AuditIFCSweep                   = audit.IFCSweep
)
