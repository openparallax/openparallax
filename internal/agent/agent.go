package agent

import (
	"context"

	"github.com/openparallax/openparallax/internal/llm"
	"github.com/openparallax/openparallax/internal/types"
)

// Agent coordinates context assembly and history compaction.
// In the tool-use architecture, the LLM decides what tools to call —
// there is no separate planner, builder, or self-evaluator.
type Agent struct {
	Context   *ContextAssembler
	Compactor *Compactor
}

// NewAgent creates an Agent.
func NewAgent(provider llm.Provider, workspacePath string, _ []types.ActionType) *Agent {
	return &Agent{
		Context:   NewContextAssembler(workspacePath),
		Compactor: NewCompactor(provider),
	}
}

// CompactHistory compresses old messages when the context window is getting full.
func (a *Agent) CompactHistory(ctx context.Context, messages []llm.ChatMessage, maxTokens int) ([]llm.ChatMessage, error) {
	return a.Compactor.Compact(ctx, messages, maxTokens)
}
