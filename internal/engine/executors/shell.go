package executors

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/openparallax/openparallax/internal/types"
	"github.com/openparallax/openparallax/platform"
)

// ShellExecutor runs shell commands with platform-aware shell selection and timeout.
type ShellExecutor struct{}

// NewShellExecutor creates a ShellExecutor.
func NewShellExecutor() *ShellExecutor { return &ShellExecutor{} }

// SupportedActions returns the shell action types.
func (s *ShellExecutor) SupportedActions() []types.ActionType {
	return []types.ActionType{types.ActionExecCommand}
}

// ToolSchemas returns the tool definition for shell command execution.
func (s *ShellExecutor) ToolSchemas() []ToolSchema {
	return []ToolSchema{
		{
			ActionType:  types.ActionExecCommand,
			Name:        "execute_command",
			Description: "Execute a shell command and return its output. Use when the user asks to run a command, script, or check system state via the terminal. ALL paths in the command MUST be absolute (e.g. /home/user/Desktop/project/db, not db or ./db). Shield evaluates the literal command string and cannot resolve relative paths against an implicit working directory. The only allowed exception is `cd <absolute-path> && <command>` — Shield resolves the implicit working directory from the cd target. Anything else with relative paths is rejected.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"command": map[string]any{
						"type":        "string",
						"description": "The shell command. All file paths must be absolute. Use `cd <absolute-path> && <cmd>` if the underlying tool requires a working directory (npm, cargo, make, etc.).",
					},
				},
				"required": []string{"command"},
			},
		},
	}
}

// Execute runs the shell command with a 30-second timeout.
func (s *ShellExecutor) Execute(ctx context.Context, action *types.ActionRequest) *types.ActionResult {
	command, _ := action.Payload["command"].(string)
	if command == "" {
		return &types.ActionResult{RequestID: action.RequestID, Success: false, Error: "empty command", Summary: "empty command"}
	}

	timeout := 30 * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	shell, flag := platform.ShellConfig()
	cmd := exec.CommandContext(ctx, shell, flag, command)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	setProcGroup(cmd)

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			if cmd.Process != nil {
				_ = platform.KillProcessTree(cmd.Process.Pid)
			}
			return &types.ActionResult{
				RequestID:  action.RequestID,
				Success:    false,
				Error:      fmt.Sprintf("command timed out after %s", timeout),
				Summary:    fmt.Sprintf("timeout: %s", truncateCmd(command)),
				DurationMs: duration.Milliseconds(),
			}
		}
		output := stdout.String()
		if stderr.Len() > 0 {
			output += "\n" + stderr.String()
		}
		return &types.ActionResult{
			RequestID:  action.RequestID,
			Success:    false,
			Error:      err.Error(),
			Output:     output,
			Summary:    fmt.Sprintf("failed: %s", truncateCmd(command)),
			DurationMs: duration.Milliseconds(),
		}
	}

	output := stdout.String()
	if stderr.Len() > 0 {
		output += "\n--- stderr ---\n" + stderr.String()
	}

	return &types.ActionResult{
		RequestID:  action.RequestID,
		Success:    true,
		Output:     output,
		Summary:    fmt.Sprintf("ran: %s", truncateCmd(command)),
		DurationMs: duration.Milliseconds(),
	}
}

func truncateCmd(cmd string) string {
	return Truncate(cmd, 60)
}
