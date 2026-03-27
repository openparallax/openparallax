package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/openparallax/openparallax/internal/llm"
	"github.com/openparallax/openparallax/internal/types"
)

// responsePreamble is prepended to the system prompt for response generation.
// It instructs the LLM on how to behave when generating user-facing responses.
const responsePreamble = `## Response Generation Rules

You are generating a response to the user based on REAL action results provided below.

CRITICAL RULES:
- NEVER generate fake tool calls, XML tags like <tool_call>, or pretend to execute actions in your response text.
- NEVER claim an action succeeded if the results show it FAILED or was BLOCKED.
- If an action was BLOCKED by security (Shield or OTR), tell the user it was blocked and why. Do not attempt to work around the block.
- If no actions were executed, respond conversationally. Do not fabricate action results.
- Your response is plain text for the user. It is NOT parsed by any system. Do not output structured formats like JSON, XML, or action blocks.
- Base your response ONLY on the actual action results provided. If no results are provided, you did not execute any actions.
`

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
	fullSystemPrompt := systemPrompt + "\n\n---\n\n" + responsePreamble

	messages := make([]llm.ChatMessage, 0, len(history)+1)
	messages = append(messages, history...)

	if len(results) > 0 {
		resultSummary := buildResultSummary(results)
		messages = append(messages, llm.ChatMessage{
			Role:    "user",
			Content: fmt.Sprintf("%s\n\n[SYSTEM: The following actions were attempted on your behalf. Report these results accurately to the user. Do NOT fabricate additional actions.]\n\n%s", userMessage, resultSummary),
		})
	} else {
		messages = append(messages, llm.ChatMessage{Role: "user", Content: userMessage})
	}

	return r.llm.StreamWithHistory(ctx, messages, llm.WithSystem(fullSystemPrompt))
}

// buildResultSummary formats action results for inclusion in the LLM prompt.
func buildResultSummary(results []*types.ActionResult) string {
	var sb strings.Builder
	for _, res := range results {
		if res.Success {
			fmt.Fprintf(&sb, "ACTION SUCCEEDED: %s\nOutput: %s\n\n", res.Summary, truncateOutput(res.Output, 500))
		} else {
			fmt.Fprintf(&sb, "ACTION BLOCKED/FAILED: %s\nReason: %s\nDo NOT tell the user this action succeeded. It did not execute.\n\n", res.Summary, res.Error)
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
