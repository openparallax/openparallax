package engine

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/openparallax/openparallax/internal/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPickNameUnique(t *testing.T) {
	used := make(map[string]bool)
	names := make([]string, 10)
	for i := range 10 {
		name := pickName(used)
		used[name] = true
		names[i] = name
	}

	// All names should be unique.
	seen := make(map[string]bool)
	for _, n := range names {
		assert.False(t, seen[n], "duplicate name: %s", n)
		seen[n] = true
	}

	// First name should be from the pool.
	assert.Contains(t, defaultNamePool, names[0])
}

func TestPickNameExhaustion(t *testing.T) {
	used := make(map[string]bool)
	for _, name := range defaultNamePool {
		used[name] = true
	}

	// All names used — should get a suffix.
	name := pickName(used)
	assert.Equal(t, defaultNamePool[0]+"-2", name)
}

func TestSubAgentSystemPrompt(t *testing.T) {
	prompt := SubAgentSystemPrompt("Research competitor pricing")
	assert.Contains(t, prompt, "Research competitor pricing")
	assert.Contains(t, prompt, "sub-agent")
}

func TestFilterSubAgentTools(t *testing.T) {
	input := []llm.ToolDefinition{
		{Name: "read_file"},
		{Name: "write_file"},
		{Name: "create_agent"},
		{Name: "agent_status"},
		{Name: "agent_result"},
		{Name: "agent_message"},
		{Name: "delete_agent"},
		{Name: "list_agents"},
		{Name: "execute_command"},
	}

	filtered := filterSubAgentTools(input)
	names := make([]string, len(filtered))
	for i, t := range filtered {
		names[i] = t.Name
	}

	assert.Contains(t, names, "read_file")
	assert.Contains(t, names, "write_file")
	assert.Contains(t, names, "execute_command")
	assert.NotContains(t, names, "create_agent")
	assert.NotContains(t, names, "agent_status")
	assert.NotContains(t, names, "agent_result")
	assert.NotContains(t, names, "agent_message")
	assert.NotContains(t, names, "delete_agent")
	assert.NotContains(t, names, "list_agents")
	assert.Len(t, filtered, 3)
}

func TestIsExcludedSubAgentGroup(t *testing.T) {
	assert.True(t, isExcludedSubAgentGroup("agents"))
	assert.True(t, isExcludedSubAgentGroup("schedule"))
	assert.True(t, isExcludedSubAgentGroup("memory"))
	assert.False(t, isExcludedSubAgentGroup("files"))
	assert.False(t, isExcludedSubAgentGroup("shell"))
	assert.False(t, isExcludedSubAgentGroup("git"))
	assert.False(t, isExcludedSubAgentGroup("browser"))
}

func TestTruncateResult(t *testing.T) {
	short := "hello"
	assert.Equal(t, "hello", truncateResult(short, 500))

	long := ""
	for range 100 {
		long += "hello world "
	}
	result := truncateResult(long, 50)
	assert.Len(t, result, 53) // 50 + "..."
	assert.True(t, len(result) <= 53)
}

func TestWriteAndClearAgentsMD(t *testing.T) {
	workspace := t.TempDir()
	now := time.Now()

	agents := []*SubAgent{
		{Name: "phoenix", Task: "Research pricing", Status: StatusWorking, CreatedAt: now, ToolGroups: []string{"browser", "files"}},
		{Name: "cortex", Task: "Refactor code", Status: StatusCompleted, CreatedAt: now},
	}

	WriteAgentsMD(workspace, agents)

	// Only working agents should appear.
	data, err := readFileContent(workspace)
	require.NoError(t, err)
	assert.Contains(t, data, "phoenix")
	assert.Contains(t, data, "Research pricing")
	assert.NotContains(t, data, "cortex") // completed, not active

	// Clear.
	ClearAgentsMD(workspace)
	_, err = readFileContent(workspace)
	assert.Error(t, err) // file removed
}

func readFileContent(workspace string) (string, error) {
	data, err := os.ReadFile(filepath.Join(workspace, "AGENTS.md"))
	return string(data), err
}

func TestItoa(t *testing.T) {
	assert.Equal(t, "0", itoa(0))
	assert.Equal(t, "5", itoa(5))
	assert.Equal(t, "10", itoa(10))
	assert.Equal(t, "123", itoa(123))
}
