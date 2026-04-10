package channels

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSplitMessageShort(t *testing.T) {
	parts := SplitMessage("hello world", 100)
	assert.Len(t, parts, 1)
	assert.Equal(t, "hello world", parts[0])
}

func TestSplitMessageLong(t *testing.T) {
	text := ""
	for range 50 {
		text += "This is a line of text that is about 50 chars long.\n"
	}
	parts := SplitMessage(text, 200)
	assert.True(t, len(parts) > 1)
	for _, part := range parts {
		assert.LessOrEqual(t, len(part), 200)
	}
}

func TestSplitMessageAtNewline(t *testing.T) {
	text := "Line one.\nLine two.\nLine three.\nLine four.\nLine five."
	parts := SplitMessage(text, 25)
	assert.True(t, len(parts) > 1)
	// Should split at newline boundaries.
	for _, part := range parts {
		assert.LessOrEqual(t, len(part), 25)
	}
}

func TestMaxMessageLen(t *testing.T) {
	assert.Equal(t, 4096, MaxMessageLen("telegram"))
	assert.Equal(t, 4096, MaxMessageLen("whatsapp"))
	assert.Equal(t, 2000, MaxMessageLen("discord"))
	assert.Equal(t, 4096, MaxMessageLen("unknown"))
}
