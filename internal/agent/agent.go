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

// NewAgent creates an Agent. The disabledSkills parameter lists skill names
// that should not be available to the LLM (from config skills.disabled).
func NewAgent(provider llm.Provider, workspacePath string, mem *memory.Manager, disabledSkills []string) *Agent {
	return &Agent{
		Context:   NewContextAssembler(workspacePath, mem),
		Compactor: NewCompactor(provider, mem),
		Skills:    NewSkillManager(workspacePath, disabledSkills),
	}
}

// CompactHistory compresses old messages when the context window is getting full.
func (a *Agent) CompactHistory(ctx context.Context, messages []llm.ChatMessage, maxTokens int) ([]llm.ChatMessage, error) {
	return a.Compactor.Compact(ctx, messages, maxTokens)
}
