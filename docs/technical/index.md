# Architecture Overview

OpenParallax is a multi-process system built in pure Go. Three OS processes collaborate at runtime, communicating over gRPC. Every security and infrastructure component is a standalone module — the Engine wires them together.

## The Three-Process Model

```
openparallax start              (Process Manager)
  └── internal-engine           (Engine: privileged, unsandboxed)
        ├── gRPC: AgentService, ClientService, SubAgentService
        ├── HTTP/WS server (:3100)
        ├── Shield pipeline
        ├── Audit, Chronicle, IFC
        └── internal-agent      (Agent: sandboxed, headless)
              ├── LLM reasoning loop
              ├── Context assembly
              └── Skill management
```

**Process Manager** (`openparallax start`) spawns the Engine and handles lifecycle — restart on exit code 75, crash recovery with budget, graceful shutdown on signals.

**Engine** (privileged) is the security gate. It runs the gRPC server, HTTP/WebSocket server, Shield pipeline, audit logging, chronicle snapshots, and all executors. It never makes LLM calls directly — that's the Agent's job. The Engine evaluates every tool call the Agent proposes before executing it.

**Agent** (sandboxed) owns the LLM. It assembles context from workspace files, manages the reasoning loop, proposes tool calls, and handles conversation compaction. The Agent runs inside a kernel sandbox — it physically cannot read files outside its workspace, make unauthorized network calls, or spawn child processes.

## Why This Separation?

The thinking/acting separation is the core architectural principle of OpenParallax, grounded in the theory presented in [*Parallax: Why AI Agents That Think Must Never Act*](https://github.com/openparallax/openparallax) (forthcoming on arXiv).

The Agent talks to external LLM APIs. If those APIs are compromised — or if the LLM is manipulated through prompt injection — the Agent could propose dangerous actions. By sandboxing the Agent and routing every action through the Engine's security pipeline, we ensure that even a fully compromised Agent cannot cause harm.

The Engine trusts nothing from the Agent. Every tool call passes through:

1. **Shield** — 3-tier security evaluation (policy → classifier → LLM judge)
2. **IFC** — information flow control labels prevent data exfiltration
3. **Chronicle** — workspace snapshot before any write operation
4. **Audit** — cryptographic hash chain logs every decision
5. **Executor** — only the Engine can execute actions

## Module Architecture

OpenParallax is composed of standalone modules that can be used independently:

| Module | Purpose | Standalone Value |
|--------|---------|-----------------|
| [Shield](/shield/) | 3-tier AI security pipeline | Drop-in security for any AI agent or MCP server |
| [Memory](/memory/) | Semantic memory (FTS5 + vectors) | Pluggable memory with multiple backends |
| [Audit](/audit/) | Tamper-evident logging | Hash-chain audit trail for any system |
| [Sandbox](/sandbox/) | Kernel process isolation | OS-level sandboxing for any process |
| [Chronicle](/modules/chronicle) | Copy-on-write snapshots | Workspace versioning with rollback |
| [Channels](/channels/) | Multi-platform messaging | WhatsApp, Telegram, Discord, Slack, Signal, Teams, iMessage adapters |
| [LLM](/modules/llm) | Provider abstraction | Anthropic, OpenAI + compatible APIs, Google, Ollama |
| [IFC](/modules/ifc) | Information flow control | Data classification and taint tracking |
| [MCP](/modules/mcp) | MCP client integration | Connect to any MCP server |
| [Crypto](/modules/crypto) | Security primitives | ID generation, hash chains, canary tokens |

Read [The Ecosystem](/technical/ecosystem) for the full story on why OpenParallax is structured as composable modules, how they interact, and how to use them independently.

## Communication

```
         gRPC (bidirectional streaming)
Agent ◄──────────────────────────────────► Engine
         AgentService.RunSession             │
                                             │
         gRPC (server streaming)             │
TUI   ◄─────────────────────────────────► Engine
Web   ◄──── WebSocket + REST ────────────► Engine
Channels ◄── HTTP webhooks ──────────────► Engine
         ClientService.SendMessage
```

- **Agent ↔ Engine**: Bidirectional gRPC stream. Agent sends LLM tokens, tool proposals, responses. Engine sends process requests, tool results, shutdown directives.
- **Clients ↔ Engine**: Server-streaming gRPC (CLI) or WebSocket (web). Clients send messages, receive pipeline events.
- **Channels ↔ Engine**: HTTP webhook handlers that normalize platform messages and call `ProcessMessageForWeb`.

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Language | Go 1.25 (`CGO_ENABLED=0`) |
| Frontend | Svelte 4, TypeScript, Vite 5 |
| CLI | Cobra + Bubbletea |
| Transport | gRPC (protobuf), WebSocket, REST |
| Database | SQLite (modernc.org, pure Go, WAL mode) |
| Search | FTS5 full-text + cosine similarity vectors |
| LLM | Anthropic Claude, OpenAI GPT (+ any OpenAI-compatible API), Google Gemini, Ollama |
| Security | Shield (policy + ONNX DeBERTa + LLM evaluator) |
| ML Inference | onnxruntime-purego (pure Go, no CGo) |
| Audit | Append-only JSONL with SHA-256 hash chain |
| Protobuf | protoc with go + go-grpc plugins |

## Next Steps

- [Process Model](/technical/process-model) — lifecycle, restart, crash recovery
- [Message Pipeline](/technical/pipeline) — the full per-message flow
- [The Ecosystem](/technical/ecosystem) — why composable modules, how they connect
- [Engine](/technical/engine) — the orchestrator internals
- [Agent](/technical/agent) — reasoning loop, context, compaction
