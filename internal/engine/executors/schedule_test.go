package executors

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/openparallax/openparallax/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScheduleAddValid(t *testing.T) {
	dir := t.TempDir()
	s := NewScheduleExecutor(dir)

	result := s.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionCreateSchedule,
		Payload: map[string]any{"cron": "0 9 * * *", "task": "Daily standup reminder"},
	})

	assert.True(t, result.Success)
	assert.Contains(t, result.Output, "0 9 * * *")
	assert.Contains(t, result.Output, "Daily standup reminder")

	// Verify HEARTBEAT.md was created.
	data, err := os.ReadFile(filepath.Join(dir, "HEARTBEAT.md"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "`0 9 * * *`")
	assert.Contains(t, string(data), "Daily standup reminder")
}

func TestScheduleAddInvalidCron(t *testing.T) {
	dir := t.TempDir()
	s := NewScheduleExecutor(dir)

	result := s.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionCreateSchedule,
		Payload: map[string]any{"cron": "invalid", "task": "Bad schedule"},
	})

	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "5 fields")
}

func TestScheduleAddTooManyFields(t *testing.T) {
	dir := t.TempDir()
	s := NewScheduleExecutor(dir)

	result := s.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionCreateSchedule,
		Payload: map[string]any{"cron": "0 9 * * * 2026", "task": "Six fields"},
	})

	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "5 fields")
}

func TestScheduleAddMissingCron(t *testing.T) {
	dir := t.TempDir()
	s := NewScheduleExecutor(dir)

	result := s.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionCreateSchedule,
		Payload: map[string]any{"task": "No cron"},
	})

	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "cron and task are required")
}

func TestScheduleAddMissingTask(t *testing.T) {
	dir := t.TempDir()
	s := NewScheduleExecutor(dir)

	result := s.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionCreateSchedule,
		Payload: map[string]any{"cron": "0 9 * * *"},
	})

	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "cron and task are required")
}

func TestScheduleListEmpty(t *testing.T) {
	dir := t.TempDir()
	s := NewScheduleExecutor(dir)

	result := s.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionListSchedules,
		Payload: map[string]any{},
	})

	assert.True(t, result.Success)
	assert.Contains(t, result.Output, "No scheduled tasks")
}

func TestScheduleListAfterAdd(t *testing.T) {
	dir := t.TempDir()
	s := NewScheduleExecutor(dir)

	// Add two schedules.
	s.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionCreateSchedule,
		Payload: map[string]any{"cron": "0 9 * * 1", "task": "Monday standup"},
	})
	s.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r2", Type: types.ActionCreateSchedule,
		Payload: map[string]any{"cron": "30 17 * * 5", "task": "Friday report"},
	})

	result := s.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r3", Type: types.ActionListSchedules,
		Payload: map[string]any{},
	})

	assert.True(t, result.Success)
	assert.Contains(t, result.Output, "Monday standup")
	assert.Contains(t, result.Output, "Friday report")
	assert.Contains(t, result.Summary, "2 scheduled tasks")
}

func TestScheduleRemoveByIndex(t *testing.T) {
	dir := t.TempDir()
	s := NewScheduleExecutor(dir)

	s.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionCreateSchedule,
		Payload: map[string]any{"cron": "0 8 * * *", "task": "Morning check"},
	})
	s.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r2", Type: types.ActionCreateSchedule,
		Payload: map[string]any{"cron": "0 17 * * *", "task": "Evening check"},
	})

	// Remove first entry.
	result := s.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r3", Type: types.ActionDeleteSchedule,
		Payload: map[string]any{"index": float64(1)},
	})

	assert.True(t, result.Success)
	assert.Contains(t, result.Output, "Morning check")

	// Verify only evening remains.
	list := s.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r4", Type: types.ActionListSchedules,
		Payload: map[string]any{},
	})
	assert.Contains(t, list.Output, "Evening check")
	assert.NotContains(t, list.Output, "Morning check")
}

func TestScheduleRemoveOutOfRange(t *testing.T) {
	dir := t.TempDir()
	s := NewScheduleExecutor(dir)

	s.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionCreateSchedule,
		Payload: map[string]any{"cron": "0 9 * * *", "task": "Only one"},
	})

	result := s.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r2", Type: types.ActionDeleteSchedule,
		Payload: map[string]any{"index": float64(5)},
	})

	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "out of range")
}

func TestScheduleRemoveZeroIndex(t *testing.T) {
	dir := t.TempDir()
	s := NewScheduleExecutor(dir)

	s.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionCreateSchedule,
		Payload: map[string]any{"cron": "0 9 * * *", "task": "Task"},
	})

	result := s.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r2", Type: types.ActionDeleteSchedule,
		Payload: map[string]any{"index": float64(0)},
	})

	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "out of range")
}

func TestScheduleRemoveMissingIndex(t *testing.T) {
	dir := t.TempDir()
	s := NewScheduleExecutor(dir)

	result := s.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionDeleteSchedule,
		Payload: map[string]any{},
	})

	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "index is required")
}

func TestScheduleRemoveStringIndex(t *testing.T) {
	dir := t.TempDir()
	s := NewScheduleExecutor(dir)

	s.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionCreateSchedule,
		Payload: map[string]any{"cron": "0 9 * * *", "task": "Task"},
	})

	result := s.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r2", Type: types.ActionDeleteSchedule,
		Payload: map[string]any{"index": "1"},
	})

	assert.True(t, result.Success)
}

func TestScheduleSupportedActions(t *testing.T) {
	s := NewScheduleExecutor(t.TempDir())
	actions := s.SupportedActions()
	assert.Len(t, actions, 3)
}

func TestScheduleToolSchemas(t *testing.T) {
	s := NewScheduleExecutor(t.TempDir())
	schemas := s.ToolSchemas()
	assert.Len(t, schemas, 3)
	names := make(map[string]bool)
	for _, schema := range schemas {
		names[schema.Name] = true
	}
	assert.True(t, names["create_schedule"])
	assert.True(t, names["list_schedules"])
	assert.True(t, names["delete_schedule"])
}
