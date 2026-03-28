// Package mcp provides an MCP (Model Context Protocol) client for connecting
// to external tool servers. Each MCP server is lazily spawned on first tool
// call and automatically shut down after an idle timeout.
package mcp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

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
// Spawns each server briefly to discover tools.
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
// Returns the client, the original tool name, and whether it matched.
func (m *Manager) Route(toolName string) (*Client, string, bool) {
	parts := strings.SplitN(toolName, ":", 3)
	if len(parts) != 3 || parts[0] != "mcp" {
		return nil, "", false
	}
	client, ok := m.clients[parts[1]]
	return client, parts[2], ok
}

// ShutdownAll cleanly stops all running MCP servers.
func (m *Manager) ShutdownAll() {
	for _, client := range m.clients {
		client.Shutdown()
	}
}

// Client manages a single MCP server process.
type Client struct {
	config   types.MCPServerConfig
	idle     time.Duration
	log      *logging.Logger
	process  *exec.Cmd
	stdin    *stdinWriter
	stdout   *stdoutReader
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

	result, err := c.callToolLocked(ctx, name, args)
	if err != nil {
		return "", err
	}
	return result, nil
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

	// Resolve environment variables.
	env := os.Environ()
	for k, v := range c.config.Env {
		expanded := os.ExpandEnv(v)
		env = append(env, k+"="+expanded)
	}

	c.process = exec.CommandContext(ctx, c.config.Command, c.config.Args...)
	c.process.Env = env

	// Set up stdio pipes for JSON-RPC communication.
	stdinPipe, err := c.process.StdinPipe()
	if err != nil {
		return fmt.Errorf("mcp stdin pipe: %w", err)
	}
	stdoutPipe, err := c.process.StdoutPipe()
	if err != nil {
		return fmt.Errorf("mcp stdout pipe: %w", err)
	}

	if err := c.process.Start(); err != nil {
		return fmt.Errorf("mcp start %s: %w", c.config.Name, err)
	}

	c.stdin = newStdinWriter(stdinPipe)
	c.stdout = newStdoutReader(stdoutPipe)
	c.running = true
	c.lastCall = time.Now()

	// Initialize MCP protocol.
	tools, err := c.initializeLocked(ctx)
	if err != nil {
		c.shutdownLocked()
		return fmt.Errorf("mcp init %s: %w", c.config.Name, err)
	}
	c.tools = tools

	// Start idle timeout goroutine.
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
	if c.process != nil && c.process.Process != nil {
		_ = c.process.Process.Kill()
		_ = c.process.Wait()
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
