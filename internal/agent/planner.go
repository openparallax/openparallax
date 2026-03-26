package agent

import (
	"context"
	"fmt"

	"github.com/openparallax/openparallax/internal/llm"
	"github.com/openparallax/openparallax/internal/types"
)

// Planner uses the LLM to decide what actions to take for a given intent.
type Planner struct {
	llm llm.Provider
}

// NewPlanner creates a Planner.
func NewPlanner(provider llm.Provider) *Planner {
	return &Planner{llm: provider}
}

// Plan asks the LLM to produce an action plan for the given intent.
// Returns the raw LLM output which the ActionBuilder will parse.
func (p *Planner) Plan(ctx context.Context, intent *types.StructuredIntent, systemPrompt string, history []llm.ChatMessage) (string, error) {
	prompt := buildPlanningPrompt(intent)
	return p.llm.Complete(ctx, prompt, llm.WithSystem(systemPrompt), llm.WithMaxTokens(2048))
}

func buildPlanningPrompt(intent *types.StructuredIntent) string {
	return fmt.Sprintf(`Based on the user's request, plan the actions needed to fulfill it.

User request: %s
Detected goal: %s
Detected primary action: %s
Confidence: %.2f
Destructive: %v

For each action you want to take, output a block in this exact format:

ACTION: <action_type>
PARAMS: <JSON object with parameters>
REASONING: <why this action is needed>

Available action types: read_file, write_file, delete_file, move_file, copy_file, create_directory, list_directory, search_files, execute_command, send_message, send_email, http_request, browser_navigate, browser_click, browser_type, browser_extract, browser_screenshot, create_schedule, delete_schedule, list_schedules, read_calendar, create_event, update_event, delete_event, git_status, git_diff, git_commit, git_push, git_pull, git_log, git_branch, git_checkout, memory_write, memory_search, canvas_create, canvas_update

If the request is purely conversational and needs no actions, output:
ACTION: none
REASONING: This is a conversation, no tool use needed.

Output ONLY the action blocks, nothing else.`,
		intent.RawInput,
		intent.Goal,
		intent.PrimaryAction,
		intent.Confidence,
		intent.Destructive,
	)
}
