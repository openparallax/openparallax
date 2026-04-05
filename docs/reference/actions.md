# Action Types

Every operation the Agent can perform is represented as a typed action. Actions are the atomic unit of the OpenParallax pipeline — each one passes through Shield evaluation, IFC checking, Chronicle snapshotting, and audit logging before execution.

There are **50+ action types** organized into 10 categories. Each action type maps to a specific executor in the Engine.

## Action Categories

### File Operations

File actions operate on the workspace filesystem. Write operations trigger Chronicle snapshots before execution.

| Action Type | Description | OTR Allowed? |
|------------|-------------|:------------:|
| `read_file` | Read the contents of a file. Returns the full text content. | Yes |
| `write_file` | Create or overwrite a file. Takes `path` and `content`. | No |
| `edit_file` | Apply a targeted edit to an existing file. Takes `path`, `old_content`, `new_content`. More precise than `write_file` — only modifies the matched section. | No |
| `delete_file` | Remove a file from the filesystem. | No |
| `list_directory` | List files and subdirectories at a given path. Returns names, sizes, and types. | Yes |
| `search_files` | Search file contents using regex or literal patterns. Returns matching lines with context. | Yes |
| `create_directory` | Create a new directory (including parent directories). | No |
| `move_file` | Move or rename a file or directory. | No |
| `copy_file` | Copy a file or directory. | No |
| `file_info` | Get metadata about a file (size, permissions, modification time). | Yes |

### Git Operations

Git actions operate on the workspace's git repository. All require the workspace to be a git repo.

| Action Type | Description | OTR Allowed? |
|------------|-------------|:------------:|
| `git_status` | Show working tree status (staged, unstaged, untracked files). | Yes |
| `git_diff` | Show diffs — staged, unstaged, or between commits. Takes optional `ref` and `staged` parameters. | Yes |
| `git_log` | Show commit history. Takes optional `count`, `author`, `since`, `path` parameters. | Yes |
| `git_commit` | Create a commit with the given message. Stages specified files first. | No |
| `git_branch` | Create, switch, or list branches. | No |
| `git_checkout` | Check out a branch or commit. | No |
| `git_merge` | Merge a branch into the current branch. | No |
| `git_stash` | Stash or pop changes. | No |
| `git_tag` | Create or list tags. | No |
| `git_remote` | Manage remotes (add, remove, list). | No |

### Shell Operations

Shell actions execute arbitrary commands in the workspace directory.

| Action Type | Description | OTR Allowed? |
|------------|-------------|:------------:|
| `run_command` | Execute a shell command and return stdout + stderr. Takes `command`, optional `cwd`, `timeout`, `env`. The most powerful action — Shield policies typically require Tier 1 or Tier 2 evaluation for this. | No |

::: danger Security Note
`run_command` is the highest-risk action type. It can execute arbitrary code with the Engine's privileges. The default policy requires Tier 1 minimum evaluation. The strict policy requires Tier 2. Never allow `run_command` without Shield evaluation in production.
:::

### Browser Operations

Browser actions control a headless browser for web interaction.

| Action Type | Description | OTR Allowed? |
|------------|-------------|:------------:|
| `browser_navigate` | Navigate to a URL and return the page content (extracted text, not raw HTML). | Yes |
| `browser_extract` | Extract specific content from the current page using CSS selectors. | Yes |
| `browser_click` | Click an element on the current page. | No |
| `browser_type` | Type text into an input field. | No |
| `browser_screenshot` | Take a screenshot of the current page. Returns base64-encoded image data. | Yes |
| `browser_scroll` | Scroll the page. | No |

### Email Operations

Email actions send and read email through configured providers.

| Action Type | Description | OTR Allowed? |
|------------|-------------|:------------:|
| `send_email` | Send an email. Takes `to`, `subject`, `body`, optional `cc`, `bcc`, `attachments`. | No |
| `read_email` | Read emails from inbox. Takes optional `count`, `unread_only`, `folder`. | Yes |
| `search_email` | Search emails by query string. | Yes |

### Calendar Operations

Calendar actions interact with configured calendar services.

| Action Type | Description | OTR Allowed? |
|------------|-------------|:------------:|
| `read_calendar` | Read upcoming events. Takes optional `days_ahead`, `calendar_name`. | Yes |
| `create_event` | Create a calendar event. Takes `title`, `start`, `end`, optional `location`, `description`, `attendees`. | No |
| `update_event` | Modify an existing calendar event. | No |
| `delete_event` | Delete a calendar event. | No |

### Canvas Operations

Canvas actions create and manage rich visual content.

| Action Type | Description | OTR Allowed? |
|------------|-------------|:------------:|
| `canvas_create` | Create a new canvas file. Takes `title`, `content_type` (markdown, html, code, mermaid), `content`. | No |
| `canvas_update` | Update an existing canvas file. | No |

### Memory Operations

Memory actions interact with the semantic memory system.

| Action Type | Description | OTR Allowed? |
|------------|-------------|:------------:|
| `memory_search` | Search memory using natural language. Uses both FTS5 and vector similarity. Returns ranked results with relevance scores. | Yes |
| `memory_store` | Store a new memory record. Takes `content`, optional `tags`, `source`. | No |
| `memory_delete` | Delete a memory record by ID. | No |
| `memory_daily_log` | Append to today's daily conversation log. | No |

### HTTP Operations

HTTP actions make external API requests.

| Action Type | Description | OTR Allowed? |
|------------|-------------|:------------:|
| `http_request` | Make an HTTP request. Takes `method`, `url`, `headers`, `body`. Returns status code, headers, and body. | No |

### Schedule Operations

Schedule actions manage deferred and recurring tasks.

| Action Type | Description | OTR Allowed? |
|------------|-------------|:------------:|
| `schedule_task` | Schedule a task for future execution. Takes `description`, `cron_expression` or `run_at`. | No |
| `cancel_task` | Cancel a scheduled task. | No |
| `list_tasks` | List all scheduled tasks with their next run time. | Yes |

### Image Operations

Image actions generate and manipulate images.

| Action Type | Description | OTR Allowed? |
|------------|-------------|:------------:|
| `image_generate` | Generate an image using a configured provider. | No |
| `image_edit` | Edit an existing image. | No |

## Tool Groups and Lazy Loading

Tools are organized into groups for efficient LLM context management. Not all 50+ tools are loaded into every conversation — that would waste context tokens. Instead, a base set of common tools is loaded at startup, and additional groups are loaded on demand via the `load_tools` meta-tool.

### Base Tools (Always Loaded)

These tools are available in every conversation:

- `read_file`, `write_file`, `edit_file`, `list_directory`, `search_files`
- `run_command`
- `git_status`, `git_diff`, `git_log`
- `memory_search`
- `browser_navigate`, `browser_extract`

### Loadable Groups

Additional tool groups are loaded when the Agent needs them:

| Group | Tools | Loaded When |
|-------|-------|-------------|
| `git_extended` | `git_commit`, `git_branch`, `git_checkout`, `git_merge`, `git_stash`, `git_tag`, `git_remote` | Agent needs to make git changes |
| `email` | `send_email`, `read_email`, `search_email` | User asks about email |
| `calendar` | `read_calendar`, `create_event`, `update_event`, `delete_event` | User asks about calendar |
| `canvas` | `canvas_create`, `canvas_update` | Agent wants to create visual content |
| `memory_write` | `memory_store`, `memory_delete`, `memory_daily_log` | Agent needs to persist information |
| `browser_extended` | `browser_click`, `browser_type`, `browser_screenshot`, `browser_scroll` | Agent needs to interact with a webpage |
| `http` | `http_request` | Agent needs to call external APIs |
| `schedule` | `schedule_task`, `cancel_task`, `list_tasks` | User asks about scheduling |
| `file_extended` | `delete_file`, `create_directory`, `move_file`, `copy_file`, `file_info` | Agent needs advanced file operations |
| `image` | `image_generate`, `image_edit` | User asks about images |

The `load_tools` meta-tool is handled locally in the Agent's reasoning loop — it doesn't go through the Engine. The Agent sends a `ToolDefsRequest` event to the Engine, which responds with the tool definitions for the requested group. This keeps tool definitions out of the initial context and loads them only when needed.

## OTR Mode Filtering

In OTR (Off-The-Record) mode, all write actions are removed at the tool definition level. The LLM never sees write tools — it can't propose actions it doesn't know about. The column "OTR Allowed?" above shows which actions are available in OTR mode.

Available OTR tools: `read_file`, `list_directory`, `search_files`, `file_info`, `git_status`, `git_diff`, `git_log`, `memory_search`, `browser_navigate`, `browser_extract`, `browser_screenshot`, `read_email`, `search_email`, `read_calendar`, `list_tasks`.

## Action Hashing

Every action is hashed (SHA-256) before execution. The hash covers the action type, arguments, and timestamp. This serves two purposes:

1. **Deduplication** — identical actions proposed in rapid succession are detected and deduplicated.
2. **Audit integrity** — the hash is stored in the audit log and can be verified independently.

## Protection Levels

Certain files have protection levels that override normal Shield evaluation:

| Level | Files | Behavior |
|-------|-------|----------|
| `ReadOnly` | SOUL.md, IDENTITY.md, TOOLS.md, BOOT.md | Read allowed, writes blocked |
| `EscalateTier2` | AGENTS.md, HEARTBEAT.md | Writes require Tier 2 evaluation |
| `WriteTier1Min` | MEMORY.md, USER.md | Writes need at least Tier 1 |
| `FullBlock` | config.yaml, canary.token, audit.jsonl, openparallax.db, evaluator-v1.md | No read or write via agent |

These are enforced by `CheckProtection()` in the pipeline, before Shield evaluation. They cannot be overridden by policy.

See [File Protection](/technical/protection) for details.
