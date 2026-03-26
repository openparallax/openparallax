package executors

import (
	"context"
	"fmt"

	"github.com/openparallax/openparallax/internal/types"
)

// Registry maps action types to executors.
type Registry struct {
	executors map[types.ActionType]Executor
}

// NewRegistry creates a Registry with all available executors registered.
func NewRegistry(workspacePath string) *Registry {
	r := &Registry{executors: make(map[types.ActionType]Executor)}

	all := []Executor{
		NewFileExecutor(workspacePath),
		NewShellExecutor(),
	}

	for _, exec := range all {
		for _, at := range exec.SupportedActions() {
			r.executors[at] = exec
		}
	}

	return r
}

// AvailableActions returns the list of action types that have registered executors.
func (r *Registry) AvailableActions() []types.ActionType {
	actions := make([]types.ActionType, 0, len(r.executors))
	for at := range r.executors {
		actions = append(actions, at)
	}
	return actions
}

// Execute dispatches an action to the appropriate executor.
func (r *Registry) Execute(ctx context.Context, action *types.ActionRequest) *types.ActionResult {
	exec, ok := r.executors[action.Type]
	if !ok {
		return &types.ActionResult{
			RequestID: action.RequestID,
			Success:   false,
			Error:     fmt.Sprintf("no executor registered for action type: %s", action.Type),
			Summary:   "unknown action type",
		}
	}
	return exec.Execute(ctx, action)
}
