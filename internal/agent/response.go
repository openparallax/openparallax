package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/openparallax/openparallax/internal/llm"
	"github.com/openparallax/openparallax/internal/types"
)

// responseSystemPrompt is the single source of truth for how the LLM should
// behave when generating user-facing responses. It is appended to the workspace
// context (SOUL.md, IDENTITY.md, etc.) before every response generation call.
//
// This is a positive instruction ("here is your role") rather than a defensive
// list of prohibitions. The LLM is told what it IS, not what it shouldn't do.
const responseSystemPrompt = `## Your Role

You are a personal AI agent responding to the user. Your actions are executed by a separate system — you describe what happened based on the results you receive.

When action results are provided below the user's message, those are real outcomes from your execution pipeline. Report them accurately. If an action failed or was blocked, explain why honestly.

When no action results are provided, the user's request was handled as a conversation. Respond naturally.

Your response is plain text for the user to read.`

// Responder generates user-facing responses using the LLM.
type Responder struct {
	llm llm.Provider
}

// NewResponder creates a Responder with the given LLM provider.
func NewResponder(provider llm.Provider) *Responder {
	return &Responder{llm: provider}
}

// Generate creates a streaming response incorporating action results.
func (r *Responder) Generate(ctx context.Context, userMessage string, systemPrompt string, history []llm.ChatMessage, results []*types.ActionResult) (llm.StreamReader, error) {
	fullSystem := systemPrompt
	if fullSystem != "" {
		fullSystem += "\n\n---\n\n"
	}
	fullSystem += responseSystemPrompt

	messages := make([]llm.ChatMessage, 0, len(history)+1)
	messages = append(messages, history...)

	if len(results) > 0 {
		messages = append(messages, llm.ChatMessage{
			Role:    "user",
			Content: userMessage + "\n\n" + formatResults(results),
		})
	} else {
		messages = append(messages, llm.ChatMessage{Role: "user", Content: userMessage})
	}

	return r.llm.StreamWithHistory(ctx, messages, llm.WithSystem(fullSystem))
}

// formatResults renders action outcomes for the LLM to reference.
func formatResults(results []*types.ActionResult) string {
	var sb strings.Builder
	sb.WriteString("[Action results from your execution pipeline:]\n\n")
	for _, res := range results {
		if res.Success {
			fmt.Fprintf(&sb, "COMPLETED: %s\n%s\n\n", res.Summary, truncateOutput(res.Output, 500))
		} else {
			fmt.Fprintf(&sb, "BLOCKED: %s (%s)\n\n", res.Summary, res.Error)
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
