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
| `canvas` | Create files and projects | 3 |
| `image_generation` | Generate and edit images with AI | 2 |
| `video_generation` | Generate videos with AI | 1 |
| `agents` | Spawn and manage sub-agents | 6 |
| `system` | Clipboard, launch, notifications, system info, screenshot | 6 |
| `utilities` | Math, archives, PDF, spreadsheets | 5 |

## Tool Reference by Group

### files

File and directory operations. The agent can touch any file on disk subject to the [default denylist](/guide/security#default-denylist) and the per-action Shield evaluation.

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

::: warning Absolute paths required
Every path argument to a file tool must be **absolute** (e.g. `/home/user/Desktop/project/main.go`). The leading `~` is expanded to the user's home directory. Relative paths are rejected at the engine before Shield evaluation. The reason is that Shield evaluates the literal path string the agent sent and cannot resolve relative paths against an implicit working directory; making path resolution unambiguous is what makes the denylist deterministic. The agent's tool descriptions enforce this and the engine returns a clear error pointing the LLM at the absolute-path requirement so it can re-roll on the next round.
:::

### shell

Execute commands on the host system.

| Tool | Description |
|------|-------------|
| `execute_command` | Run a shell command and return stdout/stderr |

Shell commands go through the full Shield pipeline by default. Two important behaviors:

**Absolute paths required.** Every path inside the command string must be absolute. `rm -rf /home/user/Desktop/project/db` is fine; `rm -rf db` is rejected. The one allowed exception is a leading `cd <absolute-path> && <command>` prefix — the cd target establishes an implicit working directory, and write targets in the rest of the command are resolved against it. Anything more complex (chained cds, env-var targets, command substitution) does not get the cd-prefix exemption and falls into the relative-path rejection path.

**Safe-command fast path.** Common dev workflow commands (`git`, `npm`, `make`, `go`, `cargo`, `docker`, `kubectl`, `pwd`, `whoami`, `date`, etc., plus the cmd.exe equivalents on Windows) bypass all four Shield tiers and return ALLOW with confidence 1.0 — no LLM call, no latency. The fast path applies only to single-statement commands; any command containing `;`, `&`, `|`, `>`, `<`, `` ` ``, or `$(...)` falls through to normal evaluation. The allowlist is curated and ships in the binary; see [Policies → Safe Command Fast Path](/shield/policies#safe-command-fast-path).

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

Browser tools require a **Chromium-based browser** to be installed — Chrome, Chromium, Edge, Brave, Opera, Vivaldi, or Arc. Other browsers (Firefox, Safari, WebKit-based) are not supported because the executor uses the Chrome DevTools Protocol (CDP) via chromedp. The `openparallax doctor` command checks for browser availability.

On Linux, Flatpak-installed browsers are detected and supported via a wrapper script. However, the Flatpak sandbox may interfere with headless browsing — if navigation consistently fails with `ERR_ABORTED`, consider installing the browser natively or adjusting `agents.max_consecutive_nav_failures` in config.yaml.

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

Create files and projects.

| Tool | Description |
|------|-------------|
| `canvas_create` | Create a single file (HTML, SVG, Markdown, etc.) |
| `canvas_update` | Update an existing canvas file |
| `canvas_project` | Create a multi-file project |

Canvas outputs are rendered inline in the chat panel with preview support for HTML content.

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

Spawn and manage sub-agents for parallel task execution. Each sub-agent runs as a separate sandboxed process with its own LLM context window. The agent is nudged through its system prompt and the tool descriptions to prefer sub-agent delegation when it has 2+ independent subtasks (research, multi-file scans, parallel processing) — keeping the parent's context lean and running the work concurrently.

| Tool | Description |
|------|-------------|
| `create_agent` | Spawn a new sub-agent with a specific task |
| `agent_status` | Check the status of a running sub-agent |
| `agent_result` | Retrieve the result from a completed sub-agent |
| `agent_message` | Send an additional instruction to a running sub-agent |
| `delete_agent` | Terminate and clean up a sub-agent |
| `list_agents` | List all active sub-agents |

#### Sub-agent context isolation

A sub-agent starts with a **blank context** — it does NOT inherit the parent's conversation, files-loaded state, or prior reasoning. The `task` parameter must be self-contained: include all background, file paths, and constraints the sub-agent needs to finish without further questions. The `create_agent` tool description in the LLM's tool schema spells this out so the model gets it right by default.

#### Sub-agent tool scoping

When spawning, the parent passes `tool_groups` to constrain which tool groups the sub-agent receives. Omit it for all available groups. Three groups are **always stripped** from sub-agents regardless of what was requested:

- `agents` — sub-agents cannot spawn their own sub-agents (recursion prevention)
- `memory` — only the parent owns memory writes
- `schedule` — only the parent owns the heartbeat scheduler

The `load_tools` meta-tool is also unavailable to sub-agents — they receive a frozen tool slice at spawn time and cannot expand their toolset mid-run. The parent's `tool_groups` parameter is the cage.

Browser tools are available to sub-agents but share the same headless browser session as the main agent. If navigation fails repeatedly (e.g. on hosts with Flatpak sandboxing issues), the browser executor disables navigation after `agents.max_consecutive_nav_failures` (default 3) consecutive failures and returns a clear error telling the sub-agent to fall back to other tools or training knowledge. The counter resets when a navigation succeeds or the browser session restarts.

#### Sub-agent model selection

The `model` parameter on `create_agent` is a **1-based index** into the workspace's `models[]` pool, not a raw provider model string. The `create_agent` tool description rendered for the LLM auto-includes a numbered menu of the available models when the pool has two or more entries:

```
Available sub-agent models — you are the judge; pick by task fit.
Entries without a hint, judge from the model name:
  1. claude-haiku-4-5-20251001 — fast, cheap, scans and lookups
  2. claude-sonnet-4-6 — balanced reasoning, multi-file context
  3. gpt-5.4-mini
```

The `purpose` annotation comes from the optional `models[].purpose` field in `config.yaml`. Entries without a `purpose` are still selectable; the LLM judges from the model name. Out-of-range indices return a graceful error so the LLM can recover on the next round. Omit the `model` parameter to use the engine default (cheapest available, or whatever `roles.sub_agent` points at). The pool snapshot is taken at engine startup, so live config edits cannot drift the index mapping mid-session.

See [Configuration → models and roles](/guide/configuration#models-and-roles) for the `purpose` field reference.

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
