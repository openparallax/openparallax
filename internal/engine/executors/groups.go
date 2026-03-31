package executors

import (
	"fmt"
	"strings"
	"sync"

	"github.com/openparallax/openparallax/internal/llm"
	"github.com/openparallax/openparallax/internal/types"
)

// ToolGroup defines a named set of tools that can be loaded on demand.
type ToolGroup struct {
	// Name is the group identifier used in load_tools calls.
	Name string

	// Description explains what the group provides for the LLM.
	Description string

	// Schemas are the tool definitions in this group.
	Schemas []ToolSchema
}

// GroupRegistry manages tool groups and the load_tools meta-tool.
type GroupRegistry struct {
	groups map[string]*ToolGroup
	mu     sync.RWMutex
}

// NewGroupRegistry creates an empty group registry.
func NewGroupRegistry() *GroupRegistry {
	return &GroupRegistry{groups: make(map[string]*ToolGroup)}
}

// Register adds a tool group.
func (r *GroupRegistry) Register(g *ToolGroup) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.groups[g.Name] = g
}

// Lookup returns a group by name.
func (r *GroupRegistry) Lookup(name string) (*ToolGroup, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	g, ok := r.groups[name]
	return g, ok
}

// AvailableGroups returns all group names and descriptions.
func (r *GroupRegistry) AvailableGroups() []ToolGroup {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]ToolGroup, 0, len(r.groups))
	for _, g := range r.groups {
		result = append(result, *g)
	}
	return result
}

// LoadToolsDefinition generates the load_tools tool definition with all
// available groups listed in the description. Groups whose underlying
// executors aren't registered are excluded.
func (r *GroupRegistry) LoadToolsDefinition() llm.ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var lines []string
	for name, g := range r.groups {
		lines = append(lines, fmt.Sprintf("- %s: %s", name, g.Description))
	}

	desc := fmt.Sprintf(
		"Load tool groups to gain access to additional capabilities. "+
			"Call this before using any tools. You start each turn with no tools loaded. "+
			"If no tools are needed, respond directly without calling this.\n\n"+
			"Available groups:\n%s", strings.Join(lines, "\n"))

	return llm.ToolDefinition{
		Name:        "load_tools",
		Description: desc,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"groups": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "List of tool group names to load.",
				},
			},
			"required": []string{"groups"},
		},
	}
}

// ResolveGroups takes group names and returns the combined tool definitions.
// Returns loaded tools and any error messages for invalid group names.
func (r *GroupRegistry) ResolveGroups(names []string, isOTR bool) ([]llm.ToolDefinition, string) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var tools []llm.ToolDefinition
	var errors []string
	var loaded []string

	for _, name := range names {
		g, ok := r.groups[name]
		if !ok {
			errors = append(errors, fmt.Sprintf("Unknown group: %q", name))
			continue
		}
		groupTools := schemasToDefinitions(g.Schemas)
		if isOTR {
			groupTools = filterOTRTools(groupTools)
		}
		tools = append(tools, groupTools...)
		loaded = append(loaded, fmt.Sprintf("  %s: %d tools", name, len(groupTools)))
	}

	summary := fmt.Sprintf("Loaded %d tools from groups [%s]:\n%s",
		len(tools), strings.Join(names, ", "), strings.Join(loaded, "\n"))
	if len(errors) > 0 {
		summary += "\n\nErrors:\n" + strings.Join(errors, "\n")
	}

	return tools, summary
}

func schemasToDefinitions(schemas []ToolSchema) []llm.ToolDefinition {
	defs := make([]llm.ToolDefinition, len(schemas))
	for i, s := range schemas {
		defs[i] = llm.ToolDefinition{
			Name:        s.Name,
			Description: s.Description,
			Parameters:  s.Parameters,
		}
	}
	return defs
}

// otrWriteTools lists tools that should be filtered in OTR mode.
var otrWriteTools = map[string]bool{
	"write_file": true, "delete_file": true, "move_file": true,
	"copy_file": true, "create_directory": true, "delete_directory": true,
	"move_directory": true, "copy_directory": true,
	"git_commit": true, "git_push": true,
	"email_send": true, "send_email": true,
	"memory_write":  true,
	"canvas_create": true, "canvas_update": true, "canvas_project": true,
	"generate_image": true, "edit_image": true, "generate_video": true,
}

func filterOTRTools(tools []llm.ToolDefinition) []llm.ToolDefinition {
	filtered := make([]llm.ToolDefinition, 0, len(tools))
	for _, t := range tools {
		if !otrWriteTools[t.Name] {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

// DefaultGroups returns the standard tool groups derived from executor schemas.
func DefaultGroups(schemas []ToolSchema) []*ToolGroup {
	groupMap := map[string]*ToolGroup{
		"files":            {Name: "files", Description: "Read, write, list, search, and delete files in the workspace"},
		"shell":            {Name: "shell", Description: "Execute shell commands on the system"},
		"git":              {Name: "git", Description: "Git version control — status, diff, log, commit, push, pull, branch, clone"},
		"browser":          {Name: "browser", Description: "Browse the web — navigate, click, type, extract, screenshot"},
		"email":            {Name: "email", Description: "Send emails via SMTP"},
		"calendar":         {Name: "calendar", Description: "Manage calendar events — list, create, update, delete"},
		"memory":           {Name: "memory", Description: "Write structured memories and search past conversations"},
		"schedule":         {Name: "schedule", Description: "Manage recurring tasks via HEARTBEAT.md cron entries"},
		"canvas":           {Name: "canvas", Description: "Create files, multi-file projects, and live-preview websites"},
		"image_generation": {Name: "image_generation", Description: "Generate images using AI (DALL-E, Imagen, Stability AI)"},
		"video_generation": {Name: "video_generation", Description: "Generate videos using AI (Sora)"},
	}

	actionToGroup := map[types.ActionType]string{
		types.ActionReadFile: "files", types.ActionWriteFile: "files",
		types.ActionDeleteFile: "files", types.ActionMoveFile: "files",
		types.ActionCopyFile: "files", types.ActionCreateDir: "files",
		types.ActionListDir: "files", types.ActionSearchFiles: "files",
		types.ActionCopyDir: "files", types.ActionMoveDir: "files",
		types.ActionDeleteDir:   "files",
		types.ActionExecCommand: "shell",
		types.ActionGitStatus:   "git", types.ActionGitDiff: "git",
		types.ActionGitCommit: "git", types.ActionGitPush: "git",
		types.ActionGitPull: "git", types.ActionGitLog: "git",
		types.ActionGitBranch: "git", types.ActionGitCheckout: "git",
		types.ActionGitClone:   "git",
		types.ActionBrowserNav: "browser", types.ActionBrowserClick: "browser",
		types.ActionBrowserType: "browser", types.ActionBrowserExtract: "browser",
		types.ActionBrowserShot:  "browser",
		types.ActionSendEmail:    "email",
		types.ActionReadCalendar: "calendar", types.ActionCreateEvent: "calendar",
		types.ActionUpdateEvent: "calendar", types.ActionDeleteEvent: "calendar",
		types.ActionMemoryWrite: "memory", types.ActionMemorySearch: "memory",
		types.ActionCreateSchedule: "schedule", types.ActionDeleteSchedule: "schedule",
		types.ActionListSchedules: "schedule",
		types.ActionCanvasCreate:  "canvas", types.ActionCanvasUpdate: "canvas",
		types.ActionCanvasProject: "canvas", types.ActionCanvasPreview: "canvas",
		types.ActionGenerateImage: "image_generation", types.ActionEditImage: "image_generation",
		types.ActionGenerateVideo: "video_generation",
	}

	for _, s := range schemas {
		groupName, ok := actionToGroup[s.ActionType]
		if !ok {
			continue
		}
		if g, exists := groupMap[groupName]; exists {
			g.Schemas = append(g.Schemas, s)
		}
	}

	// Only return groups that have at least one tool.
	var result []*ToolGroup
	for _, g := range groupMap {
		if len(g.Schemas) > 0 {
			result = append(result, g)
		}
	}
	return result
}
