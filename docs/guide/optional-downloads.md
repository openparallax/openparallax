# Optional Downloads

OpenParallax ships as a single static binary with everything needed to run a fully functional agent. Three optional downloads enhance specific subsystems but are not required:

| Download | Size | Adds | Command |
|---|---|---|---|
| **Tier 1 ONNX classifier** | ~700MB (base) or ~250MB (small) | ML-based prompt-injection detection at Tier 1 | `openparallax get-classifier` |
| **sqlite-vec extension** | ~1MB | Native in-database vector queries for semantic memory | `openparallax get-vector-ext` |
| **MCP servers** | varies | External tool integrations via the Model Context Protocol | `openparallax mcp install <name>` |
| **Skill packs** | ~10KB each | Domain-specific guidance for specific tasks | `openparallax skill install <name>` |

The `init` wizard offers the first two automatically. The last two are opt-in and can be installed at any time.

## Tier 1 ONNX Classifier

### What it adds

Without the classifier, Shield's Tier 1 runs in **heuristic-only mode**: regex pattern matching against ~79 hand-written rules. This catches obvious shell injection patterns, credential access attempts, and known attack signatures, but it cannot reason about novel attacks, encoded payloads, or social-engineering structures.

With the classifier, Tier 1 also runs a fine-tuned [DeBERTa-v3 prompt-injection model](https://huggingface.co/openparallax/shield-classifier-v1) in parallel with the heuristic engine. The classifier catches encoding/obfuscation, multi-agent payloads, and indirect injection patterns the heuristics miss. **Run-013 numbers prove the difference**: heuristic-only mode regresses C5 (encoding) from 0% → 100% ASR, C4 (toolchain) from 0% → 92%, C6 (multiagent) from 0% → 96%. The classifier is doing critical work, not optional polish.

### Honest disclosure

The default classifier achieves **98.8% accuracy** on its held-out test split. The model card, training data, and evaluation script are all public on the HuggingFace repo.

There is a known limitation: the training set is weighted toward injection-positive examples, so the model **over-fires on benign structured payloads** — `write_file` content, `send_email` body, `http_request` body. This is why the default config bypasses ONNX for those action types via `shield.classifier_skip_types`. The Tier 2 LLM evaluator handles them instead. Retraining on a balanced corpus is tracked as future work.

This trade-off is documented end-to-end in [eval reports/03-classifier-optimization](https://github.com/openparallax/openparallax/blob/main/eval-results/reports/03-classifier-optimization.md).

### What gets downloaded

`openparallax get-classifier` downloads three files into `~/.openparallax/models/prompt-injection/`:

| File | Source | Size | Purpose |
|---|---|---|---|
| `model.onnx` | [huggingface.co/openparallax/shield-classifier-v1](https://huggingface.co/openparallax/shield-classifier-v1) | ~700MB (base) or ~250MB (small) | The DeBERTa weights in ONNX format |
| `tokenizer.json` | same | <1MB | Tokenizer config matching the model |
| `libonnxruntime.{so,dylib,dll}` | [github.com/microsoft/onnxruntime/releases](https://github.com/microsoft/onnxruntime/releases) | ~30MB | Microsoft ONNX Runtime shared library, latest release for your platform |

The shared library is loaded dynamically at runtime via `dlopen`. The OpenParallax binary itself stays zero-CGo — the runtime extension is opt-in and isolated.

### Variants

```bash
openparallax get-classifier                    # base model (~700MB, 98.8% accuracy, default)
openparallax get-classifier --variant small    # small model (~250MB, faster inference)
openparallax get-classifier --force            # re-download even if files exist
```

### After downloading

Restart the engine. Shield's classifier auto-detects the files and switches from heuristic-only to dual-classifier mode. Verify with:

```bash
openparallax doctor
```

You should see `Tier 1: classifier enabled (local mode, 7 action type(s) bypassed)`.

If `openparallax doctor` reports `heuristic-only`, the files weren't found. Check that `~/.openparallax/models/prompt-injection/` contains all three files and that the engine has read permission.

### Future: CGo sidecar (separate repo)

The pure-Go ONNX runtime carries significant per-call overhead (~2s P50 in run-013). A CGo-based `openparallax-classifier` sidecar binary is planned to live in a separate repo. It will use Microsoft's C++ ONNX Runtime directly (not the pure-Go wrapper) and bring inference latency to ~30ms. The main `openparallax` binary stays zero-CGo; only the optional sidecar uses CGo. When it ships, set `shield.classifier_mode: sidecar` in your config and `openparallax-classifier` will be auto-spawned and managed by the engine.

## sqlite-vec Extension

### What it adds

Without the extension, OpenParallax's [memory subsystem](/guide/memory) uses a built-in pure-Go cosine-similarity searcher: every query iterates through all stored embeddings and computes similarity in Go. This works fine for small workspaces (<10K memories) but slows down as the corpus grows.

With the extension, vector queries run **inside SQLite** using sqlite-vec's optimized index structures. Sub-millisecond lookups regardless of corpus size, and the query planner can combine vector search with FTS5 full-text filters in a single SQL statement.

### What gets downloaded

`openparallax get-vector-ext` downloads the latest sqlite-vec release for your platform from [github.com/asg017/sqlite-vec/releases](https://github.com/asg017/sqlite-vec/releases) into `~/.openparallax/extensions/sqlite-vec.{so,dylib,dll}`.

The extension is a runtime-loaded SQLite plugin — not statically linked. The main binary stays CGo-free; the extension is loaded only if it exists, and only if the workspace's memory store opens with extension loading enabled.

### After downloading

Restart the engine. The memory indexer detects the extension and uses it for new embeddings. Existing embeddings continue to work via the built-in searcher; both code paths produce identical results, just at different speeds.

## MCP Servers

### What they add

[Model Context Protocol](https://modelcontextprotocol.io) servers expose external tools to the agent — RSS readers, GitHub APIs, Notion databases, web scrapers, vector stores, anything someone has wrapped in an MCP server. Once installed, MCP tools appear under `mcp:<server-name>` in the [load_tools](/guide/tools) meta-tool, alongside the built-in tool groups.

### What gets downloaded

`openparallax mcp install <name>` fetches a prebuilt MCP server binary from [github.com/openparallax/mcp](https://github.com/openparallax/mcp) releases (auto-selects the right artifact for your OS and architecture) and drops it into `~/.openparallax/mcp/<name>/<binary>`.

```bash
openparallax mcp install rss              # RSS feed reader
openparallax mcp install github           # GitHub API integration
openparallax mcp list                     # List installed MCP servers
openparallax mcp remove <name>            # Uninstall
```

### After installing

The `mcp install` command prints the YAML you need to add to your workspace `config.yaml`:

```yaml
mcp:
  servers:
    - name: rss
      command: ~/.openparallax/mcp/rss/rss-mcp
```

After editing the config, restart the engine. The agent can now load MCP tools via `load_tools(["mcp:rss"])`.

### Custom MCP servers

You are not limited to the openparallax/mcp registry. Any MCP-compliant binary works — point `mcp.servers[].command` at your own binary path and the engine will spawn it as a subprocess on startup. See [Configuration → MCP](/guide/configuration#mcp) for the full server config schema.

## Skill Packs

### What they add

Skills are markdown documents that give the agent domain-specific guidance for particular tasks. A "git" skill might explain your team's branching strategy and commit message conventions. A "compliance" skill might list the data-handling rules for HIPAA workflows. The agent loads skills on demand via the [load_tools](/guide/tools) meta-tool, so skill content only enters the LLM context when relevant.

### What gets downloaded

`openparallax skill install <name>` fetches a skill from [github.com/openparallax/skills](https://github.com/openparallax/skills) into `~/.openparallax/skills/<name>/SKILL.md`. The repo is curated; each skill has its own directory with a `SKILL.md` file containing YAML frontmatter and markdown body.

```bash
openparallax skill install developer        # software engineering best practices
openparallax skill install writer           # writing and editing
openparallax skill install ops              # devops and infrastructure
openparallax skill list                     # list installed skills
```

Workspace-local skills override globals: place a `SKILL.md` under `<workspace>/skills/<name>/` to use a custom version for that workspace only.

See [Custom Skills](/guide/skills) for the schema and how to author your own.

## Where everything lives

```
~/.openparallax/
├── models/
│   └── prompt-injection/
│       ├── model.onnx
│       ├── tokenizer.json
│       └── libonnxruntime.so       (or .dylib / .dll)
├── extensions/
│   └── sqlite-vec.so               (or .dylib / .dll)
├── mcp/
│   ├── rss/
│   │   └── rss-mcp                 (binary)
│   └── github/
│       └── github-mcp              (binary)
├── skills/
│   ├── developer/
│   │   └── SKILL.md
│   └── ops/
│       └── SKILL.md
└── <workspace-name>/               (one per agent)
    ├── config.yaml
    ├── SOUL.md / IDENTITY.md / etc.
    ├── policies/
    └── .openparallax/
        ├── openparallax.db
        ├── audit.jsonl
        └── canary.token
```

The first four (`models/`, `extensions/`, `mcp/`, `skills/`) are **shared across all workspaces**. Each agent's workspace is independent.

## Removing optional downloads

Each download can be removed manually by deleting its directory:

```bash
rm -rf ~/.openparallax/models/prompt-injection/   # removes ONNX classifier
rm -rf ~/.openparallax/extensions/                # removes sqlite-vec
openparallax mcp remove <name>                    # removes one MCP server (also: rm -rf ~/.openparallax/mcp/<name>/)
rm -rf ~/.openparallax/skills/<name>/             # removes one skill
```

After removing the classifier, Shield gracefully falls back to heuristic-only mode on the next engine start. After removing sqlite-vec, the memory indexer falls back to the built-in searcher. After removing an MCP server, the agent loses access to its tools but otherwise runs normally. After removing a skill, the agent simply no longer has access to it.
