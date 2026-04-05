package executors

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/openparallax/openparallax/internal/types"
)

// SubAgentManagerInterface abstracts the SubAgentManager to avoid circular imports.
type SubAgentManagerInterface interface {
	Create(req SubAgentRequest) (string, error)
	Status(name string) (SubAgentInfo, error)
	Result(name string, timeout time.Duration) (string, error)
	SendMessage(name, content string) error
	Delete(name string) error
	List() []SubAgentInfo
}

// SubAgentRequest contains the parameters for creating a sub-agent.
type SubAgentRequest struct {
	Task           string
	ToolGroups     []string
	Model          string
	TimeoutSeconds int
	SessionID      string
	IsOTR          bool
}

// SubAgentInfo is a read-only view of a sub-agent for the executor.
type SubAgentInfo struct {
	Name          string
	Task          string
	Status        string
	Model         string
	ToolGroups    []string
	Result        string
	Error         string
	LLMCallCount  int
	ToolCallCount int
	CreatedAt     time.Time
	CompletedAt   *time.Time
}

// SubAgentExecutor handles sub-agent tool calls by delegating to the SubAgentManager.
type SubAgentExecutor struct {
	manager SubAgentManagerInterface
}

// NewSubAgentExecutor creates a new SubAgentExecutor.
func NewSubAgentExecutor(manager SubAgentManagerInterface) *SubAgentExecutor {
	return &SubAgentExecutor{manager: manager}
}

// SupportedActions returns the action types this executor handles.
func (e *SubAgentExecutor) SupportedActions() []types.ActionType {
	return []types.ActionType{
		types.ActionCreateAgent, types.ActionAgentStatus, types.ActionAgentResult,
		types.ActionAgentMessage, types.ActionDeleteAgent, types.ActionListAgents,
	}
}

// ToolSchemas returns the tool definitions for sub-agent management.
func (e *SubAgentExecutor) ToolSchemas() []ToolSchema {
	return []ToolSchema{
		{
			ActionType:  types.ActionCreateAgent,
			Name:        "create_agent",
			Description: "Spawn a sub-agent to work on a task in parallel. Returns the agent's name. Use agent_result to collect the output when ready.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"task": map[string]any{
						"type":        "string",
						"description": "Clear description of the task for the sub-agent to complete.",
					},
					"tool_groups": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "Tool groups the sub-agent should have access to (e.g. [\"files\", \"shell\"]). Omit for all available tools.",
					},
					"model": map[string]any{
						"type":        "string",
						"description": "LLM model for the sub-agent. Defaults to the cheapest available model.",
					},
					"wait": map[string]any{
						"type":        "boolean",
						"description": "If true, block until the sub-agent completes and return the result directly.",
					},
				},
				"required": []string{"task"},
			},
		},
		{
			ActionType:  types.ActionAgentStatus,
			Name:        "agent_status",
			Description: "Check the status of a running sub-agent.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type":        "string",
						"description": "Sub-agent name.",
					},
				},
				"required": []string{"name"},
			},
		},
		{
			ActionType:  types.ActionAgentResult,
			Name:        "agent_result",
			Description: "Collect the result from a completed sub-agent. If still working, waits for completion (up to timeout).",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type":        "string",
						"description": "Sub-agent name.",
					},
					"timeout_seconds": map[string]any{
						"type":        "integer",
						"description": "Max seconds to wait. Default 120.",
					},
				},
				"required": []string{"name"},
			},
		},
		{
			ActionType:  types.ActionAgentMessage,
			Name:        "agent_message",
			Description: "Send an additional instruction to a running sub-agent.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type":        "string",
						"description": "Sub-agent name.",
					},
					"message": map[string]any{
						"type":        "string",
						"description": "Additional instruction or context.",
					},
				},
				"required": []string{"name", "message"},
			},
		},
		{
			ActionType:  types.ActionDeleteAgent,
			Name:        "delete_agent",
			Description: "Terminate a sub-agent immediately. Use when a sub-agent is stuck or no longer needed.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type":        "string",
						"description": "Sub-agent name.",
					},
				},
				"required": []string{"name"},
			},
		},
		{
			ActionType:  types.ActionListAgents,
			Name:        "list_agents",
			Description: "List all active sub-agents with their status.",
			Parameters: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
	}
}

// Execute dispatches a sub-agent tool call to the manager.
func (e *SubAgentExecutor) Execute(_ context.Context, action *types.ActionRequest) *types.ActionResult {
	switch action.Type {
	case types.ActionCreateAgent:
		return e.executeCreate(action)
	case types.ActionAgentStatus:
		return e.executeStatus(action)
	case types.ActionAgentResult:
		return e.executeResult(action)
	case types.ActionAgentMessage:
		return e.executeMessage(action)
	case types.ActionDeleteAgent:
		return e.executeDelete(action)
	case types.ActionListAgents:
		return e.executeList(action)
	default:
		return &types.ActionResult{Success: false, Error: "unknown sub-agent action", Summary: "unknown action"}
	}
}

func (e *SubAgentExecutor) executeCreate(action *types.ActionRequest) *types.ActionResult {
	task, _ := action.Payload["task"].(string)
	if task == "" {
		return &types.ActionResult{Success: false, Error: "task is required", Summary: "missing task"}
	}

	var toolGroups []string
	if tg, ok := action.Payload["tool_groups"].([]any); ok {
		for _, g := range tg {
			if s, ok := g.(string); ok {
				toolGroups = append(toolGroups, s)
			}
		}
	}

	model, _ := action.Payload["model"].(string)
	wait, _ := action.Payload["wait"].(bool)

	req := SubAgentRequest{
		Task:       task,
		ToolGroups: toolGroups,
		Model:      model,
	}

	name, err := e.manager.Create(req)
	if err != nil {
		return &types.ActionResult{Success: false, Error: err.Error(), Summary: "failed to create sub-agent"}
	}

	if wait {
		result, waitErr := e.manager.Result(name, 120*time.Second)
		if waitErr != nil {
			return &types.ActionResult{
				Success: false, Error: waitErr.Error(),
				Summary: fmt.Sprintf("sub-agent %s failed", name),
			}
		}
		return &types.ActionResult{
			Success: true, Output: result,
			Summary: fmt.Sprintf("sub-agent %s completed", name),
		}
	}

	return &types.ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Created sub-agent %q. It is now working on: %s\nUse agent_result(\"%s\") to collect the output when ready.", name, Truncate(task, 100), name),
		Summary: fmt.Sprintf("spawned sub-agent %s", name),
	}
}

func (e *SubAgentExecutor) executeStatus(action *types.ActionRequest) *types.ActionResult {
	name, _ := action.Payload["name"].(string)
	info, err := e.manager.Status(name)
	if err != nil {
		return &types.ActionResult{Success: false, Error: err.Error(), Summary: "agent not found"}
	}

	elapsed := time.Since(info.CreatedAt).Round(time.Second)
	status := fmt.Sprintf("%s: %s (%d LLM calls, %d tool calls, %s elapsed)",
		info.Name, info.Status, info.LLMCallCount, info.ToolCallCount, elapsed)
	if info.Status == "completed" {
		status += "\nResult ready — use agent_result(\"" + info.Name + "\") to collect."
	}
	if info.Error != "" {
		status += "\nError: " + info.Error
	}
	return &types.ActionResult{Success: true, Output: status, Summary: fmt.Sprintf("%s: %s", name, info.Status)}
}

func (e *SubAgentExecutor) executeResult(action *types.ActionRequest) *types.ActionResult {
	name, _ := action.Payload["name"].(string)
	timeoutSecs := 120
	if ts, ok := action.Payload["timeout_seconds"].(float64); ok && ts > 0 {
		timeoutSecs = int(ts)
	}

	result, err := e.manager.Result(name, time.Duration(timeoutSecs)*time.Second)
	if err != nil {
		return &types.ActionResult{Success: false, Error: err.Error(), Summary: fmt.Sprintf("%s failed", name)}
	}
	return &types.ActionResult{Success: true, Output: result, Summary: fmt.Sprintf("%s completed", name)}
}

func (e *SubAgentExecutor) executeMessage(action *types.ActionRequest) *types.ActionResult {
	name, _ := action.Payload["name"].(string)
	message, _ := action.Payload["message"].(string)
	if name == "" || message == "" {
		return &types.ActionResult{Success: false, Error: "name and message are required", Summary: "missing parameters"}
	}
	if err := e.manager.SendMessage(name, message); err != nil {
		return &types.ActionResult{Success: false, Error: err.Error(), Summary: "send failed"}
	}
	return &types.ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Message sent to %s", name),
		Summary: fmt.Sprintf("sent message to %s", name),
	}
}

func (e *SubAgentExecutor) executeDelete(action *types.ActionRequest) *types.ActionResult {
	name, _ := action.Payload["name"].(string)
	if err := e.manager.Delete(name); err != nil {
		return &types.ActionResult{Success: false, Error: err.Error(), Summary: "delete failed"}
	}
	return &types.ActionResult{Success: true, Output: fmt.Sprintf("Terminated sub-agent %s.", name), Summary: fmt.Sprintf("terminated %s", name)}
}

func (e *SubAgentExecutor) executeList(_ *types.ActionRequest) *types.ActionResult {
	agents := e.manager.List()
	if len(agents) == 0 {
		return &types.ActionResult{Success: true, Output: "No active sub-agents.", Summary: "0 sub-agents"}
	}

	var sb strings.Builder
	for _, a := range agents {
		elapsed := time.Since(a.CreatedAt).Round(time.Second)
		fmt.Fprintf(&sb, "- %s: %s — %s (%d calls, %s)\n",
			a.Name, a.Status, Truncate(a.Task, 60), a.LLMCallCount, elapsed)
	}
	return &types.ActionResult{
		Success: true, Output: sb.String(),
		Summary: fmt.Sprintf("%d sub-agents", len(agents)),
	}
}
