# MCP Integration

OpenParallax integrates with external MCP (Model Context Protocol) servers to extend the agent's tool capabilities. Any MCP-compatible tool server can be connected, and its tools are discovered automatically, namespaced to avoid collisions, and routed through the full Shield security pipeline. MCP tools receive no special treatment -- they are evaluated, audited, and executed with the same security guarantees as built-in tools.

## What MCP Is

The [Model Context Protocol](https://modelcontextprotocol.io) is an open standard for connecting AI agents to external tool providers. An MCP server exposes tools (functions the agent can call), resources (data the agent can read), and prompts (templates the agent can use). The protocol uses JSON-RPC for communication between client and server.

OpenParallax implements the MCP **client** side, connecting to external MCP servers to access their tools. This means you can connect any MCP-compatible tool server -- whether from the official MCP registry, a third-party provider, or a custom server you build -- and the agent can use its tools immediately.

Examples of MCP servers:

- **GitHub**: Create issues, list PRs, manage repositories
- **Filesystem**: Read/write files with path restrictions
- **PostgreSQL**: Query and modify databases
- **Slack**: Send messages, read channels
- **Brave Search**: Web search
- **Puppeteer**: Browser automation
- Custom servers you build in Python, Node.js, Go, or any language

## How OpenParallax Uses MCP

### Lifecycle

```
Startup                          First tool call                  Idle timeout
   │                                   │                              │
   ▼                                   ▼                              ▼
Config loaded                   Server started                   Server stopped
MCP servers registered          Protocol initialized              (auto-shutdown)
(not started yet)               Tools discovered
                                Tools available to LLM
```

1. **Configuration**: MCP servers are declared in `config.yaml` under `mcp.servers`
2. **Lazy start**: Servers are not started at engine startup. They remain dormant until their tools are needed.
3. **Discovery**: On first use, the Manager starts the server process, sends an `initialize` request with the protocol version, and calls `tools/list` to discover available tools.
4. **Namespacing**: Discovered tools are prefixed with `mcp:<server-name>:` to avoid name collisions with built-in tools or other MCP servers. For example, a `create_issue` tool from a server named `github` becomes `mcp:github:create_issue`.
5. **Execution**: When the LLM calls a tool matching `mcp:*`, the Manager routes the call to the correct server using the namespace prefix.
6. **Idle shutdown**: Servers that have not received a tool call within the configured idle timeout are automatically stopped. They restart on the next tool call.

### Tool Discovery

When the pipeline needs tool definitions, it calls `Manager.DiscoverTools()`. For each configured server:

```
DiscoverTools()
    │
    ▼
For each configured server:
    │
    ├── Is server running? ─── YES ──→ Return cached tools
    │
    └── NO ──→ Start server process (stdio)
                  │
                  ▼
              Send initialize request
              (protocol version, client info)
                  │
                  ▼
              Call tools/list
                  │
                  ▼
              Convert MCP tools to ToolDefinition format
                  │
                  ▼
              Prefix each tool name: mcp:<server>:<tool>
                  │
                  ▼
              Cache tools, start idle shutdown timer
```

If a server fails to start or discover tools, it is skipped with a warning log. The remaining servers are still queried. This means a misconfigured server does not prevent the agent from using other MCP servers or built-in tools.

### Tool Execution

When the LLM calls a tool like `mcp:github:create_issue`:

```
LLM proposes: mcp:github:create_issue
    │
    ▼
Manager.Route("mcp:github:create_issue")
    → client: github client
    → originalName: "create_issue"
    │
    ▼
Shield evaluates the action
(same pipeline as built-in tools)
    │
    ▼
Client.CallTool(ctx, "create_issue", args)
    → JSON-RPC over stdio to the MCP server
    │
    ▼
Extract result text from response
    │
    ▼
Return result to pipeline → LLM sees the result
```

### Shield Integration

MCP tool calls pass through the same Shield security pipeline as built-in tools. There is no security bypass for MCP tools.

When the LLM proposes calling an MCP tool:

1. **Tier 0 (Policy)**: The action type is the MCP tool name (e.g., `mcp:github:create_issue`). You can write policy rules that match MCP tool names.
2. **Tier 1 (Classifier)**: The heuristic engine and ONNX classifier evaluate the tool call payload for prompt injection patterns.
3. **Tier 2 (Evaluator)**: If escalated, the LLM evaluator assesses whether the tool call is safe in context.
4. **IFC**: Data flow checks apply. If the agent read sensitive data, IFC prevents it from flowing to an MCP tool that sends data externally.
5. **Audit**: The tool call, Shield verdict, and result are all recorded in the audit log with hash chain integrity.

This means adding an MCP server does not weaken security. The same rules, classifiers, and evaluators that protect built-in tools protect MCP tools.

## Configuration

MCP servers are configured in `config.yaml` under the `mcp` section:

```yaml
mcp:
  servers:
    - name: github
      command: npx
      args: ["-y", "@modelcontextprotocol/server-github"]
      env:
        GITHUB_PERSONAL_ACCESS_TOKEN: "${GITHUB_TOKEN}"
      idle_timeout: 300

    - name: filesystem
      command: npx
      args: ["-y", "@modelcontextprotocol/server-filesystem", "/workspace"]
      idle_timeout: 600

    - name: postgres
      command: npx
      args: ["-y", "@modelcontextprotocol/server-postgres"]
      env:
        POSTGRES_CONNECTION_STRING: "${DATABASE_URL}"

    - name: brave-search
      command: npx
      args: ["-y", "@modelcontextprotocol/server-brave-search"]
      env:
        BRAVE_API_KEY: "${BRAVE_API_KEY}"
```

### Server Config Fields

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `name` | string | yes | -- | Unique identifier for the server. Used in tool name prefix (`mcp:<name>:<tool>`). |
| `command` | string | yes | -- | The command to run (e.g., `npx`, `python`, `node`, `/usr/local/bin/my-server`). |
| `args` | string[] | no | -- | Command-line arguments passed to the command. |
| `env` | map[string]string | no | -- | Environment variables for the server process. Supports `${VAR}` expansion from the host environment. |
| `idle_timeout` | int | no | 300 | Seconds of inactivity before auto-shutdown. Set to 0 to disable idle shutdown. |

### Environment Variable Interpolation

Environment variables in the `env` section support `${VAR_NAME}` expansion. Values are expanded from the host environment at server start time using `os.ExpandEnv`:

```yaml
env:
  GITHUB_PERSONAL_ACCESS_TOKEN: "${GITHUB_TOKEN}"
  DATABASE_URL: "${DATABASE_URL}"
  CUSTOM_CONFIG: "/fixed/path/config.json"
```

This prevents storing secrets directly in `config.yaml`. Store secrets in your shell profile or a secrets manager, and reference them via environment variables.

## Transport Types

### stdio (Current)

OpenParallax uses the `stdio` transport for MCP communication. The MCP server runs as a child process, and the client communicates with it via JSON-RPC over stdin/stdout. This is the simplest and most portable transport option -- any process that reads from stdin and writes to stdout can be an MCP server.

```
Engine process ←─ stdio (JSON-RPC) ─→ MCP server process
```

The server process is managed by the MCP client:
- Started on first tool call
- Monitored for idle shutdown
- Killed on engine shutdown

### streamable-http (Standalone Shield)

The standalone Shield binary (`openparallax-shield`) supports `streamable-http` transport for connecting to remote MCP servers over HTTP. This transport connects to a server running on a remote host:

```yaml
# In shield.yaml (standalone Shield only)
mcp:
  servers:
    - name: remote
      transport: streamable-http
      url: https://mcp-server.example.com
      headers:
        Authorization: "Bearer ${API_TOKEN}"
```

This transport is used when the MCP server runs as a hosted service rather than a local process.

## Adding an MCP Server

### Step 1: Find or Build a Server

Browse available servers at [modelcontextprotocol.io/servers](https://modelcontextprotocol.io/servers). Most official servers can be run via `npx`:

```yaml
mcp:
  servers:
    - name: github
      command: npx
      args: ["-y", "@modelcontextprotocol/server-github"]
      env:
        GITHUB_PERSONAL_ACCESS_TOKEN: "${GITHUB_TOKEN}"
```

### Step 2: Set Environment Variables

Make sure the required environment variables are set in your shell profile:

```bash
export GITHUB_TOKEN="ghp_your_token_here"
```

### Step 3: Test the Server

Use the web UI settings panel to test the server, or use the `TestServer` API:

```go
names, err := mcp.TestServer(ctx, types.MCPServerConfig{
    Name:    "github",
    Command: "npx",
    Args:    []string{"-y", "@modelcontextprotocol/server-github"},
    Env:     map[string]string{"GITHUB_PERSONAL_ACCESS_TOKEN": token},
}, log)
// names: ["create_issue", "list_issues", "get_issue", "create_pull_request", ...]
```

`TestServer` spawns the server, discovers its tools, shuts it down, and returns the tool names. If it succeeds, the server is configured correctly.

### Step 4: Add Shield Policy Rules (Optional)

By default, MCP tool calls fall through to Tier 1 evaluation. You can add specific policy rules:

```yaml
# In your Shield policy file
verify:
  - name: evaluate_github_writes
    action_types:
      - "mcp:github:create_issue"
      - "mcp:github:create_pull_request"
      - "mcp:github:merge_pull_request"
    tier_override: 2  # Send write operations to LLM evaluator

allow:
  - name: allow_github_reads
    action_types:
      - "mcp:github:list_issues"
      - "mcp:github:get_issue"
      - "mcp:github:list_pull_requests"
```

### Step 5: Restart the Engine

```bash
openparallax restart
# Or from the web UI: /restart
```

The new MCP server tools will be available in the next conversation turn.

### Custom Servers

Any process that speaks MCP over stdio can be used as a server. You can write servers in any language:

**Python:**
```yaml
mcp:
  servers:
    - name: my-tools
      command: python
      args: ["/path/to/my_mcp_server.py"]
      env:
        MY_API_KEY: "${MY_API_KEY}"
```

**Node.js:**
```yaml
mcp:
  servers:
    - name: my-tools
      command: node
      args: ["/path/to/my_mcp_server.js"]
```

**Go binary:**
```yaml
mcp:
  servers:
    - name: my-tools
      command: /usr/local/bin/my-mcp-server
      args: ["--config", "/etc/my-server.yaml"]
```

## Idle Shutdown

Each MCP client runs a background goroutine that checks idle time every 30 seconds. If the time since the last tool call exceeds the configured `idle_timeout`, the server process is automatically stopped.

```
Tool call received → lastCall = now
         │
    30 seconds later...
         │
Check: time.Since(lastCall) > idle_timeout?
    │
   NO  → continue checking every 30 seconds
    │
   YES → shutdown the server process
         (will restart on next tool call)
```

Default idle timeout: 300 seconds (5 minutes). Set `idle_timeout: 0` to disable idle shutdown.

This prevents long-running server processes from consuming resources when not in use. MCP servers that require significant startup time (e.g., loading large models) should use a longer idle timeout.

## Go API

Package: `github.com/openparallax/openparallax/internal/mcp`

### NewManager

```go
func NewManager(configs []types.MCPServerConfig, log *logging.Logger) *Manager
```

Creates an MCP Manager from configuration. Does not start any servers -- they are started lazily on first use.

```go
manager := mcp.NewManager(cfg.MCP.Servers, log)
defer manager.ShutdownAll()
```

### DiscoverTools

```go
func (m *Manager) DiscoverTools(ctx context.Context) []llm.ToolDefinition
```

Returns tool definitions from all configured MCP servers. Starts servers that are not already running. Tool names are prefixed with `mcp:<server>:`.

```go
tools := manager.DiscoverTools(ctx)
for _, t := range tools {
    fmt.Printf("%s: %s\n", t.Name, t.Description)
}
// mcp:github:create_issue: Create a new issue in a GitHub repository
// mcp:github:list_issues: List issues in a GitHub repository
// mcp:filesystem:read_file: Read a file from the filesystem
```

### Route

```go
func (m *Manager) Route(toolName string) (*Client, string, bool)
```

Finds the correct MCP client for a namespaced tool name. Parses the `mcp:<server>:<tool>` format and returns the client, the original tool name (without prefix), and whether the route was found.

```go
client, originalName, ok := manager.Route("mcp:github:create_issue")
// client: github client
// originalName: "create_issue"
// ok: true

_, _, ok = manager.Route("read_file")
// ok: false (not an MCP tool, no "mcp:" prefix)
```

### Client.CallTool

```go
func (c *Client) CallTool(ctx context.Context, name string, args map[string]any) (string, error)
```

Forwards a tool call to the MCP server. Starts the server if not already running. Returns the tool result as a string.

```go
result, err := client.CallTool(ctx, "create_issue", map[string]any{
    "repo":  "openparallax/openparallax",
    "title": "Bug report",
    "body":  "Description of the bug",
})
```

### ServerStatus

```go
func (m *Manager) ServerStatus() []map[string]any
```

Returns the status of each configured MCP server. Used by `GET /api/status` and `openparallax status`.

```go
statuses := manager.ServerStatus()
// [
//   {"name": "github", "command": "npx", "status": "running", "tools_count": 5},
//   {"name": "filesystem", "command": "npx", "status": "idle", "tools_count": 0}
// ]
```

### ShutdownAll

```go
func (m *Manager) ShutdownAll()
```

Cleanly stops all running MCP server processes. Called during engine shutdown.

### TestServer

```go
func TestServer(ctx context.Context, cfg types.MCPServerConfig, log *logging.Logger) ([]string, error)
```

Spawns an MCP server briefly, discovers its tools, shuts it down, and returns the tool names. Used by the web settings UI to validate MCP configuration before saving.

```go
names, err := mcp.TestServer(ctx, types.MCPServerConfig{
    Name:    "test",
    Command: "npx",
    Args:    []string{"-y", "@modelcontextprotocol/server-github"},
    Env:     map[string]string{"GITHUB_PERSONAL_ACCESS_TOKEN": token},
}, log)
// names: ["create_issue", "list_issues", "get_issue", ...]
```

## MCPServerConfig

```go
type MCPServerConfig struct {
    Name        string            `yaml:"name"`
    Command     string            `yaml:"command"`
    Args        []string          `yaml:"args,omitempty"`
    Env         map[string]string `yaml:"env,omitempty"`
    IdleTimeout int               `yaml:"idle_timeout,omitempty"` // seconds, default 300
}
```

## Key Source Files

| File | Purpose |
|---|---|
| `internal/mcp/manager.go` | Manager, Client, DiscoverTools, Route, CallTool, TestServer, idle shutdown |
| `internal/types/config.go` | MCPConfig, MCPServerConfig type definitions |
