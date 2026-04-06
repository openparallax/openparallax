# Action Types

Every operation the Agent can perform is represented as a typed action. Actions are the atomic unit of the OpenParallax pipeline — each one passes through Shield evaluation, IFC checking, Chronicle snapshotting, and audit logging before execution.

OpenParallax defines **73 action types** across **15 categories**.

Source: [`internal/types/action.go`](https://github.com/openparallax/openparallax/blob/main/internal/types/action.go)

## OTR Mode

In Off-the-Record mode, only read-only actions are permitted. Actions marked **Yes** in the OTR column below are allowed; all others are blocked. See [Sessions — OTR Mode](/guide/sessions#otr-mode) for details.

Source: [`internal/session/otr.go`](https://github.com/openparallax/openparallax/blob/main/internal/session/otr.go)

## Files

Tool group: `files`

| Action | Constant | Description | OTR |
|--------|----------|-------------|-----|
| `read_file` | `ActionReadFile` | Reads a file from the filesystem | Yes |
| `write_file` | `ActionWriteFile` | Writes content to a file | No |
| `delete_file` | `ActionDeleteFile` | Deletes a file from the filesystem | No |
| `move_file` | `ActionMoveFile` | Moves or renames a file | No |
| `copy_file` | `ActionCopyFile` | Copies a file to a new location | No |
| `create_directory` | `ActionCreateDir` | Creates a directory | No |
| `list_directory` | `ActionListDir` | Lists the contents of a directory | Yes |
| `search_files` | `ActionSearchFiles` | Searches for files matching a pattern | Yes |
| `copy_directory` | `ActionCopyDir` | Copies a directory recursively | No |
| `move_directory` | `ActionMoveDir` | Moves or renames a directory | No |
| `delete_directory` | `ActionDeleteDir` | Deletes a directory recursively | No |
| `grep_files` | `ActionGrepFiles` | Searches file contents for a text pattern | No |

## Shell

Tool group: `shell`

| Action | Constant | Description | OTR |
|--------|----------|-------------|-----|
| `execute_command` | `ActionExecCommand` | Executes a shell command | No |

## Communication

These actions are not part of a single tool group — `send_message` is used internally by channel adapters, and `send_email` belongs to the `email` group.

| Action | Constant | Description | OTR |
|--------|----------|-------------|-----|
| `send_message` | `ActionSendMessage` | Sends a message via a channel adapter | No |
| `send_email` | `ActionSendEmail` | Sends an email via SMTP | No |

## HTTP

| Action | Constant | Description | OTR |
|--------|----------|-------------|-----|
| `http_request` | `ActionHTTPRequest` | Performs an HTTP request | Yes |

## Browser

Tool group: `browser`

| Action | Constant | Description | OTR |
|--------|----------|-------------|-----|
| `browser_navigate` | `ActionBrowserNav` | Navigates a browser to a URL | Yes |
| `browser_click` | `ActionBrowserClick` | Clicks an element in the browser | No |
| `browser_type` | `ActionBrowserType` | Types text into a browser element | No |
| `browser_extract` | `ActionBrowserExtract` | Extracts content from the browser | Yes |
| `browser_screenshot` | `ActionBrowserShot` | Takes a screenshot of the browser | Yes |

## Schedule

Tool group: `schedule`

| Action | Constant | Description | OTR |
|--------|----------|-------------|-----|
| `create_schedule` | `ActionCreateSchedule` | Creates a recurring schedule entry | No |
| `delete_schedule` | `ActionDeleteSchedule` | Removes a schedule entry | No |
| `list_schedules` | `ActionListSchedules` | Lists all schedule entries | No |

## Calendar

Tool group: `calendar`

| Action | Constant | Description | OTR |
|--------|----------|-------------|-----|
| `read_calendar` | `ActionReadCalendar` | Reads calendar events | Yes |
| `create_event` | `ActionCreateEvent` | Creates a calendar event | No |
| `update_event` | `ActionUpdateEvent` | Updates a calendar event | No |
| `delete_event` | `ActionDeleteEvent` | Deletes a calendar event | No |

## Git

Tool group: `git`

| Action | Constant | Description | OTR |
|--------|----------|-------------|-----|
| `git_status` | `ActionGitStatus` | Shows the git working tree status | Yes |
| `git_diff` | `ActionGitDiff` | Shows the git diff | Yes |
| `git_commit` | `ActionGitCommit` | Creates a git commit | No |
| `git_push` | `ActionGitPush` | Pushes to a git remote | No |
| `git_pull` | `ActionGitPull` | Pulls from a git remote | No |
| `git_log` | `ActionGitLog` | Shows the git log | Yes |
| `git_branch` | `ActionGitBranch` | Manages git branches | No |
| `git_checkout` | `ActionGitCheckout` | Checks out a git branch or commit | No |
| `git_clone` | `ActionGitClone` | Clones a git repository | No |

## Memory

Tool group: `memory`

| Action | Constant | Description | OTR |
|--------|----------|-------------|-----|
| `memory_write` | `ActionMemoryWrite` | Writes to a workspace memory file | No |
| `memory_search` | `ActionMemorySearch` | Searches memory via FTS5 | Yes |

## Canvas

Tool group: `canvas`

| Action | Constant | Description | OTR |
|--------|----------|-------------|-----|
| `canvas_create` | `ActionCanvasCreate` | Creates a canvas file (Mermaid, SVG, Markdown, HTML) | No |
| `canvas_update` | `ActionCanvasUpdate` | Updates an existing canvas file | No |
| `canvas_project` | `ActionCanvasProject` | Creates a multi-file project in a directory | No |
| `canvas_preview` | `ActionCanvasPreview` | Starts a local preview server | No |

## Image and Video Generation

Tool groups: `image_generation`, `video_generation`

| Action | Constant | Description | OTR |
|--------|----------|-------------|-----|
| `generate_image` | `ActionGenerateImage` | Generates an image from a text prompt | No |
| `edit_image` | `ActionEditImage` | Edits an existing image based on a prompt | No |
| `generate_video` | `ActionGenerateVideo` | Generates a short video from a text prompt | No |

## Email

Tool group: `email`

| Action | Constant | Description | OTR |
|--------|----------|-------------|-----|
| `email_list` | `ActionEmailList` | Lists emails in a mailbox folder | No |
| `email_read` | `ActionEmailRead` | Reads a specific email by UID | No |
| `email_search` | `ActionEmailSearch` | Searches emails by query | No |
| `email_move` | `ActionEmailMove` | Moves an email to a different folder | No |
| `email_mark` | `ActionEmailMark` | Marks an email as read/unread/flagged | No |

## System

Tool group: `system`

| Action | Constant | Description | OTR |
|--------|----------|-------------|-----|
| `clipboard_read` | `ActionClipboardRead` | Reads from the system clipboard | No |
| `clipboard_write` | `ActionClipboardWrite` | Writes to the system clipboard | No |
| `open` | `ActionOpen` | Launches a file or URL in the default application | No |
| `notify` | `ActionNotify` | Sends an OS notification | No |
| `system_info` | `ActionSystemInfo` | Returns system information (disk, memory, CPU, etc.) | No |
| `screenshot` | `ActionScreenshot` | Captures the desktop screen | No |

## Utilities

Tool group: `utilities`

| Action | Constant | Description | OTR |
|--------|----------|-------------|-----|
| `calculate` | `ActionCalculate` | Evaluates a mathematical expression | No |
| `archive_create` | `ActionArchiveCreate` | Creates a zip or tar.gz archive | No |
| `archive_extract` | `ActionArchiveExtract` | Extracts a zip or tar.gz archive | No |
| `pdf_read` | `ActionPDFRead` | Extracts text from a PDF file | No |
| `spreadsheet_read` | `ActionSpreadsheetRead` | Reads data from a CSV or Excel file | No |
| `spreadsheet_write` | `ActionSpreadsheetWrite` | Writes data to a CSV or Excel file | No |

## Agents

Tool group: `agents`

| Action | Constant | Description | OTR |
|--------|----------|-------------|-----|
| `create_agent` | `ActionCreateAgent` | Spawns a sub-agent to work on a task | No |
| `agent_status` | `ActionAgentStatus` | Checks the status of a sub-agent | No |
| `agent_result` | `ActionAgentResult` | Collects the result from a completed sub-agent | No |
| `agent_message` | `ActionAgentMessage` | Sends an additional instruction to a running sub-agent | No |
| `delete_agent` | `ActionDeleteAgent` | Terminates a sub-agent immediately | No |
| `list_agents` | `ActionListAgents` | Lists all active sub-agents | No |

## Tool Groups

Actions are organized into tool groups for on-demand loading via the `load_tools` meta-tool. The LLM starts each turn with no tools loaded and calls `load_tools(["files", "git"])` to gain access.

| Group | Description |
|-------|-------------|
| `files` | Read, write, list, search, and delete files in the workspace |
| `shell` | Execute shell commands on the system |
| `git` | Git version control — status, diff, log, commit, push, pull, branch, clone |
| `browser` | Browse the web — navigate, click, type, extract, screenshot |
| `email` | Send and read emails — list inbox, search, read, move, mark, and send |
| `calendar` | Manage calendar events — list, create, update, delete |
| `memory` | Write structured memories and search past conversations |
| `schedule` | Manage recurring tasks via HEARTBEAT.md cron entries |
| `canvas` | Create files, multi-file projects, and live-preview websites |
| `image_generation` | Generate images using AI if supported by model |
| `video_generation` | Generate videos using AI if supported by model |
| `agents` | Spawn and manage sub-agents for parallel task execution |
| `system` | Clipboard access, launch files/URLs, OS notifications, system info, screenshots |
| `utilities` | Math calculations, archive zip/extract, PDF text extraction, spreadsheet read/write |

Groups can be disabled via [`tools.disabled_groups`](/reference/config#tools) in config.yaml. MCP server tools are registered as `mcp:<server-name>` groups automatically.
