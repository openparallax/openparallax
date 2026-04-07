# Changelog

All notable changes to OpenParallax are documented here. This project follows [conventional commits](https://www.conventionalcommits.org/) and [semantic versioning](https://semver.org/).

---

## v0.1.0 — Initial Release

The first public release of OpenParallax. A reference implementation of the architecture described in [*Parallax: Why AI Agents That Think Must Never Act*](https://github.com/openparallax/openparallax) (forthcoming on arXiv).

### Architecture

- **Agent-Engine separation** — the Agent process owns the LLM session and runs inside a kernel sandbox. The Engine is the security gate that evaluates and executes every tool call. The Agent proposes actions but never executes them directly.
- **Three-process model** — Process Manager spawns Engine, Engine spawns sandboxed Agent. Clean restart via exit code 75. Crash recovery with budget.
- **Single pipeline** — all message processing routes through the Agent's bidirectional gRPC stream (RunSession). No fallback paths.
- **gRPC services** — AgentService (bidirectional streaming between Agent and Engine), ClientService (server streaming to CLI/web clients), SubAgentService (parallel task execution with follow-up messaging).
- **EventBroadcaster** — fan-out of 13 pipeline event types to all subscribed clients by session. Supports session-scoped and global subscriptions.
- **Transport-neutral entry point** — any channel adapter calls `ProcessMessageForWeb()` with an `EventSender` implementation.
- **Modular engine** — engine split across 5 focused files (engine.go, engine_pipeline.go, engine_grpc.go, engine_session.go, engine_tools.go).

### Security

- **4-tier Shield pipeline** — Tier 0 (YAML policy matching), Tier 1 (ONNX DeBERTa classifier + 58 platform heuristic rules + 21 cross-platform detection rules), Tier 2 (LLM evaluator with canary verification), Tier 3 (human-in-the-loop approval via all connected channels). Fail-closed at every tier.
- **In-process ONNX classifier** — DeBERTa v3 model runs in pure Go via `onnxruntime-purego`. No sidecar processes, no CGo. Auto-resolves latest ONNX Runtime from GitHub releases.
- **Kernel sandboxing** — Landlock + seccomp-bpf (Linux 5.13+), sandbox-exec (macOS), Job Objects (Windows). Platform-specific canary probes verify enforcement at startup.
- **File protection levels** — ReadOnly (SOUL/IDENTITY/TOOLS/BOOT), EscalateTier2 (AGENTS/HEARTBEAT), WriteTier1Min (MEMORY/USER), FullBlock (.openparallax/, policies/).
- **SSRF protection** — HTTP and browser executors block private IP ranges (127.0.0.0/8, 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16, 169.254.0.0/16, ::1, fc00::/7). Hostnames resolved before checking.
- **Information Flow Control** — 5 sensitivity levels with taint tracking. Prevents data exfiltration across sensitivity boundaries.
- **Tamper-evident audit** — append-only JSONL with SHA-256 hash chain. Every tool proposal, execution, and Shield verdict logged. Chain verification via `openparallax audit --verify`.
- **Canary tokens** — random tokens generated during workspace init, embedded in evaluator prompt, verified in Tier 2 responses with constant-time comparison.
- **Configurable output sanitization** — opt-in wrapping of tool results and memory content in explicit data boundaries to mitigate prompt injection via untrusted content.
- **Web security** — bcrypt password auth, HttpOnly Secure SameSite=Strict cookies, WebSocket session authentication, login rate limiting (5/min per IP), 10MB message size limit, CORS restricted to configured origins (localhost-only default).
- **Channel access control** — Discord requires guild allowlist (DMs only when empty), Telegram defaults to private-chat-only with optional group allowlist.
- **Read-only config keys** — security-sensitive settings (fail_closed, rate_limit, daily_budget, output_sanitization, pipeline parameters) cannot be changed via `/config set` or the settings API.

### Features

- **69 tool actions** — file operations, git, shell commands, browser automation, email (SMTP + IMAP), calendar (Google + Microsoft + CalDAV), canvas, memory, HTTP requests, scheduling, clipboard, system utilities, image/video generation, sub-agents. Organized into 14 groups with lazy loading via `load_tools` meta-tool.
- **Sub-agent orchestration** — parallel task execution with isolated sandboxed processes. Follow-up messaging via `agent_message`. Sub-agents poll for additional instructions after each reasoning loop.
- **Custom skills** — domain-specific guidance in `skills/<name>/SKILL.md` with YAML frontmatter. Global skills at `~/.openparallax/skills/`, workspace skills override. Configurable disable list.
- **Multi-channel messaging** — WhatsApp (Cloud API), Telegram (Bot API), Discord (bot), Slack (Socket Mode), Signal (signal-cli), Teams (Graph API), iMessage (AppleScript, macOS). Dynamic attach/detach at runtime.
- **Web UI** — glassmorphism two-panel layout (sidebar + chat). Drag-to-resize, responsive breakpoints (full/compact/mobile), real-time streaming via WebSocket. Console split into Metrics, Live Logs, and Audit views. Virtual scrolling for large conversations.
- **CLI** — Cobra + Bubbletea TUI. Shell commands: `start`, `init`, `stop`, `restart`, `status`, `doctor`, `attach`, `detach`, `delete`, `list`, `config`, `session`, `memory`, `logs`, `audit`, `skill`, `mcp`, `get-classifier`, `get-vector-ext`, `chronicle`, `auth`. Slash commands (19 total, in-session): `/help`, `/new`, `/otr`, `/quit`, `/clear`, `/sessions`, `/switch`, `/delete`, `/title`, `/history`, `/export`, `/status`, `/usage`, `/doctor`, `/audit`, `/config`, `/model`, `/restart`, `/logs`.
- **OTR mode** — off-the-record sessions with read-only tools, no memory persistence, amber UI accents, data in `sync.Map` instead of SQLite.
- **Semantic memory** — FTS5 full-text search + vector embeddings. sqlite-vec for native in-database vector queries (auto-downloads latest). Embedding cache with content hashing for skip-unchanged indexing. File watcher for automatic reindexing.
- **Chronicle** — copy-on-write workspace snapshots before every write/delete/move. Hash-chained integrity. Configurable retention. Rollback to any previous state.
- **MCP integration** — connect to external MCP tool servers via stdio transport. MCP tools registered as loadable groups. All MCP tool calls pass through Shield. Idle shutdown with automatic reconnect.
- **Context efficiency** — dynamic tool loading, markdown stripping from system prompts, stale tool result summarization, configurable compaction threshold (default 70%), memoized markdown rendering in frontend.
- **Configurable pipeline** — `max_tool_rounds` (25), `context_window` (128000), `compaction_threshold` (70%), `max_response_tokens` (4096) all adjustable via config.yaml.
- **Session management** — auto-generated titles after 3+ exchanges. Heartbeat sessions for scheduled tasks. Session search across message content.
- **Token usage tracking** — per-session and per-message LLM usage with daily aggregation. 90-day retention policy with automatic archival.
- **System prompt templates** — token-efficient identity, guardrails, and behavioral rules. ~250 tokens for the full static context.
- **13-point health check** — `openparallax doctor` verifies config, connectivity, sandbox, Shield, database, disk, and more.

### Composable Modules

Every module ships as an independently importable Go package with no dependencies on the rest of OpenParallax. Cross-language support via JSON-RPC bridge binaries for Python and Node.js:

- **Shield** — 4-tier AI security pipeline. Available as Go library, Python/Node wrappers, standalone MCP proxy.
- **Memory** — semantic memory with pluggable backends: SQLite (default), PostgreSQL + pgvector, Qdrant, Pinecone, Weaviate, ChromaDB, Redis.
- **Audit** — tamper-evident hash chain logging for any system.
- **Sandbox** — kernel-level process isolation for any child process.
- **Channels** — multi-platform messaging adapters.
- **Chronicle** — copy-on-write file versioning with rollback.
- **LLM** — unified provider abstraction (Anthropic, OpenAI + compatible APIs, Google, Ollama).
- **IFC** — information flow control with sensitivity labels and taint tracking.
- **Crypto** — ID generation, action hashing, hash chains, canary tokens, AES-256-GCM encryption.
- **MCP** — Model Context Protocol client integration.

### Infrastructure

- **Zero CGo** — single static binary with `CGO_ENABLED=0`. Pure Go SQLite (modernc.org/sqlite), pure Go ONNX Runtime (onnxruntime-purego). No C compiler needed.
- **Cross-platform** — Linux, macOS, and Windows. Platform-specific code uses build tags.
- **Embedded web UI** — Svelte 4 + Vite 5 frontend bundled into the Go binary via `go:embed`.
- **Centralized model defaults** — all LLM model names in a single file (models.go). Auto-resolves latest sqlite-vec and ONNX Runtime versions from GitHub releases.
- **CI/CD** — cross-platform binary builds for 6 OS/arch combinations. 7 binaries per release (agent, shield, 5 bridge binaries).
- **Dynamic port allocation** — each agent gets a unique port from the registry with gap scanning.

### Documentation

- **VitePress documentation site** — dark theme with module accent colors.
- **4 design philosophy pages** — Defense in Depth, Process Isolation, Token Economy, Modularity.
- **User guide** — installation, quickstart, configuration, CLI, web UI, sessions, memory, skills, tools, channels, security, heartbeat, troubleshooting.
- **Technical docs** — architecture, process model, message pipeline, ecosystem, engine, agent, gRPC, events, protection, web server, extending.
- **Module docs** — Shield (14 pages including Tier 3), Memory, Audit, Sandbox, Channels (7 platforms), Chronicle, LLM, IFC, Crypto, MCP.
- **API reference** — config schema, environment variables, 13 event types, 69 action types, gRPC API, 28 REST endpoints, WebSocket protocol, policy syntax.

---

## Roadmap

Items under active consideration for future releases:

- **Windows AppContainers** — full filesystem and network isolation on Windows without admin elevation
- **Pure-Go HNSW** — approximate nearest neighbor search for scaling vector search beyond 100K records
- **Additional memory backends** — MongoDB, DynamoDB, Milvus
- **Workspace sharing** — multi-user access to a single agent workspace
