<div align="center">

# OpenParallax

**AI agents that think must never act.**

Open-source security framework for autonomous AI systems. 3-tier defense pipeline, kernel sandboxing, tamper-evident audit, and composable modules — because an agent that can execute anything will eventually execute the wrong thing.

[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue?style=flat-square)](LICENSE)
[![CGo](https://img.shields.io/badge/CGo-disabled-success?style=flat-square)](https://pkg.go.dev)
[![Platform](https://img.shields.io/badge/Platform-Linux%20%7C%20macOS%20%7C%20Windows-lightgrey?style=flat-square)](https://openparallax.dev/guide/installation)

[Documentation](https://openparallax.dev) &bull; [Quick Start](#quick-start) &bull; [Architecture](#architecture) &bull; [Modules](#modules) &bull; [Contributing](CONTRIBUTING.md)

</div>

---

> **Research paper:** OpenParallax is a reference implementation of the ideas presented in *Parallax: Why AI Agents That Think Must Never Act* (forthcoming on [arXiv](https://github.com/openparallax/openparallax)). The paper argues that giving an LLM direct execution capability is an architectural failure — the thinking process and the acting process must be physically separated, with a security pipeline between them.

## One Binary. Zero Dependencies. Any Platform.

A single static binary for Linux, macOS, and Windows. No Python. No Node.js. No Docker. No runtime dependencies. Download it, run it, done.

```bash
# Build from source (Linux, macOS, or Windows with Git Bash / WSL)
git clone https://github.com/openparallax/openparallax.git
cd openparallax
make build-all

# Initialize and start
./dist/openparallax init
./dist/openparallax start
```

## What It Does

OpenParallax is a personal AI agent that runs on your machine — any machine. It connects to LLM providers (Anthropic, OpenAI, Google, Ollama), executes 50+ tool actions on your behalf, and remembers context across conversations. Every action passes through a security pipeline before execution.

- **Talk through CLI, web, or messaging apps** — terminal TUI, glassmorphism web UI, WhatsApp, Telegram, Discord, Slack, Signal, Teams, iMessage
- **50+ actions** — files, git, shell, browser, email, calendar, canvas, HTTP, scheduling
- **Semantic memory** — FTS5 full-text search + vector embeddings that persist across sessions
- **3-tier security** — every tool call evaluated by policy rules, ML classifier, and LLM judge before execution
- **Kernel sandboxing** — agent process isolated at the OS level (Landlock, sandbox-exec, Job Objects)
- **Tamper-evident audit** — SHA-256 hash chain on every action
- **OTR mode** — off-the-record sessions with read-only access and zero persistence
- **Custom skills** — domain-specific guidance in markdown, loaded on demand

## Quick Start

**Prerequisites:** Go 1.25+, at least one LLM API key (`ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, or `GOOGLE_AI_API_KEY`)

```bash
make build-all                 # Build everything (Go + web frontend)
./dist/openparallax init       # Interactive setup wizard
./dist/openparallax start      # Launch agent + web UI
```

The init wizard configures your LLM provider, Shield security provider, and embedding provider with connection testing. After init, `start` launches the engine, spawns the sandboxed agent, opens the CLI, and starts the web UI on port 3100.

```bash
# Other useful commands
./dist/openparallax doctor     # 13-point health check
./dist/openparallax status     # Workspace stats
./dist/openparallax attach tui # Attach CLI to running agent
./dist/openparallax logs -f    # Tail engine log
./dist/openparallax audit --verify  # Verify audit chain integrity
```

## Architecture

Three OS processes at runtime:

```
openparallax start              (Process Manager)
  └── internal-engine           (Engine: privileged, security gate)
        ├── gRPC server         (AgentService, ClientService)
        ├── HTTP/WS server      (Web UI on :3100)
        ├── Shield pipeline     (3-tier security evaluation)
        ├── Audit, Chronicle    (logging, snapshots)
        └── internal-agent      (Agent: kernel-sandboxed)
              ├── LLM reasoning (context assembly, tool proposals)
              └── Skills        (on-demand domain guidance)
```

**The Agent** owns the LLM — it assembles context, runs the reasoning loop, and proposes tool calls. It runs inside a kernel sandbox (Landlock on Linux, sandbox-exec on macOS, Job Objects on Windows) and cannot access files, network, or processes outside its allowed scope.

**The Engine** is the security gate — it evaluates every tool proposal through Shield, checks IFC labels, takes Chronicle snapshots, logs to audit, and executes approved actions. The Agent never executes anything directly.

This separation is the core thesis of the [research paper](https://github.com/openparallax/openparallax) — an agent that thinks and an agent that acts must never be the same process.

**Why?** The Agent talks to external LLM APIs. If those APIs are compromised or the LLM is manipulated through prompt injection, the Agent could propose dangerous actions. The kernel sandbox + Shield pipeline ensures that even a fully compromised Agent cannot cause harm.

### Message Pipeline

```
User input → Store → Forward to Agent → LLM stream with tools
  → For each tool call:
      Shield.Evaluate → IFC.Check → Chronicle.Snapshot
      → Audit.Log → Execute → Audit.Log → Return to Agent
  → Response complete → Memory index → Title generation
```

## Modules

OpenParallax is composed of standalone modules. Use the whole system, or import individual modules into your own project:

| Module | Description | Go | Python | Node.js |
|--------|-------------|:---:|:------:|:-------:|
| **[Shield](https://openparallax.dev/shield/)** | 3-tier AI security pipeline (policy + classifier + LLM evaluator) | &#10003; | &#10003; | &#10003; |
| **[Memory](https://openparallax.dev/memory/)** | Semantic memory with pluggable backends (SQLite, pgvector, Qdrant, Pinecone, ...) | &#10003; | &#10003; | &#10003; |
| **[Audit](https://openparallax.dev/audit/)** | Tamper-evident append-only logging with SHA-256 hash chains | &#10003; | &#10003; | &#10003; |
| **[Sandbox](https://openparallax.dev/sandbox/)** | Kernel-level process isolation (Landlock, sandbox-exec, Job Objects) | &#10003; | | |
| **[Channels](https://openparallax.dev/channels/)** | WhatsApp, Telegram, Discord, Slack, Signal, Teams, iMessage | &#10003; | &#10003; | &#10003; |
| **[Chronicle](https://openparallax.dev/modules/chronicle)** | Copy-on-write workspace snapshots with rollback | &#10003; | | |
| **[LLM](https://openparallax.dev/modules/llm)** | Unified provider abstraction (Anthropic, OpenAI, Google, Ollama) | &#10003; | | |
| **[IFC](https://openparallax.dev/modules/ifc)** | Information flow control with sensitivity labels | &#10003; | | |
| **[Crypto](https://openparallax.dev/modules/crypto)** | ID generation, hash chains, canary tokens | &#10003; | | |
| **[MCP](https://openparallax.dev/modules/mcp)** | Model Context Protocol client integration | &#10003; | | |

### Shield as a Standalone Product

Shield can run as a standalone MCP security proxy — no OpenParallax required:

```bash
# Install Shield standalone
curl -sSL https://get.openparallax.dev/shield | sh

# Point it at your MCP servers
cat > shield.yaml <<EOF
listen: localhost:9090
upstream:
  - name: filesystem
    transport: stdio
    command: npx @modelcontextprotocol/server-filesystem /home
policy:
  file: policy.yaml
EOF

# Every MCP tool call now passes through 3-tier security
openparallax-shield serve
```

## Project Structure

```
cmd/agent/              CLI (Cobra): start, init, doctor, attach, get-classifier
cmd/shield/             Standalone Shield service

internal/
  agent/                LLM reasoning loop, context assembly, skills
  audit/                Append-only JSONL with SHA-256 hash chain
  channels/cli/         Terminal TUI (Bubbletea)
  chronicle/            Copy-on-write workspace snapshots
  config/               YAML config loader
  crypto/               ID generation, hashing, canary tokens
  engine/               Pipeline orchestrator, gRPC server, protection
    executors/          10 executor types (file, shell, git, browser, email, ...)
  llm/                  Provider abstraction (Anthropic, OpenAI, Google, Ollama)
  memory/               FTS5 + vector search, daily logs, embeddings
  sandbox/              Kernel isolation (Landlock, sandbox-exec, Job Objects)
  shield/               3-tier security pipeline
    tier0/              YAML policy matching
    tier1/              ONNX DeBERTa classifier + heuristics
    tier2/              LLM evaluator with canary verification
  storage/              SQLite (pure Go, zero CGo, WAL mode)
  web/                  HTTP + WebSocket server

proto/openparallax/v1/  Protobuf service definitions
web/                    Svelte 4 + TypeScript + Vite frontend
docs/                   VitePress documentation site
```

## Development

```bash
# Full verification — run after every change
make build-all && make test && make lint && cd web && npm test && cd ..

# Individual targets
make proto              # Generate gRPC code
make build-web          # Build Svelte frontend
make build              # Go binary → dist/openparallax
make test               # go test -race -count=1 ./...
make lint               # golangci-lint

# Docs
cd docs && npm run dev  # VitePress dev server at localhost:5173
```

### Tech Stack

| Layer | Technology |
|-------|-----------|
| Language | Go 1.25 (`CGO_ENABLED=0`), TypeScript 5.5 |
| Frontend | Svelte 4, Vite 5 |
| CLI | Cobra + Bubbletea |
| Transport | gRPC, WebSocket, REST |
| Database | SQLite (modernc.org, pure Go, WAL) |
| Search | FTS5 + cosine similarity vectors |
| ML Inference | onnxruntime-purego (pure Go FFI) |
| Protobuf | protoc + go/go-grpc plugins |

### Environment Variables

Third-party API keys use standard names:

| Variable | Purpose |
|----------|---------|
| `ANTHROPIC_API_KEY` | Anthropic Claude |
| `OPENAI_API_KEY` | OpenAI (chat + embeddings) |
| `GOOGLE_AI_API_KEY` | Google Gemini |

OpenParallax-specific variables use the `OP_` prefix: `OP_LOG_LEVEL`, `OP_DATA_DIR`, `OP_WEB_PORT`, `OP_SHIELD_POLICY`, etc. See the [full reference](https://openparallax.dev/reference/env-vars).

## Security

OpenParallax takes security seriously. The entire architecture is designed around the principle that the AI agent should never be trusted with direct execution capability.

- **3-tier Shield** — policy rules, DeBERTa ML classifier, LLM evaluator with canary verification
- **Kernel sandboxing** — Landlock (Linux 5.13+), sandbox-exec (macOS), Job Objects (Windows)
- **Fail-closed** — every Shield error path returns BLOCK, sandbox failure prevents agent startup
- **Canary probes** — platform-specific verification that sandbox is actually enforcing restrictions
- **File protection levels** — critical files (config, audit, canary tokens) are fully blocked from agent access
- **IFC** — information flow control prevents data exfiltration across sensitivity boundaries
- **Tamper-evident audit** — SHA-256 hash chain, any modification breaks the chain

For vulnerability reports, see [SECURITY.md](SECURITY.md).

## License

[Apache License 2.0](LICENSE)

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines. We welcome contributions across all modules.
