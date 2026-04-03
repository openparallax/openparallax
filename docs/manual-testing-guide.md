# OpenParallax Manual Testing Guide

Step-by-step manual testing of every user-facing feature. Perform each action, observe the result. Organized by feature area.

---

## Prerequisites

```bash
# Build the binary
make build-all

# Set at least one API key
export ANTHROPIC_API_KEY="sk-ant-..."
# OR
export OPENAI_API_KEY="sk-..."

# Verify the binary works
./dist/openparallax --help
```

---

## 1. Initialization

### 1.1 First Agent Setup

**Action:** Run `./dist/openparallax init`

**Observe:**
- [ ] Welcome banner appears with "Welcome to OpenParallax" ASCII box
- [ ] Prompts for agent name (default: Atlas)
- [ ] Prompts for avatar selection (hexagon, robot, brain, lightning, shield, custom)
- [ ] LLM provider selection shows detected API keys with checkmarks
- [ ] If Ollama is running, it shows "Detected" next to Ollama
- [ ] Connection test runs ("Testing connection to Anthropic...")
- [ ] Model configuration auto-displays (chat model, shield model, embedding)
- [ ] Workspace path defaults to `~/.openparallax/<slug>/`
- [ ] Confirmation summary shows all selections
- [ ] "Start now?" prompt

**After completion:**
- [ ] `~/.openparallax/<slug>/config.yaml` exists with correct values
- [ ] `~/.openparallax/<slug>/.openparallax/openparallax.db` exists
- [ ] `~/.openparallax/<slug>/.openparallax/canary.token` exists
- [ ] `~/.openparallax/agents.json` has an entry for this agent

### 1.2 Second Agent Setup

**Action:** Run `./dist/openparallax init jarvis`

**Observe:**
- [ ] Name prompt is skipped (positional arg used)
- [ ] Port is auto-assigned (3101 if first agent got 3100)
- [ ] Separate workspace created at `~/.openparallax/jarvis/`

### 1.3 Doctor Check

**Action:** Run `./dist/openparallax doctor`

**Observe:**
- [ ] 13 checks run with checkmarks or warnings
- [ ] Config: loads correctly
- [ ] SQLite: shows file size and WAL mode
- [ ] LLM Provider: shows provider/model
- [ ] Shield: shows policy file and Tier 2 budget
- [ ] Sandbox: shows "landlock" with capabilities (on Linux 5.13+)
- [ ] Web UI: shows port number
- [ ] Summary line at bottom

---

## 2. Multi-Agent Management

### 2.1 List Agents

**Action:** Run `./dist/openparallax list`

**Observe:**
- [ ] Table with columns: NAME, STATUS, PORT, PROVIDER, MODEL, WORKSPACE
- [ ] All initialized agents appear
- [ ] Status shows "stopped" for non-running agents

### 2.2 Start Agent (Foreground)

**Action:** Run `./dist/openparallax start` (single agent) or `./dist/openparallax start nova`

**Observe:**
- [ ] "Engine started on localhost:XXXX" message
- [ ] "Web UI available at http://127.0.0.1:3100" message
- [ ] CLI TUI appears with agent greeting
- [ ] Browser opens to Web UI automatically

### 2.3 Start Agent (Daemon)

**Action:** Run `./dist/openparallax start nova --daemon`

**Observe:**
- [ ] Prints port and "Running in background."
- [ ] Returns to shell immediately
- [ ] `openparallax list` shows status as "running"
- [ ] Web UI accessible at the printed URL

### 2.4 Stop Agent

**Action:** Run `./dist/openparallax stop nova`

**Observe:**
- [ ] "Stopped Nova (PID XXXX)" message
- [ ] `openparallax list` shows status as "stopped"
- [ ] Web UI no longer accessible

### 2.5 Delete Agent

**Action:** Run `./dist/openparallax delete jarvis`

**Observe:**
- [ ] If running: "agent is running — stop it first" error
- [ ] Confirmation prompt: "Type 'jarvis' to confirm:"
- [ ] After typing name: "Agent Jarvis deleted."
- [ ] `openparallax list` no longer shows jarvis
- [ ] `~/.openparallax/jarvis/` directory removed

### 2.6 Already Running Check

**Action:** Start an agent, then try `./dist/openparallax start nova` again

**Observe:**
- [ ] "agent is already running (PID XXXX) on port 3100" error

---

## 3. CLI TUI

### 3.1 Basic Conversation

**Action:** Start agent, type a message, press Enter

**Observe:**
- [ ] Spinner appears with "Thinking..."
- [ ] Tokens stream character-by-character
- [ ] Response appears in green text
- [ ] Cursor returns to input area

### 3.2 Slash Commands

**Action:** Type each command:

| Command | Expected Result |
|---------|-----------------|
| `/help` | Shows all available commands |
| `/status` | Shows agent name, model, session count, Shield status |
| `/new` | "New session started" — conversation history cleared |
| `/otr` | "[OTR] Off the record" notice, yellow OTR badge in header |
| `/sessions` | Lists sessions with IDs and titles |
| `/clear` | Viewport clears, history preserved (scroll up to verify) |
| `/export` | Creates `session-export-YYYY-MM-DD.md` in workspace |
| `/quit` | TUI closes, returns to shell |

### 3.3 Tool Calls in CLI

**Action:** Ask "Read the file config.yaml"

**Observe:**
- [ ] Thought line appears: `🔧 read_file(config.yaml)` or similar
- [ ] Shield verdict: `→ Shield: ALLOW (Tier 0)`
- [ ] Result: `✓ read_file completed`
- [ ] Agent responds with file contents

### 3.4 OTR Mode

**Action:** Type `/otr`, then ask "Write a file called test.txt"

**Observe:**
- [ ] Yellow `[OTR]` badge in header
- [ ] Agent attempts write_file → blocked
- [ ] `✗ OTR: Writing files is not allowed in OTR mode`
- [ ] Agent explains it can't write in OTR mode

---

## 4. Web UI

### 4.1 Layout

**Action:** Open the Web UI in a browser (http://127.0.0.1:3100)

**Observe:**
- [ ] Three-panel layout: Sidebar | ArtifactCanvas | ChatPanel
- [ ] Glassmorphism styling with semi-transparent panels
- [ ] Particle background visible through panel gaps
- [ ] Agent name and avatar in the header area

### 4.2 Chat

**Action:** Type a message in the chat input and send

**Observe:**
- [ ] Message appears in chat as user bubble
- [ ] Breathing cyan border on chat panel during streaming
- [ ] Tokens stream in real-time
- [ ] Response appears as assistant message
- [ ] Session title auto-generates after 3+ exchanges

### 4.3 Tool Call Envelope

**Action:** Ask "What files are in the current directory?"

**Observe:**
- [ ] ToolCallEnvelope appears between messages
- [ ] Collapsed: "N tool calls — N/N succeeded"
- [ ] Click to expand: shows each tool call with name, result
- [ ] Shield verdict displayed per tool call

### 4.4 Artifacts

**Action:** Ask "Create an HTML file with a hello world page"

**Observe:**
- [ ] Artifact tab appears in the canvas panel
- [ ] HTML preview renders in iframe
- [ ] Tab bar shows filename with close button
- [ ] Right-click tab → pin option
- [ ] Maximum 6 unpinned tabs (7th closes oldest)

### 4.5 Sidebar Navigation

**Action:** Click each nav item:

| Nav Item | Expected View |
|----------|---------------|
| Chat | Main chat (default) |
| Artifacts | Grid of created artifacts with cards |
| Memory | Memory search with results |
| Console | Live log entries with filter buttons |

### 4.6 Session Management

**Action:** In sidebar:
- [ ] Click "New Session" → creates session, chat clears
- [ ] Click dropdown → "OTR Session" → amber accent shift, OTR badge
- [ ] Session list shows all sessions with titles
- [ ] Click a different session → switches with history loaded
- [ ] Search box → type keyword → matching sessions appear

### 4.7 Settings Panel

**Action:** Click Settings icon in sidebar footer

**Observe:**
- [ ] Panel slides in from left (~400px wide)
- [ ] Sections: Agent Identity, LLM Provider, Shield, Memory, MCP, Email, Calendar
- [ ] Agent name/avatar editable (applies immediately)
- [ ] LLM provider/model shown with "restart required" notice
- [ ] Shield shows Tier 2 budget (editable) and usage count
- [ ] Close: X button, Escape key, or click outside

### 4.8 Resize Panels

**Action:** Drag the resize handles between panels

**Observe:**
- [ ] Sidebar width changes (stored as CSS custom property)
- [ ] Chat panel width changes
- [ ] Canvas panel fills remaining space

### 4.9 Responsive Breakpoints

**Action:** Resize browser window:

| Width | Expected Layout |
|-------|-----------------|
| >1200px | Full three-panel with padding |
| 800-1200px | Sidebar collapsed to icons, panels share space |
| <800px | Single panel, nav tabs at bottom |

### 4.10 OTR Atmosphere

**Action:** Create an OTR session

**Observe:**
- [ ] All accent colors shift from cyan to amber (~500ms transition)
- [ ] Particles slow down and turn amber
- [ ] Input area shows amber border
- [ ] Sidebar shows OTR warning indicator

### 4.11 Sandbox Badge

**Action:** Look at the input area footer

**Observe:**
- [ ] Green "🛡 Sandboxed" (if Landlock verified via canary) OR
- [ ] Gray "⚠ Unsandboxed" (if sandbox not active)
- [ ] Status is proof-based (canary probe result, not just capability)

---

## 5. Security Pipeline

### 5.1 Protected File Blocking

**Action:** Ask "Write to SOUL.md with new content"

**Observe:**
- [ ] Action blocked before Shield evaluation
- [ ] Error: "Blocked: SOUL.md is protected"
- [ ] Agent reports it cannot modify the file

### 5.2 Shield Tier Escalation

**Action:** Ask "Delete /etc/passwd" or any sensitive operation

**Observe:**
- [ ] Shield evaluates at Tier 0 (policy), escalates to Tier 1 (heuristic)
- [ ] Shield verdict shows BLOCK with reasoning
- [ ] Agent reports the action was blocked by security

### 5.3 Tier 3 Human Approval (Web UI)

**Action:** Trigger a Tier 3 action (if configured in Shield policy)

**Observe:**
- [ ] Tier3Approval card appears in chat with countdown timer
- [ ] Shows action details and Shield reasoning
- [ ] "Approve" and "Deny" buttons with pulse animation
- [ ] Clicking "Deny" → action blocked
- [ ] Auto-deny after 5 minutes if no response

### 5.4 IFC (Information Flow Control)

**Action:** Ask agent to read an SSH key, then try to send it via HTTP

**Observe:**
- [ ] File read succeeds (SSH key classified as RESTRICTED)
- [ ] HTTP request with that data → blocked by IFC
- [ ] Agent explains: "sensitive data cannot flow to this destination"

---

## 6. Tools — File Operations

### 6.1 Read File

**Action:** "Read the file README.md"

**Observe:** File contents displayed with syntax highlighting hint

### 6.2 Write File

**Action:** "Create a file called notes.txt with the text 'hello world'"

**Observe:** File created, artifact shown in canvas

### 6.3 grep_files (Content Search)

**Action:** "Search all Go files for the word 'func main'"

**Observe:**
- [ ] Structured results: file paths, line numbers, matching lines
- [ ] Results grouped by file
- [ ] .git/ and vendor/ excluded automatically
- [ ] Shows match count and file count in summary

### 6.4 List Directory

**Action:** "List the files in the current directory"

**Observe:** Formatted listing with file names and sizes

---

## 7. Tools — Shell

### 7.1 Command Execution

**Action:** "Run the command 'echo hello world'"

**Observe:**
- [ ] Command executes, stdout shown
- [ ] Duration displayed
- [ ] Artifact with terminal preview type

### 7.2 Timeout

**Action:** "Run the command 'sleep 60'" (should timeout at 30s)

**Observe:**
- [ ] Command runs for 30 seconds
- [ ] Timeout error: "command timed out after 30 seconds"
- [ ] Process killed cleanly

---

## 8. Tools — Git

**Action (in a git repository):**

| Ask | Expected |
|-----|----------|
| "What's the git status?" | Shows clean/dirty status |
| "Show the git diff" | Shows working tree changes |
| "Show the last 5 git commits" | Log with hashes, messages, dates |
| "Create a new branch called test-branch" | Branch created |
| "Switch to the main branch" | Checkout to main |

---

## 9. Tools — System (Group G)

### 9.1 Calculate

**Action:** "Calculate 23.7% of 145892"

**Observe:** Exact result: `34576.404` (not an LLM approximation)

**More calculations to test:**

| Expression | Expected |
|------------|----------|
| "2^10" | 1024 |
| "sqrt(144)" | 12 |
| "sin(90 deg)" | 1 |
| "log2(1024)" | 10 |
| "pi * 2" | 6.283185... |

### 9.2 System Info

**Action:** "Show me system information"

**Observe:**
- [ ] Disk usage with mount points and percentages
- [ ] Memory (Go heap + /proc/meminfo on Linux)
- [ ] CPU cores and model
- [ ] Network interfaces with IP addresses

### 9.3 Clipboard (requires display)

**Action:** "Copy 'hello world' to my clipboard"

**Observe:** "Copied 12 characters to clipboard"

**Action:** "What's on my clipboard?"

**Observe:** Shows "hello world"

### 9.4 Open File/URL (requires display)

**Action:** "Open the file config.yaml"

**Observe:** File opens in default text editor

**Action:** "Open https://example.com"

**Observe:** URL opens in default browser

### 9.5 Notify (requires display)

**Action:** "Send me a notification saying 'Task complete'"

**Observe:** OS notification appears with title and message

### 9.6 Screenshot (requires display)

**Action:** "Take a screenshot"

**Observe:**
- [ ] Screenshot captured as PNG
- [ ] Artifact appears in canvas with image preview
- [ ] File saved to .openparallax/tmp/

---

## 10. Tools — File Formats (Group G)

### 10.1 Archive

**Action:** "Create a zip file of the docs folder called docs.zip"

**Observe:** Archive created, size displayed

**Action:** "Extract docs.zip to a folder called docs-copy"

**Observe:** Files extracted, count displayed

### 10.2 PDF Read

**Action:** Place a PDF in the workspace, then "Read the PDF file report.pdf"

**Observe:**
- [ ] Text extracted page by page
- [ ] Page separators: `--- Page N ---`
- [ ] If >100 pages: truncation notice
- [ ] If scanned/image PDF: "contains no extractable text" message

### 10.3 Spreadsheet

**Action:** "Create an Excel file called data.xlsx with columns Name and Score, add Alice=95 and Bob=88"

**Observe:** XLSX file created with headers and data

**Action:** "Read the spreadsheet data.xlsx"

**Observe:**
- [ ] Markdown table format with aligned columns
- [ ] Shows row count and column count
- [ ] Headers in first row

**Action:** "Create a CSV file with the same data"

**Observe:** Valid CSV file created

---

## 11. Tools — Canvas

### 11.1 HTML Canvas

**Action:** "Create an HTML page with a countdown timer"

**Observe:**
- [ ] HTML file created in workspace
- [ ] Artifact tab opens in canvas
- [ ] Live preview renders the countdown in iframe

### 11.2 Multi-File Project

**Action:** "Create a website project with index.html, style.css, and script.js"

**Observe:**
- [ ] Directory created with all three files
- [ ] Artifact for the main file opens
- [ ] Files are wired together (CSS linked, JS included)

### 11.3 Live Preview

**Action:** "Start a live preview of the project"

**Observe:**
- [ ] Local HTTP server starts on a random port
- [ ] URL printed in response
- [ ] Opening the URL shows the website

---

## 12. Tools — Browser

### 12.1 Navigate

**Action:** "Browse to https://example.com"

**Observe:** Page loaded, title and initial content shown

### 12.2 Extract

**Action:** "Extract the main heading from the page"

**Observe:** Returns extracted text content

### 12.3 Screenshot

**Action:** "Take a screenshot of the browser"

**Observe:** Screenshot captured as PNG artifact

---

## 13. Sub-Agents (Group D)

### 13.1 Spawn Sub-Agent

**Action:** "Delegate a sub-agent to research what Go 1.25 features were added"

**Observe (Web UI):**
- [ ] Sub-agent dock appears at bottom of canvas
- [ ] Agent pill shows with pulsing cyan dot (working)
- [ ] Name from pool (e.g., "phoenix")
- [ ] Task description shown

**Observe (CLI):**
- [ ] "Created sub-agent phoenix" in response
- [ ] Agent continues conversation while sub-agent works

### 13.2 Check Status

**Action:** "What's the status of phoenix?"

**Observe:** Shows LLM calls, tool calls, elapsed time

### 13.3 Collect Result

**Action:** "Get the result from phoenix"

**Observe:**
- [ ] Blocks until sub-agent completes (or timeout)
- [ ] Returns the sub-agent's research findings
- [ ] Dock shows green dot (completed)
- [ ] AGENTS.md cleared when no agents active

### 13.4 Cancel Sub-Agent

**Action:** Spawn a sub-agent, then "Cancel phoenix"

**Observe:**
- [ ] Sub-agent process terminated
- [ ] Dock shows gray dot (cancelled)
- [ ] "Terminated sub-agent phoenix" confirmation

### 13.5 Concurrency Limit

**Action:** Spawn 5 sub-agents, then try a 6th

**Observe:** "Maximum 5 concurrent sub-agents" error

---

## 14. Channel Adapters (Group F)

### 14.1 Telegram

**Setup:**
1. Create a bot via @BotFather on Telegram
2. Set `TELEGRAM_BOT_TOKEN` env var
3. Add to config.yaml:
```yaml
channels:
  telegram:
    enabled: true
    token_env: TELEGRAM_BOT_TOKEN
```
4. Restart engine

**Test:**
- [ ] Message the bot → agent responds
- [ ] `/new` → "New session started"
- [ ] `/help` → shows available commands
- [ ] `/status` → shows agent status
- [ ] `/otr` → starts OTR session
- [ ] Long response → split at 4096 chars
- [ ] With `allowed_users: [YOUR_USER_ID]` → other users get "This agent is private"

### 14.2 Discord

**Setup:**
1. Create a Discord app + bot at discord.com/developers
2. Set `DISCORD_BOT_TOKEN` env var
3. Add to config.yaml:
```yaml
channels:
  discord:
    enabled: true
    token_env: DISCORD_BOT_TOKEN
    respond_to_mentions: true
```
4. Invite bot to server, restart engine

**Test:**
- [ ] @mention bot → agent responds
- [ ] Without @mention → bot ignores (if respond_to_mentions: true)
- [ ] Long response → split at 2000 chars
- [ ] `/new` → new session
- [ ] File attachment from agent → sent as Discord file

### 14.3 Signal

**Setup:**
1. Install signal-cli, register a phone number
2. Add to config.yaml:
```yaml
channels:
  signal:
    enabled: true
    cli_path: "/usr/local/bin/signal-cli"
    account: "+1234567890"
```

**Test:**
- [ ] Send message from Signal → agent responds
- [ ] `/new` → new session
- [ ] Plain text only (no markdown)

---

## 15. OAuth2 (Group E)

### 15.1 Google OAuth

**Setup:**
1. Create OAuth credentials at console.cloud.google.com
2. Add to config.yaml:
```yaml
oauth:
  google:
    client_id: "YOUR_CLIENT_ID"
    client_secret: "YOUR_SECRET"
```

**Action:** Run `./dist/openparallax auth google --account user@gmail.com`

**Observe:**
- [ ] Authorization URL printed
- [ ] Browser opens to Google consent screen
- [ ] After granting access, redirects to localhost callback
- [ ] "Connected! Account user@gmail.com authorized for google."
- [ ] Settings panel shows "oauth_accounts: [user@gmail.com]" under email

### 15.2 Email Reading (IMAP)

**Setup:** After OAuth or with app password:
```yaml
email:
  imap:
    host: "imap.gmail.com"
    port: 993
    tls: true
    auth_mode: "oauth2"  # or "password"
    account: "user@gmail.com"
```

**Test:**
- [ ] "List my recent emails" → shows subjects, senders, dates
- [ ] "Read email #42" → shows full body (plain text, HTML stripped)
- [ ] "Search emails for 'invoice'" → matching results
- [ ] "Move email #42 to Trash" → email moved
- [ ] "Mark email #42 as read" → flag set

### 15.3 MS365 Calendar

**Setup:** After Microsoft OAuth:
```yaml
calendar:
  provider: "microsoft"
  microsoft_account: "user@outlook.com"
```

**Test:**
- [ ] "What's on my calendar this week?" → lists events
- [ ] "Create a meeting tomorrow at 2pm called 'Team Sync'" → event created
- [ ] "Delete the Team Sync event" → event deleted

---

## 16. Memory System

### 16.1 Memory Write

**Action:** "Remember that the project deadline is April 15th"

**Observe:**
- [ ] Agent writes to MEMORY.md
- [ ] Timestamped entry with content
- [ ] Confirmation response

### 16.2 Memory Search

**Action:** In a new session: "When is the project deadline?"

**Observe:**
- [ ] Agent searches memory via FTS5
- [ ] Finds the previous entry
- [ ] Responds with "April 15th" from memory

### 16.3 Memory Dashboard (Web UI)

**Action:** Click "Memory" in sidebar

**Observe:**
- [ ] Search bar with live results
- [ ] Memory entries displayed as cards
- [ ] Content rendered as markdown

---

## 17. Audit & Logging

### 17.1 Audit Log

**Action:** Run `./dist/openparallax audit`

**Observe:**
- [ ] Chronological entries with types: PROPOSED, EVALUATED, EXECUTED, BLOCKED
- [ ] Each entry shows session ID, action type, timestamp
- [ ] Hash chain integrity (each entry references previous hash)

### 17.2 Audit Verification

**Action:** Run `./dist/openparallax audit --verify`

**Observe:**
- [ ] "Hash chain verified: N entries, 0 tampering detected" OR
- [ ] Warning if chain is broken

### 17.3 Engine Logs

**Action:** Start with `-v` flag, then `./dist/openparallax logs --lines 20`

**Observe:**
- [ ] Structured JSON log entries
- [ ] Events: message_received, llm_call_started, executor_start, executor_complete
- [ ] Token usage metrics per call

### 17.4 Console Viewer (Web UI)

**Action:** Navigate to Console in sidebar

**Observe:**
- [ ] Live log entries appear as agent works
- [ ] Filter buttons: All, Errors, Shield, Tool Calls
- [ ] JetBrains Mono font, terminal-style rendering
- [ ] Auto-scroll with pause-on-hover

---

## 18. Sandbox Verification

### 18.1 Canary Probe

**Action:** Start the agent, check stderr output

**Observe (Linux with kernel 5.13+):**
```
sandbox: verified (canary probe blocked on /etc/passwd)
```

**Observe (Linux with old kernel or no Landlock):**
```
sandbox: NOT active (canary probe succeeded on /etc/passwd)
```

### 18.2 Web UI Badge

**Action:** Check InputArea footer in Web UI

**Observe:**
- [ ] Green ShieldCheck "Sandboxed" = canary probe confirmed sandbox active
- [ ] Gray AlertTriangle "Unsandboxed" = canary probe showed no sandbox

### 18.3 Verify Status File

**Action:** `cat ~/.openparallax/<agent>/.openparallax/sandbox.status`

**Observe:** JSON with `verified: true/false`, `status`, `canary_path`, `mechanism`

---

## 19. Edge Cases & Error Handling

### 19.1 No Workspace Found

**Action:** Run `./dist/openparallax start` without initializing

**Observe:** "workspace not found: run 'openparallax init' first"

### 19.2 Invalid API Key

**Action:** Set an invalid API key, start agent, send message

**Observe:** Clear error about authentication failure (not a crash)

### 19.3 Network Disconnection

**Action:** Disconnect network while agent is processing

**Observe:**
- [ ] LLM call fails with timeout
- [ ] Error reported to user, not a crash
- [ ] Agent recoverable on next message

### 19.4 Very Long Input

**Action:** Paste a very long message (>10,000 chars)

**Observe:**
- [ ] Message accepted and processed
- [ ] History compaction may trigger (logged as compaction_check)
- [ ] No truncation of user input

### 19.5 Concurrent Web UI + CLI

**Action:** Have both CLI and Web UI open, send messages from both

**Observe:**
- [ ] Each has its own session
- [ ] No cross-contamination of messages
- [ ] Both receive responses independently

---

## 20. Performance Sanity Checks

### 20.1 First Response Time

**Action:** Send a simple message like "Hello"

**Observe:** Response starts streaming within 1-3 seconds (depending on provider latency)

### 20.2 Tool Call Overhead

**Action:** "Read the file config.yaml" (a tool call)

**Observe:** Tool execution completes in <100ms (file read), LLM round-trip adds ~1-2s

### 20.3 Large File Read

**Action:** "Read a 1MB file" (create one first)

**Observe:** File contents returned (may be truncated by LLM context window)

### 20.4 grep_files on Large Workspace

**Action:** "Search all files for the word 'import'" (in a large codebase)

**Observe:**
- [ ] Results return within 10 seconds (timeout cap)
- [ ] If timed out: "[Search timed out after 10s. Narrow your search...]"
- [ ] Default excludes (.git, node_modules) prevent excessive scanning

---

## Checklist Summary

Use this as a sign-off checklist:

- [ ] Init wizard completes successfully
- [ ] Multi-agent: list, start, stop, delete all work
- [ ] CLI: all 11 slash commands work
- [ ] Web UI: three-panel layout renders correctly
- [ ] Web UI: chat streaming works
- [ ] Web UI: tool call envelopes display
- [ ] Web UI: artifacts render in canvas
- [ ] Web UI: settings panel opens and saves
- [ ] Web UI: OTR mode shifts to amber
- [ ] Web UI: responsive at all breakpoints
- [ ] Shield: blocks protected files
- [ ] Shield: heuristics detect dangerous commands
- [ ] Tools: file read/write/delete/move/copy
- [ ] Tools: grep_files content search
- [ ] Tools: shell execution with timeout
- [ ] Tools: git status/diff/log/commit/branch
- [ ] Tools: calculate gives exact math results
- [ ] Tools: system_info shows disk/memory/cpu/network
- [ ] Tools: archive create/extract with zip slip protection
- [ ] Tools: spreadsheet read/write (CSV + XLSX)
- [ ] Tools: canvas create with live preview
- [ ] Sub-agents: spawn, status, result, cancel
- [ ] Sub-agent dock appears in Web UI
- [ ] Sandbox canary verifies actual isolation
- [ ] Memory write + search across sessions
- [ ] Audit log with hash chain integrity
- [ ] Doctor health check passes
- [ ] Channel adapter connects (Telegram/Discord if configured)
