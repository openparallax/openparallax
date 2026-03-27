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
const responseSystemPrompt = `## Your Role

You are a personal AI agent responding to the user. Your actions are executed by a separate system — you describe what happened based on the pipeline summary provided.

A pipeline summary is included with every request. It tells you:
- How the request was classified and why
- How many actions were planned
- What happened to each action (executed, failed, or blocked)

Report these outcomes accurately. If an action was blocked, explain why. If no actions were executed, the request was handled as conversation — respond naturally without describing actions that did not happen.`

// Responder generates user-facing responses using the LLM.
type Responder struct {
	llm llm.Provider
}

// NewResponder creates a Responder with the given LLM provider.
func NewResponder(provider llm.Provider) *Responder {
	return &Responder{llm: provider}
}

// Generate creates a streaming response based on the full pipeline summary.
func (r *Responder) Generate(ctx context.Context, systemPrompt string, history []llm.ChatMessage, summary *types.PipelineSummary) (llm.StreamReader, error) {
	fullSystem := systemPrompt
	if fullSystem != "" {
		fullSystem += "\n\n---\n\n"
	}
	fullSystem += responseSystemPrompt

	messages := make([]llm.ChatMessage, 0, len(history)+1)
	messages = append(messages, history...)
	messages = append(messages, llm.ChatMessage{
		Role:    "user",
		Content: summary.UserMessage + "\n\n" + FormatPipelineSummary(summary),
	})

	return r.llm.StreamWithHistory(ctx, messages, llm.WithSystem(fullSystem))
}

// FormatPipelineSummary renders the pipeline summary as structured text for the LLM.
func FormatPipelineSummary(s *types.PipelineSummary) string {
	var sb strings.Builder

	sb.WriteString("[Pipeline summary for this request:]\n")
	fmt.Fprintf(&sb, "Classification: %s\n", s.Classification)
	if s.ClassificationReason != "" {
		fmt.Fprintf(&sb, "Reason: %s\n", s.ClassificationReason)
	}
	fmt.Fprintf(&sb, "Actions planned: %d\n", s.ActionsPlanned)

	if s.ActionsPlanned > 0 && !s.SelfEvalPassed {
		fmt.Fprintf(&sb, "Self-evaluation: FAILED (%s)\n", s.SelfEvalReason)
		sb.WriteString("No actions were executed because the safety check failed.\n")
		return sb.String()
	}

	if len(s.Outcomes) == 0 {
		if s.ActionsPlanned == 0 {
			sb.WriteString("No actions were taken. Respond conversationally.\n")
		}
		return sb.String()
	}

	sb.WriteString("\nAction outcomes:\n")
	for i, o := range s.Outcomes {
		switch o.Status {
		case types.StatusExecuted:
			fmt.Fprintf(&sb, "%d. %s → COMPLETED: %s\n", i+1, o.Action, o.Summary)
			if o.Output != "" {
				fmt.Fprintf(&sb, "   Output: %s\n", truncateOutput(o.Output, 500))
			}
		case types.StatusFailed:
			fmt.Fprintf(&sb, "%d. %s → FAILED: %s\n", i+1, o.Action, o.Reason)
		case types.StatusBlockedOTR:
			fmt.Fprintf(&sb, "%d. %s → BLOCKED by OTR mode: %s\n", i+1, o.Action, o.Reason)
		case types.StatusBlockedShield:
			fmt.Fprintf(&sb, "%d. %s → BLOCKED by security policy: %s\n", i+1, o.Action, o.Reason)
		case types.StatusBlockedEscalate:
			fmt.Fprintf(&sb, "%d. %s → BLOCKED (requires approval): %s\n", i+1, o.Action, o.Reason)
		case types.StatusBlockedHash:
			fmt.Fprintf(&sb, "%d. %s → BLOCKED (integrity check failed)\n", i+1, o.Action)
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
