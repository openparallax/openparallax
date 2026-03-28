package heartbeat

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCronEntries(t *testing.T) {
	content := `# Heartbeat

## Scheduled Tasks

- ` + "`0 9 * * *`" + ` — Morning briefing
- ` + "`*/30 * * * *`" + ` — Check email
- ` + "`0 18 * * 1-5`" + ` — Daily summary
`
	entries := ParseCronEntries(content)
	require.Len(t, entries, 3)
	assert.Equal(t, "Morning briefing", entries[0].Task)
	assert.Equal(t, "0", entries[0].Minute)
	assert.Equal(t, "9", entries[0].Hour)
	assert.Equal(t, "Check email", entries[1].Task)
	assert.Equal(t, "*/30", entries[1].Minute)
}

func TestParseCronEntries_InvalidSkipped(t *testing.T) {
	content := `- ` + "`invalid`" + ` — Bad entry
- ` + "`0 9 * * *`" + ` — Good entry
- Not a cron line`
	entries := ParseCronEntries(content)
	require.Len(t, entries, 1)
	assert.Equal(t, "Good entry", entries[0].Task)
}

func TestCronEntry_Matches(t *testing.T) {
	entry := &CronEntry{Minute: "0", Hour: "9", Day: "*", Month: "*", Weekday: "*"}
	at := time.Date(2026, 3, 28, 9, 0, 0, 0, time.UTC)
	assert.True(t, entry.Matches(at))

	notAt := time.Date(2026, 3, 28, 10, 0, 0, 0, time.UTC)
	assert.False(t, entry.Matches(notAt))
}

func TestCronEntry_MatchesStep(t *testing.T) {
	entry := &CronEntry{Minute: "*/15", Hour: "*", Day: "*", Month: "*", Weekday: "*"}
	assert.True(t, entry.Matches(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)))
	assert.True(t, entry.Matches(time.Date(2026, 1, 1, 0, 15, 0, 0, time.UTC)))
	assert.True(t, entry.Matches(time.Date(2026, 1, 1, 0, 30, 0, 0, time.UTC)))
	assert.False(t, entry.Matches(time.Date(2026, 1, 1, 0, 7, 0, 0, time.UTC)))
}

func TestCronEntry_Dedup(t *testing.T) {
	entry := &CronEntry{Minute: "*", Hour: "*", Day: "*", Month: "*", Weekday: "*"}
	now := time.Date(2026, 3, 28, 9, 0, 30, 0, time.UTC)
	assert.False(t, entry.FiredThisMinute(now))

	entry.MarkFired(now)
	assert.True(t, entry.FiredThisMinute(now))

	// Same minute, different second — still deduped.
	sameSec := time.Date(2026, 3, 28, 9, 0, 45, 0, time.UTC)
	assert.True(t, entry.FiredThisMinute(sameSec))

	// Different minute — not deduped.
	nextMin := time.Date(2026, 3, 28, 9, 1, 0, 0, time.UTC)
	assert.False(t, entry.FiredThisMinute(nextMin))
}

func TestMatchField(t *testing.T) {
	assert.True(t, matchField("*", 5))
	assert.True(t, matchField("5", 5))
	assert.False(t, matchField("5", 6))
	assert.True(t, matchField("*/5", 10))
	assert.False(t, matchField("*/5", 7))
	assert.True(t, matchField("1,3,5", 3))
	assert.False(t, matchField("1,3,5", 4))
}
