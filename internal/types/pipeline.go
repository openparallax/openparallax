package types

// PipelineSummary captures every decision made during pipeline processing.
// The response generator uses this as its sole source of truth about what
// happened — it cannot claim outcomes that aren't recorded here.
type PipelineSummary struct {
	// UserMessage is the original user input.
	UserMessage string

	// Classification is the parser's goal determination.
	Classification GoalType

	// ClassificationReason explains why the parser chose this classification.
	ClassificationReason string

	// ActionsPlanned is how many actions the planner produced.
	ActionsPlanned int

	// SelfEvalPassed indicates whether self-evaluation approved the plan.
	SelfEvalPassed bool

	// SelfEvalReason is set when self-eval fails.
	SelfEvalReason string

	// Outcomes records what happened to each planned action.
	Outcomes []ActionOutcome
}

// ActionOutcome records the pipeline's decision for a single action.
type ActionOutcome struct {
	// Action is the action type (e.g., "read_file", "write_file").
	Action ActionType

	// Status is the pipeline outcome for this action.
	Status ActionStatus

	// Summary is a human-readable description of the outcome.
	Summary string

	// Output is the action's output content (for successful executions).
	Output string

	// Reason explains why the action was blocked (for blocked actions).
	Reason string
}

// ActionStatus is the pipeline outcome for a single action.
type ActionStatus string

const (
	// StatusExecuted means the action ran successfully.
	StatusExecuted ActionStatus = "executed"
	// StatusFailed means the action ran but failed.
	StatusFailed ActionStatus = "failed"
	// StatusBlockedOTR means OTR mode prevented the action.
	StatusBlockedOTR ActionStatus = "blocked_otr"
	// StatusBlockedShield means Shield evaluation blocked the action.
	StatusBlockedShield ActionStatus = "blocked_shield"
	// StatusBlockedEscalate means the action requires human approval.
	StatusBlockedEscalate ActionStatus = "blocked_escalate"
	// StatusBlockedHash means hash verification failed (TOCTOU).
	StatusBlockedHash ActionStatus = "blocked_integrity"
)
