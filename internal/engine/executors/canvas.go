package executors

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/openparallax/openparallax/internal/types"
)

// CanvasExecutor handles creative file generation and multi-file project
// scaffolding.
type CanvasExecutor struct {
	workspacePath string
}

// NewCanvasExecutor creates a canvas executor.
func NewCanvasExecutor(workspace string) *CanvasExecutor {
	return &CanvasExecutor{workspacePath: workspace}
}

// canvasTypeExt maps a canvas content type to its default file extension.
// canvasTypes is a set view used for validation. Both must list the same
// keys; the schema's enum is generated from canvasTypeList() so the LLM
// always sees the same list the executor enforces.
var canvasTypeExt = map[string]string{
	"html":       ".html",
	"svg":        ".svg",
	"markdown":   ".md",
	"mermaid":    ".mmd",
	"css":        ".css",
	"javascript": ".js",
	"json":       ".json",
	"yaml":       ".yaml",
}

var canvasTypes = func() map[string]bool {
	out := make(map[string]bool, len(canvasTypeExt))
	for k := range canvasTypeExt {
		out[k] = true
	}
	return out
}()

func canvasTypeList() string {
	keys := make([]string, 0, len(canvasTypeExt))
	for k := range canvasTypeExt {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}

// WorkspaceScope reports that canvas writes are confined to the workspace.
func (c *CanvasExecutor) WorkspaceScope() WorkspaceScope { return ScopeScoped }

func (c *CanvasExecutor) SupportedActions() []types.ActionType {
	return []types.ActionType{
		types.ActionCanvasCreate, types.ActionCanvasUpdate,
		types.ActionCanvasProject,
	}
}

func (c *CanvasExecutor) ToolSchemas() []ToolSchema {
	typeEnum := make([]string, 0, len(canvasTypeExt))
	for k := range canvasTypeExt {
		typeEnum = append(typeEnum, k)
	}
	sort.Strings(typeEnum)
	return []ToolSchema{
		{
			ActionType:  types.ActionCanvasCreate,
			Name:        "canvas_create",
			Description: "Create a file with typed content. Use for generating HTML pages, SVG graphics, Markdown documents, Mermaid diagrams, CSS stylesheets, JavaScript files, or data files.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path":    map[string]any{"type": "string", "description": "File path relative to workspace."},
					"content": map[string]any{"type": "string", "description": "The file content."},
					"type":    map[string]any{"type": "string", "description": "Content type.", "enum": typeEnum},
				},
				"required": []string{"path", "content", "type"},
			},
		},
		{
			ActionType:  types.ActionCanvasUpdate,
			Name:        "canvas_update",
			Description: "Update an existing canvas file with new content (full replacement).",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path":    map[string]any{"type": "string", "description": "File path relative to workspace."},
					"content": map[string]any{"type": "string", "description": "The new content."},
				},
				"required": []string{"path", "content"},
			},
		},
		{
			ActionType:  types.ActionCanvasProject,
			Name:        "canvas_project",
			Description: "Create a multi-file project in a directory. Each file has a name, content, and type. Subdirectories are created automatically.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path":        map[string]any{"type": "string", "description": "Directory path for the project."},
					"description": map[string]any{"type": "string", "description": "Brief description of the project."},
					"files": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"name":    map[string]any{"type": "string"},
								"content": map[string]any{"type": "string"},
								"type":    map[string]any{"type": "string"},
							},
						},
						"description": "Array of files to create in the project.",
					},
				},
				"required": []string{"path", "files"},
			},
		},
	}
}

func (c *CanvasExecutor) Execute(_ context.Context, action *types.ActionRequest) *types.ActionResult {
	switch action.Type {
	case types.ActionCanvasCreate:
		return c.createFile(action)
	case types.ActionCanvasUpdate:
		return c.updateFile(action)
	case types.ActionCanvasProject:
		return c.createProject(action)
	default:
		return ErrorResult(action.RequestID, "unknown canvas action", "unknown action")
	}
}

func (c *CanvasExecutor) createFile(action *types.ActionRequest) *types.ActionResult {
	rawPath, _ := action.Payload["path"].(string)
	contentType, _ := action.Payload["type"].(string)
	if contentType != "" && !canvasTypes[contentType] {
		return ErrorResult(action.RequestID, fmt.Sprintf("type %q is not supported (allowed: %s)", contentType, canvasTypeList()), "invalid canvas type")
	}
	if rawPath == "" {
		// LLM omitted path — generate a default from the content type.
		suffix := canvasTypeExt[contentType]
		if suffix == "" {
			suffix = ".txt"
		}
		rawPath = "canvas" + suffix
	}
	path, err := ResolveInWorkspace(rawPath, c.workspacePath)
	if err != nil {
		return ErrorResult(action.RequestID, err.Error(), "invalid canvas path")
	}
	content, _ := action.Payload["content"].(string)
	if contentType == "" {
		contentType = inferTypeFromExt(rawPath)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return ErrorResult(action.RequestID, err.Error(), "failed to create directory for "+filepath.Base(path))
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return ErrorResult(action.RequestID, err.Error(), "failed to write "+filepath.Base(path))
	}

	return &types.ActionResult{
		RequestID: action.RequestID, Success: true,
		Output:       fmt.Sprintf("Created %s (%s, %d bytes)", filepath.Base(path), contentType, len(content)),
		Summary:      fmt.Sprintf("created %s (%s)", filepath.Base(path), contentType),
		BytesWritten: int64(len(content)),
	}
}

func (c *CanvasExecutor) updateFile(action *types.ActionRequest) *types.ActionResult {
	rawPath, _ := action.Payload["path"].(string)
	if rawPath == "" {
		return ErrorResult(action.RequestID, "path is required", "canvas_update requires a file path")
	}
	path, err := ResolveInWorkspace(rawPath, c.workspacePath)
	if err != nil {
		return ErrorResult(action.RequestID, err.Error(), "invalid canvas path")
	}
	content, _ := action.Payload["content"].(string)

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return ErrorResult(action.RequestID, err.Error(), "failed to update "+filepath.Base(path))
	}

	return &types.ActionResult{
		RequestID: action.RequestID, Success: true,
		Output:       fmt.Sprintf("Updated %s (%d bytes)", filepath.Base(path), len(content)),
		Summary:      fmt.Sprintf("updated %s", filepath.Base(path)),
		BytesWritten: int64(len(content)),
	}
}

func (c *CanvasExecutor) createProject(action *types.ActionRequest) *types.ActionResult {
	rawPath, _ := action.Payload["path"].(string)
	if rawPath == "" {
		return ErrorResult(action.RequestID, "path is required", "canvas_project requires a directory path")
	}
	dir, err := ResolveInWorkspace(rawPath, c.workspacePath)
	if err != nil {
		return ErrorResult(action.RequestID, err.Error(), "invalid project path")
	}
	filesRaw, ok := action.Payload["files"].([]any)
	if !ok || len(filesRaw) == 0 {
		return ErrorResult(action.RequestID, "files array is required", "project creation failed")
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ErrorResult(action.RequestID, err.Error(), "failed to create project directory")
	}

	// Validate every entry up front so a hostile name (path traversal,
	// empty, missing) cannot leave the project half-written.
	type planned struct {
		fullPath string
		relName  string
		content  string
	}
	plan := make([]planned, 0, len(filesRaw))
	for i, f := range filesRaw {
		file, ok := f.(map[string]any)
		if !ok {
			return ErrorResult(action.RequestID, fmt.Sprintf("files[%d] is not an object", i), "invalid project entry")
		}
		name, _ := file["name"].(string)
		content, _ := file["content"].(string)
		if name == "" {
			return ErrorResult(action.RequestID, fmt.Sprintf("files[%d] is missing name", i), "invalid project entry")
		}
		fullPath, err := ResolveInWorkspace(filepath.Join(dir, name), c.workspacePath)
		if err != nil {
			return ErrorResult(action.RequestID, fmt.Sprintf("files[%d] (%q): %s", i, name, err.Error()), "invalid project entry")
		}
		// Make sure the file lands inside the project dir, not just inside
		// the workspace, so a name like "../sibling/leak" can't escape the
		// project root.
		rel, relErr := filepath.Rel(dir, fullPath)
		if relErr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return ErrorResult(action.RequestID, fmt.Sprintf("files[%d] (%q) escapes the project directory", i, name), "invalid project entry")
		}
		plan = append(plan, planned{fullPath: fullPath, relName: name, content: content})
	}

	created := make([]string, 0, len(plan))
	for _, p := range plan {
		if err := os.MkdirAll(filepath.Dir(p.fullPath), 0o755); err != nil {
			return ErrorResult(action.RequestID,
				fmt.Sprintf("create directory for %q failed after %d files: %s", p.relName, len(created), err.Error()),
				"project creation failed")
		}
		if err := os.WriteFile(p.fullPath, []byte(p.content), 0o644); err != nil {
			return ErrorResult(action.RequestID,
				fmt.Sprintf("write %q failed after %d files: %s", p.relName, len(created), err.Error()),
				"project creation failed")
		}
		created = append(created, p.relName)
	}

	return SuccessResult(action.RequestID,
		fmt.Sprintf("Created project with %d files:\n%s", len(created), strings.Join(created, "\n")),
		fmt.Sprintf("created project %s (%d files)", filepath.Base(dir), len(created)))
}

func inferTypeFromExt(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	for typ, defaultExt := range canvasTypeExt {
		if ext == defaultExt {
			return typ
		}
	}
	switch ext {
	case ".htm":
		return "html"
	case ".markdown":
		return "markdown"
	case ".yml":
		return "yaml"
	default:
		return ""
	}
}
