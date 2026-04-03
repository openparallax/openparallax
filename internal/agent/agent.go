package agent

import (
	"context"

	"github.com/openparallax/openparallax/llm"
	"github.com/openparallax/openparallax/memory"
)

// Agent coordinates context assembly, history compaction, and skill management.
// In the tool-use architecture, the LLM decides what tools to call —
// there is no separate planner, builder, or self-evaluator.
type Agent struct {
	Context   *ContextAssembler
	Compactor *Compactor
	Skills    *SkillManager
}

// NewAgent creates an Agent.
func NewAgent(provider llm.Provider, workspacePath string, mem *memory.Manager) *Agent {
	return &Agent{
		Context:   NewContextAssembler(workspacePath),
		Compactor: NewCompactor(provider, mem),
		Skills:    NewSkillManager(workspacePath),
	}
}

// CompactHistory compresses old messages when the context window is getting full.
func (a *Agent) CompactHistory(ctx context.Context, messages []llm.ChatMessage, maxTokens int) ([]llm.ChatMessage, error) {
	return a.Compactor.Compact(ctx, messages, maxTokens)
}
