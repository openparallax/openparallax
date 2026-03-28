package executors

import (
	"context"
	"testing"

	"github.com/openparallax/openparallax/internal/types"
	"github.com/stretchr/testify/assert"
)

func TestRegistryAlwaysHasFileShellGitHTTPScheduleCanvas(t *testing.T) {
	r := NewRegistry(t.TempDir(), nil, nil)

	// These action types should always be registered.
	alwaysAvailable := []types.ActionType{
		types.ActionReadFile, types.ActionWriteFile,
		types.ActionExecCommand,
		types.ActionGitStatus, types.ActionGitCommit,
		types.ActionHTTPRequest,
		types.ActionCreateSchedule, types.ActionListSchedules,
		types.ActionCanvasCreate, types.ActionCanvasUpdate,
	}

	available := r.AvailableActions()
	for _, at := range alwaysAvailable {
		found := false
		for _, a := range available {
			if a == at {
				found = true
				break
			}
		}
		assert.True(t, found, "expected action type %s to be registered", at)
	}
}

func TestRegistryEmailNotRegisteredWithoutConfig(t *testing.T) {
	r := NewRegistry(t.TempDir(), nil, nil)
	result := r.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionSendEmail,
		Payload: map[string]any{},
	})
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "no executor registered")
}

func TestRegistryEmailRegisteredWithConfig(t *testing.T) {
	cfg := &types.AgentConfig{
		Email: types.EmailConfig{
			SMTP: types.SMTPConfig{Host: "smtp.test.com", Port: 587, From: "test@test.com"},
		},
	}
	r := NewRegistry(t.TempDir(), cfg, nil)

	available := r.AvailableActions()
	found := false
	for _, a := range available {
		if a == types.ActionSendEmail {
			found = true
			break
		}
	}
	assert.True(t, found, "send_email should be registered when SMTP is configured")
}

func TestRegistryCalendarNotRegisteredWithoutConfig(t *testing.T) {
	r := NewRegistry(t.TempDir(), nil, nil)
	result := r.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionReadCalendar,
		Payload: map[string]any{},
	})
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "no executor registered")
}

func TestRegistryAllToolSchemasNoDuplicates(t *testing.T) {
	r := NewRegistry(t.TempDir(), nil, nil)
	schemas := r.AllToolSchemas()

	names := make(map[string]int)
	for _, s := range schemas {
		names[s.Name]++
	}

	for name, count := range names {
		assert.Equal(t, 1, count, "duplicate tool schema: %s", name)
	}
}

func TestRegistryAllToolSchemasHaveRequiredFields(t *testing.T) {
	r := NewRegistry(t.TempDir(), nil, nil)
	schemas := r.AllToolSchemas()

	for _, s := range schemas {
		assert.NotEmpty(t, s.Name, "schema missing name")
		assert.NotEmpty(t, s.Description, "schema %s missing description", s.Name)
		assert.NotEmpty(t, s.ActionType, "schema %s missing action type", s.Name)
		assert.NotNil(t, s.Parameters, "schema %s missing parameters", s.Name)
	}
}

func TestRegistryDispatchToCorrectExecutor(t *testing.T) {
	r := NewRegistry(t.TempDir(), nil, nil)

	// Shell should work.
	result := r.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionExecCommand,
		Payload: map[string]any{"command": "echo dispatch_test"},
	})
	assert.True(t, result.Success)
	assert.Contains(t, result.Output, "dispatch_test")
}

func TestRegistryRejectUnknownActionType(t *testing.T) {
	r := NewRegistry(t.TempDir(), nil, nil)
	result := r.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: "totally_fake_action",
		Payload: map[string]any{},
	})
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "no executor registered")
}
