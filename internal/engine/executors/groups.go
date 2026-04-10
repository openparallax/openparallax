package executors

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/openparallax/openparallax/internal/types"
	"github.com/openparallax/openparallax/llm"
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
	groups   map[string]*ToolGroup
	disabled map[string]bool
	mu       sync.RWMutex
}

// NewGroupRegistry creates an empty group registry.
func NewGroupRegistry() *GroupRegistry {
	return &GroupRegistry{
		groups:   make(map[string]*ToolGroup),
		disabled: make(map[string]bool),
	}
}

// DisableGroups marks the given group names as unavailable to the LLM.
func (r *GroupRegistry) DisableGroups(names []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, n := range names {
		r.disabled[n] = true
	}
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

// RegisterMCPTools registers MCP server tools as loadable groups.
// Each server becomes a group named "mcp:<server>" so the LLM can call
// load_tools(["mcp:filesystem"]) to discover MCP tools.
func (r *GroupRegistry) RegisterMCPTools(serverTools map[string][]llm.ToolDefinition) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for serverName, tools := range serverTools {
		groupName := "mcp:" + serverName
		schemas := make([]ToolSchema, len(tools))
		for i, t := range tools {
			schemas[i] = ToolSchema{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			}
		}
		r.groups[groupName] = &ToolGroup{
			Name:        groupName,
			Description: fmt.Sprintf("Tools from MCP server %q", serverName),
			Schemas:     schemas,
		}
	}
}

// LoadToolsDefinition generates the load_tools meta-tool definition
// with every currently-registered group enumerated in the description.
// Groups whose underlying executors are not registered (e.g. browser
// when no Chromium binary is detected, email/calendar when not
// configured, image/video generation when no model is bound) are
// absent from r.groups and so do not appear here. Groups in
// r.disabled (driven by tools.disabled_groups in config.yaml) are
// filtered out.
//
// Group names are sorted alphabetically so the description is stable
// across sessions and across processes — this matters for prompt
// caching and for any test that compares the rendered text.
//
// This function is the single source of truth for what the agent's
// load_tools menu looks like. The engine calls it once per session
// when the agent connects (see InitialToolDefs in the gRPC
// pipeline) and ships the result over the wire. The agent never
// constructs a load_tools definition of its own.
func (r *GroupRegistry) LoadToolsDefinition() llm.ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.groups))
	for name := range r.groups {
		if r.disabled[name] {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)

	lines := make([]string, 0, len(names))
	for _, name := range names {
		lines = append(lines, fmt.Sprintf("- %s: %s", name, r.groups[name].Description))
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
					"description": "List of tool group names to load. Pick from the Available groups list above.",
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
		if r.disabled[name] {
			errors = append(errors, fmt.Sprintf("Group %q is disabled", name))
			continue
		}
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
	"create_agent": true,
	"email_move":   true, "email_mark": true,
	"clipboard_write":   true,
	"archive_create":    true,
	"spreadsheet_write": true,
}

func filterOTRTools(tools []llm.ToolDefinition) []llm.ToolDefinition {
	filtered := make([]llm.ToolDefinition, 0, len(tools))
	for _, t := range tools {
		// Block all MCP tools in OTR — external tools cannot be reliably
		// classified as read-only, so we exclude them entirely.
		if strings.HasPrefix(t.Name, "mcp:") {
			continue
		}
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
		"shell":            {Name: "shell", Description: "Run commands on the system. Use only for tasks that other tool groups cannot handle."},
		"git":              {Name: "git", Description: "Git version control — status, diff, log, commit, push, pull, branch, clone"},
		"browser":          {Name: "browser", Description: "Browse the web — navigate, click, type, extract, screenshot"},
		"email":            {Name: "email", Description: "Send and read emails — list inbox, search, read, move, mark, and send"},
		"calendar":         {Name: "calendar", Description: "Manage the user's calendar events — list, create, update, delete"},
		"memory":           {Name: "memory", Description: "Write structured memories and search past conversations"},
		"schedule":         {Name: "schedule", Description: "Manage recurring tasks via HEARTBEAT.md cron entries"},
		"canvas":           {Name: "canvas", Description: "Create files and multi-file projects"},
		"image_generation": {Name: "image_generation", Description: "Generate images using AI if supported by model"},
		"video_generation": {Name: "video_generation", Description: "Generate videos using AI if supported by model"},
		"agents":           {Name: "agents", Description: "Sub-agents — default for 2+ independent subtasks. Run them in parallel for speed, cost, and clean context. Each gets its own window."},
		"system":           {Name: "system", Description: "Clipboard access, launch files/URLs, OS notifications, system info, screenshots"},
		"utilities":        {Name: "utilities", Description: "Archive zip/extract, PDF text extraction, spreadsheet read/write"},
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
		types.ActionBrowserShot: "browser",
		types.ActionSendEmail:   "email", types.ActionEmailList: "email",
		types.ActionEmailRead: "email", types.ActionEmailSearch: "email",
		types.ActionEmailMove: "email", types.ActionEmailMark: "email",
		types.ActionReadCalendar: "calendar", types.ActionCreateEvent: "calendar",
		types.ActionUpdateEvent: "calendar", types.ActionDeleteEvent: "calendar",
		types.ActionMemoryWrite: "memory", types.ActionMemorySearch: "memory",
		types.ActionCreateSchedule: "schedule", types.ActionDeleteSchedule: "schedule",
		types.ActionListSchedules: "schedule",
		types.ActionCanvasCreate:  "canvas", types.ActionCanvasUpdate: "canvas",
		types.ActionCanvasProject: "canvas",
		types.ActionGenerateImage: "image_generation", types.ActionEditImage: "image_generation",
		types.ActionGenerateVideo: "video_generation",
		types.ActionCreateAgent:   "agents", types.ActionAgentStatus: "agents",
		types.ActionAgentResult: "agents", types.ActionAgentMessage: "agents",
		types.ActionDeleteAgent: "agents", types.ActionListAgents: "agents",
		types.ActionGrepFiles:     "files",
		types.ActionClipboardRead: "system", types.ActionClipboardWrite: "system",
		types.ActionOpen: "system", types.ActionNotify: "system",
		types.ActionSystemInfo: "system", types.ActionScreenshot: "system",
		types.ActionArchiveCreate: "utilities", types.ActionArchiveExtract: "utilities",
		types.ActionPDFRead:         "utilities",
		types.ActionSpreadsheetRead: "utilities", types.ActionSpreadsheetWrite: "utilities",
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
