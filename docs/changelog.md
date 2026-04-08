# Changelog

All notable changes to OpenParallax are documented here. This project follows [conventional commits](https://www.conventionalcommits.org/) and [semantic versioning](https://semver.org/).

---

## Unreleased

### Config persistence

- **Schema cut.** The legacy `llm:` top-level key is removed entirely. Workspace configuration uses `models[]` (a pool of provider+model entries) and `roles{}` (mapping of `chat`, `shield`, `embedding`, `sub_agent`, `image`, `video` to model names from the pool). The loader runs in strict YAML mode (`KnownFields(true)`); any leftover `llm:` block now produces a clear parse error rather than being silently ignored.
- **Single canonical writer.** A new `config.Save` in the config package marshals via `yaml.Marshal`, writes atomically through `<path>.tmp` + rename, re-loads through `Load()` to verify the round-trip succeeds, backs up the previous file to `<workspace>/.openparallax/backups/config-<timestamp>.yaml` (rotation: 10 most recent), and rolls back on any failure. All three previous writers (`init` CLI wizard, web setup wizard, web settings PUT) now go through it. Replaces three independent `strings.Builder` writers that emitted divergent schemas.
- **`SettableKeys` registry.** A single map enumerates every key writable through `/config set`, `/model`, or `PUT /api/settings`, with each key's setter and `RequiresRestart` flag. Both surfaces dispatch through the same registry so they cannot drift.
- **Persistent slash commands.** `/config set` and `/model` now persist to `config.yaml` through `config.Save` with two-layer rollback on validation failure. Previously they mutated the in-memory pointer only and silently lost the change on restart.
- **Doctor round-trip check.** `openparallax doctor` grows a 14th check that Saves the loaded config to a temp file and reloads it, catching any future writer drift on the next `doctor` run instead of on the next restart.

### Audit chain additions

Four new event types now reach `audit.jsonl`. Previously these subsystems wrote to the structured engine log only or not at all.

- **`CONFIG_CHANGED` (19)** — emitted when the canonical writer persists a successful mutation (slot reserved; emission wiring deferred).
- **`IFC_CLASSIFIED` (20)** — emitted at the metadata enricher site, once per action that received a non-empty `DataClassification`. Includes sensitivity level and source path.
- **`CHRONICLE_SNAPSHOT` (21)** and **`CHRONICLE_SNAPSHOT_FAILED` (22)** — emitted at both Chronicle call sites (gRPC pipeline path and direct tool execution path) so success and failure both land in the chain. Snapshot failures don't block the action (snapshots are best-effort) but the failure is preserved so rollback gaps remain auditable.
- **`SANDBOX_CANARY_RESULT` (23)** — the sandboxed agent process cannot write to `audit.jsonl` itself (the workspace's `.openparallax/` directory is hard-blocked), so the JSON-encoded canary verification result now rides on the `AgentReady` proto event and the engine emits the audit entry on receipt. Adds a `sandbox_canary_json` field to `AgentReady`.

### Metrics

- **Per-tier Shield latency persistence.** New `metrics_latency` table stores one sample per Shield evaluation, tagged by tier. `AddLatencySample` and `GetLatencyPercentiles` helpers mirror the existing `llm_usage.duration_ms` percentile pattern. `GET /api/metrics` exposes new fields `shield_t{0,1,2}_p{50,95}_ms` in the `performance` block. Sourced from observation samples, not the audit chain — audit stays clean of performance telemetry.

### Sub-agent delegation

- **LLM nudge.** The `agents` tool group description, the `create_agent` tool description, and the agent's behavioral rules now use a consistent vocabulary ("default for 2+ independent subtasks", "parallel", "cost", "clean context") to push the LLM toward delegating parallelizable work. Net cost ~60 tokens of static system context, amortized across every turn.
- **Numeric model index.** The `model` parameter on `create_agent` is now a 1-based integer index into a numbered menu rendered into the tool description from the workspace `models[]` pool. Out-of-range returns a graceful error so the LLM can recover on the next round. Optional `models[].purpose` annotation per pool entry surfaces a hand-written hint ("fast, cheap, scans") into the menu; entries without one are still selectable. The pool snapshot is taken at engine startup so live config edits cannot drift the index mapping mid-session.
- **Context isolation made explicit.** The `task` parameter description spells out that the sub-agent starts with a blank context — it does NOT see the parent's conversation, files, or prior reasoning. The parent must include all background.

### Tier 1 heuristic redesign

The Tier 1 heuristic engine was binary: any rule match was a hard block. This made it over-aggressive on common dev workflows (`rm -rf node_modules`, `&&` chains, `find -delete`) while still letting truly dangerous patterns through. Three changes:

- **`Escalate` flag on `HeuristicRule`.** When set, the engine returns `VerdictEscalate` instead of `VerdictBlock`. The gateway already routed escalate verdicts to Tier 2, so this is a flag, not new plumbing.
- **Cross-platform shell rules sorted into block, escalate, or drop.** Hard-block rules (curl-piped-to-shell, base64-piped-to-interpreter, reverse shells, credential dir reads, recursive chmod on system dirs, secret-env echo) stay as `VerdictBlock`. Context-dependent rules (`rm -rf`, `&&` chains, `find -delete`, `git push --force` to main, `crontab` modifications, world-writable chmods, `DROP TABLE`) flip to escalate. Twelve false-positive rules dropped entirely (backticks, `$()`, heredocs, plain `eval`, plain `ssh`, plain `nc`, plain `kill`, etc.) — the dangerous combinations of those primitives stay as their own dedicated rules.
- **Non-shell rules in `shield/tier1_rules.go`.** `PT-001 dot_dot_traversal`, `DE-003 webhook_exfil`, `SD-003 jwt_token`, `EM-001`, `EM-002` flipped to escalate. `DE-001 base64_in_url` and `DE-002 dns_exfil` deleted (false positives on signed S3 URLs and AWS hostnames).
- **`HeuristicRule.AlwaysBlock` and `Escalate` are now mutually exclusive.** `NewHeuristicEngine` skips any rule that sets both at construction time.

### Web settings panel made read-only

The `PUT /api/settings` HTTP endpoint is removed entirely. The web settings panel displays the current configuration as labels and values — no editors, no Save button. To change a setting from the web UI, the user types the slash command in the chat input (`/config set chat.model …`). Slash commands work in the web chat the same way they work in the TUI and they go through the same canonical writer.

This closes three security audit findings in one stroke: secret exfiltration via `chat.base_url` + `chat.api_key_env`, Shield evaluator disarm via `roles.shield`, and the localhost-no-auth mutation surface that an HTTP write endpoint would expose. Two more (the in-memory rollback bug in the PUT handler, the parallel-maps drift between `SettableKeys` and `settingsKeyMap`) disappear because the code paths are gone.

### Absolute paths required

Every path argument to a tool must be absolute (or `~`-prefixed). Relative paths are rejected at the engine before Shield evaluation, with a clear error pointing the LLM at the requirement so it can re-roll on the next round. The reason is that Shield evaluates the literal path string and cannot resolve relative paths against an implicit working directory; making path resolution unambiguous is what makes the denylist deterministic.

For shell commands, the one allowed exception is a leading `cd <absolute-path> && <command>` prefix. The cd target establishes an implicit working directory and write targets in the rest of the command are resolved against it. Anything else with relative paths is rejected.

### Default denylist (cross-platform)

A curated, ship-with-the-binary denylist now applies to **any path the agent touches, anywhere on disk**. Two protection levels:

- **Restricted** (no read, no write): credential directories (`~/.ssh`, `~/.aws`, `~/.gnupg`, `~/.docker`, `~/.kube`, `~/.password-store`, `~/.azure`, gcloud and 1Password CLI dirs), Linux `/etc/shadow`/`/etc/sudoers`/`/root`, macOS keychains and browser credential dirs, Windows credential vault and SAM hive. Plus filename patterns matched anywhere on disk: `id_rsa`/`id_dsa`/`id_ecdsa`/`id_ed25519`, `*.pem`/`*.key`/`*.p12`/`*.pfx`/`*.keystore`/`*.jks`/`*.asc`, `.env`/`.env.local`/`.env.production`, `credentials.json`, `secrets.{yaml,yml,json}`, `token.json`, `service-account.json`, `.pgpass`, `.my.cnf`.
- **Protected** (read OK, write/delete blocked): shell rc files (`.bashrc`, `.zshrc`, `.profile`, `.vimrc`, etc.), VCS configs (`.gitconfig`, `.npmrc`, `.yarnrc`, pip config, cargo config), Linux system reference files (`/etc/hosts`, `/etc/passwd`, etc.), Linux cron/systemd/init/apt/yum dirs, macOS `/etc/hosts`, Windows hosts file.

The data tables live in `platform/denylist_{linux,darwin,windows}.go` behind build-tagged accessors. Engine code consumes them via cross-platform accessors and snapshots the lists at package init — there are no runtime platform decisions in the engine.

The denylist is curated and ships in the binary. It is not user-extensible. If a user wants the agent to access something on the list, they relocate the data to a path that is not on the list. Moving the file is the explicit consent action.

### Safe-command fast path

A curated allowlist of common dev workflow commands (`git`, `npm`, `make`, `go`, `cargo`, `docker`, `kubectl`, `pwd`, `whoami`, `date`, etc., plus their cmd.exe equivalents on Windows) now bypasses all four Shield tiers. Single-statement commands whose first token is in the allowlist return ALLOW with confidence 1.0. The user wins back the latency and tokens of an LLM call on every routine `git status`, `npm install`, `make build`.

The fast path applies only to single-statement commands; any command containing `;`, `&`, `|`, `>`, `<`, `` ` ``, or `$(...)` falls through to normal evaluation. The allowlist excludes commands that take arbitrary path arguments (`cat`, `ls`, `head`, `tail`, `grep`, `find`, `rm`, `cp`, `mv`) — those go through normal evaluation so the heuristic and Tier 2 layers can evaluate the actual targets. The allowlist is not user-extensible; tables live in `platform/safe_commands_{unix,windows}.go`.

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
