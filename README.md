<div align="center">

# OpenParallax

**AI agents that think must never act.**

A personal AI agent where the LLM is structurally incapable of executing anything. Every action is proposed, evaluated, and executed by separate processes. A fully compromised agent cannot cause harm.

[![Go Reference](https://pkg.go.dev/badge/github.com/openparallax/openparallax.svg)](https://pkg.go.dev/github.com/openparallax/openparallax)
[![Go Report Card](https://goreportcard.com/badge/github.com/openparallax/openparallax)](https://goreportcard.com/report/github.com/openparallax/openparallax)
[![Go 1.25+](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue?style=flat-square)](LICENSE)
[![Zero CGo](https://img.shields.io/badge/CGo-disabled-success?style=flat-square)](https://pkg.go.dev)
[![Platform](https://img.shields.io/badge/Linux%20%7C%20macOS%20%7C%20Windows-grey?style=flat-square&logo=linux&logoColor=white)](https://docs.openparallax.dev/guide/installation)
[![Security](https://img.shields.io/badge/Shield-4--tier%20pipeline-ff3366?style=flat-square)](https://docs.openparallax.dev/shield/)
[![Single Binary](https://img.shields.io/badge/Deploy-single%20binary-orange?style=flat-square)](https://docs.openparallax.dev/guide/installation)

<a href="https://docs.openparallax.dev" target="_blank">Documentation</a> &bull; [Quick Start](#quick-start) &bull; [Architecture](#architecture) &bull; [Modules](#composable-modules) &bull; [Test Your Own Security](#test-your-own-security) &bull; [Contributing](CONTRIBUTING.md)

</div>

---

> **Research paper:** OpenParallax is a reference implementation of the *Parallax* paradigm, presented in [*Parallax: Why AI Agents That Think Must Never Act*](https://arxiv.org/abs/2604.12986) (arXiv:2604.12986). The paper argues that prompt-level guardrails are architecturally insufficient for agents with execution capability. Safety requires structural enforcement: the system that reasons must be unable to act, the system that acts must be unable to reason, with an independent validator between them.

## One Binary. Zero Dependencies. Any Platform.

A single static binary. No Python. No Node.js. No Docker. No runtime dependencies.

**Linux / macOS:**

```bash
curl -sSL https://get.openparallax.dev | sh
openparallax init
openparallax start
```

**Windows (PowerShell):**

```powershell
irm https://get.openparallax.dev/install.ps1 | iex
openparallax init
openparallax start
```

**From source:**

```bash
git clone https://github.com/openparallax/openparallax.git
cd openparallax
make build-all
./dist/openparallax init
./dist/openparallax start
```

Prerequisites for building: Go 1.25+, Node.js 20+ (for the web frontend).

## What It Does

OpenParallax is an AI agent that runs on your machine — under your control, with your data staying local. It connects to LLM providers (Anthropic, OpenAI + compatible APIs, Google, Ollama), executes 69 tool actions on your behalf, and remembers context across conversations. Every action passes through a security pipeline before execution.

Whether you're a developer automating workflows, a researcher handling sensitive datasets, an analyst processing financial records, or a compliance team evaluating AI tooling — the security architecture is designed so you don't have to trust the LLM to trust the system.

- **Talk through CLI, web, or messaging** — terminal TUI, glassmorphism web UI, WhatsApp, Telegram, Discord, Signal, iMessage (Slack and Teams planned)
- **69 actions across 14 tool groups** — files, git, shell, browser, email (SMTP + IMAP), calendar, canvas, HTTP, scheduling, clipboard, system utilities, sub-agents
- **Semantic memory** — FTS5 full-text search + vector embeddings, persistent across sessions
- **4-tier security** — policy rules, ML classifier, LLM evaluator, human-in-the-loop approval
- **Kernel sandboxing** — agent process isolated at the OS level (Landlock + seccomp, sandbox-exec, Job Objects)
- **Tamper-evident audit** — SHA-256 hash chain on every action, every verdict, every session
- **OTR mode** — off-the-record sessions with read-only access and zero persistence
- **Custom skills** — domain-specific guidance in markdown, global and workspace-local, loaded on demand
- **Sub-agents** — parallel task delegation to isolated sandboxed processes with follow-up messaging
- **Dynamic tool loading** — LLM loads only the tool groups it needs, reducing attack surface and token cost

## Quick Start

```bash
openparallax init       # Interactive setup wizard
openparallax start      # Launch agent + web UI
```

The `init` wizard configures your LLM provider, Shield security model, and embedding provider with connection testing. Optional steps offer to download the vector search extension, skill packs, and MCP servers. After init, `start` launches the engine, spawns the sandboxed agent, and opens the CLI. Use `--web` to also open the web UI in your browser.

```bash
openparallax doctor          # 13-point health check
openparallax status          # Workspace stats
openparallax attach tui      # Attach CLI to running agent
openparallax detach telegram # Detach a channel at runtime
openparallax logs -f         # Tail engine log
openparallax audit --verify  # Verify audit chain integrity
openparallax chronicle       # List Chronicle snapshots
openparallax chronicle rollback <id>  # Restore files from snapshot
openparallax skill install developer  # Install a skill pack
openparallax mcp install rss          # Install an MCP server
```

## Architecture

Three OS processes at runtime, enforcing **Cognitive-Executive Separation** — the agent proposes, the engine decides:

```
openparallax start              (Process Manager)
  └── internal-engine           (Engine: privileged, unsandboxed)
        ├── Shield pipeline     (4-tier security evaluation)
        ├── gRPC server         (AgentService, ClientService, SubAgentService)
        ├── HTTP/WS server      (Web UI on :3100)
        ├── Audit + Chronicle   (hash-chain logging, COW snapshots)
        ├── IFC                 (information flow control)
        └── internal-agent      (Agent: kernel-sandboxed)
              ├── LLM reasoning (context assembly, tool proposals)
              └── Skills        (on-demand domain guidance)
```

**The Agent** owns the LLM session. It assembles context, runs the reasoning loop, and proposes tool calls over gRPC. It runs inside a kernel sandbox and cannot access files, network, or processes outside its allowed scope. It authenticates with an ephemeral token generated per spawn.

**The Engine** is the security gate. It evaluates every proposal through Shield, checks IFC sensitivity labels, takes Chronicle snapshots before destructive actions, logs to the audit trail, and executes approved actions. The Agent never executes anything directly.

**A fully compromised Agent — total prompt injection, complete jailbreak — cannot cause harm**, because it has no execution capability. Shield runs in the Engine process, unreachable from the Agent.

### Shield Pipeline

```
Tier 0: Policy (YAML rules)        → instant, deterministic
Tier 1: Heuristic (79 rules)                   → in-process, no API call
Tier 2: LLM Evaluator (separate model, canary-verified) → budget-limited
Tier 3: Human Approval (all channels, timeout-to-deny) → rate-limited
```

Each tier has a different failure mode. An attacker must simultaneously evade pattern matching, jailbreak a separate LLM that treats everything as data, AND convince the user — for a single action. All tiers fail closed. ML classification via sidecar is the <a href="https://docs.openparallax.dev/project/roadmap#immediate-next-steps" target="_blank">immediate next priority</a>.

### Message Pipeline

```
User input → Store → Forward to Agent → LLM stream with tools
  → For each tool call:
      Protection.Check → Shield.Evaluate → IFC.Check
      → Chronicle.Snapshot → Audit.Log → Execute → Return to Agent
  → Response complete → Memory index → Title generation
```

## Composable Modules

Every module is a standalone Go package with zero dependencies on the rest of OpenParallax. Cross-language support via JSON-RPC bridge binaries for Python and Node.js.

| Module | Description | Go | Python | Node.js |
|--------|-------------|:---:|:------:|:-------:|
| **<a href="https://docs.openparallax.dev/shield/" target="_blank">Shield</a>** | 4-tier AI security pipeline (policy + classifier + LLM + human) | &#10003; | &#10003; | &#10003; |
| **<a href="https://docs.openparallax.dev/memory/" target="_blank">Memory</a>** | Semantic memory with pluggable backends (SQLite default; pgvector, Qdrant, Pinecone planned) | &#10003; | planned | planned |
| **<a href="https://docs.openparallax.dev/audit/" target="_blank">Audit</a>** | Tamper-evident append-only logging with SHA-256 hash chains | &#10003; | &#10003; | &#10003; |
| **<a href="https://docs.openparallax.dev/sandbox/" target="_blank">Sandbox</a>** | Kernel-level process isolation (Landlock, sandbox-exec, Job Objects) | &#10003; | &#10003; | &#10003; |
| **<a href="https://docs.openparallax.dev/channels/" target="_blank">Channels</a>** | WhatsApp, Telegram, Discord, Signal, iMessage (Slack and Teams planned) | &#10003; | planned | planned |
| **<a href="https://docs.openparallax.dev/modules/chronicle" target="_blank">Chronicle</a>** | Copy-on-write workspace snapshots with rollback | &#10003; | | |
| **<a href="https://docs.openparallax.dev/modules/llm" target="_blank">LLM</a>** | Anthropic, OpenAI + compatible APIs, Google, Ollama | &#10003; | | |
| **<a href="https://docs.openparallax.dev/security/ifc" target="_blank">IFC</a>** | Information flow control with sensitivity labels | &#10003; | | |
| **<a href="https://docs.openparallax.dev/modules/crypto" target="_blank">Crypto</a>** | ID generation, hash chains, canary tokens, AES-256-GCM | &#10003; | | |
| **<a href="https://docs.openparallax.dev/modules/mcp" target="_blank">MCP</a>** | Model Context Protocol client integration | &#10003; | | |

Import Shield into your own project:

```go
import "github.com/openparallax/openparallax/shield"

pipeline, _ := shield.NewPipeline(shield.Config{
    PolicyFile:       "policy.yaml",
    HeuristicEnabled: true,
    FailClosed:       true,
})

verdict := pipeline.Evaluate(ctx, &shield.ActionRequest{
    Type:    "execute_command",
    Payload: map[string]any{"command": userInput},
})
```

## Security

The architecture is designed around one assumption: **the AI agent is fully untrusted**. It may be compromised at any time through prompt injection, memory poisoning, or context manipulation. Safety holds even under total compromise.

- **4-tier Shield** — YAML policy, heuristic classifier (optional ML sidecar), isolated LLM evaluator with canary verification, human-in-the-loop approval broadcast to all channels
- **Kernel sandboxing** — Landlock (Linux 5.13+), sandbox-exec (macOS), Job Objects (Windows). Canary probes verify enforcement on every startup
- **Fail-closed** — every Shield error path returns BLOCK. Sandbox failure prevents agent startup
- **Ephemeral auth tokens** — random tokens per agent/sub-agent spawn, validated on first gRPC message
- **File protection levels** — critical files fully blocked, identity files read-only, memory files require minimum Tier 1
- **SSRF protection** — HTTP and browser executors block private IP ranges and cloud metadata endpoints
- **IFC** — information flow control prevents data exfiltration across sensitivity boundaries
- **Output sanitization** — memory chunks always wrapped in `[MEMORY]` boundary markers; opt-in data boundaries for tool results
- **Web security** — session-authenticated WebSocket, login rate limiting, CORS restricted to configured origins, 10MB message size limit
- **Channel access control** — Discord requires guild allowlist, Telegram defaults to private-chat-only
- **Tamper-evident audit** — SHA-256 hash chain, any modification breaks the chain
- **Read-only config** — security-sensitive settings cannot be changed via API or slash commands

For vulnerability reports, see [SECURITY.md](SECURITY.md).

## Test Your Own Security

**98.9% of attacks blocked. Zero false positives.** Default configuration, 280 adversarial test cases, 9 attack categories.

The Parallax paradigm is open and reproducible. We ship the same adversarial corpus we use to validate Shield. Run it on your OpenParallax deployment, your own LLM provider, your own policy. The result is reproducible. The methodology is documented. The data is committed. A standalone harness for testing any Shield-based system is <a href="https://docs.openparallax.dev/project/roadmap#standalone-eval-harness" target="_blank">on the roadmap</a>.

```bash
# Build the eval binary
go build -o dist/openparallax-eval ./cmd/eval

# Run a single suite (encoding/obfuscation attacks) against your workspace
./dist/openparallax-eval \
  --suite eval-results/test-suite/c5_encoding_obfuscation.yaml \
  --config C \
  --mode inject \
  --workspace ~/.openparallax/atlas \
  --output eval-results/playground/$(date +%Y%m%d-%H%M)-c5.json
```

**Run-013 baseline** (latest, default config): **277/280 attacks blocked (98.9%)**, **0 false positives on 50 legitimate operations**. The full multi-category corpus, run history, narrative reports, and methodology live under [`eval-results/`](eval-results/README.md):

- 9 attack categories — direct injection, indirect injection, multistep context, toolchain attacks, encoding/obfuscation, multi-agent, validator-targeted, helpfulness bypass, Tier 3 ambiguous
- 50 false-positive cases — real dev/sysadmin/file/comms/web operations Shield must not block
- 13 historical runs from the original methodology pivot through the current default
- 4 narrative reports walking through every architectural decision

## Development

```bash
# Full verification — run after every change
make build-all && make test && make lint && cd web && npm test && cd ..

# Individual targets
make proto              # Generate gRPC code from proto definitions
make build-web          # Build Svelte frontend (embedded via go:embed)
make build              # Go binary → dist/openparallax
make build-shield       # Shield binary → dist/openparallax-shield
make build-bridges      # 5 cross-language bridge binaries
make test               # go test -race -count=1 ./...
make lint               # golangci-lint

# E2E tests (mock LLM — no API key needed)
E2E_LLM=mock go test -tags e2e -timeout 300s ./e2e/...

# Docs
cd docs && npm run dev  # VitePress dev server
```

### Tech Stack

| Layer | Technology |
|-------|-----------|
| Language | Go 1.25 (`CGO_ENABLED=0`), TypeScript 5.5 |
| Frontend | Svelte 4, Vite 5 |
| CLI | Cobra + Bubbletea |
| Transport | gRPC (agent ↔ engine), WebSocket + REST (web ↔ engine) |
| Database | SQLite (modernc.org, pure Go, WAL mode) |
| Search | FTS5 full-text + vector embeddings (sqlite-vec or built-in cosine) |
| ML | Sidecar (planned) — heuristic-only in current release |
| Protobuf | protoc + go/go-grpc plugins |

### Environment Variables

| Variable | Purpose |
|----------|---------|
| `ANTHROPIC_API_KEY` | Anthropic Claude |
| `OPENAI_API_KEY` | OpenAI (chat + embeddings) |
| `GOOGLE_AI_API_KEY` | Google Gemini |

See the <a href="https://docs.openparallax.dev/reference/env-vars" target="_blank">full reference</a> for all configuration options.

## Documentation

Full documentation at **<a href="https://docs.openparallax.dev" target="_blank">docs.openparallax.dev</a>**:

- **<a href="https://docs.openparallax.dev/guide/" target="_blank">User Guide</a>** — installation, quickstart, configuration, every feature
- **<a href="https://docs.openparallax.dev/technical/design-security" target="_blank">Design Philosophy</a>** — why 4 tiers, why kernel sandboxing, why dynamic tool loading
- **<a href="https://docs.openparallax.dev/technical/" target="_blank">Technical Docs</a>** — architecture, pipeline, gRPC services, process model
- **<a href="https://docs.openparallax.dev/shield/" target="_blank">Module Docs</a>** — standalone usage for Shield, Memory, Audit, Sandbox, Channels
- **<a href="https://docs.openparallax.dev/reference/config" target="_blank">API Reference</a>** — config schema, event types, 69 action types, REST endpoints, WebSocket protocol
- **<a href="https://docs.openparallax.dev/project/roadmap" target="_blank">Roadmap</a>** — all planned features, from memory backends to A2A inter-agent collaboration

## License

[Apache License 2.0](LICENSE)

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.
