package agent

import (
	"github.com/openparallax/openparallax/internal/engine/executors"
	"github.com/openparallax/openparallax/internal/session"
	"github.com/openparallax/openparallax/internal/types"
	"github.com/openparallax/openparallax/llm"
)

// GenerateToolDefinitions converts executor ToolSchemas into LLM ToolDefinitions.
func GenerateToolDefinitions(schemas []executors.ToolSchema) []llm.ToolDefinition {
	tools := make([]llm.ToolDefinition, 0, len(schemas))
	for _, s := range schemas {
		tools = append(tools, llm.ToolDefinition{
			Name:        s.Name,
			Description: s.Description,
			Parameters:  s.Parameters,
		})
	}
	return tools
}

// FilterToolsForOTR removes tools that are not allowed in OTR mode.
// The LLM cannot call write/delete/execute tools because they don't exist in its tool set.
func FilterToolsForOTR(tools []llm.ToolDefinition) []llm.ToolDefinition {
	filtered := make([]llm.ToolDefinition, 0, len(tools))
	for _, t := range tools {
		if session.IsOTRAllowed(types.ActionType(t.Name)) {
			filtered = append(filtered, t)
		}
	}
	return filtered
}
