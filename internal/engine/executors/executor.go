// Package executors implements action handlers that execute agent-proposed
// operations such as file I/O, shell commands, and other system interactions.
package executors

import (
	"context"

	"github.com/openparallax/openparallax/internal/types"
)

// Executor handles one category of action.
type Executor interface {
	// Execute runs the action and returns a result.
	Execute(ctx context.Context, action *types.ActionRequest) *types.ActionResult

	// SupportedActions returns the action types this executor handles.
	SupportedActions() []types.ActionType

	// ToolSchemas returns tool definitions for LLM consumption.
	// Each schema describes one tool with its name, description, and JSON Schema parameters.
	ToolSchemas() []ToolSchema
}

// ToolSchema describes one tool for LLM consumption.
type ToolSchema struct {
	// ActionType maps to the executor action type.
	ActionType types.ActionType

	// Name is the tool identifier the LLM will use.
	Name string

	// Description tells the LLM when and how to use this tool.
	Description string

	// Parameters is the JSON Schema for the tool's input.
	Parameters map[string]any
}
