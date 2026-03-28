package executors

import (
	"context"
	"fmt"

	"github.com/openparallax/openparallax/internal/logging"
	"github.com/openparallax/openparallax/internal/memory"
	"github.com/openparallax/openparallax/internal/types"
)

// Registry maps action types to executors.
type Registry struct {
	executors map[types.ActionType]Executor
}

// NewRegistry creates a Registry with all always-available executors registered.
// Conditional executors (browser, email, calendar) are added separately.
func NewRegistry(workspacePath string, cfg *types.AgentConfig, log *logging.Logger) *Registry {
	r := &Registry{executors: make(map[types.ActionType]Executor)}

	// Always available.
	r.register(NewFileExecutor(workspacePath))
	r.register(NewShellExecutor())
	r.register(NewGitExecutor(workspacePath))
	r.register(NewHTTPExecutor())
	r.register(NewScheduleExecutor(workspacePath))
	r.register(NewCanvasExecutor(workspacePath))

	// Conditionally available.
	if browser := NewBrowserExecutor(log); browser != nil {
		r.register(browser)
	}

	if cfg != nil {
		if email := NewEmailExecutor(cfg.Email); email != nil {
			r.register(email)
			if log != nil {
				log.Info("executor_registered", "executor", "email")
			}
		}

		if calendar := NewCalendarExecutor(cfg.Calendar); calendar != nil {
			r.register(calendar)
			if log != nil {
				log.Info("executor_registered", "executor", "calendar")
			}
		}
	}

	return r
}

// RegisterMemory adds the memory executor to the registry.
// Called after the memory.Manager is initialized.
func (r *Registry) RegisterMemory(manager *memory.Manager) {
	exec := NewMemoryExecutor(manager)
	for _, at := range exec.SupportedActions() {
		r.executors[at] = exec
	}
}

func (r *Registry) register(exec Executor) {
	for _, at := range exec.SupportedActions() {
		r.executors[at] = exec
	}
}

// AvailableActions returns the list of action types that have registered executors.
func (r *Registry) AvailableActions() []types.ActionType {
	actions := make([]types.ActionType, 0, len(r.executors))
	for at := range r.executors {
		actions = append(actions, at)
	}
	return actions
}

// AllToolSchemas collects tool schemas from all registered executors.
func (r *Registry) AllToolSchemas() []ToolSchema {
	seen := make(map[string]bool)
	var schemas []ToolSchema
	for _, exec := range r.executors {
		for _, schema := range exec.ToolSchemas() {
			if !seen[schema.Name] {
				seen[schema.Name] = true
				schemas = append(schemas, schema)
			}
		}
	}
	return schemas
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
