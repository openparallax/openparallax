# Changelog

All notable changes to OpenParallax are documented here. This project follows [conventional commits](https://www.conventionalcommits.org/) and [semantic versioning](https://semver.org/).

---

## Unreleased

### Architecture

- **Agent-Engine separation** — Agent process now owns the LLM and runs inside a kernel sandbox. Engine is the security gate that evaluates and executes every tool call. The Agent can propose actions but never execute them directly.
- **Three-process model** — Process Manager spawns Engine, Engine spawns sandboxed Agent. Clean restart via exit code 75. Crash recovery with budget.
- **gRPC service redesign** — AgentService (bidirectional streaming), ClientService (server streaming), SubAgentService. Replaced monolithic PipelineService.
- **EventBroadcaster** — fan-out pipeline events to all subscribed clients by session. Supports session-scoped and global subscriptions.

### Security

- **3-tier Shield pipeline** — Tier 0 (YAML policy matching), Tier 1 (ONNX DeBERTa classifier + heuristic patterns), Tier 2 (LLM evaluator with canary verification). Fail-closed by design.
- **In-process ONNX classifier** — DeBERTa v3 base model runs in pure Go via `onnxruntime-purego`. No Node.js sidecar, no CGo. Download via `openparallax get-classifier`.
- **Kernel sandboxing** — Landlock (Linux 5.13+), sandbox-exec (macOS), Job Objects (Windows). Platform-specific canary probes verify enforcement. Fail-closed — agent refuses to start if sandbox isn't working.
- **File protection levels** — ReadOnly (SOUL/IDENTITY/TOOLS/BOOT), EscalateTier2 (AGENTS/HEARTBEAT), WriteTier1Min (MEMORY/USER), FullBlock (config/canary/audit/db/evaluator).
- **Information Flow Control** — 5 sensitivity levels with taint tracking. Prevents data exfiltration across sensitivity boundaries.
- **Cookie-based web authentication** — bcrypt password, HttpOnly Secure SameSite=Strict cookie for remote access.

### Features

- **Custom skills** — domain-specific guidance in `skills/<name>/SKILL.md` with YAML frontmatter. Discovery via system prompt, on-demand loading via `load_skills` meta-tool. Replaced built-in skills (redundant with tool schemas).
- **Multi-channel messaging** — WhatsApp (Cloud API), Telegram (Bot API), Discord (bot), Slack (Socket Mode), Signal (signal-cli), Teams (Graph API).
- **Web UI** — glassmorphism three-panel layout (sidebar, artifact canvas, chat). Drag-to-resize, responsive breakpoints, OTR mode with amber accents.
- **CLI** — Cobra + Bubbletea TUI. Commands: start, init, status, doctor, attach, detach, session, memory, logs, audit, get-classifier.
- **OTR mode** — off-the-record sessions with read-only tools, no memory persistence, data in sync.Map instead of SQLite.
- **Semantic memory** — FTS5 full-text search + vector embeddings. Daily conversation logs. Pluggable backend architecture.
- **Chronicle** — copy-on-write workspace snapshots before every write operation. Rollback to any previous state.
- **Tamper-evident audit** — append-only JSONL with SHA-256 hash chain. Every proposal and execution logged.
- **50+ tool actions** — file, git, shell, browser, email, calendar, canvas, memory, HTTP, scheduling. Lazy loading via `load_tools` meta-tool.
- **MCP integration** — connect to external MCP servers. All MCP tool calls pass through Shield.
- **Context compaction** — automatic history summarization when approaching 70% of context window budget.

### Infrastructure

- **Zero CGo** — single static binary with `CGO_ENABLED=0`. Pure Go SQLite (modernc.org), pure Go ONNX Runtime (onnxruntime-purego).
- **Embedded web UI** — Svelte 4 + Vite 5 frontend bundled into the Go binary via `go:embed`.
- **13-point health check** — `openparallax doctor` verifies config, connectivity, sandbox, Shield, database, and disk.
- **Startup validation** — engine refuses to start if security-critical files (policy, evaluator prompt) are missing.
- **Template embedding** — policies, evaluator prompt, and skills embedded and copied during `openparallax init`.
- **Port management** — dynamic port allocation with gap scanning for deleted agents.

### Documentation

- **VitePress documentation site** — 91 pages with glassmorphism theme and per-module accent colors.
- **User guide** — installation, quickstart, configuration, CLI, web UI, sessions, memory, skills, tools, channels, security, heartbeat, troubleshooting.
- **Technical docs** — architecture, process model, message pipeline, ecosystem, engine, agent, gRPC, events, protection, crypto, web server, extending.
- **Module docs** — Shield (13 pages), Memory (14 pages with 7 backend guides), Audit, Sandbox, Channels (all platforms), Chronicle, LLM, IFC, Crypto, MCP.
- **API reference** — config schema, environment variables, 8 event types, 50+ action types, gRPC API, REST API, WebSocket protocol, policy syntax.

### Composable Modules

- **Shield** — standalone 3-tier AI security pipeline. Available as Go library, Python/Node wrappers, standalone MCP proxy binary.
- **Memory** — semantic memory with pluggable backends: SQLite (default), PostgreSQL + pgvector, Qdrant, Pinecone, Weaviate, ChromaDB, Redis.
- **Audit** — tamper-evident logging usable in any system.
- **Sandbox** — kernel process isolation for any child process.
- **Channels** — multi-platform messaging adapters.
- **Chronicle** — copy-on-write file versioning.
- **LLM** — unified provider abstraction (Anthropic, OpenAI, Google, Ollama).
- **IFC** — information flow control with sensitivity labels.
- **Crypto** — ID generation, hash chains, canary tokens.
- **MCP** — Model Context Protocol client.

---

## Timeline

| Date | Milestone |
|------|-----------|
| Q1 2026 | Initial architecture and core pipeline |
| Q1 2026 | Shield 3-tier security pipeline |
| Q1 2026 | Kernel sandboxing (Landlock, sandbox-exec, Job Objects) |
| Q1 2026 | Web UI with glassmorphism design |
| Q1 2026 | Multi-channel messaging adapters |
| Q2 2026 | Agent-Engine process separation |
| Q2 2026 | In-process ONNX classifier |
| Q2 2026 | Custom skills system |
| Q2 2026 | Composable module architecture |
| Q2 2026 | Documentation site (91 pages) |

---

## Roadmap

Items under active consideration. No commitments on timelines.

- **Pure-Go HNSW** — approximate nearest neighbor search for the SQLite memory backend, scaling vector search beyond 100K records while maintaining zero CGo
- **Shield standalone proxy** — production-ready standalone binary for MCP security gateway deployments
- **Python and Node.js wrappers** — cross-language module wrappers via JSON-RPC bridges
- **Additional memory backends** — MongoDB, DynamoDB, Milvus
- **Sub-agent orchestration** — parallel task execution with isolated sub-agent processes
- **Workspace sharing** — multi-user access to a single agent workspace
