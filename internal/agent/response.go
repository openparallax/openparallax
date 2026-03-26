package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/openparallax/openparallax/internal/llm"
	"github.com/openparallax/openparallax/internal/types"
)

// Responder generates user-facing responses using the LLM.
type Responder struct {
	llm llm.Provider
}

// NewResponder creates a Responder with the given LLM provider.
func NewResponder(provider llm.Provider) *Responder {
	return &Responder{llm: provider}
}

// Generate creates a streaming response for the user's message in the context
// of conversation history and the assembled system prompt. If action results
// are provided, they are included so the LLM can reference what was done.
func (r *Responder) Generate(ctx context.Context, userMessage string, systemPrompt string, history []llm.ChatMessage, results []*types.ActionResult) (llm.StreamReader, error) {
	messages := make([]llm.ChatMessage, 0, len(history)+1)
	messages = append(messages, history...)

	if len(results) > 0 {
		resultSummary := buildResultSummary(results)
		messages = append(messages, llm.ChatMessage{
			Role:    "user",
			Content: fmt.Sprintf("%s\n\n[Action results:\n%s]", userMessage, resultSummary),
		})
	} else {
		messages = append(messages, llm.ChatMessage{Role: "user", Content: userMessage})
	}

	return r.llm.StreamWithHistory(ctx, messages, llm.WithSystem(systemPrompt))
}

// buildResultSummary formats action results for inclusion in the LLM prompt.
func buildResultSummary(results []*types.ActionResult) string {
	var sb strings.Builder
	for _, res := range results {
		if res.Success {
			fmt.Fprintf(&sb, "- %s: %s\n", res.Summary, truncateOutput(res.Output, 500))
		} else {
			fmt.Fprintf(&sb, "- FAILED: %s — %s\n", res.Summary, res.Error)
		}
	}
	return sb.String()
}

func truncateOutput(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
