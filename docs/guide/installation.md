# Installation

OpenParallax ships as a single static binary with zero runtime dependencies. There is no CGo, no shared libraries, and no container required.

There are two ways to install:

- **[Path A — Install the Binary](#path-a-install-the-binary)** is for end users who just want to run an agent. One command, no toolchain.
- **[Path B — Build from Source](#path-b-build-from-source)** is for contributors and anyone who wants to track main, run the test suite, or modify the code.

Both paths produce the same binary. Both require an LLM API key (covered below).

## An LLM API Key

You need at least one API key from a supported provider before running `openparallax init`:

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

For embedding-based semantic search, an additional key may be needed (OpenAI's `text-embedding-3-small` is recommended). The `init` wizard handles this configuration.

## Path A: Install the Binary

This is the fastest path. The install script detects your OS and architecture, downloads the prebuilt binary, and drops it on your PATH. No Go, no Node.js, no toolchain needed on your machine.

### Linux / macOS

```bash
curl -sSL https://get.openparallax.dev | sh
openparallax init
openparallax start
```

### Windows (PowerShell)

```powershell
irm https://get.openparallax.dev/install.ps1 | iex
openparallax init
openparallax start
```

### What the script does

1. Detects your OS (`linux`, `darwin`, `windows`) and architecture (`amd64`, `arm64`)
2. Downloads the matching `openparallax-<version>-<os>-<arch>` archive from the [latest GitHub release](https://github.com/openparallax/openparallax/releases/latest)
3. Verifies the archive against its SHA-256 checksum from the release manifest
4. Extracts to `/usr/local/bin/openparallax` (Linux/macOS) or `%LOCALAPPDATA%\openparallax\` (Windows) and ensures it is on your PATH
5. Prints the installed version and a one-line "next step" pointing at `openparallax init`

### Verify the install

```bash
openparallax --version
openparallax doctor
```

`doctor` runs a 13-point health check that confirms the binary, sandbox capabilities, default policy, and (after `init`) your workspace and provider connectivity.

### Upgrading

Re-run the same install command. The script downloads the latest release and replaces the binary in place. Your workspace at `~/.openparallax/<agent-name>/` is untouched — sessions, memory, and configuration carry over across upgrades.

### Package managers (planned)

Native package manager installs are planned for a future release:

- **macOS (Homebrew)**: `brew install openparallax/tap/openparallax`
- **Windows (Scoop)**: `scoop bucket add openparallax https://github.com/openparallax/scoop-bucket && scoop install openparallax`
- **Windows (winget)**: `winget install OpenParallax.OpenParallax`

Until these ship, the curl/PowerShell one-liners above are the supported install path.

## Path B: Build from Source

Use this path if you are contributing to OpenParallax, tracking `main`, or want to run the test suite. You need a Go toolchain and Node.js on your machine.

### Prerequisites

#### Go 1.25+

OpenParallax requires Go 1.25 or later:

```bash
go version
```

If you need to install or upgrade Go, follow the [official instructions](https://go.dev/doc/install).

#### Node.js 20+ (build-time only)

The web UI is built with Vite during compilation. Node.js is only needed at build time — the compiled assets are embedded into the Go binary via `go:embed`.

```bash
node --version   # v20.x or later
npm --version
```

#### protoc (optional)

Only needed if you modify the `.proto` files in `proto/openparallax/v1/`:

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
make proto
```

### Clone and build

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

### Run tests (recommended)

```bash
make test           # Go tests with race detection
make lint           # golangci-lint
cd web && npm test  # Frontend tests (Vitest)
```

### Run the eval suite (recommended for security-sensitive use)

```bash
go build -o dist/openparallax-eval ./cmd/eval
```

See [Test Your Own Security](/eval/) for the full reproduction recipe.

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

**Install via Homebrew** (when published):

```bash
brew install openparallax/tap/openparallax
```

**Notes:**
- `sandbox-exec` is deprecated by Apple but still functional on current macOS versions
- The engine wraps the agent spawn with a sandbox profile that restricts filesystem access, network access, and process spawning
- Chromium-based browsers are detected automatically for browser automation
- On Apple Silicon, the Go binary compiles natively for arm64
- iMessage channel integration is available on macOS only (via AppleScript bridge to Messages.app)

### Windows

OpenParallax runs on Windows with Job Objects providing process-level isolation.

**Install via Scoop or winget** (when published):

```powershell
# Scoop
scoop bucket add openparallax https://github.com/openparallax/scoop-bucket
scoop install openparallax

# winget
winget install OpenParallax.OpenParallax
```

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

This downloads three files into `~/.openparallax/models/prompt-injection/`:

- `model.onnx` — the DeBERTa-v3 weights, fetched from [huggingface.co/openparallax/shield-classifier-v1](https://huggingface.co/openparallax/shield-classifier-v1)
- `tokenizer.json` — matching tokenizer config
- `libonnxruntime.{so,dylib,dll}` — Microsoft ONNX Runtime shared library, fetched from the [microsoft/onnxruntime](https://github.com/microsoft/onnxruntime/releases) releases for your platform

The classifier runs in-process using a pure-Go wrapper around the dynamically loaded shared library. The main `openparallax` binary itself stays zero-CGo — the runtime extension is opt-in and isolated.

After downloading, restart the agent. Shield's classifier auto-detects the files and switches from heuristic-only to dual-classifier mode. Verify with:

```bash
./dist/openparallax doctor
```

You should see `Tier 1: classifier enabled (local mode, 7 action type(s) bypassed)`. The 7 bypassed action types are `write_file`, `delete_file`, `move_file`, `copy_file`, `send_email`, `send_message`, `http_request` — see [Shield Tier 1 → Per-Action-Type ONNX Skip List](/shield/tier1#per-action-type-onnx-skip-list) for the rationale.

## Optional: sqlite-vec Extension

For semantic memory search at scale, the optional `sqlite-vec` extension provides native in-database vector queries:

```bash
./dist/openparallax get-vector-ext
```

This downloads the latest sqlite-vec release for your platform from [github.com/asg017/sqlite-vec](https://github.com/asg017/sqlite-vec/releases) into `~/.openparallax/extensions/sqlite-vec.{so,dylib,dll}`. Without it, Memory falls back to a built-in pure-Go cosine searcher (slower on large workspaces but functionally identical).

For the full optional download story including skill packs and the future CGo classifier sidecar, see [Optional Downloads](/guide/optional-downloads).

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
