package executors

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/openparallax/openparallax/internal/heartbeat"
	"github.com/openparallax/openparallax/internal/types"
)

// ScheduleExecutor manages HEARTBEAT.md cron entries.
type ScheduleExecutor struct {
	workspacePath string
}

// NewScheduleExecutor creates a schedule executor.
func NewScheduleExecutor(workspace string) *ScheduleExecutor {
	return &ScheduleExecutor{workspacePath: workspace}
}

// WorkspaceScope reports that schedule writes are confined to the workspace.
func (s *ScheduleExecutor) WorkspaceScope() WorkspaceScope { return ScopeScoped }

func (s *ScheduleExecutor) SupportedActions() []types.ActionType {
	return []types.ActionType{types.ActionCreateSchedule, types.ActionDeleteSchedule, types.ActionListSchedules}
}

func (s *ScheduleExecutor) ToolSchemas() []ToolSchema {
	return []ToolSchema{
		{
			ActionType:  types.ActionCreateSchedule,
			Name:        "create_schedule",
			Description: "Add a scheduled task that runs on a cron schedule. The task description is what the agent will do when the schedule fires.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"cron": map[string]any{"type": "string", "description": "5-field cron expression (minute hour day month weekday). Example: '0 9 * * *' for daily at 9am."},
					"task": map[string]any{"type": "string", "description": "Description of what to do when the schedule fires."},
				},
				"required": []string{"cron", "task"},
			},
		},
		{
			ActionType:  types.ActionListSchedules,
			Name:        "list_schedules",
			Description: "List all scheduled tasks from HEARTBEAT.md.",
			Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			ActionType:  types.ActionDeleteSchedule,
			Name:        "delete_schedule",
			Description: "Remove a scheduled task by its position number (1-based).",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"index": map[string]any{"type": "integer", "description": "1-based position of the task to remove."},
				},
				"required": []string{"index"},
			},
		},
	}
}

func (s *ScheduleExecutor) Execute(_ context.Context, action *types.ActionRequest) *types.ActionResult {
	switch action.Type {
	case types.ActionCreateSchedule:
		return s.addSchedule(action)
	case types.ActionListSchedules:
		return s.listSchedules(action)
	case types.ActionDeleteSchedule:
		return s.removeSchedule(action)
	default:
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "unknown schedule action"}
	}
}

func (s *ScheduleExecutor) heartbeatPath() string {
	return filepath.Join(s.workspacePath, "HEARTBEAT.md")
}

func (s *ScheduleExecutor) addSchedule(action *types.ActionRequest) *types.ActionResult {
	cronExpr, _ := action.Payload["cron"].(string)
	task, _ := action.Payload["task"].(string)

	if cronExpr == "" || task == "" {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "cron and task are required", Summary: "schedule add failed"}
	}

	// Validate cron expression.
	fields := strings.Fields(cronExpr)
	if len(fields) != 5 {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "cron expression must have 5 fields (minute hour day month weekday)", Summary: "invalid cron expression"}
	}

	entry := fmt.Sprintf("- `%s` — %s\n", cronExpr, task)

	f, err := os.OpenFile(s.heartbeatPath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: err.Error(), Summary: "schedule add failed"}
	}
	defer func() { _ = f.Close() }()

	if _, err := f.WriteString(entry); err != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: err.Error(), Summary: "schedule add failed"}
	}

	return &types.ActionResult{
		RequestID: action.RequestID, Success: true,
		Output:  fmt.Sprintf("Added: %s → %s", cronExpr, task),
		Summary: "schedule added",
	}
}

func (s *ScheduleExecutor) listSchedules(action *types.ActionRequest) *types.ActionResult {
	data, err := os.ReadFile(s.heartbeatPath())
	if err != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: true, Output: "No scheduled tasks.", Summary: "no schedules"}
	}

	entries := heartbeat.ParseCronEntries(string(data))
	if len(entries) == 0 {
		return &types.ActionResult{RequestID: action.RequestID, Success: true, Output: "No scheduled tasks.", Summary: "no schedules"}
	}

	var lines []string
	for i, e := range entries {
		lines = append(lines, fmt.Sprintf("%d. `%s %s %s %s %s` — %s", i+1, e.Minute, e.Hour, e.Day, e.Month, e.Weekday, e.Task))
	}

	return &types.ActionResult{
		RequestID: action.RequestID, Success: true,
		Output:  strings.Join(lines, "\n"),
		Summary: fmt.Sprintf("%d scheduled tasks", len(entries)),
	}
}

func (s *ScheduleExecutor) removeSchedule(action *types.ActionRequest) *types.ActionResult {
	idxRaw := action.Payload["index"]
	var idx int
	switch v := idxRaw.(type) {
	case float64:
		idx = int(v)
	case int:
		idx = v
	case string:
		var err error
		idx, err = strconv.Atoi(v)
		if err != nil {
			return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "invalid index", Summary: "schedule remove failed"}
		}
	default:
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "index is required", Summary: "schedule remove failed"}
	}

	data, err := os.ReadFile(s.heartbeatPath())
	if err != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: err.Error(), Summary: "schedule remove failed"}
	}

	entries := heartbeat.ParseCronEntries(string(data))
	if idx < 1 || idx > len(entries) {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: fmt.Sprintf("index %d out of range (1-%d)", idx, len(entries)), Summary: "schedule remove failed"}
	}

	// Remove the entry by rebuilding the file without the target line.
	removed := entries[idx-1]
	lines := strings.Split(string(data), "\n")
	removeLine := fmt.Sprintf("- `%s %s %s %s %s` — %s", removed.Minute, removed.Hour, removed.Day, removed.Month, removed.Weekday, removed.Task)

	var newLines []string
	removedOne := false
	for _, line := range lines {
		if !removedOne && strings.TrimSpace(line) == strings.TrimSpace(removeLine) {
			removedOne = true
			continue
		}
		newLines = append(newLines, line)
	}

	if err := os.WriteFile(s.heartbeatPath(), []byte(strings.Join(newLines, "\n")), 0o644); err != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: err.Error(), Summary: "schedule remove failed"}
	}

	return &types.ActionResult{
		RequestID: action.RequestID, Success: true,
		Output:  fmt.Sprintf("Removed: %s", removed.Task),
		Summary: "schedule removed",
	}
}
