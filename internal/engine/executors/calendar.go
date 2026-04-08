package executors

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/openparallax/openparallax/internal/oauth"
	"github.com/openparallax/openparallax/internal/types"
)

// CalendarEvent represents a calendar event.
type CalendarEvent struct {
	ID          string
	Title       string
	Start       time.Time
	End         time.Time
	Description string
	Location    string
	Attendees   []string
}

// CalendarProvider is the interface for calendar backends.
// Implementations: Google Calendar, CalDAV, Microsoft Graph.
type CalendarProvider interface {
	ListEvents(ctx context.Context, from, to time.Time) ([]CalendarEvent, error)
	CreateEvent(ctx context.Context, event *CalendarEvent) (*CalendarEvent, error)
	UpdateEvent(ctx context.Context, id string, event *CalendarEvent) (*CalendarEvent, error)
	DeleteEvent(ctx context.Context, id string) error
}

// CalendarExecutor manages calendar events through a provider.
type CalendarExecutor struct {
	provider CalendarProvider
}

// NewCalendarExecutor creates a calendar executor. Returns nil if not configured.
// The oauthMgr parameter is required for Microsoft 365 calendar.
func NewCalendarExecutor(cfg types.CalendarConfig, oauthMgr *oauth.Manager) *CalendarExecutor {
	switch cfg.Provider {
	case "google":
		if cfg.GoogleCredFile == "" {
			return nil
		}
		return nil
	case "caldav":
		if cfg.CalDAVURL == "" {
			return nil
		}
		return nil
	case "microsoft":
		if oauthMgr == nil || cfg.MicrosoftAccount == "" {
			return nil
		}
		ctx := context.Background()
		has, _ := oauthMgr.HasTokens(ctx, "microsoft", cfg.MicrosoftAccount)
		if !has {
			return nil
		}
		return &CalendarExecutor{
			provider: newMS365CalendarProvider(oauthMgr, cfg.MicrosoftAccount),
		}
	default:
		return nil
	}
}

// WorkspaceScope reports that the calendar executor does not write to the filesystem.
func (c *CalendarExecutor) WorkspaceScope() WorkspaceScope { return ScopeNoFilesystem }

func (c *CalendarExecutor) SupportedActions() []types.ActionType {
	return []types.ActionType{
		types.ActionReadCalendar, types.ActionCreateEvent,
		types.ActionUpdateEvent, types.ActionDeleteEvent,
	}
}

func (c *CalendarExecutor) ToolSchemas() []ToolSchema {
	return []ToolSchema{
		{ActionType: types.ActionReadCalendar, Name: "read_calendar", Description: "List upcoming calendar events for the next N days.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"days_ahead": map[string]any{"type": "integer", "description": "Number of days to look ahead. Default 7."}}}},
		{ActionType: types.ActionCreateEvent, Name: "create_event", Description: "Create a new calendar event.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"title": map[string]any{"type": "string"}, "start": map[string]any{"type": "string", "description": "Start time in ISO 8601 format."}, "end": map[string]any{"type": "string", "description": "End time in ISO 8601 format."}, "description": map[string]any{"type": "string"}, "location": map[string]any{"type": "string"}, "attendees": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}}, "required": []string{"title", "start", "end"}}},
		{ActionType: types.ActionUpdateEvent, Name: "update_event", Description: "Update an existing calendar event.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"event_id": map[string]any{"type": "string"}, "title": map[string]any{"type": "string"}, "start": map[string]any{"type": "string"}, "end": map[string]any{"type": "string"}, "description": map[string]any{"type": "string"}, "location": map[string]any{"type": "string"}}, "required": []string{"event_id"}}},
		{ActionType: types.ActionDeleteEvent, Name: "delete_event", Description: "Delete a calendar event.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"event_id": map[string]any{"type": "string", "description": "The event ID to delete."}}, "required": []string{"event_id"}}},
	}
}

func (c *CalendarExecutor) Execute(ctx context.Context, action *types.ActionRequest) *types.ActionResult {
	switch action.Type {
	case types.ActionReadCalendar:
		return c.readCalendar(ctx, action)
	case types.ActionCreateEvent:
		return c.createEvent(ctx, action)
	case types.ActionUpdateEvent:
		return c.updateEvent(ctx, action)
	case types.ActionDeleteEvent:
		return c.deleteEvent(ctx, action)
	default:
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "unknown calendar action"}
	}
}

func (c *CalendarExecutor) readCalendar(ctx context.Context, action *types.ActionRequest) *types.ActionResult {
	daysAhead := 7
	if d, ok := action.Payload["days_ahead"].(float64); ok && d > 0 {
		daysAhead = int(d)
	}

	from := time.Now()
	to := from.AddDate(0, 0, daysAhead)

	events, err := c.provider.ListEvents(ctx, from, to)
	if err != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: err.Error(), Summary: "calendar read failed"}
	}

	if len(events) == 0 {
		return &types.ActionResult{RequestID: action.RequestID, Success: true, Output: "No upcoming events.", Summary: "no events"}
	}

	var lines []string
	for _, e := range events {
		line := fmt.Sprintf("- %s: %s (%s - %s)", e.Start.Format("Jan 02"), e.Title, e.Start.Format("15:04"), e.End.Format("15:04"))
		if e.Location != "" {
			line += " @ " + e.Location
		}
		lines = append(lines, line)
	}

	return &types.ActionResult{
		RequestID: action.RequestID, Success: true,
		Output:  strings.Join(lines, "\n"),
		Summary: fmt.Sprintf("%d events in next %d days", len(events), daysAhead),
	}
}

func (c *CalendarExecutor) createEvent(ctx context.Context, action *types.ActionRequest) *types.ActionResult {
	title, _ := action.Payload["title"].(string)
	startStr, _ := action.Payload["start"].(string)
	endStr, _ := action.Payload["end"].(string)

	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "invalid start time: " + err.Error()}
	}
	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "invalid end time: " + err.Error()}
	}

	event := &CalendarEvent{
		Title: title, Start: start, End: end,
	}
	if desc, ok := action.Payload["description"].(string); ok {
		event.Description = desc
	}
	if loc, ok := action.Payload["location"].(string); ok {
		event.Location = loc
	}

	created, err := c.provider.CreateEvent(ctx, event)
	if err != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: err.Error(), Summary: "event creation failed"}
	}

	return &types.ActionResult{
		RequestID: action.RequestID, Success: true,
		Output:  fmt.Sprintf("Created event: %s on %s", created.Title, created.Start.Format("Jan 02 15:04")),
		Summary: "event created",
	}
}

func (c *CalendarExecutor) updateEvent(ctx context.Context, action *types.ActionRequest) *types.ActionResult {
	eventID, _ := action.Payload["event_id"].(string)
	if eventID == "" {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "event_id is required"}
	}

	event := &CalendarEvent{}
	if title, ok := action.Payload["title"].(string); ok {
		event.Title = title
	}

	_, err := c.provider.UpdateEvent(ctx, eventID, event)
	if err != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: err.Error(), Summary: "event update failed"}
	}

	return &types.ActionResult{RequestID: action.RequestID, Success: true, Summary: "event updated"}
}

func (c *CalendarExecutor) deleteEvent(ctx context.Context, action *types.ActionRequest) *types.ActionResult {
	eventID, _ := action.Payload["event_id"].(string)
	if eventID == "" {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "event_id is required"}
	}

	if err := c.provider.DeleteEvent(ctx, eventID); err != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: err.Error(), Summary: "event delete failed"}
	}

	return &types.ActionResult{RequestID: action.RequestID, Success: true, Summary: "event deleted"}
}
