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

## One Binary. Zero Dependencies.

OpenParallax is a single file. Download it, run three commands, and you have a fully functional AI agent with a web UI, security pipeline, and semantic memory. No Python, no Node.js, no Docker, no package managers — nothing to install first.

```bash
curl -sSL https://get.openparallax.dev | sh
openparallax init
openparallax start
```

That's it. Works on Linux, macOS, and Windows. The init wizard walks you through choosing your LLM provider and configuring security. After that, `start` launches the agent, opens the CLI, and starts the web UI.

::: tip For Developers
Building from source? See the [Quick Start guide](/guide/quickstart) for the full `git clone && make build-all` setup, or jump straight to the [Architecture](/technical/) if you want to understand how the pieces fit together.
:::

---

## Two Products in One

**Use it as a complete AI agent** — talk through CLI, web, or messaging apps. It reads your files, runs commands, manages your calendar, sends emails, and learns across conversations. Every action is secured by a 3-tier pipeline and every decision is audit-logged.

**Use it as a module library** — every component is independently importable. Need security for your own agent? Use [Shield](/shield/). Need semantic memory? Use [Memory](/memory/). Need process isolation? Use [Sandbox](/sandbox/). No need to buy into the full system.

### Modules

| Module | What It Does | Go | Python | Node.js | Standalone |
|--------|-------------|:---:|:------:|:-------:|:----------:|
| [Shield](/shield/) | 3-tier AI security pipeline | &#10003; | &#10003; | &#10003; | &#10003; |
| [Memory](/memory/) | Semantic memory with pluggable backends | &#10003; | &#10003; | &#10003; | |
| [Audit](/audit/) | Tamper-evident hash chain logging | &#10003; | &#10003; | &#10003; | |
| [Channels](/channels/) | WhatsApp, Telegram, Discord, Slack, Signal, Teams | &#10003; | &#10003; | &#10003; | |
| [Sandbox](/sandbox/) | Kernel-level process isolation | &#10003; | | | |
| [Chronicle](/modules/chronicle) | Copy-on-write snapshots with rollback | &#10003; | | | |
| [LLM](/modules/llm) | Multi-provider abstraction | &#10003; | | | |

Every module is a standalone Go package with zero dependencies on the rest of OpenParallax. Read [The Ecosystem](/technical/ecosystem) for the full story on why it's built this way.
