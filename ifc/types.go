// Package ifc implements Information Flow Control for AI agent pipelines.
// It classifies data by sensitivity level and prevents unauthorized data
// movement across sensitivity boundaries.
package ifc

// ActionType represents the category of action for flow control decisions.
type ActionType string

// Action type constants used by IFC for flow control routing.
const (
	// ActionReadFile reads a file.
	ActionReadFile ActionType = "read_file"
	// ActionWriteFile writes content to a file.
	ActionWriteFile ActionType = "write_file"
	// ActionDeleteFile deletes a file.
	ActionDeleteFile ActionType = "delete_file"
	// ActionMoveFile moves or renames a file.
	ActionMoveFile ActionType = "move_file"
	// ActionCopyFile copies a file.
	ActionCopyFile ActionType = "copy_file"
	// ActionCreateDir creates a directory.
	ActionCreateDir ActionType = "create_directory"
	// ActionListDir lists directory contents.
	ActionListDir ActionType = "list_directory"
	// ActionSearchFiles searches for files.
	ActionSearchFiles ActionType = "search_files"
	// ActionExecCommand executes a shell command.
	ActionExecCommand ActionType = "execute_command"
	// ActionSendMessage sends a message.
	ActionSendMessage ActionType = "send_message"
	// ActionSendEmail sends an email.
	ActionSendEmail ActionType = "send_email"
	// ActionHTTPRequest makes an HTTP request.
	ActionHTTPRequest ActionType = "http_request"
	// ActionMemoryWrite writes to memory.
	ActionMemoryWrite ActionType = "memory_write"
	// ActionMemorySearch searches memory.
	ActionMemorySearch ActionType = "memory_search"
)

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
	// Sensitivity is the data sensitivity level.
	Sensitivity SensitivityLevel `json:"sensitivity"`
	// SourcePath is where the data came from.
	SourcePath string `json:"source_path,omitempty"`
	// ContentType classifies the data content.
	ContentType string `json:"content_type,omitempty"`
}
