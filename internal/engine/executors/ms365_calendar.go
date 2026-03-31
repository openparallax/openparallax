package executors

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/openparallax/openparallax/internal/oauth"
)

const graphBaseURL = "https://graph.microsoft.com/v1.0"

// ms365CalendarProvider implements CalendarProvider using Microsoft Graph API.
type ms365CalendarProvider struct {
	oauthMgr *oauth.Manager
	account  string
	client   *http.Client
}

// newMS365CalendarProvider creates a Microsoft 365 calendar provider.
func newMS365CalendarProvider(oauthMgr *oauth.Manager, account string) *ms365CalendarProvider {
	return &ms365CalendarProvider{
		oauthMgr: oauthMgr,
		account:  account,
		client:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *ms365CalendarProvider) ListEvents(ctx context.Context, from, to time.Time) ([]CalendarEvent, error) {
	fromStr := from.UTC().Format("2006-01-02T15:04:05")
	toStr := to.UTC().Format("2006-01-02T15:04:05")

	filter := fmt.Sprintf("start/dateTime ge '%s' and end/dateTime le '%s'", fromStr, toStr)
	params := url.Values{
		"$filter":  {filter},
		"$orderby": {"start/dateTime"},
		"$top":     {"50"},
		"$select":  {"id,subject,start,end,location,attendees,isAllDay,body"},
	}

	reqURL := fmt.Sprintf("%s/me/calendar/events?%s", graphBaseURL, params.Encode())
	body, err := p.doRequest(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Value []ms365Event `json:"value"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse events: %w", err)
	}

	events := make([]CalendarEvent, 0, len(resp.Value))
	for _, e := range resp.Value {
		events = append(events, ms365ToCalendarEvent(e))
	}
	return events, nil
}

func (p *ms365CalendarProvider) CreateEvent(ctx context.Context, event *CalendarEvent) (*CalendarEvent, error) {
	ms := calendarToMS365Event(event)
	payload, err := json.Marshal(ms)
	if err != nil {
		return nil, fmt.Errorf("marshal event: %w", err)
	}

	reqURL := fmt.Sprintf("%s/me/calendar/events", graphBaseURL)
	body, err := p.doRequest(ctx, http.MethodPost, reqURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	var created ms365Event
	if err := json.Unmarshal(body, &created); err != nil {
		return nil, fmt.Errorf("parse created event: %w", err)
	}

	result := ms365ToCalendarEvent(created)
	return &result, nil
}

func (p *ms365CalendarProvider) UpdateEvent(ctx context.Context, id string, event *CalendarEvent) (*CalendarEvent, error) {
	ms := calendarToMS365Event(event)
	payload, err := json.Marshal(ms)
	if err != nil {
		return nil, fmt.Errorf("marshal event: %w", err)
	}

	reqURL := fmt.Sprintf("%s/me/calendar/events/%s", graphBaseURL, url.PathEscape(id))
	body, err := p.doRequest(ctx, http.MethodPatch, reqURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	var updated ms365Event
	if err := json.Unmarshal(body, &updated); err != nil {
		return nil, fmt.Errorf("parse updated event: %w", err)
	}

	result := ms365ToCalendarEvent(updated)
	return &result, nil
}

func (p *ms365CalendarProvider) DeleteEvent(ctx context.Context, id string) error {
	reqURL := fmt.Sprintf("%s/me/calendar/events/%s", graphBaseURL, url.PathEscape(id))
	_, err := p.doRequest(ctx, http.MethodDelete, reqURL, nil)
	return err
}

// doRequest executes an authenticated HTTP request against Microsoft Graph.
// Handles 401 retry (token refresh) and 429 rate limiting.
func (p *ms365CalendarProvider) doRequest(ctx context.Context, method, reqURL string, body io.Reader) ([]byte, error) {
	for attempt := range 2 {
		token, err := p.oauthMgr.GetValidToken(ctx, "microsoft", p.account)
		if err != nil {
			return nil, fmt.Errorf("get token: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, method, reqURL, body)
		if err != nil {
			return nil, fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")

		resp, err := p.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("request failed: %w", err)
		}

		respBody, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		switch {
		case resp.StatusCode >= 200 && resp.StatusCode < 300:
			return respBody, nil

		case resp.StatusCode == http.StatusUnauthorized && attempt == 0:
			// Token might be stale — force refresh and retry.
			continue

		case resp.StatusCode == http.StatusTooManyRequests:
			retryAfter := resp.Header.Get("Retry-After")
			waitSec, _ := strconv.Atoi(retryAfter)
			if waitSec <= 0 {
				waitSec = 5
			}
			if waitSec > 30 {
				waitSec = 30
			}
			time.Sleep(time.Duration(waitSec) * time.Second)
			continue

		case resp.StatusCode == http.StatusNotFound:
			return nil, fmt.Errorf("event not found")

		case resp.StatusCode == http.StatusForbidden:
			return nil, fmt.Errorf("calendar access denied: re-authorize with calendar permissions")

		default:
			snippet := string(respBody)
			if len(snippet) > 200 {
				snippet = snippet[:200]
			}
			return nil, fmt.Errorf("graph API error %d: %s", resp.StatusCode, snippet)
		}
	}
	return nil, fmt.Errorf("request failed after retries")
}

// --- MS365 event types ---

type ms365Event struct {
	ID       string        `json:"id,omitempty"`
	Subject  string        `json:"subject"`
	Body     ms365Body     `json:"body,omitempty"`
	Start    ms365DateTime `json:"start"`
	End      ms365DateTime `json:"end"`
	Location struct {
		DisplayName string `json:"displayName,omitempty"`
	} `json:"location,omitempty"`
	Attendees []ms365Attendee `json:"attendees,omitempty"`
	IsAllDay  bool            `json:"isAllDay,omitempty"`
}

type ms365DateTime struct {
	DateTime string `json:"dateTime"`
	TimeZone string `json:"timeZone"`
}

type ms365Body struct {
	ContentType string `json:"contentType"`
	Content     string `json:"content"`
}

type ms365Attendee struct {
	EmailAddress struct {
		Address string `json:"address"`
		Name    string `json:"name,omitempty"`
	} `json:"emailAddress"`
	Type string `json:"type"` // "required", "optional"
}

// --- Conversion functions ---

func ms365ToCalendarEvent(e ms365Event) CalendarEvent {
	start := parseMS365DateTime(e.Start)
	end := parseMS365DateTime(e.End)

	var attendees []string
	for _, a := range e.Attendees {
		attendees = append(attendees, a.EmailAddress.Address)
	}

	desc := e.Body.Content
	if e.Body.ContentType == "html" {
		desc = StripHTML(desc)
	}

	return CalendarEvent{
		ID:          e.ID,
		Title:       e.Subject,
		Start:       start,
		End:         end,
		Description: desc,
		Location:    e.Location.DisplayName,
		Attendees:   attendees,
	}
}

func calendarToMS365Event(e *CalendarEvent) ms365Event {
	var attendees []ms365Attendee
	for _, email := range e.Attendees {
		attendees = append(attendees, ms365Attendee{
			EmailAddress: struct {
				Address string `json:"address"`
				Name    string `json:"name,omitempty"`
			}{Address: email},
			Type: "required",
		})
	}

	return ms365Event{
		Subject: e.Title,
		Body: ms365Body{
			ContentType: "text",
			Content:     e.Description,
		},
		Start: ms365DateTime{
			DateTime: e.Start.UTC().Format("2006-01-02T15:04:05.0000000"),
			TimeZone: "UTC",
		},
		End: ms365DateTime{
			DateTime: e.End.UTC().Format("2006-01-02T15:04:05.0000000"),
			TimeZone: "UTC",
		},
		Location: struct {
			DisplayName string `json:"displayName,omitempty"`
		}{DisplayName: e.Location},
		Attendees: attendees,
	}
}

// parseMS365DateTime parses Microsoft's datetime format with separate timezone.
func parseMS365DateTime(dt ms365DateTime) time.Time {
	// Try to load the timezone.
	loc := time.UTC
	if dt.TimeZone != "" && !strings.EqualFold(dt.TimeZone, "UTC") {
		if loaded, err := time.LoadLocation(dt.TimeZone); err == nil {
			loc = loaded
		} else {
			// Try Windows timezone names mapped to IANA.
			loc = mapWindowsTimezone(dt.TimeZone)
		}
	}

	// Parse the datetime string. Microsoft uses variable precision.
	for _, layout := range []string{
		"2006-01-02T15:04:05.0000000",
		"2006-01-02T15:04:05",
		"2006-01-02T15:04:05Z",
		time.RFC3339,
	} {
		if t, err := time.ParseInLocation(layout, dt.DateTime, loc); err == nil {
			return t.UTC()
		}
	}

	// Fallback: try RFC3339.
	t, _ := time.Parse(time.RFC3339, dt.DateTime)
	return t
}

// mapWindowsTimezone maps common Windows timezone names to IANA locations.
func mapWindowsTimezone(name string) *time.Location {
	mapping := map[string]string{
		"Pacific Standard Time":   "America/Los_Angeles",
		"Mountain Standard Time":  "America/Denver",
		"Central Standard Time":   "America/Chicago",
		"Eastern Standard Time":   "America/New_York",
		"GMT Standard Time":       "Europe/London",
		"W. Europe Standard Time": "Europe/Berlin",
		"Tokyo Standard Time":     "Asia/Tokyo",
	}
	if iana, ok := mapping[name]; ok {
		if loc, err := time.LoadLocation(iana); err == nil {
			return loc
		}
	}
	return time.UTC
}
