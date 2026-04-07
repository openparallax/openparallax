---
layout: home

hero:
  name: OpenParallax
  text: AI Agents That Think Must Never Act
  tagline: Open-source security framework for autonomous AI systems. 4-tier defense pipeline, kernel sandboxing, tamper-evident audit — because an agent that can execute anything will eventually execute the wrong thing.
  actions:
    - theme: brand
      text: Get Started
      link: /guide/
    - theme: alt
      text: Read the Paper
      link: https://github.com/openparallax/openparallax
    - theme: alt
      text: Architecture
      link: /technical/

features:
  - title: Shield
    details: 4-tier AI security pipeline — YAML policy matching, ONNX classifier with DeBERTa, and LLM evaluator with canary verification. Fail-closed by design. Use standalone as an MCP security proxy.
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
    details: Multi-platform messaging adapters — WhatsApp, Telegram, Discord, Slack, Signal, Teams, iMessage. Import the module and connect your agent to any platform without reimplementing OAuth and webhooks.
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

## The Core Principle

> *An agent that thinks and the system that acts must never be the same process.*

OpenParallax is the reference implementation of the ideas presented in [*Parallax: Why AI Agents That Think Must Never Act*](https://github.com/openparallax/openparallax) (forthcoming on arXiv). The central argument: giving an LLM direct execution capability is an architectural failure. The thinking process — which talks to external APIs and can be manipulated through prompt injection — must be physically separated from the execution process, with a security pipeline between them.

In OpenParallax, the **Agent** (sandboxed, kernel-isolated) proposes actions. The **Engine** (privileged, unsandboxed) evaluates every proposal through a 4-tier security pipeline before executing anything. Even a fully compromised Agent cannot cause harm — the sandbox prevents unauthorized access, and Shield blocks dangerous actions.

---

## One Binary. Zero Dependencies. Any Platform.

A single file. No Python, no Node.js, no Docker, no package managers. Download it and run it — on Linux, macOS, or Windows.

```bash
curl -sSL https://get.openparallax.dev | sh
openparallax init
openparallax start
```

The `init` wizard configures your LLM provider, security pipeline, and memory backend. After that, `start` launches the agent, opens the CLI, and starts the web UI. Three commands, any operating system, zero prerequisites.

::: tip Building from Source
See the [Quick Start guide](/guide/quickstart) for `git clone && make build-all`, or jump to the [Architecture](/technical/) to understand the design.
:::

---

## Two Products in One

**A complete AI agent** — talk through CLI, web, WhatsApp, Telegram, Discord, Slack, Signal, Teams, or iMessage. It reads your files, runs commands, manages your calendar, sends emails, and learns across conversations. Every action is secured and audit-logged.

**A composable security toolkit** — every component is independently importable. Building your own agent? Use [Shield](/shield/) for security, [Memory](/memory/) for semantic search, [Sandbox](/sandbox/) for process isolation — without buying into the full system.

### Composable Modules

| Module | What It Does | Go | Python | Node.js | Standalone |
|--------|-------------|:---:|:------:|:-------:|:----------:|
| [Shield](/shield/) | 4-tier AI security pipeline | &#10003; | &#10003; | &#10003; | &#10003; |
| [Memory](/memory/) | Semantic memory with pluggable backends | &#10003; | &#10003; | &#10003; | |
| [Audit](/audit/) | Tamper-evident hash chain logging | &#10003; | &#10003; | &#10003; | |
| [Channels](/channels/) | WhatsApp, Telegram, Discord, Slack, Signal, Teams, iMessage | &#10003; | &#10003; | &#10003; | |
| [Sandbox](/sandbox/) | Kernel-level process isolation | &#10003; | &#10003; | &#10003; | |
| [Chronicle](/modules/chronicle) | Copy-on-write snapshots with rollback | &#10003; | | | |
| [LLM](/modules/llm) | Anthropic, OpenAI + compatible APIs, Google, Ollama | &#10003; | | | |

Every module is a standalone Go package with zero dependencies on the rest of OpenParallax. Read [The Ecosystem](/technical/ecosystem) for the full story.
