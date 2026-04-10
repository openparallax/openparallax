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

// NewAgent creates an Agent. The memoryFlusher callback ships extracted
// facts from compaction to the engine for persistence (the agent is
// sandboxed and cannot write to the workspace directly). Pass nil to
// skip memory extraction (e.g. sub-agents).
func NewAgent(provider llm.Provider, workspacePath string, mem *memory.Manager, memoryFlusher MemoryFlusher, disabledSkills []string) *Agent {
	return &Agent{
		Context:   NewContextAssembler(workspacePath, mem),
		Compactor: NewCompactor(provider, memoryFlusher),
		Skills:    NewSkillManager(workspacePath, disabledSkills),
	}
}

// CompactHistory compresses old messages when the context window is getting full.
// The threshold parameter (0-100) controls what percentage of maxTokens is
// reserved for history; pass 0 to use the default (70%).
func (a *Agent) CompactHistory(ctx context.Context, messages []llm.ChatMessage, maxTokens, threshold int) ([]llm.ChatMessage, error) {
	return a.Compactor.Compact(ctx, messages, maxTokens, threshold)
}
