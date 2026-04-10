---
description: Install OpenParallax on Linux, macOS, or Windows — single static binary, zero runtime dependencies. Includes API key setup and build-from-source instructions.
---

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

### Manual verification

If you downloaded the binary manually instead of using the install script, verify the checksum and signature yourself:

```bash
# Download the archive, checksums, and signature
VERSION=v0.1.0
curl -sSLO "https://github.com/openparallax/openparallax/releases/download/${VERSION}/openparallax_${VERSION#v}_linux_amd64.tar.gz"
curl -sSLO "https://github.com/openparallax/openparallax/releases/download/${VERSION}/checksums.txt"
curl -sSLO "https://github.com/openparallax/openparallax/releases/download/${VERSION}/checksums.txt.sig"
curl -sSLO "https://github.com/openparallax/openparallax/releases/download/${VERSION}/checksums.txt.pem"

# Verify SHA-256 checksum
sha256sum --check --ignore-missing checksums.txt

# Verify cosign signature (keyless, sigstore)
cosign verify-blob \
  --certificate checksums.txt.pem \
  --signature checksums.txt.sig \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp "github.com/openparallax/openparallax" \
  checksums.txt
```

If you don't have cosign installed, the checksum verification alone is sufficient for integrity. Cosign adds provenance — it proves the binary was built by the GitHub Actions release workflow, not a third party.

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

To put the binary on your PATH the same way Path A does, copy or symlink it:

```bash
sudo cp dist/openparallax /usr/local/bin/openparallax
# or:
sudo ln -s "$PWD/dist/openparallax" /usr/local/bin/openparallax
```

After this, every `openparallax …` command in the rest of the documentation works exactly as written.

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

## Optional Downloads

Four optional downloads enhance specific subsystems but are not required:

| Download | Adds |
|---|---|
| **Tier 1 ONNX classifier** (`# Removed — see roadmap for sidecar`) | ML-based prompt-injection detection at Tier 1 |
| **sqlite-vec extension** (`openparallax get-vector-ext`) | Native in-database vector queries for semantic memory |
| **MCP servers** (`openparallax mcp install <name>`) | External tool integrations via the Model Context Protocol |
| **Skill packs** (`openparallax skill install <name>`) | Domain-specific guidance the agent loads on demand |

The `init` wizard offers the first two automatically. See [Optional Downloads](/guide/optional-downloads) for the full reference — what each adds, where files live on disk, how to remove, and what falls back if absent.

## Workspace Layout

After running `openparallax init`, your workspace looks like this (default location: `~/.openparallax/<agent-name>/`):

```
~/.openparallax/<agent-name>/
  config.yaml                    # Agent configuration
  SOUL.md                        # Core values and guardrails
  IDENTITY.md                    # Agent name, role, style
  USER.md                        # Your preferences
  MEMORY.md                      # Accumulated knowledge
  HEARTBEAT.md                   # Scheduled tasks
  AGENTS.md                      # Multi-agent roster
  security/shield/
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

Optional downloads (classifier, sqlite-vec, MCP servers, skill packs) live under `~/.openparallax/` directly and are shared across all workspaces. See [Optional Downloads → Where everything lives](/guide/optional-downloads#where-everything-lives) for the full layout.

## Next Steps

- [Quick Start](/guide/quickstart) — initialize a workspace and run your first conversation
- [Configuration](/guide/configuration) — every config.yaml option explained
