// Package types defines the shared data structures used across all OpenParallax packages.
package types

import (
	"github.com/openparallax/openparallax/shield"
)

// ActionType is an alias for the public shield type.
type ActionType = shield.ActionType

const (
	// ActionReadFile reads a file from the filesystem.
	ActionReadFile ActionType = "read_file"
	// ActionWriteFile writes content to a file.
	ActionWriteFile ActionType = "write_file"
	// ActionDeleteFile deletes a file from the filesystem.
	ActionDeleteFile ActionType = "delete_file"
	// ActionMoveFile moves or renames a file.
	ActionMoveFile ActionType = "move_file"
	// ActionCopyFile copies a file to a new location.
	ActionCopyFile ActionType = "copy_file"
	// ActionCreateDir creates a directory.
	ActionCreateDir ActionType = "create_directory"
	// ActionListDir lists the contents of a directory.
	ActionListDir ActionType = "list_directory"
	// ActionSearchFiles searches for files matching a pattern.
	ActionSearchFiles ActionType = "search_files"
	// ActionCopyDir copies a directory recursively.
	ActionCopyDir ActionType = "copy_directory"
	// ActionMoveDir moves or renames a directory.
	ActionMoveDir ActionType = "move_directory"
	// ActionDeleteDir deletes a directory recursively.
	ActionDeleteDir ActionType = "delete_directory"
	// ActionExecCommand executes a shell command.
	ActionExecCommand ActionType = "execute_command"
	// ActionSendMessage sends a message via a channel adapter.
	ActionSendMessage ActionType = "send_message"
	// ActionSendEmail sends an email via SMTP.
	ActionSendEmail ActionType = "send_email"
	// ActionHTTPRequest performs an HTTP request.
	ActionHTTPRequest ActionType = "http_request"
	// ActionBrowserNav navigates a browser to a URL.
	ActionBrowserNav ActionType = "browser_navigate"
	// ActionBrowserClick clicks an element in the browser.
	ActionBrowserClick ActionType = "browser_click"
	// ActionBrowserType types text into a browser element.
	ActionBrowserType ActionType = "browser_type"
	// ActionBrowserExtract extracts content from the browser.
	ActionBrowserExtract ActionType = "browser_extract"
	// ActionBrowserShot takes a screenshot of the browser.
	ActionBrowserShot ActionType = "browser_screenshot"
	// ActionCreateSchedule creates a recurring schedule entry.
	ActionCreateSchedule ActionType = "create_schedule"
	// ActionDeleteSchedule removes a schedule entry.
	ActionDeleteSchedule ActionType = "delete_schedule"
	// ActionListSchedules lists all schedule entries.
	ActionListSchedules ActionType = "list_schedules"
	// ActionReadCalendar reads calendar events.
	ActionReadCalendar ActionType = "read_calendar"
	// ActionCreateEvent creates a calendar event.
	ActionCreateEvent ActionType = "create_event"
	// ActionUpdateEvent updates a calendar event.
	ActionUpdateEvent ActionType = "update_event"
	// ActionDeleteEvent deletes a calendar event.
	ActionDeleteEvent ActionType = "delete_event"
	// ActionGitStatus shows the git working tree status.
	ActionGitStatus ActionType = "git_status"
	// ActionGitDiff shows the git diff.
	ActionGitDiff ActionType = "git_diff"
	// ActionGitCommit creates a git commit.
	ActionGitCommit ActionType = "git_commit"
	// ActionGitPush pushes to a git remote.
	ActionGitPush ActionType = "git_push"
	// ActionGitPull pulls from a git remote.
	ActionGitPull ActionType = "git_pull"
	// ActionGitLog shows the git log.
	ActionGitLog ActionType = "git_log"
	// ActionGitBranch manages git branches.
	ActionGitBranch ActionType = "git_branch"
	// ActionGitCheckout checks out a git branch or commit.
	ActionGitCheckout ActionType = "git_checkout"
	// ActionMemoryWrite writes to a workspace memory file.
	ActionMemoryWrite ActionType = "memory_write"
	// ActionMemorySearch searches memory via FTS5.
	ActionMemorySearch ActionType = "memory_search"
	// ActionCanvasCreate creates a canvas file (Mermaid, SVG, Markdown, HTML).
	ActionCanvasCreate ActionType = "canvas_create"
	// ActionCanvasUpdate updates an existing canvas file.
	ActionCanvasUpdate ActionType = "canvas_update"
	// ActionCanvasProject creates a multi-file project in a directory.
	ActionCanvasProject ActionType = "canvas_project"
	// ActionGitClone clones a git repository.
	ActionGitClone ActionType = "git_clone"

	// ActionGenerateImage generates an image from a text prompt.
	ActionGenerateImage ActionType = "generate_image"
	// ActionEditImage edits an existing image based on a prompt.
	ActionEditImage ActionType = "edit_image"
	// ActionGenerateVideo generates a short video from a text prompt.
	ActionGenerateVideo ActionType = "generate_video"

	// ActionEmailList lists emails in a mailbox folder.
	ActionEmailList ActionType = "email_list"
	// ActionEmailRead reads a specific email by UID.
	ActionEmailRead ActionType = "email_read"
	// ActionEmailSearch searches emails by query.
	ActionEmailSearch ActionType = "email_search"
	// ActionEmailMove moves an email to a different folder.
	ActionEmailMove ActionType = "email_move"
	// ActionEmailMark marks an email as read/unread/flagged.
	ActionEmailMark ActionType = "email_mark"

	// ActionGrepFiles searches file contents for a text pattern.
	ActionGrepFiles ActionType = "grep_files"
	// ActionClipboardRead reads from the system clipboard.
	ActionClipboardRead ActionType = "clipboard_read"
	// ActionClipboardWrite writes to the system clipboard.
	ActionClipboardWrite ActionType = "clipboard_write"
	// ActionOpen launches a file or URL in the default application.
	ActionOpen ActionType = "open"
	// ActionNotify sends an OS notification.
	ActionNotify ActionType = "notify"
	// ActionSystemInfo returns system information (disk, memory, CPU, etc.).
	ActionSystemInfo ActionType = "system_info"
	// ActionScreenshot captures the desktop screen.
	ActionScreenshot ActionType = "screenshot"
	// ActionCalculate evaluates a mathematical expression.
	ActionCalculate ActionType = "calculate"
	// ActionArchiveCreate creates a zip or tar.gz archive.
	ActionArchiveCreate ActionType = "archive_create"
	// ActionArchiveExtract extracts a zip or tar.gz archive.
	ActionArchiveExtract ActionType = "archive_extract"
	// ActionPDFRead extracts text from a PDF file.
	ActionPDFRead ActionType = "pdf_read"
	// ActionSpreadsheetRead reads data from a CSV or Excel file.
	ActionSpreadsheetRead ActionType = "spreadsheet_read"
	// ActionSpreadsheetWrite writes data to a CSV or Excel file.
	ActionSpreadsheetWrite ActionType = "spreadsheet_write"

	// ActionCreateAgent spawns a sub-agent to work on a task.
	ActionCreateAgent ActionType = "create_agent"
	// ActionAgentStatus checks the status of a sub-agent.
	ActionAgentStatus ActionType = "agent_status"
	// ActionAgentResult collects the result from a completed sub-agent.
	ActionAgentResult ActionType = "agent_result"
	// ActionAgentMessage sends an additional instruction to a running sub-agent.
	ActionAgentMessage ActionType = "agent_message"
	// ActionDeleteAgent terminates a sub-agent immediately.
	ActionDeleteAgent ActionType = "delete_agent"
	// ActionListAgents lists all active sub-agents.
	ActionListAgents ActionType = "list_agents"
)

// AllActionTypes contains every defined action type for enumeration and validation.
var AllActionTypes = []ActionType{
	ActionReadFile, ActionWriteFile, ActionDeleteFile, ActionMoveFile,
	ActionCopyFile, ActionCreateDir, ActionListDir, ActionSearchFiles,
	ActionCopyDir, ActionMoveDir, ActionDeleteDir,
	ActionExecCommand, ActionSendMessage, ActionSendEmail, ActionHTTPRequest,
	ActionBrowserNav, ActionBrowserClick, ActionBrowserType, ActionBrowserExtract,
	ActionBrowserShot, ActionCreateSchedule, ActionDeleteSchedule, ActionListSchedules,
	ActionReadCalendar, ActionCreateEvent, ActionUpdateEvent, ActionDeleteEvent,
	ActionGitStatus, ActionGitDiff, ActionGitCommit, ActionGitPush,
	ActionGitPull, ActionGitLog, ActionGitBranch, ActionGitCheckout,
	ActionMemoryWrite, ActionMemorySearch, ActionCanvasCreate, ActionCanvasUpdate,
	ActionCanvasProject, ActionGitClone,
	ActionGenerateImage, ActionEditImage, ActionGenerateVideo,
	ActionEmailList, ActionEmailRead, ActionEmailSearch,
	ActionEmailMove, ActionEmailMark,
	ActionCreateAgent, ActionAgentStatus, ActionAgentResult,
	ActionAgentMessage, ActionDeleteAgent, ActionListAgents,
	ActionGrepFiles,
	ActionClipboardRead, ActionClipboardWrite, ActionOpen,
	ActionNotify, ActionSystemInfo, ActionScreenshot,
	ActionCalculate,
	ActionArchiveCreate, ActionArchiveExtract,
	ActionPDFRead, ActionSpreadsheetRead, ActionSpreadsheetWrite,
}

// ActionRequest is an alias for the public shield type.
type ActionRequest = shield.ActionRequest

// ActionResult is the outcome of executing an action.
type ActionResult struct {
	// RequestID matches the ActionRequest.
	RequestID string `json:"request_id"`

	// Success indicates whether the action completed without error.
	Success bool `json:"success"`

	// Output is the primary output (file content, stdout, etc.).
	Output string `json:"output,omitempty"`

	// Error is the error message if !Success.
	Error string `json:"error,omitempty"`

	// Summary is a human-readable one-line summary.
	Summary string `json:"summary"`

	// DurationMs is how long execution took.
	DurationMs int64 `json:"duration_ms"`

	// BytesWritten is set for write operations.
	BytesWritten int64 `json:"bytes_written,omitempty"`
}
