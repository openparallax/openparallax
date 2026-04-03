# OpenParallax

Open-source personal AI agent with a 3-tier security pipeline, multi-channel messaging, and a glassmorphism web UI.

## Quick Reference

```bash
# Full verification (run after EVERY change, no exceptions)
make build-all && make test && make lint && cd web && npm test && cd ..

# Individual targets
make proto          # Generate gRPC code from proto/openparallax/v1/*.proto
make build-web      # npm install + vite build (output embedded via go:embed)
make build          # Go binary → dist/openparallax
make build-shield   # Shield binary → dist/openparallax-shield
make test           # go test -race -count=1 ./...
make lint           # golangci-lint run ./...
cd web && npm test  # Vitest (44 frontend tests)

# Run
./dist/openparallax init           # Interactive workspace setup
./dist/openparallax start          # Start engine + CLI + web UI
./dist/openparallax start -v       # Verbose (writes engine.log)
./dist/openparallax doctor         # 13-point health check
```

## Architecture

Three OS processes at runtime:

```
openparallax start              (process manager)
  └── internal-engine           (Engine: parent, privileged, unsandboxed)
        ├── gRPC server
        ├── HTTP/WS server
        └── internal-agent      (Agent: child, kernel-sandboxed)
              └── bubbletea TUI (opens /dev/tty directly)
```

- **Engine** (privileged): gRPC server (dynamic port), HTTP/WebSocket server (:3100), Shield, executors, memory, audit, chronicle. Spawns the Agent as a sandboxed child.
- **Agent** (sandboxed): LLM conversation, context assembly, skills, CLI TUI. Kernel-restricted via Landlock (Linux), sandbox-exec (macOS), or Job Objects (Windows).
- **Process manager** (`start` command): spawns the Engine, handles restart (exit code 75) and crash recovery.

`openparallax start` spawns `internal-engine`. Engine starts gRPC + web, then spawns `internal-agent` as a sandboxed child. The agent opens `/dev/tty` for terminal I/O and connects to the engine via gRPC.

### Pipeline Flow (per message)

```
User input → Store message → Load history → Build system prompt → Compact if needed
  → Load tools (filter if OTR) → LLM stream with tools
  → LOOP (max 25 rounds):
      TextDelta → redact secrets → emit llm_token
      ToolCall  → enrich metadata → CheckProtection → audit:PROPOSED
               → OTR check → Shield.Evaluate (Tier 0 → 1 → 2)
               → verify hash → chronicle snapshot → IFC check
               → execute (MCP or built-in) → audit:EXECUTED
               → emit action_completed + action_artifact
  → Store assistant message + thoughts → emit response_complete
  → Log to memory (non-OTR) → generate title (once, at 3+ exchanges)
```

### Transport-Neutral Entry Point

Any channel adapter calls:
```go
engine.ProcessMessageForWeb(ctx, sender, sessionID, messageID, content, mode)
```

The `EventSender` interface (`internal/engine/eventsender.go`) has one method:
```go
type EventSender interface {
    SendEvent(event *PipelineEvent) error
}
```

8 event types: `llm_token`, `action_started`, `shield_verdict`, `action_completed`, `action_artifact`, `response_complete`, `otr_blocked`, `error`.

Implementations: `grpcEventSender` (CLI), `wsEventSender` (web), and any future channel adapter.

## Project Structure

```
cmd/
  agent/              CLI binary (Cobra): start, init, status, config, session, memory, logs, audit, doctor
  shield/             Standalone Shield gRPC service

internal/
  agent/              LLM context assembly, skills, compaction
  audit/              Append-only JSONL with SHA-256 hash chain
  channels/cli/       Terminal TUI (bubbletea)
  chronicle/          Copy-on-write workspace snapshots
  config/             YAML config loader + defaults
  crypto/             ID generation, action hashing, canary tokens
  engine/             Pipeline orchestrator, gRPC server, protection, redaction
    executors/        10 executor types (file, shell, git, browser, email, calendar, canvas, memory, http, schedule)
  heartbeat/          Cron scheduler (HEARTBEAT.md)
  llm/                Provider abstraction (Anthropic, OpenAI, Google, Ollama)
  logging/            Structured JSON logging with LogHook for live broadcast
  mcp/                MCP server integration
  memory/             FTS5 + vector search, daily logs, embedding
  platform/           OS detection, shell rules, build-tagged kill
  sandbox/            Kernel-level process isolation (Landlock, sandbox-exec, Job Objects)
  session/            Session lifecycle (normal + OTR)
  shield/             3-tier security pipeline
    tier0/            YAML policy matching
    tier1/            ONNX + heuristic classifier
    tier2/            LLM evaluator with canary verification
  storage/            SQLite persistence (pure Go, zero CGo)
  templates/          Embedded workspace templates
  types/              Shared structs, action types (50+), protobuf generated code
  web/                HTTP + WebSocket server, REST handlers

proto/openparallax/v1/
  pipeline.proto      PipelineService (11 RPCs)
  shield.proto        ShieldService (3 RPCs)
  types.proto         Shared enums + messages

web/                  Svelte 4 + TypeScript + Vite frontend
  src/
    components/       14 Svelte components
    stores/           Reactive state (messages, artifacts, session, connection, console, settings)
    lib/              websocket.ts, api.ts, types.ts, format.ts
    __tests__/        Vitest test suite
  app.css             Global design tokens, glass system, OTR override
```

## Hard Rules

These are non-negotiable. Every commit, every file, every line.

1. **Zero TODO/FIXME.** No `// TODO`, `// HACK`, `// XXX`, `// TEMP`, `// PLACEHOLDER` in any file.
2. **No planning traces.** No `// Will be implemented in Chunk N`, `// Future: add X`. Comments describe what the code IS.
3. **No AI attribution.** No `// Generated by Claude`, `// AI-assisted`.
4. **No dead code.** No commented-out blocks, unused functions, unreachable branches, unused imports/variables/parameters.
5. **Conventional commits.** `type: description` — types: `feat`, `fix`, `refactor`, `test`, `chore`, `docs`. Lowercase, present tense, no period.
6. **gofmt + go vet + golangci-lint = zero issues.** No `//nolint` unless genuine false positive with explanation.
7. **GoDoc on every exported symbol.** Comment starts with the symbol name.
8. **Tests are real.** Meaningful assertions, integration tests with real LLM + real filesystem. No `assert.True(true)`.
9. **Race detection passes.** `go test -race` with zero warnings.
10. **No skipped tests.** No `t.Skip()` in committed code (conditional skip on missing env vars is OK).
11. **Fail-closed everywhere.** Every Shield error path returns BLOCK. Missing policy: BLOCK. Heuristic error: BLOCK. Evaluator unreachable: BLOCK.
12. **Zero CGo.** `CGO_ENABLED=0`. Pure Go SQLite via `modernc.org/sqlite`. Single static binary.
13. **Platform-specific code uses build tags.** `//go:build` directive. Files come in pairs/triples. No `runtime.GOOS` switches.
14. **Full verification after every change.** `make build-all && make test && make lint && cd web && npm test && cd ..` — all must pass before committing. No exceptions.
15. **Engineer stages and commits, owner pushes.** Never push directly.
16. **Escalate before:** adding dependencies (`go get`), protobuf changes, config schema changes, spec deviations.
17. **No workarounds.** Fix the actual problem. No retry loops to mask bugs, no special-case hacks.

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Language | Go 1.25, TypeScript 5.5 |
| CLI | Cobra + Bubbletea (TUI) |
| Frontend | Svelte 4, Vite 5, Vitest |
| Transport | gRPC (CLI ↔ engine), WebSocket + REST (web ↔ engine) |
| Database | SQLite (modernc.org, pure Go, WAL mode) |
| Search | FTS5 full-text + vector embeddings |
| LLM | Anthropic, OpenAI, Google, Ollama |
| Security | 3-tier Shield (YAML policy → ONNX+heuristic → LLM evaluator) |
| Audit | Append-only JSONL with SHA-256 hash chain |
| Snapshots | Copy-on-write workspace chronicle |
| Icons | lucide-svelte |
| Fonts | Exo 2 (body), JetBrains Mono (code/badges) |
| Sanitization | DOMPurify (HTML), marked (markdown) |
| Protobuf | protoc with go + go-grpc plugins |
| Lint | golangci-lint v2 (.golangci.yml) |

## Key Patterns

### Adding a New Channel Adapter

Follow the CLI adapter pattern (`internal/channels/cli/adapter.go`):

1. Implement `EventSender` for your transport
2. On incoming message: create/lookup session → call `engine.ProcessMessageForWeb(ctx, sender, sid, mid, content, mode)`
3. Handle events from your `EventSender` → format and send back to the user
4. Register the adapter startup in `cmd/agent/start.go`

### Shield Evaluation Flow

```
Tier 0 (policy.yaml)  →  DENY → BLOCK immediately
                         ALLOW (minTier ≤ 0) → ALLOW immediately
                         ESCALATE → continue
Tier 1 (ONNX+heuristic) → BLOCK → BLOCK
                           ALLOW (minTier ≤ 1) → ALLOW
                           otherwise → continue
Tier 2 (LLM evaluator)  → final decision (rate-limited, daily budget)
```

### Kernel Sandbox

The Agent process is kernel-sandboxed — it physically cannot read files, make network calls, or spawn children outside its allowed scope. Defense in depth on top of Shield.

| Platform | Mechanism | Filesystem | Network | Spawn |
|----------|-----------|-----------|---------|-------|
| Linux 5.13+ | Landlock LSM | blocked | V4+ (6.7+) | blocked |
| macOS | sandbox-exec | blocked | blocked | blocked |
| Windows | Job Objects | no restriction | no restriction | blocked |

Sandbox is best-effort: if unavailable (old kernel, unsupported platform), the agent starts normally. Never fails to start because of sandboxing.

- `internal/sandbox/` — interface + 4 platform implementations (build-tagged)
- Agent self-sandboxes on Linux via `ApplySelf()` before any gRPC calls
- Engine wraps Agent spawn via `WrapCommand()` on macOS/Windows
- `GET /api/status` includes `sandbox` field
- `openparallax doctor` reports sandbox capabilities

### Process Manager Restart

Exit code 75 from the engine signals a restart request (not a crash). The process manager in `cmd/agent/start.go` respawns the engine without impacting the crash budget. Used by `/restart` command and `POST /api/restart`.

### OTR Mode

- `.otr` CSS class on root toggles all `--accent-*` tokens from cyan to amber
- OTR sessions filter tools at definition level (no filesystem writes, no memory persistence)
- OTR session data lives in `sync.Map` (never hits SQLite)

### Web UI Layout

Three-panel glassmorphism: `Sidebar (240px) | ArtifactCanvas (flex:1) | ChatPanel (380px)`

- Drag-to-resize with CSS custom properties (`--sw`, `--cw`)
- Responsive breakpoints: full (>1200px), compact (800-1200px), mobile (<800px)
- Artifact tabs: 6 max (unpinned), pinning via right-click, localStorage persistence
- WebSocket events filtered by `session_id` to prevent cross-session corruption
- `log_entry` events are global (processed before session filter)

### CSS Token System

All accent colors use `--accent-*` tokens (8 variants: base, dim, subtle, ghost, glow, glow-strong, border, border-active). The `.otr` class overrides all 8 to amber. Never use hardcoded `rgba(0, 220, 255, ...)` in components — use tokens.

## Environment Variables

| Variable | Purpose |
|----------|---------|
| `ANTHROPIC_API_KEY` | Anthropic Claude API |
| `OPENAI_API_KEY` | OpenAI API (also used for embeddings) |
| `GOOGLE_AI_API_KEY` | Google Gemini API |

Set in shell profile. Referenced by `config.yaml` via `api_key_env` fields.

## Config

Workspace config lives at `<workspace>/config.yaml`. Created by `openparallax init`. Key sections:

- `llm` — provider, model, api_key_env
- `shield.evaluator` — provider, model for Tier 2
- `shield.policy_file` — path to YAML policy
- `identity.name` — agent display name (default: "Atlas")
- `web.enabled`, `web.port` — web UI toggle and port (default: 3100)
- `channels` — WhatsApp, Telegram, Discord, Slack, Signal, Teams (enable + credentials)
- `memory.embedding` — provider + model for vector search
- `mcp.servers` — external MCP tool servers
- `chronicle.max_snapshots` — rollback budget

## Testing

Go tests use `testify` for assertions. Tests are integration-style — they hit real filesystems, real SQLite, and (when env vars are set) real LLM APIs. Frontend tests use Vitest + jsdom.

```bash
# Run everything
make build-all && make test && make lint && cd web && npm test && cd ..

# Run specific Go package tests
go test -race -count=1 ./internal/shield/...
go test -race -count=1 ./internal/engine/...

# Run frontend tests
cd web && npx vitest run
cd web && npx vitest run src/__tests__/specific.test.ts
```

## CLI Commands

| Command | Description |
|---------|------------|
| `start` | Start engine + CLI + web UI |
| `init` | Interactive workspace setup wizard |
| `status` | Show workspace stats |
| `config` | Print config (secrets masked) |
| `session delete [id\|--all]` | Delete sessions |
| `memory [show\|search]` | List, read, or search memory |
| `logs` | Tail engine.log (`--level`, `--event`, `--lines`) |
| `audit` | Query audit log (`--verify`, `--session`, `--type`) |
| `doctor` | 12-point system health check |
| `restart` | Stop + restart engine |

### Slash Commands (Web UI + CLI)

`/help`, `/new`, `/otr`, `/quit`, `/clear`, `/status`, `/restart`, `/export`, `/delete`, `/sessions`
