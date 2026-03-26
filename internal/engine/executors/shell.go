package executors

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/openparallax/openparallax/internal/crypto"
	"github.com/openparallax/openparallax/internal/platform"
	"github.com/openparallax/openparallax/internal/types"
)

// ShellExecutor runs shell commands with platform-aware shell selection and timeout.
type ShellExecutor struct{}

// NewShellExecutor creates a ShellExecutor.
func NewShellExecutor() *ShellExecutor { return &ShellExecutor{} }

// SupportedActions returns the shell action types.
func (s *ShellExecutor) SupportedActions() []types.ActionType {
	return []types.ActionType{types.ActionExecCommand}
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
		Artifact: &types.Artifact{
			ID: crypto.NewID(), Type: "command_output", Title: truncateCmd(command),
			Content: output, SizeBytes: int64(len(output)), PreviewType: "terminal",
		},
	}
}

func truncateCmd(cmd string) string {
	if len(cmd) > 60 {
		return cmd[:60] + "..."
	}
	return cmd
}
