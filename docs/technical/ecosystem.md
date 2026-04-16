# The Ecosystem

OpenParallax is not a monolith. It is a collection of standalone modules that happen to work together as a complete AI agent. This page explains why it was built this way, how the modules compose, and how you can use any of them independently.

## Why Composable?

The AI agent ecosystem has a problem: every project reinvents the same infrastructure. Security evaluation, semantic memory, audit logging, process sandboxing, messaging adapters — these are solved problems, but every agent framework builds them from scratch, tightly coupled to their specific architecture.

OpenParallax takes a different approach. Each infrastructure component is a self-contained Go package with its own types, its own configuration, and zero dependencies on the rest of the system. The Engine — the OpenParallax-specific orchestrator — imports these modules and wires them together. But the modules themselves know nothing about the Engine.

This means:

- **You can use Shield in your FastAPI agent** without importing anything else from OpenParallax
- **You can use Memory with PostgreSQL + pgvector** in a system that has nothing to do with agents
- **You can drop Audit into your existing microservice** for tamper-evident logging
- **You can use Sandbox to isolate any child process**, not just AI agents

## The Module Map

```
PUBLIC MODULES (independently importable)
┌─────────────────────────────────────────────────────────┐
│                                                         │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌───────────┐ │
│  │ Shield  │  │ Memory  │  │  Audit  │  │  Sandbox  │ │
│  │ (4-tier │  │ (FTS5 + │  │ (hash   │  │ (kernel   │ │
│  │  AI     │  │  vector │  │  chain  │  │  process  │ │
│  │  sec)   │  │  search)│  │  JSONL) │  │  isolation)│ │
│  └────┬────┘  └────┬────┘  └────┬────┘  └─────┬─────┘ │
│       │            │            │              │        │
│  ┌────┴────┐  ┌────┴────┐  ┌───┴────┐  ┌─────┴─────┐ │
│  │Chronicle│  │ Channels│  │  LLM   │  │    IFC    │ │
│  │ (CoW    │  │ (multi- │  │(multi- │  │ (info     │ │
│  │  snaps) │  │ platform│  │provider│  │  flow     │ │
│  │         │  │  msg)   │  │ API)   │  │  control) │ │
│  └─────────┘  └─────────┘  └────────┘  └───────────┘ │
│                                                         │
│  ┌─────────┐  ┌─────────┐                              │
│  │  Crypto │  │   MCP   │                              │
│  │ (IDs,   │  │ (client │                              │
│  │  hashes,│  │  integ- │                              │
│  │  canary)│  │  ration)│                              │
│  └─────────┘  └─────────┘                              │
└─────────────────────────────────────────────────────────┘
                          │
                          │ imports
                          ▼
┌─────────────────────────────────────────────────────────┐
│              OPENPARALLAX (the orchestrator)            │
│                                                         │
│  ┌──────────────────────────────────────────────────┐  │
│  │                    Engine                         │  │
│  │  Wires all modules together into the pipeline:    │  │
│  │  message → Shield → IFC → Chronicle → Executor   │  │
│  │          → Audit → Memory → Response              │  │
│  └──────────────────────────────────────────────────┘  │
│                                                         │
│  ┌───────┐  ┌─────────┐  ┌────────┐  ┌────────────┐  │
│  │ Agent │  │ Session │  │ Config │  │    Web     │  │
│  │(reason│  │(OTR,    │  │(YAML   │  │ (HTTP/WS  │  │
│  │ loop) │  │lifecycle│  │ schema)│  │  server)  │  │
│  └───────┘  └─────────┘  └────────┘  └────────────┘  │
└─────────────────────────────────────────────────────────┘
```

Everything in the top box is a standalone module. Everything in the bottom box is OpenParallax-specific — it only makes sense as part of the full agent system.

## How OpenParallax Uses Its Modules

When you run `openparallax start`, the Engine initializes each module independently and wires them into the message pipeline:

### Startup

```go
// Simplified — each module initializes independently
shield := shield.New(shield.Config{PolicyFile: "security/shield/default.yaml"})
audit  := audit.New("audit.jsonl")
chronicle := chronicle.New(workspacePath, chronicle.MaxSnapshots(50))
memory := memory.New(sqlite.New("memory.db"), memory.WithEmbedder(embedder))
sandbox := sandbox.New(sandbox.Config{AllowRead: []string{workspacePath}})
```

No module knows about any other module. The Engine holds references to all of them and orchestrates their interaction.

### Per-Message Flow

When a message arrives, the Engine runs this pipeline:

```
1. RECEIVE    → Client sends message via gRPC/WebSocket
2. STORE      → Message saved to session history
3. FORWARD    → Message sent to Agent over gRPC stream
4. AGENT LOOP → Agent assembles context, calls LLM, proposes tool calls
5. EVALUATE   → For each tool call proposal:
   a. Shield.Evaluate(action)     → ALLOW / BLOCK / ESCALATE
   b. IFC.Check(action, labels)   → verify information flow constraints
   c. Chronicle.Snapshot()        → save workspace state before write
   d. Audit.Log(proposal)         → record proposal with hash chain
   e. Executor.Execute(action)    → actually do the thing
   f. Audit.Log(result)           → record result with hash chain
6. RETURN     → Results sent back to Agent for next LLM round
7. COMPLETE   → Final response stored, broadcast to all clients
8. MEMORY     → Conversation indexed for future retrieval
```

Each step uses a different module. If you removed Shield, the pipeline would still work — just without security evaluation. If you removed Chronicle, writes would still happen — just without snapshots. The modules are additive, not load-bearing for basic functionality.

### The Dependency Graph

The module dependency graph is strictly acyclic and layered:

```
FOUNDATION (zero internal dependencies)
├── crypto       — IDs, hash chains, canary tokens
├── sandbox      — kernel process isolation
├── platform     — OS detection utilities
└── logging      — structured logging

INFRASTRUCTURE (depends only on foundation)
├── storage      — SQLite abstraction
├── llm          — LLM provider abstraction
├── ifc          — information flow labels
└── mcp          — MCP client

SERVICES (depends on foundation + infrastructure)
├── shield       — uses crypto, llm
├── audit        — uses crypto
├── chronicle    — uses crypto, storage
├── memory       — uses crypto, llm, storage
├── channels     — uses crypto, storage
└── session      — uses crypto, llm, storage

ORCHESTRATION (depends on everything)
└── engine       — wires all modules into the pipeline
```

No module at a lower layer imports from a higher layer. This is enforced by the Go compiler (import cycles are compile errors) and verified by the dependency analysis.

## Using Modules Independently

### In Go

Every module is a standard Go package:

```go
import "github.com/openparallax/openparallax/shield"

s, err := shield.New(shield.Config{
    PolicyFile: "policy.yaml",
    Classifier: shield.WithLocalONNX("~/.openparallax/models/"),
})

verdict := s.Evaluate(ctx, shield.Action{
    Type: "file_write",
    Path: "/etc/passwd",
    Content: "root::0:0...",
})

if verdict.Decision == shield.Block {
    log.Fatal("blocked:", verdict.Reason)
}
```

### In Python and Node.js

Core modules ship with cross-language wrappers that communicate with a small Go binary over JSON-RPC (stdin/stdout) — the same protocol MCP uses:

```python
# pip install openparallax-shield
from openparallax_shield import Shield

shield = Shield(policy_file="policy.yaml")
verdict = shield.evaluate(action="file_write", path="/etc/passwd")
```

```typescript
// npm install @openparallax/shield
import { Shield } from '@openparallax/shield'

const shield = new Shield({ policyFile: 'policy.yaml' })
const verdict = await shield.evaluate({ action: 'file_write', path: '/etc/passwd' })
```

The Go binary is pre-built for all platforms (linux/darwin/windows, amd64/arm64) and downloaded automatically on package install — like esbuild or Prisma.

### Shield as a Standalone Product

Shield can run as a standalone MCP security proxy — no OpenParallax required:

```bash
# Install
curl -sSL https://get.openparallax.dev/shield | sh

# Configure
cat > shield.yaml <<EOF
listen: localhost:9090
upstream:
  - name: filesystem
    transport: stdio
    command: npx @modelcontextprotocol/server-filesystem /home
  - name: github
    transport: streamable-http
    url: https://mcp-github.example.com
policy:
  file: policy.yaml
classifier:
  model_dir: ~/.openparallax/models/prompt-injection/
EOF

# Run — every MCP tool call now passes through 4-tier security
openparallax-shield serve
```

Point any MCP client (Claude Desktop, Cursor, custom agents) at `http://localhost:9090/mcp` instead of directly at your MCP servers. Shield evaluates every tool call, blocks dangerous ones, forwards safe ones, and audit-logs everything.

## Design Decisions

### Why Go?

Single static binary. Cross-compile to every platform from one machine. No runtime dependencies. The ONNX classifier runs via `onnxruntime-purego` (pure Go FFI, no CGo). SQLite runs via `modernc.org/sqlite` (pure Go transpilation). The result: one binary, zero install friction.

### Why Not a Plugin System?

Plugins add complexity without proportional value. Go's package system already gives you composability — import what you need, ignore what you don't. The module interfaces are the plugin system.

### Why JSON-RPC for Cross-Language?

It's what MCP uses, it's simple, it works over stdin/stdout (no port conflicts), and it's supported in every language. The alternative — gRPC — would require protobuf compilation in consumer projects. JSON-RPC just works.

### JSON-RPC Bridge Protocol

Cross-language bridge binaries (`openparallax-shield-bridge`, `openparallax-audit-bridge`, etc.) use the `internal/jsonrpc` package — a minimal JSON-RPC 2.0 server over stdin/stdout. It reads newline-delimited JSON requests from stdin and writes responses to stdout.

```go
srv := jsonrpc.NewServer()
srv.Handle("evaluate", func(params json.RawMessage) (any, error) { ... })
srv.Serve() // blocks, reading from stdin
```

Python and Node.js wrappers spawn the bridge binary as a subprocess and communicate via this protocol.

### Why SQLite as the Default Memory Backend?

Personal agents run on personal machines. SQLite needs no server, no configuration, no network. It's a single file. For production deployments with millions of vectors, swap to PostgreSQL + pgvector or Qdrant — the interface is the same.

The current SQLite backend uses brute-force cosine similarity (no ANN index). This works well up to ~100K vectors. We're exploring pure-Go HNSW implementations for larger scales while maintaining the zero-CGo constraint. See [SQLite Backend — Limitations & Roadmap](/memory/backends/sqlite) for details.

### Why Separate Processes?

The Agent makes LLM API calls. Those APIs could be compromised. The LLM could be manipulated through prompt injection. By isolating the Agent in a kernel sandbox and routing every action through the Engine's security pipeline, we ensure that a compromised Agent cannot cause harm — even with full code execution capability.

This is defense in depth: the sandbox prevents unauthorized system access, Shield prevents dangerous actions, IFC prevents data exfiltration, Chronicle enables rollback, and Audit provides forensic evidence. No single layer is sufficient. Together, they make the system trustworthy.

The composable module architecture and the strict separation between thinking (Agent) and acting (Engine) are grounded in the research presented in [*Parallax: Why AI Agents That Think Must Never Act*](https://arxiv.org/abs/2604.12986) (arXiv:2604.12986).
