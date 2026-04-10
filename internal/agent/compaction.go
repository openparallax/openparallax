package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/openparallax/openparallax/llm"
)

// defaultCompactionSplitPct is the percentage of the context budget reserved
// for history when compacting. The remaining portion is for the current turn.
const defaultCompactionSplitPct = 70

// defaultCompactionSummaryTokens is the max tokens for the LLM summary call.
const defaultCompactionSummaryTokens = 500

// defaultCompactionExtractTokens is the max tokens for the memory extraction call.
const defaultCompactionExtractTokens = 300

// MemoryFlusher is the callback the compactor uses to ship extracted facts
// to the engine for persistence. The agent process is sandboxed and cannot
// write to the workspace — the engine receives the content via
// EventMemoryFlush and calls memory.Append on its behalf.
type MemoryFlusher func(content string)

// Compactor compresses conversation history when approaching context limits.
// Uses a 70/30 split: 70% for history budget, 30% reserved for current turn.
// Before compaction, flushes important facts via the MemoryFlusher callback
// so they persist in MEMORY.md even though the messages are being evicted.
type Compactor struct {
	llm   llm.Provider
	flush MemoryFlusher
}

// NewCompactor creates a Compactor. Pass nil for flush to skip memory
// extraction (e.g. sub-agents that don't persist memory).
func NewCompactor(provider llm.Provider, flush MemoryFlusher) *Compactor {
	return &Compactor{llm: provider, flush: flush}
}

// Compact summarizes old messages to free context space.
// Before summarizing, flushes important facts to memory.
// The threshold parameter (0-100) controls what percentage of
// maxTokens is reserved for history; the rest is for the current
// turn. Pass 0 to use the default (70%).
func (c *Compactor) Compact(ctx context.Context, messages []llm.ChatMessage, maxTokens, threshold int) ([]llm.ChatMessage, error) {
	if threshold <= 0 || threshold > 100 {
		threshold = defaultCompactionSplitPct
	}
	historyBudget := int(float64(maxTokens) * float64(threshold) / 100)

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
		llm.WithMaxTokens(defaultCompactionSummaryTokens),
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
// to be compacted, then ships them via the flush callback for the engine to
// persist. The agent cannot write to the workspace directly (sandboxed).
func (c *Compactor) flushToMemory(ctx context.Context, messages []llm.ChatMessage) {
	if c.flush == nil || len(messages) == 0 {
		return
	}

	var conv strings.Builder
	for _, m := range messages {
		fmt.Fprintf(&conv, "%s: %s\n", m.Role, m.Content)
	}

	flushResult, err := c.llm.Complete(ctx, fmt.Sprintf(
		`Review this conversation and extract any important facts, decisions, preferences, or information that should be remembered long-term. Write them as concise bullet points. Only include facts worth remembering across sessions. If nothing is worth remembering, respond with "NONE".

%s`, conv.String()), llm.WithMaxTokens(defaultCompactionExtractTokens))
	if err != nil {
		return
	}

	trimmed := strings.TrimSpace(flushResult)
	if trimmed == "" || strings.EqualFold(trimmed, "NONE") {
		return
	}

	header := fmt.Sprintf("\n## Auto-captured — %s\n", time.Now().Format("2006-01-02"))
	c.flush(header + trimmed + "\n")
}
