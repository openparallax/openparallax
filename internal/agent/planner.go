package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/openparallax/openparallax/internal/llm"
	"github.com/openparallax/openparallax/internal/types"
)

// Planner uses the LLM to decide what actions to take for a given intent.
type Planner struct {
	llm              llm.Provider
	availableActions []types.ActionType
}

// NewPlanner creates a Planner. Pass the list of action types that have
// registered executors so the planning prompt only offers executable actions.
func NewPlanner(provider llm.Provider, availableActions []types.ActionType) *Planner {
	return &Planner{llm: provider, availableActions: availableActions}
}

// Plan asks the LLM to produce an action plan for the given intent.
// Returns the raw LLM output which the ActionBuilder will parse.
func (p *Planner) Plan(ctx context.Context, intent *types.StructuredIntent, systemPrompt string, history []llm.ChatMessage) (string, error) {
	prompt := buildPlanningPrompt(intent, p.availableActions)
	return p.llm.Complete(ctx, prompt, llm.WithSystem(systemPrompt), llm.WithMaxTokens(2048))
}

func buildPlanningPrompt(intent *types.StructuredIntent, available []types.ActionType) string {
	actionList := make([]string, len(available))
	for i, a := range available {
		actionList[i] = string(a)
	}

	return fmt.Sprintf(`You are an AI agent with direct tool access. When the user asks you to do something, you EXECUTE it using your tools — do not suggest commands for the user to run manually.

User request: %s
Detected goal: %s
Detected primary action: %s

You have these tools available for direct execution:
%s

IMPORTANT:
- For file operations (read, write, delete, move, copy, list, search), use the file tools directly. Do NOT use execute_command with shell redirects for file operations.
- write_file PARAMS must include "path" and "content" fields.
- read_file PARAMS must include "path".
- execute_command PARAMS must include "command".
- Use paths relative to the workspace unless the user specifies an absolute path.

For each action, output a block in this exact format:

ACTION: <action_type>
PARAMS: <JSON object with parameters>
REASONING: <why this action is needed>

If the request is purely conversational and needs no actions, output:
ACTION: none
REASONING: This is a conversation, no tool use needed.

Output ONLY the action blocks, nothing else.`,
		intent.RawInput,
		intent.Goal,
		intent.PrimaryAction,
		strings.Join(actionList, ", "),
	)
}
