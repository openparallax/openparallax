---
description: All install commands for OpenParallax — the main agent, Shield standalone, and language wrappers for Python and Node.js.
---

# Install Reference

Every way to install OpenParallax and its components.

## OpenParallax (the agent)

The full AI agent with Shield, IFC, memory, channels, and web UI.

**Linux / macOS:**
```bash
curl -sSL https://get.openparallax.dev | sh
```

**Windows (PowerShell):**
```powershell
irm https://get.openparallax.dev/install.ps1 | iex
```

**From source:**
```bash
git clone https://github.com/openparallax/openparallax.git
cd openparallax && make build-all
```

After install:
```bash
openparallax init    # interactive setup wizard
openparallax start   # launch agent + web UI
```

## Shield (standalone)

The 4-tier security pipeline as a standalone binary — no OpenParallax agent required. Use it to protect any AI agent, MCP server, or tool-calling pipeline.

**Linux / macOS:**
```bash
curl -sSL https://get.openparallax.dev/shield | sh
```

**Windows (PowerShell):**
```powershell
irm https://get.openparallax.dev/install.ps1 -OutFile install.ps1
.\install.ps1 -Component shield
```

**From source:**
```bash
git clone https://github.com/openparallax/openparallax.git
cd openparallax && make build-shield
```

After install:
```bash
openparallax-shield serve --config shield.yaml
```

## Python Wrappers

Use individual OpenParallax modules in your Python projects. Each package auto-downloads its Go bridge binary on first use.

```bash
pip install openparallax-shield      # 4-tier security pipeline
pip install openparallax-audit       # tamper-evident audit logging
pip install openparallax-memory      # semantic memory with search
pip install openparallax-sandbox     # kernel-level process isolation
pip install openparallax-channels    # multi-channel messaging
```

## Node.js Wrappers

Use individual OpenParallax modules in your TypeScript/JavaScript projects. Each package auto-downloads its Go bridge binary during install.

```bash
npm install @openparallax/shield     # 4-tier security pipeline
npm install @openparallax/audit      # tamper-evident audit logging
npm install @openparallax/memory     # semantic memory with search
npm install @openparallax/sandbox    # kernel-level process isolation
npm install @openparallax/channels   # multi-channel messaging
```

## Go Library

Import modules directly — no bridge binary needed.

```bash
go get github.com/openparallax/openparallax/shield
go get github.com/openparallax/openparallax/audit
go get github.com/openparallax/openparallax/memory
go get github.com/openparallax/openparallax/sandbox
go get github.com/openparallax/openparallax/channels
```

## Install Options

Both install scripts support version pinning and custom install directories:

**Linux / macOS:**
```bash
# Pin to a specific version
curl -sSL https://get.openparallax.dev | sh -s -- --version v0.1.0

# Custom install directory
curl -sSL https://get.openparallax.dev | sh -s -- --dir /opt/openparallax/bin

# Shield with version pin
curl -sSL https://get.openparallax.dev/shield | sh -s -- --version v0.1.0
```

**Windows (PowerShell):**
```powershell
# Pin to a specific version
.\install.ps1 -Version v0.1.0

# Custom install directory
.\install.ps1 -Dir "C:\tools\openparallax"

# Shield with version pin
.\install.ps1 -Component shield -Version v0.1.0
```

## Verify Your Install

```bash
openparallax doctor    # 13-point health check (agent)
openparallax-shield --help    # Shield standalone
python -c "import openparallax_shield; print('ok')"    # Python wrapper
node -e "import('@openparallax/shield').then(() => console.log('ok'))"    # Node.js wrapper
```

## Requirements

| Component | Requirements |
|---|---|
| OpenParallax agent | An LLM API key (Anthropic, OpenAI, Google, or Ollama) |
| Shield standalone | A YAML policy file |
| Python wrappers | Python 3.9+ |
| Node.js wrappers | Node.js 18+ |
| Go library | Go 1.25+ |
| Building from source | Go 1.25+, Node.js 20+ |
