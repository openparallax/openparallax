# Getting Started

OpenParallax is an open-source personal AI agent that runs on your machine, under your control. It connects to LLM providers (Anthropic, OpenAI and any OpenAI-compatible API, Google, Ollama), executes actions on your behalf, and remembers context across conversations — all secured by a 3-tier security pipeline.

OpenParallax is a reference implementation of the architecture proposed in [*Parallax: Why AI Agents That Think Must Never Act*](https://github.com/openparallax/openparallax) (forthcoming on arXiv).

## What You Get

- **CLI interface** — a terminal-native conversation UI built with Bubbletea
- **Web UI** — a glassmorphism interface with session management and real-time streaming
- **73 tool actions** — file operations, git, shell commands, browser automation, email, calendar, canvas, HTTP requests, sub-agents, clipboard, system utilities, and more
- **Semantic memory** — FTS5 full-text search plus vector embeddings that persist across sessions
- **Custom skills** — define domain-specific guidance in markdown that the agent loads on demand
- **Multi-channel messaging** — connect to WhatsApp, Telegram, Discord, Slack, Signal, or Teams
- **3-tier security** — every tool call passes through policy matching, ML classification, and LLM evaluation before execution
- **Kernel sandboxing** — the agent process is isolated at the OS level (Landlock, sandbox-exec, Job Objects)
- **Tamper-evident audit** — every action logged with SHA-256 hash chains
- **OTR mode** — off-the-record sessions with read-only access and no memory persistence

## Prerequisites

- **Go 1.25+** (for building from source)
- **An LLM API key** — at least one of: `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `GOOGLE_AI_API_KEY`
- **Linux, macOS, or Windows**

## Quick Install

```bash
# Clone and build
git clone https://github.com/openparallax/openparallax.git
cd openparallax
make build-all

# Initialize a workspace
./dist/openparallax init

# Start the agent
./dist/openparallax start
```

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
