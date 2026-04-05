package agent

import (
	"context"
	"os"
	"testing"

	"github.com/openparallax/openparallax/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompactUnderBudget(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set")
	}
	model := os.Getenv("OPENAI_MODEL")
	if model == "" {
		model = "gpt-5.4-mini"
	}
	provider, err := llm.NewOpenAIProvider(apiKey, model, os.Getenv("OPENAI_BASE_URL"))
	require.NoError(t, err)

	c := NewCompactor(provider, nil)
	messages := []llm.ChatMessage{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there!"},
	}

	result, err := c.Compact(context.Background(), messages, 10000)
	require.NoError(t, err)
	assert.Equal(t, messages, result, "under budget, messages should be unchanged")
}

func TestCompactOverBudget(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set")
	}
	model := os.Getenv("OPENAI_MODEL")
	if model == "" {
		model = "gpt-5.4-mini"
	}
	provider, err := llm.NewOpenAIProvider(apiKey, model, os.Getenv("OPENAI_BASE_URL"))
	require.NoError(t, err)

	c := NewCompactor(provider, nil)

	// Create messages that exceed the budget.
	var messages []llm.ChatMessage
	for i := 0; i < 20; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		messages = append(messages, llm.ChatMessage{
			Role:    role,
			Content: "This is message number " + string(rune('A'+i)) + " with some content to fill tokens.",
		})
	}

	// Very small budget to force compaction.
	result, err := c.Compact(context.Background(), messages, 100)
	require.NoError(t, err)
	assert.Less(t, len(result), len(messages), "compacted should have fewer messages")
}

func TestCompactPreservesRecent(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set")
	}
	model := os.Getenv("OPENAI_MODEL")
	if model == "" {
		model = "gpt-5.4-mini"
	}
	provider, err := llm.NewOpenAIProvider(apiKey, model, os.Getenv("OPENAI_BASE_URL"))
	require.NoError(t, err)

	c := NewCompactor(provider, nil)

	messages := []llm.ChatMessage{
		{Role: "user", Content: "Old message 1"},
		{Role: "assistant", Content: "Old response 1"},
		{Role: "user", Content: "Old message 2"},
		{Role: "assistant", Content: "Old response 2"},
		{Role: "user", Content: "Recent message"},
		{Role: "assistant", Content: "Recent response"},
	}

	result, err := c.Compact(context.Background(), messages, 50)
	require.NoError(t, err)

	// The most recent message should be preserved.
	last := result[len(result)-1]
	assert.Equal(t, "Recent response", last.Content)
}
