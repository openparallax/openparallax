# Tools

OpenParallax provides 50+ tool actions organized into groups. Tools are the agent's hands — they let it read files, run commands, browse the web, send emails, manage git repositories, and more.

## Tool Groups and Lazy Loading

Tools are organized into groups and loaded on demand. At the start of each turn, the agent has access only to the `load_tools` meta-tool. When the agent determines it needs specific capabilities, it calls `load_tools` with the group names.

This lazy loading approach keeps the LLM context compact and focused. The agent only loads the tools it actually needs for the current task.

### The load_tools Meta-Tool

```
load_tools(groups: ["files", "git"])
```

Returns the full tool definitions for the requested groups, making them available for the rest of the turn.

### Available Groups

| Group | Description | Tool Count |
|-------|-------------|-----------|
| `files` | Read, write, list, search, and delete files | 12 |
| `shell` | Execute shell commands | 1 |
| `git` | Git version control operations | 9 |
| `browser` | Web browsing and content extraction | 5 |
| `email` | Send and read emails | 6 |
| `calendar` | Manage calendar events | 4 |
| `memory` | Write and search persistent memory | 2 |
| `schedule` | Manage recurring tasks | 3 |
| `canvas` | Create files, projects, and live previews | 4 |
| `image_generation` | Generate and edit images with AI | 2 |
| `video_generation` | Generate videos with AI | 1 |
| `agents` | Spawn and manage sub-agents | 6 |
| `system` | Clipboard, launch, notifications, system info | 5 |
| `utilities` | Math, archives, PDF, spreadsheets | 5 |

## Tool Reference by Group

### files

File and directory operations within the workspace.

| Tool | Description |
|------|-------------|
| `read_file` | Read the contents of a file |
| `write_file` | Create or overwrite a file |
| `delete_file` | Delete a file |
| `move_file` | Move or rename a file |
| `copy_file` | Copy a file |
| `create_directory` | Create a directory (including parents) |
| `delete_directory` | Delete a directory and its contents |
| `move_directory` | Move or rename a directory |
| `copy_directory` | Recursively copy a directory |
| `list_directory` | List files and subdirectories |
| `search_files` | Search for files by name pattern |
| `grep_files` | Search file contents with regex patterns |

### shell

Execute commands on the host system.

| Tool | Description |
|------|-------------|
| `execute_command` | Run a shell command and return stdout/stderr |

Shell commands are always evaluated by Shield (Tier 1 minimum in the default policy). The command, arguments, and working directory are sent to the evaluator for security review.

### git

Git version control operations using pure Go (no git binary required).

| Tool | Description |
|------|-------------|
| `git_status` | Show working tree status |
| `git_diff` | Show changes between commits, working tree, etc. |
| `git_log` | Show commit history |
| `git_commit` | Create a commit with a message |
| `git_push` | Push commits to a remote |
| `git_pull` | Pull changes from a remote |
| `git_branch` | List, create, or delete branches |
| `git_checkout` | Switch branches or restore files |
| `git_clone` | Clone a repository |

Read-only git operations (`git_status`, `git_diff`, `git_log`) are allowed by default policy. Write operations (`git_commit`, `git_push`) require higher-tier evaluation.

### browser

Web browsing and content extraction using a Chromium-based browser.

| Tool | Description |
|------|-------------|
| `browser_navigate` | Navigate to a URL and return page content |
| `browser_click` | Click an element on the page |
| `browser_type` | Type text into an input field |
| `browser_extract` | Extract structured data from the page |
| `browser_screenshot` | Take a screenshot of the page |

Browser tools require a Chromium-based browser to be installed (Chrome, Chromium, Edge, Brave). The `openparallax doctor` command checks for browser availability.

### email

Email operations via SMTP and IMAP.

| Tool | Description |
|------|-------------|
| `send_email` | Compose and send an email |
| `email_list` | List emails in the inbox |
| `email_read` | Read a specific email |
| `email_search` | Search emails by query |
| `email_move` | Move an email to a folder |
| `email_mark` | Mark an email as read/unread/starred |

Email requires SMTP and IMAP configuration in config.yaml. Sending emails is evaluated at Tier 1 minimum by the default Shield policy.

### calendar

Calendar event management.

| Tool | Description |
|------|-------------|
| `read_calendar` | List upcoming events |
| `create_event` | Create a new calendar event |
| `update_event` | Modify an existing event |
| `delete_event` | Remove a calendar event |

Supports Google Calendar and CalDAV. Read operations are allowed by default; write operations are evaluated at higher tiers in strict policy.

### memory

Persistent memory operations.

| Tool | Description |
|------|-------------|
| `memory_write` | Store a structured memory entry |
| `memory_search` | Search past memories using FTS5 and vector similarity |

`memory_search` is always allowed. `memory_write` is evaluated at Tier 1 in the default policy.

### schedule

Manage recurring tasks defined in HEARTBEAT.md.

| Tool | Description |
|------|-------------|
| `create_schedule` | Add a new cron entry to HEARTBEAT.md |
| `delete_schedule` | Remove a cron entry |
| `list_schedules` | List all scheduled tasks |

See [Heartbeat](/guide/heartbeat) for details on the cron format.

### canvas

Create files and projects with live preview.

| Tool | Description |
|------|-------------|
| `canvas_create` | Create a single file (HTML, SVG, Markdown, etc.) |
| `canvas_update` | Update an existing canvas artifact |
| `canvas_project` | Create a multi-file project |
| `canvas_preview` | Generate a live preview URL for an HTML artifact |

Canvas artifacts appear in the web UI's Artifact Canvas panel with live preview support for HTML content.

### image_generation

Generate and edit images using AI providers.

| Tool | Description |
|------|-------------|
| `generate_image` | Generate an image from a text prompt |
| `edit_image` | Edit an existing image with AI |

Supports DALL-E (OpenAI), Imagen (Google), and Stability AI backends.

### video_generation

Generate videos using AI.

| Tool | Description |
|------|-------------|
| `generate_video` | Generate a video from a text prompt |

### agents

Spawn and manage sub-agents for parallel task execution.

| Tool | Description |
|------|-------------|
| `create_agent` | Spawn a new sub-agent with a specific task |
| `agent_status` | Check the status of a running sub-agent |
| `agent_result` | Retrieve the result from a completed sub-agent |
| `agent_message` | Send a message to a running sub-agent |
| `delete_agent` | Terminate and clean up a sub-agent |
| `list_agents` | List all active sub-agents |

Sub-agents run as separate sandboxed processes with their own LLM context, enabling parallel task execution.

### system

System-level operations.

| Tool | Description |
|------|-------------|
| `clipboard_read` | Read from the system clipboard |
| `clipboard_write` | Write to the system clipboard |
| `open` | Open a file or URL with the default application |
| `notify` | Send an OS notification |
| `system_info` | Get system information (OS, architecture, memory, etc.) |
| `screenshot` | Take a screenshot of the desktop |

### utilities

General-purpose utility tools.

| Tool | Description |
|------|-------------|
| `calculate` | Evaluate a mathematical expression |
| `archive_create` | Create a zip/tar archive |
| `archive_extract` | Extract a zip/tar archive |
| `pdf_read` | Extract text from a PDF file |
| `spreadsheet_read` | Read data from a spreadsheet (CSV, XLSX) |
| `spreadsheet_write` | Write data to a spreadsheet |

## Shield Evaluation

Every tool call passes through the Shield security pipeline before execution. The evaluation tier depends on the action type and the active policy:

- **Tier 0 (Policy)** — YAML rules match action type and path patterns. Actions matching `allow` rules with no `tier_override` pass immediately. Actions matching `deny` rules are blocked immediately.
- **Tier 1 (Heuristic + ONNX)** — Pattern matching and ML classification. Actions with `tier_override: 1` must pass this tier.
- **Tier 2 (LLM Evaluator)** — Full LLM-based security evaluation. Reserved for high-risk actions (identity modification, schedule changes, etc.).

See [Security](/guide/security) for details on each tier.

## Tools in OTR Mode

OTR sessions filter out write tools at the definition level. The agent physically cannot call write operations because they are not present in the available tool list. Read-only tools remain available. See [Sessions](/guide/sessions) for the complete list of filtered tools.

## MCP Server Integration

Additional tools can be provided by external MCP (Model Context Protocol) servers. Configure them in config.yaml:

```yaml
mcp:
  servers:
    - name: github
      command: npx
      args: ["-y", "@modelcontextprotocol/server-github"]
      env:
        GITHUB_TOKEN: "${GITHUB_TOKEN}"
```

MCP tools are registered alongside built-in tools and appear in the tool group listing. They go through the same Shield evaluation pipeline — no tool, built-in or external, bypasses security evaluation.

### How MCP Works

1. The engine spawns the MCP server as a child process on startup
2. It queries the server for available tool definitions
3. Tools are registered in the group registry
4. When the agent calls an MCP tool, the engine routes the call to the MCP server
5. Shield evaluates MCP tool calls identically to built-in tools

## The load_skills Meta-Tool

In addition to `load_tools`, there is a `load_skills` meta-tool for loading custom skill guidance. See [Skills](/guide/skills) for details. The two meta-tools serve different purposes:

- **`load_tools`** — loads executable capabilities (actions the agent can take)
- **`load_skills`** — loads instructional context (guidance for how to approach a task)

## Next Steps

- [Security](/guide/security) — how Shield evaluates tool calls
- [Skills](/guide/skills) — custom domain guidance
- [Channels](/guide/channels) — how tools work across messaging channels
