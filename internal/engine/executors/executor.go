// Package executors implements action handlers that execute agent-proposed
// operations such as file I/O, shell commands, and other system interactions.
package executors

import (
	"context"

	"github.com/openparallax/openparallax/internal/types"
)

// WorkspaceScope describes whether an executor's filesystem writes are
// confined to the workspace directory. Every executor must declare its
// scope so future authors are forced to make a conscious decision before
// adding a new tool group.
type WorkspaceScope int

const (
	// ScopeScoped means every disk write performed by this executor MUST go
	// through ResolveInWorkspace, which rejects any path that escapes the
	// workspace directory.
	ScopeScoped WorkspaceScope = iota

	// ScopeUnscoped means the executor intentionally operates outside the
	// workspace boundary (e.g. shell, system, http). The executor is
	// responsible for any other safety enforcement it needs.
	ScopeUnscoped

	// ScopeNoFilesystem means the executor does not write to the local
	// filesystem at all (e.g. http, email send).
	ScopeNoFilesystem
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

	// WorkspaceScope declares the executor's filesystem boundary discipline.
	// See the WorkspaceScope constants for the meaning of each value.
	WorkspaceScope() WorkspaceScope
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
