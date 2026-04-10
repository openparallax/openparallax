---
description: Get started with OpenParallax — an open-source AI agent with 4-tier security, 69 tools, semantic memory, and multi-channel messaging support.
---

# Getting Started

OpenParallax is an open-source AI agent that runs on your machine, under your control, with your data staying local. It connects to LLM providers (Anthropic, OpenAI and any OpenAI-compatible API, Google, Ollama), executes actions on your behalf, and remembers context across conversations — all secured by a 4-tier security pipeline that structurally prevents the LLM from causing harm even if fully compromised.

Whether you're a developer automating code workflows, a researcher handling sensitive datasets, an analyst processing financial records, a legal team reviewing contracts, or a compliance officer evaluating AI tooling — OpenParallax is built so you don't have to trust the LLM to trust the system.

OpenParallax is a reference implementation of the architecture proposed in [*Parallax: Why AI Agents That Think Must Never Act*](https://github.com/openparallax/openparallax/releases/download/v0.1.0/parallax-paper.pdf) (PDF, arXiv forthcoming).

## What You Get

- **Work through any interface** — terminal TUI, glassmorphism web UI, WhatsApp, Telegram, Discord, Signal, iMessage (Slack and Teams planned)
- **69 tool actions across 14 groups** — file operations, git, shell commands, browser automation, email (SMTP + IMAP), calendar, canvas, HTTP requests, sub-agents, clipboard, system utilities, and more
- **Semantic memory** — FTS5 full-text search plus vector embeddings that persist across sessions
- **Custom skills** — define domain-specific guidance in markdown that the agent loads on demand
- **4-tier security** — every tool call passes through policy matching, ML classification, LLM evaluation, and human approval before execution
- **Information flow control** — sensitivity labels prevent credentials, financial data, medical records, and other classified information from leaking across trust boundaries
- **Kernel sandboxing** — the agent process is isolated at the OS level (Landlock, sandbox-exec, Job Objects)
- **Tamper-evident audit** — every action logged with SHA-256 hash chains
- **OTR mode** — off-the-record sessions with read-only access and no memory persistence

## Prerequisites

- **An LLM API key** — at least one of: `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `GOOGLE_AI_API_KEY`. See [Installation → An LLM API Key](/guide/installation#an-llm-api-key) for where to get one and how to set it.
- **Linux, macOS, or Windows**

## Quick Install

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

The install script downloads a single static binary, drops it on your PATH, and verifies its checksum. No Go, no Node.js, no toolchain required.

If you prefer to build from source (contributors, anyone tracking `main`), see [Installation → Path B](/guide/installation#path-b-build-from-source).

The `init` wizard walks you through:
1. Choosing your agent name and identity
2. Selecting LLM provider and model for chat
3. Configuring Shield (security) provider and model
4. Setting up embedding provider for vector search
5. Connection testing for both chat and shield

After init, `start` launches the engine, spawns the sandboxed agent, and opens both the CLI and web UI.

## Next Steps

- [Installation](/guide/installation) — detailed build instructions and platform-specific notes
- [Quick Start](/guide/quickstart) — your first conversation, end to end
- [Configuration](/guide/configuration) — every `config.yaml` option explained
- [CLI Commands](/guide/cli) — all commands, flags, and slash commands
