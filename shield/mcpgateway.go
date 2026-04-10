package shield

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	mcptypes "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/openparallax/openparallax/llm"
	"github.com/openparallax/openparallax/mcp"
)

// MCPGatewayConfig configures the MCP security gateway.
type MCPGatewayConfig struct {
	Pipeline    *Pipeline
	MCPManager  *mcp.Manager
	ToolMapping map[string]ActionType
	AuditFunc   func(entry MCPAuditEntry)
	Log         Logger
}

// MCPAuditEntry holds the details of a single MCP tool call evaluation.
type MCPAuditEntry struct {
	Timestamp  time.Time `json:"timestamp"`
	Tool       string    `json:"tool"`
	Server     string    `json:"server"`
	ActionType string    `json:"action_type"`
	Payload    any       `json:"payload"`
	Verdict    string    `json:"verdict"`
	Tier       int       `json:"tier"`
	Confidence float64   `json:"confidence"`
	Reasoning  string    `json:"reasoning"`
}

// MCPGateway is an MCP server that proxies tool calls through Shield.
type MCPGateway struct {
	pipeline    *Pipeline
	mcpManager  *mcp.Manager
	toolMapping map[string]ActionType
	auditFunc   func(entry MCPAuditEntry)
	log         Logger
	server      *mcpserver.MCPServer
}

// NewMCPGateway creates an MCP gateway that evaluates all tool calls through Shield.
func NewMCPGateway(cfg MCPGatewayConfig) *MCPGateway {
	log := cfg.Log
	if log == nil {
		log = nopLogger{}
	}
	mapping := cfg.ToolMapping
	if mapping == nil {
		mapping = make(map[string]ActionType)
	}
	return &MCPGateway{
		pipeline:    cfg.Pipeline,
		mcpManager:  cfg.MCPManager,
		toolMapping: mapping,
		auditFunc:   cfg.AuditFunc,
		log:         log,
	}
}

// DiscoverAndServe discovers upstream tools and returns a configured MCPServer.
func (g *MCPGateway) DiscoverAndServe(ctx context.Context) (*mcpserver.MCPServer, error) {
	tools := g.mcpManager.DiscoverTools(ctx)
	if len(tools) == 0 {
		g.log.Warn("mcp_gateway_no_tools", "message", "no tools discovered from upstream servers")
	}

	g.server = mcpserver.NewMCPServer(
		"openparallax-shield",
		"1.0.0",
	)

	for _, tool := range tools {
		toolName := tool.Name
		g.server.AddTool(g.convertTool(tool), g.makeHandler(toolName))
	}

	g.log.Info("mcp_gateway_ready", "tools", len(tools))
	return g.server, nil
}

// Shutdown stops all upstream MCP servers.
func (g *MCPGateway) Shutdown() {
	g.mcpManager.ShutdownAll()
}

func (g *MCPGateway) convertTool(tool llm.ToolDefinition) mcptypes.Tool {
	schema, _ := json.Marshal(tool.Parameters)
	return mcptypes.NewToolWithRawSchema(tool.Name, tool.Description, schema)
}

func (g *MCPGateway) makeHandler(toolName string) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcptypes.CallToolRequest) (*mcptypes.CallToolResult, error) {
		actionType := g.resolveActionType(toolName)

		payload := make(map[string]any)
		for k, v := range request.GetArguments() {
			payload[k] = v
		}

		action := &ActionRequest{
			Type:      actionType,
			Payload:   payload,
			Timestamp: time.Now(),
		}

		verdict := g.pipeline.Evaluate(ctx, action)

		server := serverFromToolName(toolName)
		g.emitAudit(MCPAuditEntry{
			Timestamp:  time.Now(),
			Tool:       originalToolName(toolName),
			Server:     server,
			ActionType: string(actionType),
			Payload:    payload,
			Verdict:    string(verdict.Decision),
			Tier:       verdict.Tier,
			Confidence: verdict.Confidence,
			Reasoning:  verdict.Reasoning,
		})

		if verdict.Decision == VerdictBlock {
			g.log.Info("mcp_gateway_blocked", "tool", toolName, "tier", verdict.Tier, "reason", verdict.Reasoning)
			return mcptypes.NewToolResultError(
				fmt.Sprintf("Action blocked by security policy: %s", verdict.Reasoning),
			), nil
		}

		client, originalName, ok := g.mcpManager.Route(toolName)
		if !ok {
			return mcptypes.NewToolResultError(
				fmt.Sprintf("no upstream server for tool: %s", toolName),
			), nil
		}

		result, err := client.CallTool(ctx, originalName, request.GetArguments())
		if err != nil {
			return mcptypes.NewToolResultError(err.Error()), nil
		}

		return mcptypes.NewToolResultText(result), nil
	}
}

// resolveActionType maps an MCP tool name to a Shield action type.
func (g *MCPGateway) resolveActionType(toolName string) ActionType {
	if mapped, ok := g.toolMapping[toolName]; ok {
		return mapped
	}

	original := originalToolName(toolName)

	if mapped, ok := g.toolMapping[original]; ok {
		return mapped
	}

	return inferActionType(original)
}

// inferActionType maps common MCP tool names to Shield action types.
func inferActionType(name string) ActionType {
	lower := strings.ToLower(name)
	switch {
	case containsAny(lower, "read_file", "get_file_contents", "cat"):
		return ActionReadFile
	case containsAny(lower, "write_file", "create_file", "put_file"):
		return ActionWriteFile
	case containsAny(lower, "delete_file", "remove_file", "rm"):
		return ActionDeleteFile
	case containsAny(lower, "move_file", "rename_file", "mv"):
		return ActionMoveFile
	case containsAny(lower, "copy_file", "cp"):
		return ActionCopyFile
	case containsAny(lower, "create_directory", "mkdir"):
		return ActionCreateDir
	case containsAny(lower, "list_directory", "ls", "list_dir"):
		return ActionListDir
	case containsAny(lower, "search", "grep", "find", "glob"):
		return ActionSearchFiles
	case containsAny(lower, "run_command", "execute", "bash", "shell", "exec"):
		return ActionExecCommand
	case containsAny(lower, "send_message", "post_message"):
		return ActionSendMessage
	case containsAny(lower, "send_email"):
		return ActionSendEmail
	case containsAny(lower, "http", "fetch", "request", "curl", "api"):
		return ActionHTTPRequest
	default:
		return ActionType(lower)
	}
}

func containsAny(s string, patterns ...string) bool {
	for _, p := range patterns {
		if s == p || strings.Contains(s, p) {
			return true
		}
	}
	return false
}

// serverFromToolName extracts the server name from "mcp:<server>:<tool>".
func serverFromToolName(toolName string) string {
	parts := strings.SplitN(toolName, ":", 3)
	if len(parts) >= 2 && parts[0] == "mcp" {
		return parts[1]
	}
	return ""
}

// originalToolName extracts the original tool name from "mcp:<server>:<tool>".
func originalToolName(toolName string) string {
	parts := strings.SplitN(toolName, ":", 3)
	if len(parts) == 3 && parts[0] == "mcp" {
		return parts[2]
	}
	return toolName
}

func (g *MCPGateway) emitAudit(entry MCPAuditEntry) {
	if g.auditFunc != nil {
		g.auditFunc(entry)
	}
}
