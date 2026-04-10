package chronicle

import (
	"errors"
	"time"
)

// Config configures the Chronicle snapshot system.
type Config struct {
	// MaxSnapshots is the maximum number of snapshots to retain.
	MaxSnapshots int `yaml:"max_snapshots" json:"max_snapshots"`
	// MaxAgeDays is the maximum age of snapshots in days.
	MaxAgeDays int `yaml:"max_age_days" json:"max_age_days"`
}

// ActionRequest is the minimal view Chronicle needs of an action proposal.
type ActionRequest struct {
	// Type is the action type string (e.g., "write_file").
	Type string
	// Payload holds action-specific parameters.
	Payload map[string]any
}

// SnapshotMetadata describes a Chronicle snapshot.
type SnapshotMetadata struct {
	// ID is the unique snapshot identifier.
	ID string `json:"id"`
	// TransactionID links this snapshot to a transaction, if any.
	TransactionID string `json:"transaction_id,omitempty"`
	// Timestamp is when the snapshot was created.
	Timestamp time.Time `json:"timestamp"`
	// ActionType is the type of action that triggered this snapshot.
	ActionType string `json:"action_type"`
	// ActionSummary is a human-readable description of the action.
	ActionSummary string `json:"action_summary"`
	// FilesBackedUp is the list of file paths copied into this snapshot.
	FilesBackedUp []string `json:"files_backed_up"`
	// Hash is the integrity hash of this snapshot's metadata.
	Hash string `json:"hash"`
	// PreviousHash is the hash of the preceding snapshot in the chain.
	PreviousHash string `json:"previous_hash"`
}

// FileChange describes a single file change in a diff.
type FileChange struct {
	// Path is the file path that changed.
	Path string `json:"path"`
	// ChangeType is "created", "modified", or "deleted".
	ChangeType string `json:"change_type"`
	// BeforeHash is the SHA-256 of the file before the change.
	BeforeHash string `json:"before_hash,omitempty"`
	// AfterHash is the SHA-256 of the file after the change.
	AfterHash string `json:"after_hash,omitempty"`
	// SizeBytes is the file size after the change.
	SizeBytes int64 `json:"size_bytes"`
}

// Diff is the set of changes between two points in time.
type Diff struct {
	// FromSnapshot is the starting snapshot ID.
	FromSnapshot string `json:"from_snapshot"`
	// ToSnapshot is the ending snapshot ID.
	ToSnapshot string `json:"to_snapshot"`
	// Changes is the list of file changes.
	Changes []FileChange `json:"changes"`
	// Timestamp is when the diff was computed.
	Timestamp time.Time `json:"timestamp"`
}

// Transaction represents an active or completed transaction.
type Transaction struct {
	// ID is the unique transaction identifier.
	ID string `json:"id"`
	// Status is "active", "committed", or "rolled_back".
	Status string `json:"status"`
	// StartedAt is when the transaction began.
	StartedAt time.Time `json:"started_at"`
	// FinishedAt is when the transaction was committed or rolled back.
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	// Snapshots is the list of snapshot IDs belonging to this transaction.
	Snapshots []string `json:"snapshots"`
}

// Action type constants Chronicle uses for filtering.
const (
	// ActionWriteFile is a file write action.
	ActionWriteFile = "write_file"
	// ActionDeleteFile is a file deletion action.
	ActionDeleteFile = "delete_file"
	// ActionMoveFile is a file move action.
	ActionMoveFile = "move_file"
)

// Sentinel errors.
var (
	// ErrSnapshotNotFound indicates the requested snapshot does not exist.
	ErrSnapshotNotFound = errors.New("snapshot not found")
	// ErrIntegrityViolation indicates the hash chain has been broken.
	ErrIntegrityViolation = errors.New("integrity chain violation detected")
)
