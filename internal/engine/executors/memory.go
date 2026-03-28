package executors

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/openparallax/openparallax/internal/memory"
	"github.com/openparallax/openparallax/internal/types"
)

// MemoryExecutor handles memory_write and memory_search operations.
type MemoryExecutor struct {
	manager *memory.Manager
}

// NewMemoryExecutor creates a MemoryExecutor.
func NewMemoryExecutor(manager *memory.Manager) *MemoryExecutor {
	return &MemoryExecutor{manager: manager}
}

// SupportedActions returns the memory action types.
func (m *MemoryExecutor) SupportedActions() []types.ActionType {
	return []types.ActionType{types.ActionMemoryWrite, types.ActionMemorySearch}
}

// ToolSchemas returns tool definitions for memory operations.
func (m *MemoryExecutor) ToolSchemas() []ToolSchema {
	return []ToolSchema{
		{
			ActionType:  types.ActionMemoryWrite,
			Name:        "memory_write",
			Description: "Append a note to the agent's memory. Use when you learn something important about the user, their preferences, or decisions made during the conversation. Content is appended to MEMORY.md.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"content": map[string]any{
						"type":        "string",
						"description": "The content to append to memory. Should be concise and factual.",
					},
					"category": map[string]any{
						"type":        "string",
						"description": "Optional category tag.",
						"enum":        []string{"fact", "preference", "decision", "task"},
					},
				},
				"required": []string{"content"},
			},
		},
		{
			ActionType:  types.ActionMemorySearch,
			Name:        "memory_search",
			Description: "Search the agent's memory for relevant information. Use when the user references something from a previous conversation or you need context from past interactions.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "The search query.",
					},
				},
				"required": []string{"query"},
			},
		},
	}
}

// Execute dispatches to the appropriate memory operation.
func (m *MemoryExecutor) Execute(_ context.Context, action *types.ActionRequest) *types.ActionResult {
	switch action.Type {
	case types.ActionMemoryWrite:
		return m.write(action)
	case types.ActionMemorySearch:
		return m.search(action)
	default:
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "unknown memory action"}
	}
}

func (m *MemoryExecutor) write(action *types.ActionRequest) *types.ActionResult {
	content, _ := action.Payload["content"].(string)
	if content == "" {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "content is required", Summary: "memory write failed"}
	}

	category, _ := action.Payload["category"].(string)
	timestamp := time.Now().Format("2006-01-02 15:04")

	var entry string
	if category != "" {
		entry = fmt.Sprintf("\n- **%s** [%s]: %s\n", timestamp, category, strings.TrimSpace(content))
	} else {
		entry = fmt.Sprintf("\n- **%s**: %s\n", timestamp, strings.TrimSpace(content))
	}

	if err := m.manager.Append(types.MemoryMain, entry); err != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: err.Error(), Summary: "memory write failed"}
	}

	return &types.ActionResult{
		RequestID: action.RequestID, Success: true,
		Output:  fmt.Sprintf("Saved to memory: %s", content),
		Summary: "memory updated",
	}
}

func (m *MemoryExecutor) search(action *types.ActionRequest) *types.ActionResult {
	query, _ := action.Payload["query"].(string)
	if query == "" {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "query is required", Summary: "memory search failed"}
	}

	results, err := m.manager.Search(query, 10)
	if err != nil {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: err.Error(), Summary: "memory search failed"}
	}

	if len(results) == 0 {
		return &types.ActionResult{
			RequestID: action.RequestID, Success: true,
			Output:  "No matching memories found.",
			Summary: fmt.Sprintf("searched memory for '%s': no results", query),
		}
	}

	var lines []string
	for _, r := range results {
		lines = append(lines, fmt.Sprintf("[%s] %s", r.Path, r.Snippet))
	}

	output := strings.Join(lines, "\n")
	return &types.ActionResult{
		RequestID: action.RequestID, Success: true,
		Output:  output,
		Summary: fmt.Sprintf("found %d memory results for '%s'", len(results), query),
	}
}
