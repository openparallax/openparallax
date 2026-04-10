package types

import "errors"

// Sentinel errors used across packages.
var (
	// ErrPipelineNotReady indicates the pipeline has not been initialized.
	ErrPipelineNotReady = errors.New("pipeline not initialized")
	// ErrParserFailed indicates intent parsing could not complete.
	ErrParserFailed = errors.New("intent parsing failed")
	// ErrSelfEvalFailed indicates the self-evaluation rejected the action.
	ErrSelfEvalFailed = errors.New("self-evaluation rejected the action")
	// ErrShieldUnavailable indicates the Shield evaluation service is not reachable.
	ErrShieldUnavailable = errors.New("shield evaluation unavailable")

	// ErrActionBlocked indicates an action was blocked by Shield.
	ErrActionBlocked = errors.New("action blocked by Shield")
	// ErrActionTimeout indicates an action exceeded its execution timeout.
	ErrActionTimeout = errors.New("action execution timed out")
	// ErrOTRBlocked indicates an action is not allowed in OTR mode.
	ErrOTRBlocked = errors.New("action not allowed in OTR mode")
	// ErrHashMismatch indicates the action hash does not match the evaluated hash.
	ErrHashMismatch = errors.New("action hash does not match evaluated hash")

	// ErrApprovalTimeout indicates an approval request was not resolved in time.
	ErrApprovalTimeout = errors.New("approval request timed out")
	// ErrApprovalDenied indicates the user denied the action.
	ErrApprovalDenied = errors.New("action denied by user")

	// ErrSessionNotFound indicates the requested session does not exist.
	ErrSessionNotFound = errors.New("session not found")
	// ErrSessionModeChange indicates an attempt to change mode after messages were sent.
	ErrSessionModeChange = errors.New("cannot change mode after messages have been sent")

	// ErrSnapshotNotFound indicates the requested Chronicle snapshot does not exist.
	ErrSnapshotNotFound = errors.New("snapshot not found")
	// ErrTransactionActive indicates a transaction is already in progress.
	ErrTransactionActive = errors.New("a transaction is already active")
	// ErrNoActiveTransaction indicates no transaction is currently in progress.
	ErrNoActiveTransaction = errors.New("no active transaction")
	// ErrIntegrityViolation indicates the integrity hash chain has been broken.
	ErrIntegrityViolation = errors.New("integrity chain violation detected")

	// ErrConfigNotFound indicates the configuration file could not be found.
	ErrConfigNotFound = errors.New("configuration file not found")
	// ErrConfigInvalid indicates the configuration failed validation.
	ErrConfigInvalid = errors.New("configuration validation failed")

	// ErrMemoryFileNotFound indicates a memory file does not exist in the workspace.
	ErrMemoryFileNotFound = errors.New("memory file not found")
	// ErrPathTraversal indicates a path traversal attempt was detected.
	ErrPathTraversal = errors.New("path traversal detected")
)
