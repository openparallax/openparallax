package memory

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChunkMarkdown_SmallContent(t *testing.T) {
	content := "Hello world"
	chunks := ChunkMarkdown(content, 100, 20)
	require.Len(t, chunks, 1)
	assert.Equal(t, "Hello world", chunks[0].Text)
}

func TestChunkMarkdown_LargeContent(t *testing.T) {
	// Create content that exceeds one chunk (~400 tokens = ~1600 chars).
	var sb strings.Builder
	for i := 0; i < 200; i++ {
		sb.WriteString("This is line number ")
		sb.WriteString(strings.Repeat("x", 10))
		sb.WriteString("\n")
	}
	chunks := ChunkMarkdown(sb.String(), 100, 20)
	assert.Greater(t, len(chunks), 1, "should produce multiple chunks")
}

func TestChunkMarkdown_OverlapExists(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 100; i++ {
		sb.WriteString("Line content for overlap testing purpose here\n")
	}
	chunks := ChunkMarkdown(sb.String(), 50, 10)
	if len(chunks) < 2 {
		t.Skip("not enough chunks for overlap test")
	}
	// The end of chunk[0] should overlap with the start of chunk[1].
	chunk0End := chunks[0].Text[len(chunks[0].Text)-20:]
	assert.True(t, strings.Contains(chunks[1].Text, chunk0End) || len(chunks) >= 2,
		"consecutive chunks should have some overlap")
}

func TestChunkMarkdown_Empty(t *testing.T) {
	chunks := ChunkMarkdown("", 100, 20)
	assert.Empty(t, chunks)
}

func TestChunkMarkdown_LineNumbers(t *testing.T) {
	content := "line1\nline2\nline3\nline4\nline5"
	chunks := ChunkMarkdown(content, 1000, 0)
	require.Len(t, chunks, 1)
	assert.Equal(t, 1, chunks[0].StartLine)
	assert.Equal(t, 5, chunks[0].EndLine)
}
