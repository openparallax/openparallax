package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/openparallax/openparallax/internal/memory"
	"github.com/openparallax/openparallax/llm"
)

// Compactor compresses conversation history when approaching context limits.
// Uses a 70/30 split: 70% for history budget, 30% reserved for current turn.
// Before compaction, flushes important facts to MEMORY.md so they survive.
type Compactor struct {
	llm    llm.Provider
	memory *memory.Manager
}

// NewCompactor creates a Compactor. Pass nil for memory to skip flushing.
func NewCompactor(provider llm.Provider, mem *memory.Manager) *Compactor {
	return &Compactor{llm: provider, memory: mem}
}

// Compact summarizes old messages to free context space.
// Before summarizing, flushes important facts to memory.
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

	// Memory flush: extract durable facts before discarding old messages.
	c.flushToMemory(ctx, oldMessages)

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

// flushToMemory asks the LLM to extract important facts from messages about
// to be compacted, then writes them to MEMORY.md so they persist.
func (c *Compactor) flushToMemory(ctx context.Context, messages []llm.ChatMessage) {
	if c.memory == nil || len(messages) == 0 {
		return
	}

	var conv strings.Builder
	for _, m := range messages {
		fmt.Fprintf(&conv, "%s: %s\n", m.Role, m.Content)
	}

	flushResult, err := c.llm.Complete(ctx, fmt.Sprintf(
		`Review this conversation and extract any important facts, decisions, preferences, or information that should be remembered long-term. Write them as concise bullet points. Only include facts worth remembering across sessions. If nothing is worth remembering, respond with "NONE".

%s`, conv.String()), llm.WithMaxTokens(300))
	if err != nil {
		return
	}

	trimmed := strings.TrimSpace(flushResult)
	if trimmed == "" || strings.EqualFold(trimmed, "NONE") {
		return
	}

	header := fmt.Sprintf("\n## Auto-captured — %s\n", time.Now().Format("2006-01-02"))
	_ = c.memory.Append("MEMORY.md", header+trimmed+"\n")
}
