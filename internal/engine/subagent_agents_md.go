package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// WriteAgentsMD writes the AGENTS.md file listing active sub-agents.
func WriteAgentsMD(workspace string, agents []*SubAgent) {
	path := filepath.Join(workspace, "AGENTS.md")

	var active []*SubAgent
	for _, a := range agents {
		if a.Status == StatusWorking || a.Status == StatusSpawning {
			active = append(active, a)
		}
	}

	if len(active) == 0 {
		ClearAgentsMD(workspace)
		return
	}

	var sb strings.Builder
	sb.WriteString("# Active Sub-Agents\n\n")
	for _, a := range active {
		fmt.Fprintf(&sb, "## %s\n", a.Name)
		fmt.Fprintf(&sb, "- **Task:** %s\n", a.Task)
		fmt.Fprintf(&sb, "- **Status:** %s\n", a.Status)
		if len(a.ToolGroups) > 0 {
			fmt.Fprintf(&sb, "- **Tools:** %s\n", strings.Join(a.ToolGroups, ", "))
		}
		fmt.Fprintf(&sb, "- **Created:** %s\n", a.CreatedAt.Format(time.RFC3339))
		sb.WriteString("\n")
	}

	_ = os.WriteFile(path, []byte(sb.String()), 0o644)
}

// ClearAgentsMD removes the AGENTS.md file when no agents are active.
func ClearAgentsMD(workspace string) {
	path := filepath.Join(workspace, "AGENTS.md")
	_ = os.Remove(path)
}
