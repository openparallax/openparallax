# MCP Security Gateway

Shield can sit between any MCP client and its MCP servers, evaluating every tool call before it reaches the server. This turns Shield into a transparent security gateway -- your MCP client connects to Shield, Shield connects to the actual MCP servers, and every tool call passes through the 4-tier pipeline.

## The Problem

MCP servers are powerful. A filesystem server can read and write any file. A shell server can execute arbitrary commands. A database server can drop tables. When an LLM proposes a tool call, the MCP client typically executes it immediately with no security evaluation.

```
         Without Shield
┌────────────┐    tool call    ┌────────────────┐
│ MCP Client │ ──────────────▶ │   MCP Server   │
│ (Claude,   │                 │ (filesystem,   │
│  Cursor)   │ ◀────────────── │  shell, DB)    │
└────────────┘    result       └────────────────┘
                                     ▲
                                     │
                              No security check.
                              LLM says "rm -rf /"?
                              Server executes it.
```

## The Solution

Insert Shield as a proxy between the client and server:

```
         With Shield
┌────────────┐    tool call    ┌─────────────┐    tool call    ┌────────────────┐
│ MCP Client │ ──────────────▶ │   Shield    │ ──────────────▶ │   MCP Server   │
│ (Claude,   │                 │   Gateway   │                 │ (filesystem,   │
│  Cursor)   │ ◀────────────── │  4-tier     │ ◀────────────── │  shell, DB)    │
└────────────┘    result       │  pipeline   │    result       └────────────────┘
                               └─────────────┘
                                     │
                              Every tool call
                              evaluated before
                              forwarding.
```

Shield is transparent to both the client and server. The client sees the same tools, the same schemas, the same responses. But every tool call passes through policy matching, ML classification, and (when needed) LLM evaluation before it reaches the server.

## Configuration

### `shield.yaml` for MCP Proxy

```yaml
listen: localhost:9090

policy:
  file: policies/default.yaml

classifier:
  model_dir: ~/.openparallax/models/prompt-injection/
  threshold: 0.85

heuristic:
  enabled: true

evaluator:
  provider: anthropic
  model: claude-sonnet-4-6
  api_key_env: ANTHROPIC_API_KEY

fail_closed: true
daily_budget: 100

# MCP upstream servers
mcp:
  servers:
    - name: filesystem
      transport: stdio
      command: npx
      args:
        - "@modelcontextprotocol/server-filesystem"
        - "/home/user/workspace"

    - name: github
      transport: streamable-http
      url: https://api.github.com/mcp

    - name: database
      transport: stdio
      command: npx
      args:
        - "@modelcontextprotocol/server-postgres"
        - "postgresql://localhost/mydb"

# Audit logging
audit:
  enabled: true
  file: shield-mcp-audit.jsonl
```

### Upstream Server Types

Shield supports two MCP transport types for upstream servers:

#### stdio Transport

The server runs as a child process. Shield spawns it, communicates over stdin/stdout:

```yaml
mcp:
  servers:
    - name: filesystem
      transport: stdio
      command: npx
      args: ["@modelcontextprotocol/server-filesystem", "/home/user"]
      env:
        NODE_ENV: production
```

#### Streamable HTTP Transport

The server runs as a remote HTTP service:

```yaml
mcp:
  servers:
    - name: remote-tools
      transport: streamable-http
      url: https://mcp-server.example.com
      headers:
        Authorization: "Bearer ${MCP_AUTH_TOKEN}"
```

## Running the Proxy

```bash
# Start Shield as MCP proxy.
openparallax-shield mcp-proxy --config shield.yaml

# The proxy listens at http://localhost:9090/mcp
# Point your MCP client here instead of directly at the servers.
```

Shield starts all upstream servers, discovers their tools, and presents a unified tool list to the client. When a tool call arrives, Shield:

1. Identifies which upstream server owns the tool
2. Maps the MCP tool name to a Shield action type
3. Runs the action through the 4-tier pipeline
4. If ALLOW: forwards the call to the upstream server and returns the result
5. If BLOCK: returns an error to the client with the reason

## Deployment with MCP Clients

### Claude Desktop

Edit `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS) or `%APPDATA%\Claude\claude_desktop_config.json` (Windows):

**Before (direct connection):**

```json
{
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": ["@modelcontextprotocol/server-filesystem", "/home/user"]
    },
    "github": {
      "command": "npx",
      "args": ["@modelcontextprotocol/server-github"],
      "env": { "GITHUB_TOKEN": "..." }
    }
  }
}
```

**After (through Shield):**

```json
{
  "mcpServers": {
    "shield": {
      "url": "http://localhost:9090/mcp"
    }
  }
}
```

All tools from all upstream servers are available through the single Shield endpoint. Shield handles server management, tool routing, and security evaluation.

### Cursor

In Cursor settings, replace individual MCP server configurations with the Shield proxy URL:

```json
{
  "mcp.servers": {
    "shield": {
      "url": "http://localhost:9090/mcp"
    }
  }
}
```

### Custom Agents

Any MCP client library can connect to Shield. Point the MCP client URL at Shield instead of the upstream server:

```python
# Python (mcp SDK)
from mcp import ClientSession
from mcp.client.streamable_http import streamablehttp_client

async with streamablehttp_client("http://localhost:9090/mcp") as (read, write):
    async with ClientSession(read, write) as session:
        await session.initialize()
        tools = await session.list_tools()
        # All upstream tools available, all calls go through Shield
```

```typescript
// TypeScript (mcp SDK)
import { Client } from '@modelcontextprotocol/sdk/client/index.js'
import { StreamableHTTPClientTransport } from '@modelcontextprotocol/sdk/client/streamableHttp.js'

const transport = new StreamableHTTPClientTransport(
  new URL('http://localhost:9090/mcp'),
)
const client = new Client({ name: 'my-agent', version: '1.0' })
await client.connect(transport)

const tools = await client.listTools()
// All upstream tools, secured by Shield
```

## Tool-to-Action Mapping

Shield maps MCP tool names to Shield action types for policy evaluation:

| MCP Tool Pattern | Shield Action Type |
|------------------|--------------------|
| `read_file`, `get_file_contents` | `read_file` |
| `write_file`, `create_file` | `write_file` |
| `run_command`, `execute`, `bash` | `execute_command` |
| `list_directory`, `ls` | `list_directory` |
| `search`, `grep`, `find` | `search_files` |
| `git_*` | Corresponding `git_*` action |
| Other tool names | Used as-is for action type |

The mapping is configurable in `shield.yaml`:

```yaml
mcp:
  tool_mapping:
    # Custom mappings: MCP tool name → Shield action type
    "my_custom_tool": "execute_command"
    "query_database": "http_request"
```

## Audit Logging

Every MCP tool call is logged to the audit file with full details:

```jsonl
{"timestamp":"2026-04-03T10:30:00Z","tool":"read_file","server":"filesystem","action_type":"read_file","payload":{"path":"/home/user/workspace/main.go"},"verdict":"ALLOW","tier":0,"confidence":1.0,"reasoning":"policy allow: allow_workspace_reads","hash":"sha256:...","prev_hash":"sha256:..."}
{"timestamp":"2026-04-03T10:30:01Z","tool":"bash","server":"shell","action_type":"execute_command","payload":{"command":"rm -rf /"},"verdict":"BLOCK","tier":1,"confidence":0.95,"reasoning":"heuristic: rm -rf detected (critical)","hash":"sha256:...","prev_hash":"sha256:..."}
```

Each audit entry includes:
- The MCP tool name and upstream server
- The mapped Shield action type
- The full payload
- The verdict, tier, confidence, and reasoning
- SHA-256 hash chain for tamper detection

### Reviewing the Audit Log

```bash
# View recent decisions
tail -20 shield-mcp-audit.jsonl | jq '.'

# Find all blocked actions
grep '"verdict":"BLOCK"' shield-mcp-audit.jsonl | jq '.'

# Count decisions by verdict
jq -r '.verdict' shield-mcp-audit.jsonl | sort | uniq -c

# Verify hash chain integrity
openparallax-shield audit verify --file shield-mcp-audit.jsonl
```

## Before and After

### Before: Unprotected MCP

```
User: "Delete all files in /home/user"

  Claude Desktop ──→ filesystem server
                     rm -rf /home/user/*
                     ✓ Executed immediately
                     Files deleted. No undo.
```

### After: Shield-Protected MCP

```
User: "Delete all files in /home/user"

  Claude Desktop ──→ Shield proxy ──→ filesystem server
                     │
                     Tier 0: deny rule matches
                     delete_file on /home/user/*
                     BLOCK (confidence 1.0)
                     │
                     ✗ Blocked. Files safe.
                     Error returned to Claude:
                     "Action blocked by security policy"
```

## Multiple Upstream Servers

Shield aggregates tools from all configured upstream servers into a single endpoint. Tool names are namespaced to avoid collisions:

```yaml
mcp:
  servers:
    - name: fs
      transport: stdio
      command: npx
      args: ["@modelcontextprotocol/server-filesystem", "/home/user"]

    - name: github
      transport: streamable-http
      url: https://api.github.com/mcp

    - name: postgres
      transport: stdio
      command: npx
      args: ["@modelcontextprotocol/server-postgres", "postgresql://localhost/db"]
```

The client sees tools from all three servers through the single Shield proxy.

## Performance

Shield adds minimal latency to MCP tool calls:

| Tier | Added Latency | When Used |
|------|---------------|-----------|
| Tier 0 only | < 1ms | Action matches a policy rule |
| Tier 1 (heuristic only) | ~2ms | No policy match, heuristic-only mode |
| Tier 1 (ONNX + heuristic) | ~50ms | No policy match, ONNX model installed |
| Tier 2 (LLM evaluator) | 1-3s | Escalated from Tier 1 |

Most tool calls are resolved at Tier 0 (policy match) or Tier 1 (classifier). Tier 2 evaluations are rare -- they only occur when the classifier is uncertain about a potentially dangerous action.

## Next Steps

- [Standalone binary](/shield/standalone) -- installation and service deployment
- [Policy Syntax](/shield/policies) -- write policies tuned for your MCP servers
- [Configuration](/shield/configuration) -- full configuration reference
- [Tier 0 -- Policy](/shield/tier0) -- understand policy matching
