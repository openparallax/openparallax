package audit

// EventType identifies the category of audit event.
type EventType int

const (
	// ActionProposed records that an action was proposed by the agent.
	ActionProposed EventType = 1
	// ActionEvaluated records that an action was evaluated by Shield.
	ActionEvaluated EventType = 2
	// ActionApproved records that an action was approved.
	ActionApproved EventType = 3
	// ActionBlocked records that an action was blocked by Shield.
	ActionBlocked EventType = 4
	// ActionExecuted records that an action was executed.
	ActionExecuted EventType = 5
	// ActionFailed records that an action execution failed.
	ActionFailed EventType = 6
	// ShieldError records a Shield evaluation error.
	ShieldError EventType = 7
	// CanaryVerified records successful canary token verification.
	CanaryVerified EventType = 8
	// CanaryMissing records a missing canary token.
	CanaryMissing EventType = 9
	// RateLimitHit records a rate limit event.
	RateLimitHit EventType = 10
	// BudgetExhausted records that the daily evaluation budget was exhausted.
	BudgetExhausted EventType = 11
	// SelfProtection records a self-protection rule trigger.
	SelfProtection EventType = 12
	// TransactionBegin records the start of a transaction.
	TransactionBegin EventType = 13
	// TransactionCommit records a transaction commit.
	TransactionCommit EventType = 14
	// TransactionRollback records a transaction rollback.
	TransactionRollback EventType = 15
	// IntegrityViolation records an integrity chain violation.
	IntegrityViolation EventType = 16
	// SessionStarted records the start of a session.
	SessionStarted EventType = 17
	// SessionEnded records the end of a session.
	SessionEnded EventType = 18
	// ConfigChanged records a successful config.yaml mutation through
	// the canonical config.Save writer.
	ConfigChanged EventType = 19
	// IFCClassified records that an action was tagged with a data
	// sensitivity classification by the metadata enricher. Emitted
	// once per action at enrichment time, before Shield evaluation.
	IFCClassified EventType = 20
	// ChronicleSnapshot records a successful copy-on-write snapshot
	// taken before a state-mutating action.
	ChronicleSnapshot EventType = 21
	// ChronicleSnapshotFailed records a snapshot attempt that errored.
	// The action still runs (snapshots are best-effort), but the
	// failure is preserved in the audit chain so rollback gaps are
	// auditable after the fact.
	ChronicleSnapshotFailed EventType = 22
	// SandboxCanaryResult records the agent process's startup sandbox
	// canary verification outcome, plumbed from the sandboxed agent
	// child to the engine via the AgentReady stream event.
	SandboxCanaryResult EventType = 23
	// SecurityModeChanged records a change to the security override_mode
	// field in config.yaml (e.g., flipping IFC between enforce and audit).
	SecurityModeChanged EventType = 24
	// SealedConfigTamperDetected records that the sealed security config
	// digest check failed at runtime. The engine panics immediately after
	// emitting this event — it is the last entry before fail-closed death.
	SealedConfigTamperDetected EventType = 25
	// ProtectionBypassAttempted records a code path that tried to skip the
	// hardcoded protection layer. Should never fire in a correct build;
	// exists as a paranoia guardrail.
	ProtectionBypassAttempted EventType = 26
	// IFCBlocked records that an action was blocked by the IFC flow policy.
	// Distinct from ActionBlocked (which records Shield blocks) so security
	// operators can filter drill-down queries by subsystem.
	IFCBlocked EventType = 27
	// IFCAuditWouldBlock records that an action would have been blocked by
	// IFC policy but was allowed because the policy mode is "audit" (shadow
	// mode). Used during IFC policy rollout to observe before enforcing.
	IFCAuditWouldBlock EventType = 28
)

// LogEntry is the native Go representation of an audit log entry.
type LogEntry struct {
	// ID is the unique entry identifier.
	ID string `json:"id"`
	// EventType categorizes the audit event.
	EventType EventType `json:"event_type"`
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
	// Source identifies the origin of the event.
	Source string `json:"source,omitempty"`
}
