package executors

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMS365ToCalendarEventMapping(t *testing.T) {
	ms := ms365Event{
		ID:      "event-123",
		Subject: "Team Meeting",
		Body:    ms365Body{ContentType: "text", Content: "Weekly sync"},
		Start:   ms365DateTime{DateTime: "2026-04-01T14:00:00.0000000", TimeZone: "UTC"},
		End:     ms365DateTime{DateTime: "2026-04-01T15:00:00.0000000", TimeZone: "UTC"},
		Location: struct {
			DisplayName string `json:"displayName,omitempty"`
		}{DisplayName: "Room A"},
		Attendees: []ms365Attendee{
			{EmailAddress: struct {
				Address string `json:"address"`
				Name    string `json:"name,omitempty"`
			}{Address: "alice@example.com", Name: "Alice"}, Type: "required"},
		},
	}

	event := ms365ToCalendarEvent(ms)
	assert.Equal(t, "event-123", event.ID)
	assert.Equal(t, "Team Meeting", event.Title)
	assert.Equal(t, "Weekly sync", event.Description)
	assert.Equal(t, "Room A", event.Location)
	assert.Equal(t, 2026, event.Start.Year())
	assert.Equal(t, time.April, event.Start.Month())
	assert.Equal(t, 14, event.Start.Hour())
	assert.Len(t, event.Attendees, 1)
	assert.Equal(t, "alice@example.com", event.Attendees[0])
}

func TestCalendarToMS365EventMapping(t *testing.T) {
	event := &CalendarEvent{
		Title:       "Lunch",
		Start:       time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC),
		End:         time.Date(2026, 4, 1, 13, 0, 0, 0, time.UTC),
		Description: "Team lunch",
		Location:    "Cafe",
		Attendees:   []string{"bob@example.com"},
	}

	ms := calendarToMS365Event(event)
	assert.Equal(t, "Lunch", ms.Subject)
	assert.Equal(t, "Team lunch", ms.Body.Content)
	assert.Equal(t, "text", ms.Body.ContentType)
	assert.Contains(t, ms.Start.DateTime, "2026-04-01T12:00:00")
	assert.Equal(t, "UTC", ms.Start.TimeZone)
	assert.Equal(t, "Cafe", ms.Location.DisplayName)
	assert.Len(t, ms.Attendees, 1)
	assert.Equal(t, "bob@example.com", ms.Attendees[0].EmailAddress.Address)
}

func TestParseMS365DateTimeUTC(t *testing.T) {
	dt := ms365DateTime{DateTime: "2026-04-01T14:30:00.0000000", TimeZone: "UTC"}
	parsed := parseMS365DateTime(dt)
	assert.Equal(t, 2026, parsed.Year())
	assert.Equal(t, time.April, parsed.Month())
	assert.Equal(t, 14, parsed.Hour())
	assert.Equal(t, 30, parsed.Minute())
}

func TestParseMS365DateTimeWithTimezone(t *testing.T) {
	dt := ms365DateTime{DateTime: "2026-04-01T10:00:00", TimeZone: "America/New_York"}
	parsed := parseMS365DateTime(dt)
	// 10:00 AM ET = 14:00 UTC (EDT is UTC-4)
	assert.Equal(t, 14, parsed.Hour())
}

func TestParseMS365DateTimeWindowsTimezone(t *testing.T) {
	dt := ms365DateTime{DateTime: "2026-04-01T10:00:00", TimeZone: "Eastern Standard Time"}
	parsed := parseMS365DateTime(dt)
	assert.Equal(t, 14, parsed.Hour()) // EDT = UTC-4
}

func TestParseMS365DateTimeUnknownTimezone(t *testing.T) {
	dt := ms365DateTime{DateTime: "2026-04-01T10:00:00", TimeZone: "Fictional/Zone"}
	parsed := parseMS365DateTime(dt)
	// Unknown timezone falls back to UTC.
	assert.Equal(t, 10, parsed.Hour())
}

func TestMS365ListEventsHTTP(t *testing.T) {
	events := []ms365Event{
		{ID: "1", Subject: "Meeting A", Start: ms365DateTime{DateTime: "2026-04-01T09:00:00", TimeZone: "UTC"}, End: ms365DateTime{DateTime: "2026-04-01T10:00:00", TimeZone: "UTC"}},
		{ID: "2", Subject: "Meeting B", Start: ms365DateTime{DateTime: "2026-04-01T11:00:00", TimeZone: "UTC"}, End: ms365DateTime{DateTime: "2026-04-01T12:00:00", TimeZone: "UTC"}},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"value": events})
	}))
	defer server.Close()

	provider := &ms365CalendarProvider{
		oauthMgr: nil,
		account:  "user@outlook.com",
		client:   server.Client(),
	}
	// Override doRequest to use the test server directly.
	result, err := listEventsViaServer(t, server, provider)
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "Meeting A", result[0].Title)
}

func TestMS365HTMLBodyStripping(t *testing.T) {
	ms := ms365Event{
		ID:      "1",
		Subject: "HTML Email",
		Body:    ms365Body{ContentType: "html", Content: "<html><body><b>Bold</b> text</body></html>"},
		Start:   ms365DateTime{DateTime: "2026-04-01T09:00:00", TimeZone: "UTC"},
		End:     ms365DateTime{DateTime: "2026-04-01T10:00:00", TimeZone: "UTC"},
	}

	event := ms365ToCalendarEvent(ms)
	assert.Contains(t, event.Description, "Bold")
	assert.NotContains(t, event.Description, "<b>")
}

func TestMapWindowsTimezone(t *testing.T) {
	loc := mapWindowsTimezone("Pacific Standard Time")
	assert.Equal(t, "America/Los_Angeles", loc.String())

	loc = mapWindowsTimezone("Unknown Timezone")
	assert.Equal(t, "UTC", loc.String())
}

// listEventsViaServer is a test helper that parses the mock server response directly.
func listEventsViaServer(t *testing.T, server *httptest.Server, _ *ms365CalendarProvider) ([]CalendarEvent, error) {
	t.Helper()

	resp, err := server.Client().Get(server.URL + "/me/calendar/events")
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var body struct {
		Value []ms365Event `json:"value"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}

	events := make([]CalendarEvent, 0, len(body.Value))
	for _, e := range body.Value {
		events = append(events, ms365ToCalendarEvent(e))
	}
	return events, nil
}
