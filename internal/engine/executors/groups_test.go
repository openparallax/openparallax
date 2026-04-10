package executors

import (
	"testing"

	"github.com/openparallax/openparallax/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGroupRegistryRegisterAndLookup(t *testing.T) {
	r := NewGroupRegistry()
	r.Register(&ToolGroup{
		Name:        "test",
		Description: "Test group",
		Schemas: []ToolSchema{
			{ActionType: types.ActionReadFile, Name: "read_file", Description: "Read a file"},
		},
	})

	g, ok := r.Lookup("test")
	require.True(t, ok)
	assert.Equal(t, "test", g.Name)
	assert.Len(t, g.Schemas, 1)
}

func TestGroupRegistryLookupMissing(t *testing.T) {
	r := NewGroupRegistry()
	_, ok := r.Lookup("nonexistent")
	assert.False(t, ok)
}

func TestGroupRegistryAvailableGroups(t *testing.T) {
	r := NewGroupRegistry()
	r.Register(&ToolGroup{Name: "a", Description: "A"})
	r.Register(&ToolGroup{Name: "b", Description: "B"})

	groups := r.AvailableGroups()
	assert.Len(t, groups, 2)
}

func TestLoadToolsDefinition(t *testing.T) {
	r := NewGroupRegistry()
	r.Register(&ToolGroup{Name: "files", Description: "File operations"})
	r.Register(&ToolGroup{Name: "git", Description: "Git operations"})

	def := r.LoadToolsDefinition()
	assert.Equal(t, "load_tools", def.Name)
	assert.Contains(t, def.Description, "files")
	assert.Contains(t, def.Description, "git")
}

func TestResolveGroupsValid(t *testing.T) {
	r := NewGroupRegistry()
	r.Register(&ToolGroup{
		Name:        "files",
		Description: "File ops",
		Schemas: []ToolSchema{
			{ActionType: types.ActionReadFile, Name: "read_file", Description: "Read"},
			{ActionType: types.ActionWriteFile, Name: "write_file", Description: "Write"},
		},
	})

	tools, summary := r.ResolveGroups([]string{"files"}, false)
	assert.Len(t, tools, 2)
	assert.Contains(t, summary, "files")
	assert.Contains(t, summary, "2 tools")
}

func TestResolveGroupsInvalid(t *testing.T) {
	r := NewGroupRegistry()
	r.Register(&ToolGroup{Name: "files", Description: "File ops", Schemas: []ToolSchema{
		{Name: "read_file", Description: "Read"},
	}})

	tools, summary := r.ResolveGroups([]string{"files", "nonexistent"}, false)
	assert.Len(t, tools, 1)
	assert.Contains(t, summary, "nonexistent")
}

func TestResolveGroupsOTRFiltering(t *testing.T) {
	r := NewGroupRegistry()
	r.Register(&ToolGroup{
		Name:        "files",
		Description: "File ops",
		Schemas: []ToolSchema{
			{Name: "read_file", Description: "Read"},
			{Name: "write_file", Description: "Write"},
			{Name: "delete_file", Description: "Delete"},
		},
	})

	tools, _ := r.ResolveGroups([]string{"files"}, true)
	names := make([]string, len(tools))
	for i, t := range tools {
		names[i] = t.Name
	}
	assert.Contains(t, names, "read_file")
	assert.NotContains(t, names, "write_file")
	assert.NotContains(t, names, "delete_file")
}

func TestDefaultGroupsFromSchemas(t *testing.T) {
	schemas := []ToolSchema{
		{ActionType: types.ActionReadFile, Name: "read_file", Description: "Read"},
		{ActionType: types.ActionWriteFile, Name: "write_file", Description: "Write"},
		{ActionType: types.ActionExecCommand, Name: "execute_command", Description: "Exec"},
		{ActionType: types.ActionGitStatus, Name: "git_status", Description: "Git status"},
	}

	groups := DefaultGroups(schemas)
	assert.GreaterOrEqual(t, len(groups), 3, "should have at least files, shell, git")

	groupNames := make(map[string]bool)
	for _, g := range groups {
		groupNames[g.Name] = true
	}
	assert.True(t, groupNames["files"])
	assert.True(t, groupNames["shell"])
	assert.True(t, groupNames["git"])
}
