package agent

import (
	"context"
	"fmt"

	"github.com/openparallax/openparallax/internal/llm"
)

// Compactor compresses conversation history when approaching context limits.
// Uses a 70/30 split: 70% for history budget, 30% reserved for current turn.
type Compactor struct {
	llm llm.Provider
}

// NewCompactor creates a Compactor.
func NewCompactor(provider llm.Provider) *Compactor {
	return &Compactor{llm: provider}
}

// Compact summarizes old messages to free context space.
// Keeps the most recent messages intact and summarizes older ones.
func (c *Compactor) Compact(ctx context.Context, messages []llm.ChatMessage, maxTokens int) ([]llm.ChatMessage, error) {
	historyBudget := int(float64(maxTokens) * 0.7)

	tokenCount := 0
	splitIdx := 0
	for i := len(messages) - 1; i >= 0; i-- {
		tokens := c.llm.EstimateTokens(messages[i].Content)
		if tokenCount+tokens > historyBudget {
			splitIdx = i + 1
			break
		}
		tokenCount += tokens
	}

	if splitIdx == 0 {
		return messages, nil
	}

	oldMessages := messages[:splitIdx]
	recentMessages := messages[splitIdx:]

	conv := ""
	for _, m := range oldMessages {
		conv += fmt.Sprintf("%s: %s\n", m.Role, m.Content)
	}

	summary, err := c.llm.Complete(ctx, fmt.Sprintf(
		"Summarize this conversation history concisely. Preserve key facts, decisions, and context:\n\n%s", conv),
		llm.WithMaxTokens(500),
	)
	if err != nil {
		return recentMessages, nil
	}

	result := []llm.ChatMessage{
		{Role: "system", Content: fmt.Sprintf("[Previous conversation summary: %s]", summary)},
	}
	result = append(result, recentMessages...)
	return result, nil
}
