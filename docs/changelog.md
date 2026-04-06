# Changelog

All notable changes to OpenParallax are documented here. This project follows [conventional commits](https://www.conventionalcommits.org/) and [semantic versioning](https://semver.org/).

---

## 2026-04-06

### Security

- Fix 3 Shield gateway bugs (nil pointer panic, fail-open bypass, budget bypass)
- Wire Tier 3 human-in-the-loop approval to all channels
- Add WebSocket session authentication and Tier 3 session binding
- Add SSRF protection (block private IP ranges in HTTP/browser executors)
- Add configurable output sanitization for prompt injection defense
- Add login rate limiting (5/min per IP)
- Add 10MB WebSocket message size limit
- Require Discord guild allowlist, Telegram private-chat default

### Architecture

- Remove dual pipeline — single agent stream path
- Split engine.go into 5 focused files
- Remove artifact system dead code
- Implement agent_message for sub-agent follow-up instructions
- Implement detach command with dynamic channel management

### Performance

- Virtual scroll for message list (activates at 50+ messages)
- llm_usage retention policy (90-day aggregate + prune)
- Memoize markdown rendering (500-entry cache)

### Quality

- Extract pipeline magic numbers into configurable defaults
- Add read-only protection for security-sensitive config keys
- Update model defaults (gpt-5.4, gemini-3.1-pro, claude-sonnet-4-6)
- Centralize model names in models.go
- Auto-resolve latest sqlite-vec and ONNX Runtime from GitHub
- Fix vector-ext 404 download
- Rewrite system prompt templates for token efficiency
- Add RunLoop, indexer, and cascade delete tests
- Split ConsoleViewer into 3 sub-components
- Normalize command file naming

### Documentation

- Add 4 design philosophy pages
- Complete Tier 3 Shield documentation
- Regenerate actions, events, config, REST API, CLI references from code
- Fix 110+ documentation mismatches

---

## v0.1.0 — Initial Release

The first public release of OpenParallax. A reference implementation of the architecture described in [*Parallax: Why AI Agents That Think Must Never Act*](https://github.com/openparallax/openparallax) (forthcoming on arXiv).

### Architecture

- **Agent-Engine separation** — the Agent process owns the LLM and runs inside a kernel sandbox. The Engine is the security gate that evaluates and executes every tool call. The Agent proposes actions but never executes them directly.
- **Three-process model** — Process Manager spawns Engine, Engine spawns sandboxed Agent. Clean restart via exit code 75. Crash recovery with budget.
- **gRPC services** — AgentService (bidirectional streaming between Agent and Engine), ClientService (server streaming to CLI/web clients), SubAgentService (parallel task execution).
- **EventBroadcaster** — fan-out pipeline events to all subscribed clients by session. Supports session-scoped and global subscriptions.
- **Transport-neutral entry point** — any channel adapter calls `ProcessMessageForWeb()` with an `EventSender` implementation. 7 event types: `llm_token`, `action_started`, `shield_verdict`, `action_completed`, `response_complete`, `otr_blocked`, `error`.

### Security

- **3-tier Shield pipeline** — Tier 0 (YAML policy matching), Tier 1 (ONNX DeBERTa classifier + heuristic patterns), Tier 2 (LLM evaluator with canary verification). Fail-closed at every tier.
- **In-process ONNX classifier** — DeBERTa v3 base model runs in pure Go via `onnxruntime-purego`. No sidecar processes, no CGo. Install via `openparallax get-classifier`.
- **Kernel sandboxing** — Landlock (Linux 5.13+), sandbox-exec (macOS), Job Objects (Windows). Platform-specific canary probes verify enforcement at startup. Fail-closed — agent refuses to start if sandbox verification fails.
- **File protection levels** — ReadOnly (SOUL/IDENTITY/TOOLS/BOOT), EscalateTier2 (AGENTS/HEARTBEAT), WriteTier1Min (MEMORY/USER), FullBlock (config.yaml, canary.token, audit.jsonl, openparallax.db, evaluator-v1.md).
- **Information Flow Control** — 5 sensitivity levels (Public, Internal, Confidential, Restricted, Critical) with taint tracking. Prevents data exfiltration across sensitivity boundaries.
- **Tamper-evident audit** — append-only JSONL with SHA-256 hash chain. Every tool proposal, execution result, and Shield verdict is logged. Chain verification via `openparallax audit --verify`.
- **Canary tokens** — random tokens generated during workspace init, embedded in the evaluator prompt, verified in Tier 2 LLM responses to prove evaluation integrity.
- **Cookie-based web authentication** — bcrypt password, HttpOnly Secure SameSite=Strict cookie for remote web access.

### Features

- **50+ tool actions** — file operations, git, shell commands, browser automation, email, calendar, canvas, memory, HTTP requests, scheduling. Organized into groups with lazy loading via `load_tools` meta-tool.
- **Custom skills** — domain-specific guidance in `skills/<name>/SKILL.md` with YAML frontmatter. Discovery summary in system prompt, on-demand loading via `load_skills` meta-tool.
- **Multi-channel messaging** — WhatsApp (Cloud API), Telegram (Bot API), Discord (bot), Slack (Socket Mode), Signal (signal-cli), Teams (Graph API), iMessage (AppleScript bridge, macOS).
- **Web UI** — glassmorphism two-panel layout (sidebar, chat panel). Drag-to-resize, responsive breakpoints (full/compact/mobile), real-time streaming via WebSocket.
- **CLI** — Cobra + Bubbletea TUI. Commands: `start`, `init`, `status`, `doctor`, `attach`, `detach`, `session`, `memory`, `logs`, `audit`, `get-classifier`. Slash commands: `/help`, `/new`, `/otr`, `/quit`, `/clear`, `/status`, `/restart`, `/export`, `/delete`, `/sessions`.
- **OTR mode** — off-the-record sessions with read-only tools, no memory persistence, amber UI accents, data in `sync.Map` instead of SQLite.
- **Semantic memory** — FTS5 full-text search + vector embeddings with pluggable backend architecture. Daily conversation logs. Memory files: SOUL.md, IDENTITY.md, USER.md, MEMORY.md, TOOLS.md, BOOT.md, HEARTBEAT.md, AGENTS.md.
- **Chronicle** — copy-on-write workspace snapshots before every write operation. Rollback to any previous state. Configurable snapshot retention budget.
- **MCP integration** — connect to external MCP tool servers via stdio or streamable-http transport. MCP tool calls pass through the full Shield pipeline.
- **Context compaction** — automatic history summarization when token usage approaches 70% of context window budget.
- **Session title generation** — LLM-generated session titles after 3+ exchanges for meaningful headlines.
- **13-point health check** — `openparallax doctor` verifies config, connectivity, sandbox, Shield, database, disk, and more.

### Composable Modules

Every module ships as an independently importable Go package with no dependencies on the rest of OpenParallax:

- **Shield** — 3-tier AI security pipeline. Available as Go library, Python/Node wrappers, standalone MCP proxy binary.
- **Memory** — semantic memory with pluggable backends: SQLite (default), PostgreSQL + pgvector, Qdrant, Pinecone, Weaviate, ChromaDB, Redis.
- **Audit** — tamper-evident hash chain logging for any system.
- **Sandbox** — kernel-level process isolation for any child process.
- **Channels** — multi-platform messaging adapters (WhatsApp, Telegram, Discord, Slack, Signal, Teams, iMessage).
- **Chronicle** — copy-on-write file versioning with rollback.
- **LLM** — unified provider abstraction (Anthropic, OpenAI, Google, Ollama).
- **IFC** — information flow control with sensitivity labels and taint tracking.
- **Crypto** — ID generation, action hashing, hash chains, canary tokens.
- **MCP** — Model Context Protocol client integration.

### Infrastructure

- **Zero CGo** — single static binary with `CGO_ENABLED=0`. Pure Go SQLite (modernc.org/sqlite), pure Go ONNX Runtime (onnxruntime-purego). No C compiler needed.
- **Cross-platform** — Linux, macOS, and Windows. Platform-specific code uses build tags, not runtime switches.
- **Embedded web UI** — Svelte 4 + Vite 5 frontend bundled into the Go binary via `go:embed`. No external file serving.
- **Startup validation** — engine refuses to start if security-critical files (policy file, evaluator prompt) are missing.
- **Template embedding** — policies, evaluator prompt, and skills are embedded in the binary and copied during `openparallax init`.
- **Dynamic port allocation** — each agent gets a unique port from the registry with gap scanning for deleted agents.

### Documentation

- **VitePress documentation site** — 91+ pages with glassmorphism dark theme and per-module accent colors.
- **User guide** — installation, quickstart, configuration, CLI, web UI, sessions, memory, skills, tools, channels, security, heartbeat, troubleshooting.
- **Technical docs** — architecture, process model, message pipeline, ecosystem, engine, agent, gRPC, events, protection, crypto, web server, extending.
- **Module docs** — Shield (13 pages), Memory (14 pages with 7 backend guides), Audit, Sandbox, Channels (8 platforms), Chronicle, LLM, IFC, Crypto, MCP.
- **API reference** — config schema, environment variables, event types, action types, gRPC API, REST API, WebSocket protocol, policy syntax.

---

## Development Timeline

| Date | Milestone |
|------|-----------|
| Q1 2026 | Core pipeline architecture and Engine orchestrator |
| Q1 2026 | Shield 3-tier security pipeline with YAML policy engine |
| Q1 2026 | Kernel sandboxing with Landlock, sandbox-exec, and Job Objects |
| Q1 2026 | Web UI with glassmorphism design system |
| Q1 2026 | Multi-channel messaging adapters (6 platforms) |
| Q2 2026 | Agent-Engine process separation with gRPC streaming |
| Q2 2026 | In-process ONNX classifier replacing HTTP sidecar |
| Q2 2026 | Custom skills system with on-demand loading |
| Q2 2026 | Composable module architecture with standalone APIs |
| Q2 2026 | Documentation site with 91+ pages |
| Q2 2026 | iMessage adapter for macOS |

---

## Roadmap

Items under active consideration for future releases. No commitments on timelines — items move to the changelog once shipped.

- **Pure-Go HNSW** — approximate nearest neighbor search for the SQLite memory backend, scaling vector search beyond 100K records while maintaining zero CGo
- **Shield standalone proxy** — production-ready standalone binary for MCP security gateway deployments
- **Python and Node.js wrappers** — cross-language module wrappers via JSON-RPC stdin/stdout bridges
- **Additional memory backends** — MongoDB, DynamoDB, Milvus
- **Sub-agent orchestration** — parallel task execution with isolated sub-agent processes
- **Workspace sharing** — multi-user access to a single agent workspace
