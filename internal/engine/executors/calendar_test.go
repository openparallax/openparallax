package executors

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/openparallax/openparallax/internal/types"
	"github.com/stretchr/testify/assert"
)

// mockCalendarProvider records calls for verification.
type mockCalendarProvider struct {
	events    []CalendarEvent
	created   *CalendarEvent
	updated   *CalendarEvent
	deletedID string
	listErr   error
	createErr error
	updateErr error
	deleteErr error
}

func (m *mockCalendarProvider) ListEvents(_ context.Context, _, _ time.Time) ([]CalendarEvent, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.events, nil
}

func (m *mockCalendarProvider) CreateEvent(_ context.Context, event *CalendarEvent) (*CalendarEvent, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	m.created = event
	event.ID = "new-123"
	return event, nil
}

func (m *mockCalendarProvider) UpdateEvent(_ context.Context, id string, event *CalendarEvent) (*CalendarEvent, error) {
	if m.updateErr != nil {
		return nil, m.updateErr
	}
	m.updated = event
	return event, nil
}

func (m *mockCalendarProvider) DeleteEvent(_ context.Context, id string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	m.deletedID = id
	return nil
}

func newTestCalendarExecutor(provider CalendarProvider) *CalendarExecutor {
	return &CalendarExecutor{provider: provider}
}

func TestCalendarReadEmpty(t *testing.T) {
	mock := &mockCalendarProvider{}
	c := newTestCalendarExecutor(mock)

	result := c.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionReadCalendar,
		Payload: map[string]any{},
	})

	assert.True(t, result.Success)
	assert.Contains(t, result.Output, "No upcoming events")
}

func TestCalendarReadWithEvents(t *testing.T) {
	mock := &mockCalendarProvider{
		events: []CalendarEvent{
			{Title: "Standup", Start: time.Date(2026, 3, 28, 9, 0, 0, 0, time.UTC), End: time.Date(2026, 3, 28, 9, 15, 0, 0, time.UTC)},
			{Title: "Lunch", Start: time.Date(2026, 3, 28, 12, 0, 0, 0, time.UTC), End: time.Date(2026, 3, 28, 13, 0, 0, 0, time.UTC), Location: "Cafeteria"},
		},
	}
	c := newTestCalendarExecutor(mock)

	result := c.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionReadCalendar,
		Payload: map[string]any{},
	})

	assert.True(t, result.Success)
	assert.Contains(t, result.Output, "Standup")
	assert.Contains(t, result.Output, "Lunch")
	assert.Contains(t, result.Output, "Cafeteria")
	assert.Contains(t, result.Summary, "2 events")
}

func TestCalendarReadCustomDays(t *testing.T) {
	mock := &mockCalendarProvider{
		events: []CalendarEvent{
			{Title: "Event", Start: time.Date(2026, 4, 5, 10, 0, 0, 0, time.UTC), End: time.Date(2026, 4, 5, 11, 0, 0, 0, time.UTC)},
		},
	}
	c := newTestCalendarExecutor(mock)

	result := c.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionReadCalendar,
		Payload: map[string]any{"days_ahead": float64(14)},
	})

	assert.True(t, result.Success)
	assert.Contains(t, result.Summary, "14 days")
}

func TestCalendarReadError(t *testing.T) {
	mock := &mockCalendarProvider{listErr: errors.New("API unreachable")}
	c := newTestCalendarExecutor(mock)

	result := c.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionReadCalendar,
		Payload: map[string]any{},
	})

	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "API unreachable")
}

func TestCalendarCreateEvent(t *testing.T) {
	mock := &mockCalendarProvider{}
	c := newTestCalendarExecutor(mock)

	result := c.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionCreateEvent,
		Payload: map[string]any{
			"title":       "Team Meeting",
			"start":       "2026-03-30T14:00:00Z",
			"end":         "2026-03-30T15:00:00Z",
			"description": "Quarterly review",
			"location":    "Room 42",
		},
	})

	assert.True(t, result.Success)
	assert.Contains(t, result.Output, "Team Meeting")
	assert.NotNil(t, mock.created)
	assert.Equal(t, "Team Meeting", mock.created.Title)
	assert.Equal(t, "Quarterly review", mock.created.Description)
	assert.Equal(t, "Room 42", mock.created.Location)
}

func TestCalendarCreateInvalidStartTime(t *testing.T) {
	mock := &mockCalendarProvider{}
	c := newTestCalendarExecutor(mock)

	result := c.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionCreateEvent,
		Payload: map[string]any{"title": "Bad", "start": "not-a-time", "end": "2026-03-30T15:00:00Z"},
	})

	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "invalid start time")
}

func TestCalendarCreateInvalidEndTime(t *testing.T) {
	mock := &mockCalendarProvider{}
	c := newTestCalendarExecutor(mock)

	result := c.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionCreateEvent,
		Payload: map[string]any{"title": "Bad", "start": "2026-03-30T14:00:00Z", "end": "not-a-time"},
	})

	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "invalid end time")
}

func TestCalendarCreateProviderError(t *testing.T) {
	mock := &mockCalendarProvider{createErr: errors.New("quota exceeded")}
	c := newTestCalendarExecutor(mock)

	result := c.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionCreateEvent,
		Payload: map[string]any{"title": "Test", "start": "2026-03-30T14:00:00Z", "end": "2026-03-30T15:00:00Z"},
	})

	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "quota exceeded")
}

func TestCalendarUpdateEvent(t *testing.T) {
	mock := &mockCalendarProvider{}
	c := newTestCalendarExecutor(mock)

	result := c.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionUpdateEvent,
		Payload: map[string]any{"event_id": "evt-456", "title": "Updated Title"},
	})

	assert.True(t, result.Success)
	assert.NotNil(t, mock.updated)
	assert.Equal(t, "Updated Title", mock.updated.Title)
}

func TestCalendarUpdateMissingID(t *testing.T) {
	mock := &mockCalendarProvider{}
	c := newTestCalendarExecutor(mock)

	result := c.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionUpdateEvent,
		Payload: map[string]any{"title": "No ID"},
	})

	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "event_id is required")
}

func TestCalendarUpdateProviderError(t *testing.T) {
	mock := &mockCalendarProvider{updateErr: errors.New("event not found")}
	c := newTestCalendarExecutor(mock)

	result := c.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionUpdateEvent,
		Payload: map[string]any{"event_id": "evt-999"},
	})

	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "event not found")
}

func TestCalendarDeleteEvent(t *testing.T) {
	mock := &mockCalendarProvider{}
	c := newTestCalendarExecutor(mock)

	result := c.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionDeleteEvent,
		Payload: map[string]any{"event_id": "evt-789"},
	})

	assert.True(t, result.Success)
	assert.Equal(t, "evt-789", mock.deletedID)
}

func TestCalendarDeleteMissingID(t *testing.T) {
	mock := &mockCalendarProvider{}
	c := newTestCalendarExecutor(mock)

	result := c.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionDeleteEvent,
		Payload: map[string]any{},
	})

	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "event_id is required")
}

func TestCalendarDeleteProviderError(t *testing.T) {
	mock := &mockCalendarProvider{deleteErr: errors.New("permission denied")}
	c := newTestCalendarExecutor(mock)

	result := c.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: types.ActionDeleteEvent,
		Payload: map[string]any{"event_id": "evt-123"},
	})

	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "permission denied")
}

func TestCalendarUnknownAction(t *testing.T) {
	mock := &mockCalendarProvider{}
	c := newTestCalendarExecutor(mock)

	result := c.Execute(context.Background(), &types.ActionRequest{
		RequestID: "r1", Type: "calendar_unknown",
		Payload: map[string]any{},
	})

	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "unknown calendar action")
}

func TestNewCalendarExecutorNilWhenUnconfigured(t *testing.T) {
	assert.Nil(t, NewCalendarExecutor(types.CalendarConfig{}, nil))
}

func TestNewCalendarExecutorWithProviderButNoCreds(t *testing.T) {
	exec := NewCalendarExecutor(types.CalendarConfig{Provider: "google"}, nil)
	assert.Nil(t, exec, "google provider without credentials returns nil")
}

func TestNewCalendarExecutorWithCalDAVButNoURL(t *testing.T) {
	exec := NewCalendarExecutor(types.CalendarConfig{Provider: "caldav"}, nil)
	assert.Nil(t, exec, "caldav provider without URL returns nil")
}

func TestNewCalendarExecutorMicrosoftNilWithoutOAuth(t *testing.T) {
	exec := NewCalendarExecutor(types.CalendarConfig{
		Provider:         "microsoft",
		MicrosoftAccount: "user@outlook.com",
	}, nil)
	assert.Nil(t, exec, "microsoft provider without OAuth manager returns nil")
}

func TestCalendarSupportedActions(t *testing.T) {
	mock := &mockCalendarProvider{}
	c := newTestCalendarExecutor(mock)
	actions := c.SupportedActions()
	assert.Len(t, actions, 4)
}

func TestCalendarToolSchemas(t *testing.T) {
	mock := &mockCalendarProvider{}
	c := newTestCalendarExecutor(mock)
	schemas := c.ToolSchemas()
	assert.Len(t, schemas, 4)
	names := make(map[string]bool)
	for _, s := range schemas {
		names[s.Name] = true
	}
	assert.True(t, names["read_calendar"])
	assert.True(t, names["create_event"])
	assert.True(t, names["update_event"])
	assert.True(t, names["delete_event"])
}
