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

	return fmt.Sprintf(`Plan the actions needed to fulfill this request.

User request: %s
Detected goal: %s
Detected primary action: %s

Available tools: %s

Parameter requirements:
- write_file: {"path": "...", "content": "..."}
- read_file: {"path": "..."}
- execute_command: {"command": "..."}
- Paths are relative to the workspace unless absolute.

Output format (one block per action):

ACTION: <action_type>
PARAMS: <JSON object>
REASONING: <why>

If no actions are needed:
ACTION: none
REASONING: <why>

Output ONLY action blocks.`,
		intent.RawInput,
		intent.Goal,
		intent.PrimaryAction,
		strings.Join(actionList, ", "),
	)
}
