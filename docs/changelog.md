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
- **Platform abstraction** — every OS-specific table, parser, and accessor lives behind build-tagged files in the `platform/` package. Engine and Shield code makes zero runtime platform decisions of its own.

### Security

#### Shield pipeline

- **4-tier Shield pipeline** — Tier 0 (YAML policy matching), Tier 1 (ONNX DeBERTa classifier + curated heuristic ruleset), Tier 2 (LLM evaluator with canary verification), Tier 3 (human-in-the-loop approval via all connected channels). Fail-closed at every tier.
- **Safe-command fast path** — a curated allowlist of common dev workflow commands (`git`, `npm`, `make`, `go`, `cargo`, `docker`, `kubectl`, `pwd`, `whoami`, `date`, etc., plus their cmd.exe equivalents on Windows) bypasses all four tiers and returns ALLOW with confidence 1.0. Single-statement commands only — anything containing `;`, `&`, `|`, `>`, `<`, `` ` ``, or `$(...)` falls through to normal evaluation. The allowlist excludes commands that take arbitrary path arguments (`cat`, `ls`, `grep`, `find`, `rm`, `cp`, `mv`) so the heuristic and Tier 2 layers can still evaluate actual targets. Curated, ships in the binary, not user-extensible. Tables in `platform/safe_commands_{unix,windows}.go`.
- **In-process ONNX classifier** — DeBERTa v3 model runs in pure Go via `onnxruntime-purego`. No sidecar processes, no CGo. Auto-resolves latest ONNX Runtime from GitHub releases.
- **Tier 1 heuristic engine** — block / escalate / allow outcomes per rule. Hard-block patterns (curl-piped-to-shell, base64-piped-to-interpreter, reverse shells, credential directory reads, recursive chmod on system dirs, secret-env echo) return `VerdictBlock`. Context-dependent patterns (`rm -rf`, `&&` chains, `find -delete`, `git push --force` to main, crontab modifications, world-writable chmods, `DROP TABLE`, JWT handling, webhook destinations) escalate to Tier 2 via the `Escalate` flag instead of hard-blocking. `AlwaysBlock` and `Escalate` are mutually exclusive; `NewHeuristicEngine` skips any rule that sets both. False-positive-prone primitive rules (backticks, `$()`, heredocs, plain `eval`/`exec`, plain `ssh`, plain `nc`, plain `kill`) are not in the rule set; the dangerous *combinations* of those primitives have their own dedicated rules.
- **Tier 2 evaluator with inverted context** — independent LLM with no agent identity, no conversation history, no tools. Sees only the action type and payload as JSON data. Decides ALLOW / BLOCK / ESCALATE on action shape alone. ESCALATE routes to Tier 3.
- **Canary tokens** — random tokens generated during workspace init, embedded in evaluator prompt, verified in Tier 2 responses with constant-time comparison.
- **SSRF protection** — HTTP and browser executors block private IP ranges (127.0.0.0/8, 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16, 169.254.0.0/16, ::1, fc00::/7). Hostnames resolved before checking.
- **Information Flow Control** — 5 sensitivity levels with taint tracking. Prevents data exfiltration across sensitivity boundaries.
- **Configurable output sanitization** — opt-in wrapping of tool results and memory content in explicit data boundaries to mitigate prompt injection via untrusted content.

#### Cross-platform default denylist

A curated, ship-with-the-binary denylist applies to **any path the agent touches, anywhere on disk** — not just paths inside the workspace. Curated, not user-extensible. If a user wants the agent to access something on the list, they relocate the data to a path that is not on the list. Moving the file is the explicit consent action.

- **Restricted** (no read, no write): credential directories (`~/.ssh`, `~/.aws`, `~/.gnupg`, `~/.docker`, `~/.kube`, `~/.password-store`, `~/.azure`, `~/.config/gcloud`, `~/.config/op`), Linux `/etc/shadow`, `/etc/sudoers`, `/root`, `/etc/sudoers.d`, `/etc/ssh`, macOS keychains and browser credential directories, Windows `C:\Windows\System32\config` and credential vault. Filename patterns matched anywhere on disk: `id_rsa`/`id_dsa`/`id_ecdsa`/`id_ed25519`, `*.pem`/`*.key`/`*.p12`/`*.pfx`/`*.keystore`/`*.jks`/`*.asc`, `.env`/`.env.local`/`.env.production`, `credentials.json`, `secrets.{yaml,yml,json}`, `token.json`, `service-account.json`, `.pgpass`, `.my.cnf`.
- **Protected** (read OK, write/delete blocked): shell rc files (`.bashrc`, `.bash_profile`, `.zshrc`, `.zprofile`, `.profile`, fish config), VCS and package manager configs (`.gitconfig`, `.npmrc`, `.yarnrc`, `pip.conf`, cargo config), editor configs (`.vimrc`, nvim init files, `.tmux.conf`), Linux system reference files (`/etc/hosts`, `/etc/passwd`, `/etc/group`, `/etc/fstab`, `/etc/resolv.conf`, `/etc/crontab`, `/etc/environment`), Linux cron/systemd/init/apt/yum directories, macOS `/etc/hosts`, Windows hosts file.

The denylist runs after symlink resolution. A symlink in `/tmp/safe.txt` pointing at `~/.ssh/id_rsa` resolves to `~/.ssh/id_rsa` and is blocked. Data tables live in `platform/denylist_{linux,darwin,windows}.go` behind build-tagged accessors. Engine code consumes them via cross-platform accessors and snapshots the lists at package init.

#### Workspace file protection

Files inside the agent's own workspace have a separate protection layer that runs after the cross-platform denylist:

- **FullBlock** — `config.yaml`, `canary.token`, `audit.jsonl`, `openparallax.db`, `evaluator-v1.md`, `.openparallax/`, `policies/`. Always blocked.
- **ReadOnly** — SOUL.md, IDENTITY.md, `skills/`. Read OK, write/delete blocked.
- **EscalateTier2** — AGENTS.md, HEARTBEAT.md. Writes proceed but require Tier 2 LLM evaluation.
- **WriteTier1Min** — USER.md, MEMORY.md, `memory/`. Writes proceed but require Tier 1 minimum (heuristic/ONNX check).

#### Path enforcement

- **Absolute paths required** — every path argument to a tool must be absolute (or `~`-prefixed for home expansion). Relative paths are rejected at the engine before Shield evaluation, with a clear error pointing the LLM at the requirement so it can re-roll on the next round. Shield evaluates the literal path string and cannot resolve relative paths against an implicit working directory; making path resolution unambiguous is what makes the denylist deterministic.
- **Shell `cd <abs> && cmd` exemption** — for shell commands, the one allowed exception is a leading `cd <absolute-path> && <command>` prefix. The cd target establishes an implicit working directory, and write targets in the rest of the command are resolved against it. Anything else with relative paths is rejected.

#### Kernel sandboxing

- **Kernel sandboxing** — Landlock + seccomp-bpf (Linux 5.13+), sandbox-exec (macOS), Job Objects (Windows). Platform-specific canary probes verify enforcement at startup. The agent process has near-zero filesystem and network capability of its own — every action goes through the gRPC stream to the Engine, which is the policy authority.

#### Audit chain

- **Tamper-evident audit** — append-only JSONL with SHA-256 hash chain. Every tool proposal, execution, and Shield verdict logged. Chain verification via `openparallax audit --verify`.
- **24 audit event types**: `ActionProposed`, `ActionEvaluated`, `ActionApproved`, `ActionBlocked`, `ActionExecuted`, `ActionFailed`, `ShieldError`, `CanaryVerified`, `CanaryMissing`, `RateLimitHit`, `BudgetExhausted`, `SelfProtection`, `TransactionBegin`, `TransactionCommit`, `TransactionRollback`, `IntegrityViolation`, `SessionStarted`, `SessionEnded`, `ConfigChanged`, `IFCClassified`, `ChronicleSnapshot`, `ChronicleSnapshotFailed`, `SandboxCanaryResult`. The IFC classification, Chronicle snapshot success/failure, and sandbox canary result events ensure every security-relevant decision lands in the chain rather than just the structured engine log.
- **Sandbox canary plumbing** — the sandboxed agent process cannot write to `audit.jsonl` itself, so the JSON-encoded canary verification result rides on the `AgentReady` proto event and the engine emits the audit entry on receipt.

#### Web and channel security

- **Settings panel is read-only over HTTP** — there is no `PUT /api/settings` endpoint. The web settings panel displays the current configuration as labels and values; to change a setting from the web UI, the user types `/config set …` in the chat input, which goes through the canonical writer the same way the CLI does. This eliminates the secret-exfiltration and Shield-disarm vectors that an HTTP write surface would expose.
- **Web security** — bcrypt password auth, HttpOnly Secure SameSite=Strict cookies, WebSocket session authentication, login rate limiting (5/min per IP), 10MB message size limit, CORS restricted to configured origins (localhost-only default).
- **Channel access control** — Discord requires guild allowlist (DMs only when empty), Telegram defaults to private-chat-only with optional group allowlist.

### Configuration

- **`models[]` and `roles{}` schema** — the workspace config defines a pool of provider+model entries in `models[]` and maps functional roles (`chat`, `shield`, `embedding`, `sub_agent`, `image`, `video`) to entries in that pool via `roles{}`. There is no top-level `llm:` key; the loader runs in strict YAML mode and rejects any unknown top-level field.
- **Single canonical writer** — `config.Save` marshals via `yaml.Marshal`, writes atomically through `<path>.tmp` + rename, re-loads through `Load()` to verify the round-trip succeeds, backs up the previous file to `<workspace>/.openparallax/backups/config-<timestamp>.yaml` (rotation: 100 most recent), emits a `ConfigChanged` audit entry with the SHA-256 of the previous and new file contents on slash-command-driven saves, and rolls back on any failure. All four mutation surfaces (`init` CLI wizard, web setup wizard, `/config set`, `/model`) go through it.
- **`SettableKeys` registry** — single map enumerating every key writable through `/config set` and `/model`, with each key's optional validator, setter, and `RequiresRestart` flag. The slash command surface is the only consumer; there is no HTTP write surface. Identity values are validated against `^[a-zA-Z0-9 _-]{1,40}$` (newlines/escapes rejected); the `chat.base_url` setting is constrained to loopback when the chat model uses the `ollama` provider; the setup wizard's workspace path must resolve under `$HOME` or `$OP_DATA_DIR`.
- **Persistent slash commands** — `/config set` and `/model` persist to `config.yaml` through `config.Save` with two-layer rollback on validation failure. Identity changes apply immediately on the live engine; model and provider changes require a restart to bind.
- **Read-only config keys** — security-sensitive settings (`general.fail_closed`, `general.rate_limit`, `general.daily_budget`, `general.output_sanitization`, `agents.*` pipeline parameters, `shield.policy_file`, `shield.onnx_threshold`, `shield.heuristic_enabled`, `web.host`, `web.port`, `web.password_hash`) cannot be changed via `/config set`. They must be edited directly in `config.yaml` and require a restart.

### Features

- **69 tool actions** — file operations, git, shell commands, browser automation, email (SMTP + IMAP), calendar (Google + Microsoft + CalDAV), canvas, memory, HTTP requests, scheduling, clipboard, system utilities, image/video generation, sub-agents. Organized into 14 groups with lazy loading via `load_tools` meta-tool.
- **Sub-agent orchestration** — parallel task execution with isolated sandboxed processes. Follow-up messaging via `agent_message`. Sub-agents poll for additional instructions after each reasoning loop.
- **LLM nudge for sub-agent delegation** — the `agents` tool group description, the `create_agent` tool description, and the agent's behavioral rules use a consistent vocabulary ("default for 2+ independent subtasks", "parallel", "cost", "clean context") to push the LLM toward delegating parallelizable work. Net cost ~60 tokens of static system context, amortized across every turn.
- **Numeric model index for sub-agents** — the `model` parameter on `create_agent` is a 1-based integer index into a numbered menu rendered into the tool description from the workspace `models[]` pool. Out-of-range returns a graceful error so the LLM can recover on the next round. Optional `models[].purpose` annotation per pool entry surfaces a hand-written hint ("fast, cheap, scans") into the menu; entries without one are still selectable. The pool snapshot is taken at engine startup so live config edits cannot drift the index mapping mid-session.
- **Sub-agent context isolation made explicit** — the `task` parameter description spells out that the sub-agent starts with a blank context: it does NOT see the parent's conversation, files, or prior reasoning. The parent must include all background.
- **Custom skills** — domain-specific guidance in `skills/<name>/SKILL.md` with YAML frontmatter. Global skills at `~/.openparallax/skills/`, workspace skills override. Configurable disable list.
- **Multi-channel messaging** — WhatsApp (Cloud API), Telegram (Bot API), Discord (bot), Slack (Socket Mode), Signal (signal-cli), Teams (Graph API), iMessage (AppleScript, macOS). Dynamic attach/detach at runtime.
- **Web UI** — glassmorphism two-panel layout (sidebar + chat). Drag-to-resize, responsive breakpoints (full/compact/mobile), real-time streaming via WebSocket. Console split into Metrics, Live Logs, and Audit views. Read-only settings panel.
- **CLI** — Cobra + Bubbletea TUI. Shell commands: `start`, `init`, `stop`, `restart`, `status`, `doctor`, `attach`, `detach`, `delete`, `list`, `config`, `session`, `memory`, `logs`, `audit`, `skill`, `mcp`, `get-classifier`, `get-vector-ext`, `chronicle`, `auth`. Slash commands (19 total, in-session): `/help`, `/new`, `/otr`, `/quit`, `/clear`, `/sessions`, `/switch`, `/delete`, `/title`, `/history`, `/export`, `/status`, `/usage`, `/doctor`, `/audit`, `/config`, `/model`, `/restart`, `/logs`.
- **OTR mode** — off-the-record sessions with read-only tools, no memory persistence, amber UI accents, data in `sync.Map` instead of SQLite.
- **Semantic memory** — FTS5 full-text search + vector embeddings. sqlite-vec for native in-database vector queries (auto-downloads latest). Embedding cache with content hashing for skip-unchanged indexing. File watcher for automatic reindexing.
- **Chronicle** — copy-on-write workspace snapshots before every write/delete/move. Hash-chained integrity. Configurable retention. Rollback to any previous state.
- **MCP integration** — connect to external MCP tool servers via stdio transport. MCP tools registered as loadable groups. All MCP tool calls pass through Shield. Idle shutdown with automatic reconnect.
- **Context efficiency** — dynamic tool loading, markdown stripping from system prompts, stale tool result summarization, configurable compaction threshold (default 70%), memoized markdown rendering in frontend.
- **Configurable pipeline** — `max_tool_rounds` (25), `context_window` (128000), `compaction_threshold` (70%), `max_response_tokens` (4096) all adjustable via config.yaml.
- **Session management** — auto-generated titles after 3+ exchanges. Heartbeat sessions for scheduled tasks. Session search across message content.
- **Token usage tracking** — per-session and per-message LLM usage with daily aggregation. 90-day retention policy with automatic archival.
- **Per-tier Shield latency metrics** — `metrics_latency` table stores one sample per Shield evaluation, tagged by tier. `GET /api/metrics` exposes per-tier `shield_t{0,1,2}_p{50,95}_ms` percentiles in the `performance` block. Sourced from observation samples, not the audit chain — audit stays clean of performance telemetry.
- **System prompt templates** — token-efficient identity, guardrails, and behavioral rules. ~250 tokens for the full static context.
- **14-point health check** — `openparallax doctor` verifies config, the canonical writer round-trip, workspace, SQLite, LLM provider, Shield, embedding, browser, email, calendar, HEARTBEAT, audit (chain integrity), sandbox, and web UI.

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
- **Platform** — OS detection, shell configuration, sensitive paths, denylist tables, safe-command allowlist, cross-platform shell parser. All OS-conditional code lives here behind build-tagged files; consumers make zero runtime platform decisions.

### Infrastructure

- **Zero CGo** — single static binary with `CGO_ENABLED=0`. Pure Go SQLite (modernc.org/sqlite), pure Go ONNX Runtime (onnxruntime-purego). No C compiler needed.
- **Cross-platform** — Linux, macOS, and Windows. Platform-specific code uses build tags exclusively; no `runtime.GOOS` branches outside the `platform/` package.
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
- **API reference** — config schema, environment variables, 24 audit event types, 69 action types, gRPC API, REST endpoints, WebSocket protocol, policy syntax.

---

## Roadmap

Items under active consideration for future releases:

- **Windows AppContainers** — full filesystem and network isolation on Windows without admin elevation
- **Pure-Go HNSW** — approximate nearest neighbor search for scaling vector search beyond 100K records
- **Additional memory backends** — MongoDB, DynamoDB, Milvus
- **Workspace sharing** — multi-user access to a single agent workspace
