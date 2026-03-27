package agent

import (
	"context"

	"github.com/openparallax/openparallax/internal/llm"
	"github.com/openparallax/openparallax/internal/types"
)

// Agent coordinates the reasoning pipeline: context assembly, planning,
// building, self-evaluation, response generation, and history compaction.
type Agent struct {
	Context   *ContextAssembler
	Planner   *Planner
	Builder   *ActionBuilder
	SelfEval  *SelfEvaluator
	Responder *Responder
	Compactor *Compactor
}

// NewAgent creates an Agent with all reasoning components initialized.
func NewAgent(provider llm.Provider, workspacePath string, availableActions []types.ActionType) *Agent {
	return &Agent{
		Context:   NewContextAssembler(workspacePath),
		Planner:   NewPlanner(provider, availableActions),
		Builder:   NewActionBuilder(),
		SelfEval:  NewSelfEvaluator(provider),
		Responder: NewResponder(provider),
		Compactor: NewCompactor(provider),
	}
}

// PlanActions takes a StructuredIntent and produces ActionRequests.
// Returns nil with no error for pure conversation (no actions needed).
func (a *Agent) PlanActions(ctx context.Context, intent *types.StructuredIntent, systemPrompt string, history []llm.ChatMessage) ([]*types.ActionRequest, error) {
	rawPlan, err := a.Planner.Plan(ctx, intent, systemPrompt, history)
	if err != nil {
		return nil, err
	}
	return a.Builder.Build(rawPlan)
}

// CompactHistory compresses old messages when the context window is getting full.
func (a *Agent) CompactHistory(ctx context.Context, messages []llm.ChatMessage, maxTokens int) ([]llm.ChatMessage, error) {
	return a.Compactor.Compact(ctx, messages, maxTokens)
}
