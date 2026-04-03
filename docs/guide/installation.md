# Installation

OpenParallax is distributed as source code and compiles to a single static binary with zero runtime dependencies. There is no CGo, no shared libraries, and no container required.

## Prerequisites

### Go 1.25+

OpenParallax requires Go 1.25 or later. Check your version:

```bash
go version
```

If you need to install or upgrade Go, follow the [official instructions](https://go.dev/doc/install).

### Node.js 20+ (build-time only)

The web UI is built with Vite during compilation. Node.js is only needed at build time — the compiled assets are embedded into the Go binary via `go:embed`.

```bash
node --version   # v20.x or later
npm --version
```

### An LLM API Key

You need at least one API key from a supported provider:

| Provider | Environment Variable | Get a Key |
|----------|---------------------|-----------|
| Anthropic (Claude) | `ANTHROPIC_API_KEY` | [console.anthropic.com](https://console.anthropic.com/settings/keys) |
| OpenAI (GPT) | `OPENAI_API_KEY` | [platform.openai.com](https://platform.openai.com/api-keys) |
| Google (Gemini) | `GOOGLE_AI_API_KEY` | [aistudio.google.com](https://aistudio.google.com/apikey) |
| Ollama (local) | None required | [ollama.com](https://ollama.com/) |

Set the key in your shell profile:

```bash
# ~/.bashrc, ~/.zshrc, or equivalent
export ANTHROPIC_API_KEY="sk-ant-..."
```

For embedding-based semantic search, an additional key may be needed (OpenAI's `text-embedding-3-small` is recommended). The init wizard handles this configuration.

### protoc (optional)

Only needed if you modify the `.proto` files in `proto/openparallax/v1/`. Install `protoc` along with the Go plugins:

```bash
# Install protoc plugins
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Regenerate
make proto
```

## Build from Source

```bash
# Clone the repository
git clone https://github.com/openparallax/openparallax.git
cd openparallax

# Build everything (web UI + Go binaries)
make build-all
```

This runs three steps:

1. **`make build-web`** — installs npm dependencies and runs `vite build`, producing optimized assets in `web/dist/` that get embedded into the binary.
2. **`make build`** — compiles the main binary to `dist/openparallax` with `CGO_ENABLED=0`.
3. **`make build-shield`** — compiles the standalone Shield service to `dist/openparallax-shield`.

The resulting binary at `dist/openparallax` is fully self-contained: the web UI, templates, default policies, and all Go dependencies are embedded inside it.

### Verify the build

```bash
./dist/openparallax --version
```

### Run tests (optional but recommended)

```bash
make test           # Go tests with race detection
make lint           # golangci-lint
cd web && npm test  # Frontend tests (Vitest)
```

## Platform Notes

### Linux

Linux is the primary development platform. All features work, including full kernel sandboxing via Landlock LSM.

**Landlock requirements:**
- Kernel 5.13+ for filesystem isolation
- Kernel 6.7+ for network isolation (Landlock V4)
- No additional packages or configuration needed — Landlock is a built-in LSM

The agent process self-sandboxes on startup. If Landlock is unavailable (older kernel, disabled in boot config), the agent starts normally without sandbox restrictions. Run `openparallax doctor` to check sandbox status.

### macOS

All features work on macOS. Kernel sandboxing uses Apple's `sandbox-exec` facility.

**Notes:**
- `sandbox-exec` is deprecated by Apple but still functional on current macOS versions
- The engine wraps the agent spawn with a sandbox profile that restricts filesystem access, network access, and process spawning
- Chromium-based browsers are detected automatically for browser automation
- On Apple Silicon, the Go binary compiles natively for arm64

### Windows

OpenParallax runs on Windows with Job Objects providing process-level isolation.

**Notes:**
- Job Objects restrict the agent from spawning child processes
- Filesystem and network restrictions are not enforced at the kernel level on Windows — Shield policies remain the primary guard
- Use PowerShell or Windows Terminal for the TUI
- WSL is fully supported and provides the Linux sandbox capabilities

## Optional: ONNX Classifier Setup

Shield's Tier 1 includes an ONNX-based prompt-injection classifier. Without it, Tier 1 runs in heuristic-only mode (pattern matching and rule-based detection). The heuristic mode is effective for common injection patterns, but the ML classifier provides broader coverage.

To download the classifier model:

```bash
# Download the base model (~700MB, 98.8% accuracy)
./dist/openparallax get-classifier

# Or the smaller variant (~250MB, faster inference)
./dist/openparallax get-classifier --variant small

# Force re-download if files already exist
./dist/openparallax get-classifier --force
```

This downloads:
- `model.onnx` — the DeBERTa-v3 ONNX model
- `tokenizer.json` — the tokenizer configuration

The classifier runs in-process using pure Go ONNX inference. No Python, no Node.js sidecar, no external runtime.

After downloading, restart the agent. Shield will automatically detect and use the classifier. Verify with:

```bash
./dist/openparallax doctor
```

The Shield check will report whether the classifier is loaded.

## Directory Layout After Installation

After building and running `openparallax init`, your filesystem looks like this:

```
openparallax/                    # Source checkout
  dist/
    openparallax                 # Main binary
    openparallax-shield          # Standalone Shield binary

~/.openparallax/<agent-name>/    # Workspace (default location)
  config.yaml                    # Agent configuration
  SOUL.md                        # Core values and guardrails
  IDENTITY.md                    # Agent name, role, style
  USER.md                        # Your preferences
  MEMORY.md                      # Accumulated knowledge
  TOOLS.md                       # Tool descriptions
  BOOT.md                        # Startup checklist
  HEARTBEAT.md                   # Scheduled tasks
  AGENTS.md                      # Multi-agent roster
  policies/
    default.yaml                 # Default Shield policy
    permissive.yaml              # Low-friction policy
    strict.yaml                  # Maximum security policy
  skills/                        # Custom skill definitions
  .openparallax/
    openparallax.db              # SQLite database (sessions, memory)
    canary.token                 # Canary token for Shield verification
    audit.jsonl                  # Tamper-evident audit log
    engine.log                   # Engine log (when started with -v)
```

## Upgrading

Pull the latest source and rebuild:

```bash
cd openparallax
git pull
make build-all
```

Your workspace, sessions, memory, and configuration are preserved across upgrades. The SQLite database schema is versioned and migrates automatically on first start after an upgrade.

## Next Steps

- [Quick Start](/guide/quickstart) — initialize a workspace and run your first conversation
- [Configuration](/guide/configuration) — every config.yaml option explained
