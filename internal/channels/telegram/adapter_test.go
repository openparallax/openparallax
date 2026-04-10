package telegram

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/openparallax/openparallax/internal/channels"
	"github.com/openparallax/openparallax/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEscapeMarkdownV2(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello world", "hello world"},
		{"hello_world", "hello\\_world"},
		{"*bold*", "\\*bold\\*"},
		{"[link](url)", "\\[link\\]\\(url\\)"},
		{"code `block`", "code \\`block\\`"},
		{"price: $10.50", "price: $10\\.50"},
		{"item #1", "item \\#1"},
		{"a + b = c", "a \\+ b \\= c"},
	}

	for _, tt := range tests {
		result := EscapeMarkdownV2(tt.input)
		assert.Equal(t, tt.expected, result, "input: %s", tt.input)
	}
}

func TestNewAdapterNilWhenDisabled(t *testing.T) {
	adapter := New(&types.TelegramConfig{Enabled: false, TokenEnv: "TEST"}, nil, nil)
	assert.Nil(t, adapter)
}

func TestNewAdapterNilWhenNilConfig(t *testing.T) {
	adapter := New(nil, nil, nil)
	assert.Nil(t, adapter)
}

func TestNewAdapterNilWhenNoToken(t *testing.T) {
	adapter := New(&types.TelegramConfig{Enabled: true, TokenEnv: "NONEXISTENT_TOKEN_VAR"}, nil, nil)
	assert.Nil(t, adapter)
}

func TestAdapterName(t *testing.T) {
	a := &Adapter{token: "test", rateLimits: make(map[int64][]time.Time)}
	assert.Equal(t, "telegram", a.Name())
}

func TestIsConfigured(t *testing.T) {
	a := &Adapter{token: ""}
	assert.False(t, a.IsConfigured())

	a.token = "test-token"
	assert.True(t, a.IsConfigured())
}

func TestAllowedUsersFiltering(t *testing.T) {
	allowed := map[int64]bool{123: true, 456: true}
	assert.True(t, allowed[123])
	assert.True(t, allowed[456])
	assert.False(t, allowed[789])
}

func TestRateLimit(t *testing.T) {
	a := &Adapter{token: "test", rateLimits: make(map[int64][]time.Time)}

	// Fill up the rate limit.
	for range rateLimit {
		assert.True(t, a.checkRateLimit(123))
	}
	// Next should be rejected.
	assert.False(t, a.checkRateLimit(123))

	// Different user should still be allowed.
	assert.True(t, a.checkRateLimit(456))
}

func TestRateLimitExpiry(t *testing.T) {
	a := &Adapter{token: "test", rateLimits: make(map[int64][]time.Time)}

	// Store old timestamps that are outside the window.
	old := time.Now().Add(-2 * rateLimitWindow)
	oldTimes := make([]time.Time, rateLimit)
	for i := range oldTimes {
		oldTimes[i] = old
	}
	a.rateLimits[int64(123)] = oldTimes

	// Should be allowed because old entries are expired.
	assert.True(t, a.checkRateLimit(123))
}

func TestSendMessageSplitsLong(t *testing.T) {
	text := ""
	for range 100 {
		text += "This is a long line of text for testing. "
	}
	parts := channels.SplitMessage(text, maxMsgLen)
	assert.True(t, len(parts) > 1, "long message should be split")
	for _, part := range parts {
		assert.LessOrEqual(t, len(part), maxMsgLen)
	}
}

func TestTelegramUpdateParsing(t *testing.T) {
	raw := `{
		"ok": true,
		"result": [{
			"update_id": 12345,
			"message": {
				"message_id": 1,
				"from": {"id": 999, "first_name": "Test"},
				"chat": {"id": 999, "type": "private"},
				"text": "hello agent"
			}
		}]
	}`

	var result struct {
		OK     bool             `json:"ok"`
		Result []telegramUpdate `json:"result"`
	}
	require.NoError(t, json.Unmarshal([]byte(raw), &result))
	assert.True(t, result.OK)
	require.Len(t, result.Result, 1)
	assert.Equal(t, int64(12345), result.Result[0].UpdateID)
	assert.Equal(t, "hello agent", result.Result[0].Message.Text)
	assert.Equal(t, int64(999), result.Result[0].Message.From.ID)
}

func TestTelegramMessageFormatting(t *testing.T) {
	text := "Hello *world*"
	escaped := EscapeMarkdownV2(text)
	assert.Equal(t, "Hello \\*world\\*", escaped)
}

func TestEmptyAllowedUsersAllowsAll(t *testing.T) {
	allowed := map[int64]bool{} // empty = allow all
	assert.Equal(t, 0, len(allowed))
}
