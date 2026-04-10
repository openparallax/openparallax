package session

import "github.com/openparallax/openparallax/internal/types"

// otrAllowed is the whitelist of action types permitted in OTR mode.
// Only read-only, non-persisting actions are allowed.
var otrAllowed = map[types.ActionType]bool{
	types.ActionReadFile:       true,
	types.ActionListDir:        true,
	types.ActionSearchFiles:    true,
	types.ActionHTTPRequest:    true,
	types.ActionBrowserNav:     true,
	types.ActionBrowserExtract: true,
	types.ActionBrowserShot:    true,
	types.ActionReadCalendar:   true,
	types.ActionGitStatus:      true,
	types.ActionGitDiff:        true,
	types.ActionGitLog:         true,
	types.ActionMemorySearch:   true,
}

// IsOTRAllowed returns true if the action type is permitted in OTR mode.
func IsOTRAllowed(at types.ActionType) bool {
	return otrAllowed[at]
}

// OTRBlockReason returns a user-facing explanation for why the action is blocked in OTR mode.
func OTRBlockReason(at types.ActionType) string {
	reasons := map[types.ActionType]string{
		types.ActionWriteFile:      "File writes are not available in Off the Record mode. Switch to a normal session to write files.",
		types.ActionDeleteFile:     "File deletion is not available in Off the Record mode.",
		types.ActionMoveFile:       "Moving files is not available in Off the Record mode.",
		types.ActionCopyFile:       "Copying files is not available in Off the Record mode.",
		types.ActionCreateDir:      "Creating directories is not available in Off the Record mode.",
		types.ActionExecCommand:    "Command execution is not available in Off the Record mode.",
		types.ActionSendMessage:    "Sending messages is not available in Off the Record mode.",
		types.ActionSendEmail:      "Sending emails is not available in Off the Record mode.",
		types.ActionMemoryWrite:    "Memory updates are not available in Off the Record mode.",
		types.ActionCreateSchedule: "Creating schedules is not available in Off the Record mode.",
		types.ActionDeleteSchedule: "Deleting schedules is not available in Off the Record mode.",
		types.ActionCreateEvent:    "Creating calendar events is not available in Off the Record mode.",
		types.ActionUpdateEvent:    "Updating calendar events is not available in Off the Record mode.",
		types.ActionDeleteEvent:    "Deleting calendar events is not available in Off the Record mode.",
		types.ActionGitCommit:      "Git commits are not available in Off the Record mode.",
		types.ActionGitPush:        "Git push is not available in Off the Record mode.",
		types.ActionGitPull:        "Git pull is not available in Off the Record mode.",
		types.ActionGitBranch:      "Git branch operations are not available in Off the Record mode.",
		types.ActionGitCheckout:    "Git checkout is not available in Off the Record mode.",
		types.ActionBrowserClick:   "Browser interactions are not available in Off the Record mode.",
		types.ActionBrowserType:    "Browser form input is not available in Off the Record mode.",
		types.ActionCanvasCreate:   "Creating files is not available in Off the Record mode.",
		types.ActionCanvasUpdate:   "Updating files is not available in Off the Record mode.",
	}
	if reason, ok := reasons[at]; ok {
		return reason
	}
	return string(at) + " is not available in Off the Record mode. Switch to a normal session."
}
