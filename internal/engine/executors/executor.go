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
}
