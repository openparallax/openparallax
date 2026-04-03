---
layout: home

hero:
  name: OpenParallax
  text: Your AI Agent, Your Rules
  tagline: Open-source personal AI agent with composable security, semantic memory, and multi-channel messaging. Use the whole system — or pick the modules you need.
  actions:
    - theme: brand
      text: Get Started
      link: /guide/
    - theme: alt
      text: View on GitHub
      link: https://github.com/openparallax/openparallax
    - theme: alt
      text: Architecture
      link: /technical/

features:
  - title: Shield
    details: 3-tier AI security pipeline — YAML policy matching, ONNX classifier with DeBERTa, and LLM evaluator with canary verification. Fail-closed by design. Use standalone as an MCP security proxy.
    link: /shield/
    linkText: Explore Shield
  - title: Memory
    details: Semantic memory with FTS5 full-text search and vector embeddings. Pluggable backends — SQLite for personal agents, PostgreSQL + pgvector, Qdrant, Pinecone, and more.
    link: /memory/
    linkText: Explore Memory
  - title: Sandbox
    details: Kernel-level process isolation. Landlock on Linux, sandbox-exec on macOS, Job Objects on Windows. Canary probes verify enforcement. Fail-closed — agent refuses to start if sandboxing fails.
    link: /sandbox/
    linkText: Explore Sandbox
  - title: Audit
    details: Tamper-evident append-only logging with SHA-256 hash chains. Every decision, every tool call, every security verdict — cryptographically verifiable.
    link: /audit/
    linkText: Explore Audit
  - title: Channels
    details: Multi-platform messaging adapters — WhatsApp, Telegram, Discord, Slack, Signal, Teams. Import the module and connect your agent to any platform without reimplementing OAuth and webhooks.
    link: /channels/
    linkText: Explore Channels
  - title: Chronicle
    details: Copy-on-write workspace snapshots with rollback. Every file modification is tracked. Undo any action, restore any state.
    link: /modules/chronicle
    linkText: Explore Chronicle
---

<style>
.VPHome {
  padding-bottom: 64px;
}
</style>

## The Composable AI Agent Toolkit

OpenParallax is two things:

1. **A complete personal AI agent** — install it, configure it, talk to it through CLI, web, or messaging apps. It reads your files, executes commands, manages your calendar, sends emails, and learns from every conversation.

2. **A modular toolkit** — every security, memory, and infrastructure component is independently importable. Building your own AI agent? Use our [Shield](/shield/) for security, our [Memory](/memory/) for semantic search, our [Sandbox](/sandbox/) for process isolation — without buying into the full system.

### Architecture at a Glance

```
Process Manager → Engine (privileged) → Agent (sandboxed)
                     ↓                      ↓
              Shield, Audit,          LLM reasoning,
              Chronicle, IFC         context assembly,
                     ↓               tool proposals
              Channels, Web              ↓
              (external clients)    gRPC ← Engine evaluates
                                        → executes
                                        → returns result
```

Every module in this diagram is a standalone Go package. The Engine wires them together for the full OpenParallax experience, but each one works independently. Read [The Ecosystem](/technical/ecosystem) for the full story.

### Available in Go, Python, and Node.js

Core modules ship as Go packages with cross-language wrappers via JSON-RPC bridges:

| Module | Go | Python | Node.js | Standalone |
|--------|:---:|:------:|:-------:|:----------:|
| [Shield](/shield/) | &#10003; | &#10003; | &#10003; | &#10003; |
| [Memory](/memory/) | &#10003; | &#10003; | &#10003; | |
| [Audit](/audit/) | &#10003; | &#10003; | &#10003; | |
| [Channels](/channels/) | &#10003; | &#10003; | &#10003; | |
| [Sandbox](/sandbox/) | &#10003; | | | |
| [Chronicle](/modules/chronicle) | &#10003; | | | |
| [LLM](/modules/llm) | &#10003; | | | |

### One Binary. Zero Dependencies. Any Platform.

OpenParallax is a single static binary. No Python. No Node.js. No Docker. No runtime dependencies. Download one file, run it, and you have a fully functional AI agent with security, memory, and a web UI.

```bash
# That's it. No package managers, no pip, no npm, no brew.
curl -sSL https://get.openparallax.dev | sh
openparallax init
openparallax start
```

Under the hood: pure Go with `CGO_ENABLED=0`. SQLite compiled to Go via `modernc.org/sqlite`. ONNX inference via `onnxruntime-purego`. The web UI is embedded in the binary via `go:embed`. Everything ships as one file — no installers, no PATH configuration, no version conflicts. It works on Linux, macOS, and Windows out of the box.
