// Package mcp provides an MCP (Model Context Protocol) client for connecting
// to external tool servers using the mcp-go SDK. Each MCP server is lazily
// spawned on first tool call and automatically shut down after an idle timeout.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	mcpclient "github.com/mark3labs/mcp-go/client"
	mcptypes "github.com/mark3labs/mcp-go/mcp"

	"github.com/openparallax/openparallax/internal/llm"
	"github.com/openparallax/openparallax/internal/logging"
	"github.com/openparallax/openparallax/internal/types"
)

// Manager manages all configured MCP server connections.
type Manager struct {
	clients map[string]*Client
	log     *logging.Logger
}

// NewManager creates an MCP Manager from config.
func NewManager(configs []types.MCPServerConfig, log *logging.Logger) *Manager {
	clients := make(map[string]*Client)
	for _, cfg := range configs {
		timeout := time.Duration(cfg.IdleTimeout) * time.Second
		if timeout == 0 {
			timeout = 5 * time.Minute
		}
		clients[cfg.Name] = &Client{
			config: cfg,
			idle:   timeout,
			log:    log,
		}
	}
	return &Manager{clients: clients, log: log}
}

// DiscoverTools returns tool definitions from all configured MCP servers.
func (m *Manager) DiscoverTools(ctx context.Context) []llm.ToolDefinition {
	var tools []llm.ToolDefinition
	for _, client := range m.clients {
		clientTools, err := client.DiscoverTools(ctx)
		if err != nil {
			if m.log != nil {
				m.log.Warn("mcp_discovery_failed", "server", client.config.Name, "error", err)
			}
			continue
		}
		for _, tool := range clientTools {
			tool.Name = fmt.Sprintf("mcp:%s:%s", client.config.Name, tool.Name)
			tools = append(tools, tool)
		}
	}
	return tools
}

// Route finds the correct MCP client for a namespaced tool name.
func (m *Manager) Route(toolName string) (*Client, string, bool) {
	parts := strings.SplitN(toolName, ":", 3)
	if len(parts) != 3 || parts[0] != "mcp" {
		return nil, "", false
	}
	client, ok := m.clients[parts[1]]
	return client, parts[2], ok
}

// ServerStatus returns the status of each configured MCP server.
func (m *Manager) ServerStatus() []map[string]any {
	result := make([]map[string]any, 0, len(m.clients))
	for name, c := range m.clients {
		c.mu.Lock()
		status := "idle"
		toolCount := len(c.tools)
		if c.running {
			status = "running"
		}
		c.mu.Unlock()
		result = append(result, map[string]any{
			"name":        name,
			"command":     c.config.Command,
			"status":      status,
			"tools_count": toolCount,
		})
	}
	return result
}

// ShutdownAll cleanly stops all running MCP servers.
func (m *Manager) ShutdownAll() {
	for _, client := range m.clients {
		client.Shutdown()
	}
}

// Client manages a single MCP server process via the mcp-go SDK.
type Client struct {
	config   types.MCPServerConfig
	idle     time.Duration
	log      *logging.Logger
	conn     *mcpclient.Client
	tools    []llm.ToolDefinition
	lastCall time.Time
	mu       sync.Mutex
	running  bool
}

// DiscoverTools starts the server if needed and returns available tools.
func (c *Client) DiscoverTools(ctx context.Context) ([]llm.ToolDefinition, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.ensureRunningLocked(ctx); err != nil {
		return nil, err
	}
	return c.tools, nil
}

// CallTool forwards a tool call to the MCP server.
func (c *Client) CallTool(ctx context.Context, name string, args map[string]any) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.ensureRunningLocked(ctx); err != nil {
		return "", err
	}
	c.lastCall = time.Now()

	result, err := c.conn.CallTool(ctx, mcptypes.CallToolRequest{
		Params: mcptypes.CallToolParams{
			Name:      name,
			Arguments: args,
		},
	})
	if err != nil {
		return "", fmt.Errorf("mcp call %s/%s: %w", c.config.Name, name, err)
	}

	if result.IsError {
		return extractResultText(result), fmt.Errorf("tool error: %s", extractResultText(result))
	}

	return extractResultText(result), nil
}

// Shutdown stops the MCP server process.
func (c *Client) Shutdown() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.shutdownLocked()
}

func (c *Client) ensureRunningLocked(ctx context.Context) error {
	if c.running {
		c.lastCall = time.Now()
		return nil
	}

	// Build env vars with expansion.
	var env []string
	for k, v := range c.config.Env {
		env = append(env, k+"="+os.ExpandEnv(v))
	}

	conn, err := mcpclient.NewStdioMCPClient(c.config.Command, env, c.config.Args...)
	if err != nil {
		return fmt.Errorf("mcp start %s: %w", c.config.Name, err)
	}

	// Initialize protocol.
	initReq := mcptypes.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcptypes.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcptypes.Implementation{
		Name:    "openparallax",
		Version: "1.0.0",
	}
	if _, initErr := conn.Initialize(ctx, initReq); initErr != nil {
		_ = conn.Close()
		return fmt.Errorf("mcp init %s: %w", c.config.Name, initErr)
	}

	// Discover tools.
	toolsResult, err := conn.ListTools(ctx, mcptypes.ListToolsRequest{})
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("mcp list tools %s: %w", c.config.Name, err)
	}

	c.conn = conn
	c.tools = convertMCPTools(toolsResult.Tools)
	c.running = true
	c.lastCall = time.Now()

	go c.idleShutdownLoop()

	if c.log != nil {
		c.log.Info("mcp_started", "server", c.config.Name, "tools", len(c.tools))
	}
	return nil
}

func (c *Client) shutdownLocked() {
	if !c.running {
		return
	}
	c.running = false
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}
	if c.log != nil {
		c.log.Info("mcp_stopped", "server", c.config.Name)
	}
}

func (c *Client) idleShutdownLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		c.mu.Lock()
		if c.running && time.Since(c.lastCall) > c.idle {
			c.shutdownLocked()
			c.mu.Unlock()
			return
		}
		if !c.running {
			c.mu.Unlock()
			return
		}
		c.mu.Unlock()
	}
}

func convertMCPTools(tools []mcptypes.Tool) []llm.ToolDefinition {
	var defs []llm.ToolDefinition
	for _, t := range tools {
		params := make(map[string]any)
		if t.RawInputSchema != nil {
			_ = json.Unmarshal(t.RawInputSchema, &params)
		} else {
			params["type"] = "object"
			props := make(map[string]any)
			for name, prop := range t.InputSchema.Properties {
				props[name] = prop
			}
			params["properties"] = props
			if len(t.InputSchema.Required) > 0 {
				params["required"] = t.InputSchema.Required
			}
		}
		defs = append(defs, llm.ToolDefinition{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  params,
		})
	}
	return defs
}

// TestServer spawns an MCP server briefly, discovers its tools, shuts it down,
// and returns the tool names. Used by the settings UI to validate MCP config.
func TestServer(ctx context.Context, cfg types.MCPServerConfig, log *logging.Logger) ([]string, error) {
	c := &Client{
		config: cfg,
		idle:   30 * time.Second,
		log:    log,
	}

	tools, err := c.DiscoverTools(ctx)
	defer c.Shutdown()
	if err != nil {
		return nil, err
	}

	names := make([]string, len(tools))
	for i, t := range tools {
		names[i] = t.Name
	}
	return names, nil
}

func extractResultText(result *mcptypes.CallToolResult) string {
	if result == nil {
		return ""
	}
	var texts []string
	for _, c := range result.Content {
		if tc, ok := c.(mcptypes.TextContent); ok {
			texts = append(texts, tc.Text)
		}
	}
	if len(texts) == 0 {
		return ""
	}
	return strings.Join(texts, "\n")
}
