# OpenParallax Testing Guide

Complete guide for testing every feature implemented across Groups A through G, plus the core platform.

---

## Prerequisites

### System Requirements

```bash
# Required
go version    # Go 1.25+
node --version # Node.js 20+
protoc --version # Protocol Buffers compiler

# Verify CGo is disabled
CGO_ENABLED=0 go build ./...

# Optional (for integration tests)
export ANTHROPIC_API_KEY="sk-ant-..."    # Anthropic Claude
export OPENAI_API_KEY="sk-..."           # OpenAI (some tests skip without this)
export GOOGLE_AI_API_KEY="AIza..."       # Google Gemini
```

### Build Everything First

```bash
make build-all
# This runs: make proto → npm build → go build (agent + shield binaries)
```

---

## Quick Verification (Run After Every Change)

```bash
make build-all && make test && make lint && cd web && npm test && cd ..
```

This runs:
1. Protobuf code generation
2. Vite frontend build (embeds into Go binary)
3. Go binary compilation (agent + shield)
4. `go test -race -count=1 ./...` (all Go tests with race detection)
5. `golangci-lint run ./...` (zero issues required)
6. `vitest run` (all 69 frontend tests)

**Expected output:** All packages OK, 0 lint issues, all vitest tests pass.

---

## Test Inventory

| Layer | Tests | Packages |
|-------|-------|----------|
| Go backend | 670 | 29 packages |
| Frontend (Vitest) | 69 | 8 test files |
| **Total** | **739** | **37** |

---

## 1. Core Platform Tests

### Storage (SQLite)

```bash
go test -race -count=1 -v ./internal/storage/...
```

**What it tests:**
- Database creation and WAL mode
- Session CRUD (insert, get, list, delete, rename)
- Message persistence with timestamps
- FTS5 full-text search (index, search, no-match, clear+reindex)
- Chronicle snapshots (insert, get, hash chain)
- Audit entries (insert, count, timestamp index)
- OAuth tokens table creation (new in Group E)
- Cascade deletes (session → messages)

**Expected:** 19 tests pass, database created in temp dir.

### Configuration

```bash
go test -race -count=1 -v ./internal/config/...
```

**What it tests:**
- Valid YAML loading with all fields
- Validation: missing provider, model, API key
- Ollama without API key (allowed)
- Invalid provider names, ONNX thresholds, rate limits
- Path resolution (tilde, relative, absolute)
- Default values applied correctly

**Expected:** 14 tests pass.

### Cryptography

```bash
go test -race -count=1 -v ./internal/crypto/...
```

**What it tests:**
- SHA-256 hashing (known values, determinism)
- Canonical JSON serialization (key ordering, nested maps)
- Action hashing (deterministic, different for different payloads)
- Canary token generation (format, uniqueness)
- Canary verification (present, absent, partial, wrong length)
- UUID generation (format, uniqueness)
- Random hex generation
- **AES-256-GCM encryption** (Group E): round-trip, wrong key, truncated/tampered ciphertext, nonce uniqueness, HKDF key derivation

**Expected:** 27 tests pass.

### Types

```bash
go test -race -count=1 -v ./internal/types/...
```

**What it tests:**
- Action type count (69 types — verify all registered)
- Action type uniqueness (no duplicates)
- Goal type count (13 types)
- Verdict expiry (before/after TTL)
- Sentinel errors are distinct
- Session mode values (normal/otr)
- AgentConfig YAML round-trip
- Default identity values
- Memory file count (8 entries)
- Messaging platform count (9 entries)

**Expected:** 11 tests pass. If the action count is wrong, a new action type was added/removed without updating AllActionTypes.

---

## 2. Security Pipeline (Shield)

### Tier 0 — Policy Rules

```bash
go test -race -count=1 -v ./internal/shield/tier0/...
```

**What it tests:**
- Default, strict, and permissive policy loading
- DENY blocks SSH key access
- ALLOW permits workspace reads
- VERIFY escalates SOUL.md writes and shell commands
- Glob pattern matching (SSH wildcard)
- Missing policy file handling

**Expected:** 11 tests pass.

### Tier 1 — Heuristic Classifier

```bash
go test -race -count=1 -v ./internal/shield/tier1/...
```

**What it tests:**
- Detections: curl|bash, reverse shell, base64 decode, prompt injection, system message spoof, path traversal, null byte, private key, AWS key, zero-width chars, jailbreak, instruction override, webhook exfil
- Non-detections: benign echo, ls, cat readme
- Rule count validation (all rules loaded)
- Dual classifier with ONNX unavailable
- Block/combine logic

**Expected:** 22 tests pass.

### Tier 2 — LLM Evaluator

```bash
go test -race -count=1 -v ./internal/shield/tier2/...
```

**What it tests:**
- Prompt loading with canary injection
- Hash integrity verification
- Response parsing (valid, block, malformed JSON, code fences)
- Canary verification (missing = block, present = pass)

**Expected:** 9 tests pass (no LLM calls — uses mock evaluator).

### Shield Pipeline Integration

```bash
go test -race -count=1 -v ./internal/shield/...
```

**What it tests:**
- Gateway: deny rule blocks, allow rule approves
- Heuristic: blocks curl|bash pipes
- SOUL.md write escalation
- Verdict hash verification
- Rate limiter
- IFC flow matrix (public→HTTP allowed, confidential→HTTP blocked, restricted→anything blocked)

**Expected:** 36 tests across 4 sub-packages.

### Protection Layer (Hardcoded Rules)

```bash
go test -race -count=1 -v -run TestProtection ./internal/engine/...
```

**What it tests (82 tests):**
- SOUL.md: write/delete/copy/move blocked, read allowed
- IDENTITY.md, TOOLS.md, BOOT.md: write blocked, read allowed
- Shell injection to protected files (redirect, cp, tee, rm, del)
- config.yaml, audit.jsonl, .openparallax/ dir: read blocked
- HEARTBEAT.md, AGENTS.md: escalate to Tier 2
- USER.md, MEMORY.md: WriteTier1Min
- Case-insensitive protection (lowercase, mixed case)
- Windows-specific commands (copy, Set-Content, del)
- Normal file operations: allowed
- Path traversal through symlinks

**Expected:** 82 tests pass.

---

## 3. Engine Pipeline

### Core Engine

```bash
go test -race -count=1 -v ./internal/engine/...
```

**What it tests:**
- Engine start and stop lifecycle
- gRPC health check
- Process message (basic + empty content)
- Shutdown RPC
- Full pipeline: read file with tool use
- Conversation mode (multi-turn)
- Sub-agent name pool (uniqueness, exhaustion fallback)
- Sub-agent system prompt generation
- Sub-agent tool filtering (agents/schedule/memory excluded)
- AGENTS.md write/clear lifecycle
- Tier 3 HITL: approve, deny, timeout, rate limit, context cancellation

**Expected:** 141 tests pass (includes protection + tier3 + subagent tests).

### Tier 3 Human-in-the-Loop

```bash
go test -race -count=1 -v -run TestTier3 ./internal/engine/...
```

**What it tests:**
- Approve flow (submit → decide → result)
- Deny flow
- Auto-deny timeout (5 minutes)
- Rate limit (10/hour)
- Rate limit not exceeded
- Unknown action ID handling
- Hourly remaining count
- Pending list
- Context cancellation

**Expected:** 9 tests pass.

---

## 4. Executors (Tools)

### All Executors

```bash
go test -race -count=1 -v ./internal/engine/executors/...
```

**Expected:** 216+ tests across 13 test files.

### File Operations

```bash
go test -race -count=1 -v -run "TestFile|TestDetectLang|TestDetectPreview" ./internal/engine/executors/...
```

**What it tests:**
- Read existing/nonexistent file
- Write creates file + parent dirs
- Delete, move, copy
- Create directory
- List directory (flat + recursive)
- Search files (glob pattern)
- Tilde expansion, relative path resolution
- Language detection (Go, Python, JS, Rust, etc.)
- Preview type detection (code, html, markdown, image)

### grep_files (Content Search)

```bash
go test -race -count=1 -v -run TestGrep ./internal/engine/executors/...
```

**What it tests:**
- Literal string search
- Regex pattern search
- Include glob (*.go only)
- Exclude glob
- Default excludes (.git, vendor, node_modules)
- Case-insensitive search
- No matches → clean message
- Invalid regex → clear error
- Binary file detection and skipping
- Max results cap

**Expected:** 12 tests pass.

### Shell Executor

```bash
go test -race -count=1 -v -run TestShell ./internal/engine/executors/...
```

**What it tests:**
- Echo command execution
- 30-second timeout with process kill
- Nonexistent command → error
- Empty command → error

### Git Executor

```bash
go test -race -count=1 -v -run TestGit ./internal/engine/executors/...
```

**What it tests:**
- Status, diff (working + staged), log (default + limit)
- Commit (with message, specific files, missing message)
- Branch (list, create, switch, missing name, invalid action)
- Checkout (valid ref, missing ref)
- Custom repository path
- Not-a-repo error

**Expected:** 19 tests pass. Creates temp git repos.

### HTTP Executor

```bash
go test -race -count=1 -v -run TestHTTP ./internal/engine/executors/...
```

**What it tests:**
- GET, POST, DELETE requests (mock HTTP server)
- Custom headers
- Empty/missing URL
- Default method is GET
- 4xx returns failure
- Response truncation at limit

### Email Executor (SMTP + IMAP)

```bash
go test -race -count=1 -v -run "TestEmail|TestNewEmailExecutor|TestParseRecipients|TestToolSchemas" ./internal/engine/executors/...
```

**What it tests:**
- **SMTP sending:** single/multiple recipients, CC, reply-to, missing fields, provider error
- **IMAP reading (Group E):** list, read, search, move, mark with mock IMAPReader
- **Dynamic tool schemas:** SMTP-only → 1 tool, IMAP-only → 5 tools, both → 6 tools
- HTML stripping logic
- Email body truncation at 10k chars
- Constructor nil checks (no config, no host, both nil)

### Calendar Executor (+ MS365)

```bash
go test -race -count=1 -v -run "TestCalendar|TestMS365|TestParseMS365|TestMapWindows" ./internal/engine/executors/...
```

**What it tests:**
- **Calendar operations:** read empty/with events, custom days, create (valid/invalid times), update (valid/missing ID), delete (valid/missing ID)
- **MS365 (Group E):** event mapping (MS365 ↔ CalendarEvent), timezone parsing (IANA + Windows names + unknown fallback), HTTP mock for list/create, HTML body stripping
- Constructor nil checks (unconfigured, no creds, microsoft without OAuth)

### System Tools (Group G)

```bash
go test -race -count=1 -v -run "TestSystem|TestClipboard|TestOpen|TestNotify|TestIsWithin|TestFormatFileSize" ./internal/engine/executors/...
```

**What it tests:**
- Clipboard write: empty content, too large (1MB limit)
- Open: rejects unsupported schemes (file://, javascript:), rejects paths outside workspace, empty target
- Notify: empty fields, rate limiting (5/minute)
- System info: all/memory/cpu/network/disk categories, unknown category error
- Path containment validation
- File size formatting

**Note:** Clipboard read/write, notify, and screenshot tests don't call actual OS commands — they test validation, rate limiting, and error paths. Integration tests require a display server.

### Calculate Tool (Group G)

```bash
go test -race -count=1 -v -run "TestEvaluate|TestFormat|TestCalculate" ./internal/engine/executors/...
```

**What it tests:**
- Basic arithmetic with operator precedence: `2 + 3 * 4 = 14`
- Percentage: `23.7% of 145892 = 34576.404`
- Exponents: `2^10 = 1024`
- Square root: `sqrt(144) = 12`
- Trigonometry: `sin(90 deg) = 1`, `cos(0) = 1`
- Logarithms: `log2(1024) = 10`, `log10(100) = 2`, `ln(e) = 1`
- Constants: `pi * 2`, `e ^ 1`
- Absolute value: `abs(-42) = 42`
- Nested parentheses
- Errors: division by zero, invalid expression, unmatched parens, unknown function, sqrt(-1)
- Result formatting (no trailing zeros)

### File Format Tools (Group G)

```bash
go test -race -count=1 -v -run "TestArchive|TestZipSlip|TestCSV|TestTSV|TestXLSX|TestSpreadsheet|TestFileFormat" ./internal/engine/executors/...
```

**What it tests:**
- **Archive:** Create + extract zip, create + extract tar.gz, unsupported format error
- **Zip slip protection:** Archive with `../../etc/passwd` path → rejected
- **CSV/TSV:** Write with headers, read back, verify data
- **XLSX:** Write with headers, read back, verify data
- **Spreadsheet limits:** max_rows truncation with count notice
- **Error handling:** unsupported format, empty rows, corrupt files

### Tool Groups (load_tools)

```bash
go test -race -count=1 -v ./internal/engine/executors/ -run TestGroup
```

**What it tests:**
- Register + lookup
- Lookup missing group
- Available groups count
- LoadToolsDefinition contains all groups
- ResolveGroups: valid groups, invalid group error
- OTR filtering (write_file, delete_file, clipboard_write blocked)
- DefaultGroups from schemas (files, shell, git, system, utilities, agents present)

---

## 5. OAuth2 Token Manager (Group E)

```bash
go test -race -count=1 -v ./internal/oauth/...
```

**What it tests:**
- Store + retrieve round-trip (encrypted)
- Expired token triggers refresh (mock HTTP)
- Proactive refresh (within 5-minute buffer)
- Refresh failure (revoked token) → ErrTokenRevoked + tokens deleted
- Refresh with rotated refresh token (Microsoft pattern)
- RevokeTokens removes entries
- HasTokens returns correct bool
- ListAccounts returns all accounts for a provider
- **Encrypted storage verification:** raw DB read confirms no plaintext
- Concurrent access (10 goroutines)
- Overwrite existing tokens

**Expected:** 11 tests pass.

### Verifying Encrypted Storage Manually

```bash
# After running the agent and authorizing with OAuth:
sqlite3 ~/.openparallax/<agent>/.openparallax/openparallax.db \
  "SELECT provider, account, hex(access_token_enc) FROM oauth_tokens;"
# Should show hex blob, NOT plaintext token
```

---

## 6. Multi-Agent System (Group D)

### Agent Registry

```bash
go test -race -count=1 -v ./internal/registry/...
```

**What it tests:**
- Load missing file (creates empty)
- Add + save + reload round-trip
- Duplicate name/port rejection
- Remove by slug
- Lookup (case-insensitive by name and slug)
- Port allocation (sequential from 3100)
- FindSingle (0, 1, 2+ agents)
- PID write/read/remove cycle
- IsRunning with current process PID
- Stale PID cleanup
- Migration: zero/one/multiple workspaces, sentinel prevents re-run, skips "workspace" dir

**Expected:** 17 tests pass.

### Multi-Agent CLI (Manual Testing)

```bash
# Initialize two agents
./dist/openparallax init nova
./dist/openparallax init jarvis

# List agents
./dist/openparallax list
# Expected output:
#   NAME    STATUS   PORT   PROVIDER    MODEL               WORKSPACE
#   Nova    stopped  3100   anthropic   claude-sonnet-4-...  ~/.openparallax/nova
#   Jarvis  stopped  3101   anthropic   claude-sonnet-4-...  ~/.openparallax/jarvis

# Start in daemon mode
./dist/openparallax start nova --daemon
./dist/openparallax list  # Nova should show "running"

# Stop
./dist/openparallax stop nova

# Delete (requires confirmation)
./dist/openparallax delete jarvis --yes

# Verify registry
cat ~/.openparallax/agents.json
```

### Sub-Agent System

```bash
go test -race -count=1 -v -run "TestPickName|TestSubAgent|TestIsExcluded|TestTruncateResult|TestWriteAndClear|TestItoa|TestFilterSubAgent" ./internal/engine/...
```

**What it tests:**
- Name pool: unique names, exhaustion fallback (phoenix-2)
- System prompt generation
- Tool filtering (agents/schedule/memory/load_tools excluded)
- Excluded group detection
- AGENTS.md write + clear
- Result truncation

### Sub-Agent Web UI

```bash
cd web && npx vitest run src/__tests__/subagents.test.ts
```

**What it tests:**
- Store starts empty
- Add sub-agent (with/without tool groups)
- Progress updates change status to "working"
- Complete sets result + completedAt
- Fail sets error
- Cancel sets status
- Dismiss removes from store
- Multiple agents tracked independently
- Unknown agent events ignored

**Expected:** 11 tests pass.

---

## 7. Channel Adapters (Group F)

### Shared Infrastructure

```bash
go test -race -count=1 -v ./internal/channels/...
```

**What it tests:**
- Message splitting (short, long, at newlines)
- MaxMessageLen per platform (Telegram 4096, Discord 2000)

### Telegram

```bash
go test -race -count=1 -v ./internal/channels/telegram/...
```

**What it tests:**
- MarkdownV2 escaping (`_`, `*`, `[`, `)`, etc.)
- Nil when disabled/nil config/no token
- Rate limiting (30/min/user, expiry)
- Access control (allowed_users filtering)
- Update JSON parsing
- Message splitting at 4096 chars

### WhatsApp

```bash
go test -race -count=1 -v ./internal/channels/whatsapp/...
```

**What it tests:**
- Webhook verification challenge (valid/invalid token)
- Webhook payload JSON parsing
- Access control (allowed_numbers)
- API payload structure
- Nil when disabled/nil config

### Discord

```bash
go test -race -count=1 -v ./internal/channels/discord/...
```

**What it tests:**
- Nil when disabled/nil config/no token
- Channel and user filtering
- Message splitting at 2000 chars

### Signal

```bash
go test -race -count=1 -v ./internal/channels/signal/...
```

**What it tests:**
- Nil when disabled/nil config/no CLI
- JSON-RPC message parsing
- Access control (allowed_numbers)

### Manual Channel Testing

**Telegram (easiest to test live):**
1. Create a bot via @BotFather on Telegram
2. Add to config.yaml:
   ```yaml
   channels:
     telegram:
       enabled: true
       token_env: TELEGRAM_BOT_TOKEN
       allowed_users: []  # empty = allow all
   ```
3. `export TELEGRAM_BOT_TOKEN="123456:ABC..."`
4. `./dist/openparallax start`
5. Message the bot on Telegram → agent responds
6. Test `/new`, `/otr`, `/status`, `/help` commands

---

## 8. Frontend Tests

```bash
cd web && npm test
# Or with verbose output:
cd web && npx vitest run --reporter=verbose
```

### Test Files

| File | Tests | What it covers |
|------|-------|----------------|
| `messages.test.ts` | 11 | User messages, streaming tokens, tool calls, verdicts, artifacts, finalize |
| `session.test.ts` | 6 | Session store, currentSessionId, OTR mode toggle, derived stores |
| `connection.test.ts` | 4 | WebSocket connection state |
| `artifacts.test.ts` | 9 | Tab management, 6-tab limit, pin/unpin, clear |
| `settings.test.ts` | 5 | Settings panel open/close, nav items |
| `format.test.ts` | 9 | Markdown rendering, HTML sanitization, timestamp formatting |
| `ux.test.ts` | 14 | System messages, pin tabs, console logs, token counting |
| `subagents.test.ts` | 11 | Sub-agent lifecycle (add/progress/complete/fail/cancel/dismiss) |

**Expected:** 69 tests pass.

---

## 9. Supporting Systems

### Memory

```bash
go test -race -count=1 -v ./internal/memory/...
```

**What it tests:**
- Markdown chunking (small/large content, overlap, line numbers)
- File read/append/reindex
- FTS5 search (match/no-match)
- Daily log creation
- Vector search (cosine similarity, insert/search/delete)

### Sessions

```bash
go test -race -count=1 -v ./internal/session/...
```

**What it tests:**
- Normal session creation + persistence
- OTR session creation (in-memory only)
- List excludes OTR sessions
- Delete, rename, get history
- Auto-title generation
- OTR allows/blocks (16 action types tested)

### Chronicle (Snapshots)

```bash
go test -race -count=1 -v ./internal/chronicle/...
```

**What it tests:**
- Snapshot before file write
- Metadata in SQLite
- Rollback restores file
- Diff detection (modification, deletion)
- Hash chain integrity
- Retention pruning

### Audit

```bash
go test -race -count=1 -v ./internal/audit/...
```

**What it tests:**
- JSONL log creation
- SHA-256 hash chain (each entry hashes the previous)
- Chain verification (valid + tampered)
- Viewer filters (event type, session)
- Chain continuity across restarts

### Heartbeat (CRON)

```bash
go test -race -count=1 -v ./internal/heartbeat/...
```

**What it tests:**
- Cron entry parsing (valid, invalid skipped)
- Cron matching (exact, step patterns)
- Deduplication

### Platform Detection

```bash
go test -race -count=1 -v ./internal/platform/...
```

**What it tests:**
- Current platform detection
- Path normalization (tilde, forward slashes, clean)
- IsWithinDirectory (true, equal, traversal, sibling prefix, relative)
- Sensitive paths list
- Shell config (command + flag per OS)
- Shell injection rules (count, platform-specific)

### Sandbox

```bash
go test -race -count=1 -v ./internal/sandbox/...
```

**What it tests:**
- New() returns implementation
- Mode detection
- Config defaults
- Probe for system capabilities
- Status fields
- WrapCommand no-op
- Platform availability (build-tag tests)

---

## 10. Integration Tests (Require API Keys)

These tests call real LLM APIs and skip gracefully without keys:

```bash
# Set at least one:
export OPENAI_API_KEY="sk-..."

# Run integration tests
go test -race -count=1 -v -run Integration ./internal/llm/...
go test -race -count=1 -v -run TestEngineFullPipeline ./internal/engine/...
```

**What they test:**
- Real LLM completion (streaming and non-streaming)
- Tool-use round-trip with real LLM
- Full pipeline: user message → LLM → tool call → execution → response

**Expected:** Tests skip with "API key not set" if env vars are missing. This is normal for CI.

---

## 11. Race Detection

All tests run with `-race` by default via `make test`. Pay special attention to:

```bash
# Sub-agent manager (concurrent spawns, status checks)
go test -race -count=5 -run TestTier3 ./internal/engine/...

# OAuth manager (concurrent token access)
go test -race -count=5 ./internal/oauth/...

# Registry (concurrent writes)
go test -race -count=5 ./internal/registry/...

# Channel adapters (rate limiting with concurrent messages)
go test -race -count=5 ./internal/channels/telegram/...
```

---

## 12. End-to-End Manual Testing

### Full Startup

```bash
./dist/openparallax init
# Follow wizard: name, avatar, provider, API key, workspace
# On completion: "Workspace initialized!"

./dist/openparallax start
# Expected: engine starts, web UI opens at http://127.0.0.1:3100
# CLI TUI appears with agent greeting
```

### Web UI Checklist

1. **Three-panel layout:** Sidebar (240px) | ArtifactCanvas (flex) | ChatPanel (380px)
2. **Send message:** Type and send → streaming response with breathing border
3. **Tool calls:** Ask agent to read a file → ToolCallEnvelope shows tool execution
4. **Artifacts:** Ask agent to create an HTML file → appears in canvas with tabs
5. **Settings:** Click settings → slide-over panel with provider, model, Shield config
6. **OTR mode:** Type `/otr` → amber accent shift, write tools filtered
7. **Sub-agent dock:** Ask agent to delegate a task → dock appears at bottom of canvas
8. **Console:** Navigate to Console view → live log entries
9. **Memory:** Navigate to Memory view → search past conversations
10. **Sessions:** Sidebar shows session list, click to switch

### CLI Checklist

1. **Slash commands:** `/new`, `/otr`, `/status`, `/help`, `/sessions`, `/quit`
2. **Streaming:** Tokens appear character-by-character
3. **Tool calls:** Visible as thought entries in the viewport
4. **Session switching:** `/sessions` lists, `/switch <id>` switches

### Doctor Check

```bash
./dist/openparallax doctor
# Runs 13-point health check:
# ✓ config.yaml exists
# ✓ database accessible
# ✓ LLM provider reachable
# ✓ Shield pipeline operational
# ✓ Sandbox capabilities
# ... etc.
```

---

## 13. Performance & Stress

### Large Workspace grep

```bash
# Create a large workspace with many files
go test -race -count=1 -v -run TestGrepMaxResults ./internal/engine/executors/...
# Verify timeout protection (10-second cap)
```

### Concurrent Sub-Agents

```bash
# Via the agent, ask it to spawn 5 sub-agents simultaneously
# Verify: all 5 start, 6th is rejected with clear error
# Verify: AGENTS.md tracks all active agents
# Verify: web dock shows all agents with status dots
```

### Channel Rate Limiting

```bash
# Via Telegram, send 31 messages in quick succession
# Verify: 31st message gets "Rate limit exceeded" response
# Verify: after 60 seconds, messages are accepted again
```

---

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| `go test` fails with `CGO_ENABLED` error | CGo dependency detected | Verify all deps are pure Go |
| Lint fails with `gofmt` | File not formatted | Run `gofmt -w <file>` |
| Frontend tests fail | Stale build | Run `cd web && npm install` |
| OAuth tests fail | Missing canary token | Ensure `.openparallax/canary.token` exists |
| Engine tests skip | No API key in env | Set `OPENAI_API_KEY` for integration tests |
| Sandbox tests skip | Wrong platform | Expected — tests are build-tag gated |
| Channel tests fail | Missing bot token | Expected — channel integration requires real tokens |
