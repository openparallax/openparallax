// Package agent implements the core reasoning loop that assembles context,
// plans actions, and generates responses via the LLM.
package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ContextAssembler builds the system prompt from workspace memory files.
type ContextAssembler struct {
	workspacePath string
}

// NewContextAssembler creates a ContextAssembler for the given workspace.
func NewContextAssembler(workspacePath string) *ContextAssembler {
	return &ContextAssembler{workspacePath: workspacePath}
}

// memoryFile defines a workspace file and the header to use in the system prompt.
type memoryFile struct {
	name   string
	header string
}

// memoryFiles lists the workspace files loaded into the system prompt, in order.
var memoryFiles = []memoryFile{
	{"SOUL.md", "## Core Values and Guardrails"},
	{"IDENTITY.md", "## Identity"},
	{"USER.md", "## User Profile"},
	{"MEMORY.md", "## Memory"},
	{"TOOLS.md", "## Available Tools"},
	{"BOOT.md", "## Startup Checklist"},
}

// Assemble reads all memory files and constructs the system prompt.
// Missing files are silently skipped — this is expected on a fresh workspace.
func (c *ContextAssembler) Assemble() (string, error) {
	var parts []string

	for _, f := range memoryFiles {
		content, err := os.ReadFile(filepath.Join(c.workspacePath, f.name))
		if err != nil {
			continue
		}
		trimmed := strings.TrimSpace(string(content))
		if trimmed == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s\n\n%s", f.header, trimmed))
	}

	result := strings.Join(parts, "\n\n---\n\n")

	// Core behavioral instruction that applies to all interactions.
	result += "\n\n---\n\n## Behavioral Rules\n\n"
	result += "You are an AI agent. You can execute real actions through your action pipeline.\n"
	result += "NEVER generate fake tool calls, XML tags, or pretend to execute actions in your response text.\n"
	result += "NEVER output <tool_call>, <tool_response>, or similar markup. Your responses are plain text for the user.\n"
	result += "When you want to perform an action, the system handles it through the planning pipeline. You describe results based on what actually happened.\n"

	return result, nil
}
