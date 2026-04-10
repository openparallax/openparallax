# Roadmap

This page consolidates all planned features across the project. Each item links to its detailed documentation page where available.

---

## Immediate Next Steps

These are the highest-priority items from the roadmap below:

1. **Classifier sidecar binary** — The DeBERTa ONNX classifier has been deactivated in the main binary due to model quality issues (high false positive rate) and a Go runtime incompatibility between `onnxruntime-purego` and Landlock sandboxing. The classifier will ship as a separate CGo sidecar binary (`openparallax-classifier`) in its own repository. The infrastructure is ready: `classifier_mode: sidecar` and `classifier_addr` config fields exist, and the HTTP client is implemented. What's needed: model retraining on a revised dataset, the CGo sidecar binary wrapping Microsoft's C++ ONNX Runtime, and a new `get-classifier` command to download the sidecar.

2. **Image and video generation** — Implement `edit_image` across providers. Add video providers beyond OpenAI Sora.

3. **Cross-language wrappers** — Ship the Memory and Channels Python and Node.js packages. Shield, Audit, and Sandbox wrappers are already published.

4. **Standalone eval harness** — Decouple from OpenParallax internals so any Shield-based system can run the adversarial test suite.

---

## Channels

### Slack Adapter

**Status:** Config schema defined, adapter not built.

The [Slack adapter](/channels/slack) will use Socket Mode for real-time messaging without requiring a public webhook URL. Messages will be formatted in Slack's mrkdwn syntax. The adapter will support thread replies (responding in-thread rather than top-level), @mention triggers, and DM conversations. The `SlackConfig` struct is already defined in the codebase with `bot_token_env` and `app_token_env` fields.

### Microsoft Teams Adapter

**Status:** Config schema defined, adapter not built.

The [Teams adapter](/channels/teams) will use the Microsoft Bot Framework v4 with Adaptive Cards for rich message rendering. Inbound messages will be validated via JWT token verification against Microsoft's signing keys. The adapter will support 1:1 conversations, group chats, and channel mentions. The `TeamsConfig` struct is defined with `app_id_env` and `password_env` fields.

### Cross-Language Wrappers

**Status:** API designed, packages not built.

The [Channels Python wrapper](/channels/python) (`openparallax-channels` on PyPI) and [Channels Node.js wrapper](/channels/node) (`@openparallax/channels` on npm) will follow the same JSON-RPC bridge pattern used by the Shield, Audit, and Sandbox wrappers. A pre-built Go binary communicates over stdin/stdout with the host language. The Python wrapper will export a `ChannelAdapter` base class and built-in adapters for Telegram and WhatsApp. The Node.js wrapper will export a TypeScript `ChannelAdapter` interface.

---

## Memory

### Additional Backends

**Status:** Six backends have complete design docs. Three more are on the roadmap without designs yet.

The [SQLite backend](/memory/backends/sqlite) is the current default and only implemented backend. The `ChunkStore` interface is designed for pluggable backends — switching backends is a configuration change, not a code change.

| Backend | Design Status | Key Advantage |
|---------|--------------|---------------|
| [PostgreSQL + pgvector](/memory/backends/pgvector) | Complete | HNSW/IVFFlat vector indexing, ACID transactions, TSQUERY full-text search. Best choice for production deployments with existing PostgreSQL infrastructure. |
| [Qdrant](/memory/backends/qdrant) | Complete | Purpose-built vector database with gRPC and REST APIs, distributed mode, and advanced payload filtering. Best for large-scale deployments (millions of vectors). |
| [Pinecone](/memory/backends/pinecone) | Complete | Fully managed serverless vector database. Scale-to-zero pricing, zero ops overhead. Best for teams that don't want to manage infrastructure. |
| [ChromaDB](/memory/backends/chroma) | Complete | Simple API with sensible defaults, in-memory mode for testing. Best for rapid prototyping and small-to-medium deployments. |
| [Weaviate](/memory/backends/weaviate) | Complete | Native hybrid search combining vector and keyword in a single query. GraphQL API. Best when hybrid search quality is the top priority. |
| [Redis + RediSearch](/memory/backends/redis) | Complete | Sub-millisecond latency with HNSW indexes and combined vector+keyword queries. Best for latency-sensitive applications with existing Redis infrastructure. |
| MongoDB | Not yet designed | Document-oriented vector search via Atlas Vector Search. |
| DynamoDB | Not yet designed | AWS-native key-value store with vector search via OpenSearch integration. |
| Milvus | Not yet designed | Open-source vector database designed for billion-scale similarity search. |

### Cross-Language Wrappers

**Status:** API designed, packages not built.

The [Memory Python wrapper](/memory/python) (`openparallax-memory` on PyPI) and [Memory Node.js wrapper](/memory/node) (`@openparallax/memory` on npm) will provide native access to the memory subsystem including hybrid search, embedding, and all backends. Same JSON-RPC bridge pattern as the shipped Shield, Audit, and Sandbox wrappers.

---

## Generation

### Image Editing

**Status:** Tool schema defined, not implemented for any provider.

The `edit_image` tool is registered in the action type system and exposed to the LLM, but all three image providers (OpenAI, Google Imagen, Stability AI) return "not yet implemented" when called. The implementation requires provider-specific edit APIs: OpenAI's image edit endpoint accepts a source image + mask + prompt, Google Imagen supports inpainting and outpainting, and Stability AI provides image-to-image transformation. Each has different capabilities and constraints that need individual implementation.

### Additional Video Providers

**Status:** Only OpenAI Sora implemented.

Video generation currently supports only OpenAI's Sora model (`sora-2`). As Google, Stability AI, and other providers release video generation APIs, they will be added to the `VideoProvider` interface. The interface is ready — only the provider implementations are missing.

---

## Eval Suite

### Standalone Eval Harness

**Status:** Currently coupled to OpenParallax internals. Standalone mode planned.

The eval suite contains adversarial test cases that are universal — the attack scenarios (prompt injection, encoding tricks, helpfulness bypass, multi-agent exploitation) apply to any agent framework. However, the harness binary is currently wired to OpenParallax internals (`internal/config`, `internal/agent`, `internal/engine/executors`), so it cannot be imported or run against another project's security pipeline.

**What's planned:**
- Make inject mode work with just a Shield config and policy file — no OpenParallax workspace required
- Extract the test case YAML format into a framework-agnostic schema that any security tool can consume
- Remove `internal/` imports from the harness so external projects can use the Go API
- Publish the test suite dataset independently so other agent frameworks can benchmark against the same attacks

The attack data itself (the YAML files in `eval-results/test-suite/`) is already portable — it's just descriptions of attacks with prompts and expected outcomes. The implementation coupling is in the harness, not the test cases.

---

## Security & Sandbox

### Landlock Network Restriction (Linux)

**Status:** Detected by Probe, not enforced by ApplySelf.

Linux kernel 6.7+ with Landlock ABI v4 supports TCP connection restriction. OpenParallax detects this capability and reports it via `Probe()` and `Status.Network`, but `ApplySelf()` does not currently add network rules to the Landlock ruleset. When implemented, the agent process will be restricted to connecting only to the Engine's gRPC address and the LLM API host — all other outbound connections will be blocked by the kernel, preventing data exfiltration even if the agent is fully compromised.

### Windows AppContainers

**Status:** Roadmap, not yet designed.

The current Windows sandbox uses Job Objects, which restrict child process spawning but do not provide filesystem or network isolation. Windows AppContainers offer full filesystem and network isolation without requiring admin elevation. This would bring Windows sandboxing to parity with Linux (Landlock) and macOS (sandbox-exec).

### Classifier Model Retrain + CGo Sidecar

**Status:** [Planned](/guide/optional-downloads#planned-model-retrain-and-cgo-sidecar).

The current DeBERTa classifier works but has two limitations: inference is slow (~2s P50 via the pure-Go ONNX runtime) and the model is skewed toward false positives on encoding and obfuscation attack categories. Two improvements are planned:

1. **Model retrain** — a revised training set to reduce false positive rates while maintaining detection coverage across all attack categories.
2. **CGo sidecar binary** (`openparallax-classifier`) — a separate repository using Microsoft's C++ ONNX Runtime directly for ~30ms inference latency. The main `openparallax` binary stays zero-CGo; only the optional sidecar uses CGo. The infrastructure is ready: `shield.classifier_mode: sidecar` and `shield.classifier_addr` config fields already exist.

---

## Performance

### Pure-Go HNSW

**Status:** Roadmap, not yet designed.

The current vector search uses brute-force cosine similarity — every query iterates through all stored embeddings. This works well for personal agent workloads (up to ~100K vectors) but becomes a bottleneck at larger scales. A pure-Go HNSW (Hierarchical Navigable Small World) implementation would provide approximate nearest neighbor search with sub-linear query time while maintaining the zero-CGo constraint. The optional [sqlite-vec extension](/guide/optional-downloads#sqlite-vec-extension) provides an interim solution for users who need faster vector search today.

---

## Multi-Agent

### Inter-Agent Collaboration via A2A

**Status:** Architecture decided, not yet implemented.

We will adopt Google's [Agent-to-Agent (A2A)](https://a2a-protocol.org/latest/) protocol as the transport layer for inter-agent collaboration. A2A is an open standard (now under the Linux Foundation) with 150+ supporting organizations, production deployments, and official SDKs in five languages including Go ([`a2aproject/a2a-go`](https://github.com/a2aproject/a2a-go)).

This follows the same philosophy as our MCP adoption for tools: adopt the open standard instead of building a custom protocol.

**Architecture:**
- Each agent serves an A2A endpoint (on the existing web server)
- Agent Card at `/.well-known/agent.json` advertises identity, skills, and auth requirements
- Message passing, memory search, and file sharing are exposed as A2A Skills
- Our bilateral permission model (`allow_messages_from`, `allow_memory_search_from`) layers as auth middleware on the A2A server
- The `a2a-go` SDK provides both client and server packages with JSON-RPC, REST, and gRPC transport support

**Why A2A over a custom protocol:**
- Agents on different machines (laptop + server) can collaborate over HTTP — no shared filesystem required
- Any A2A-compatible agent framework can interact with OpenParallax agents out of the box
- The Go SDK is production-ready with extensible auth middleware and custom transport hooks
- No custom protocol to maintain, version, or debug

The implementation will use A2A as the wire protocol to deliver the capability requirements: message passing, federated memory search, shared files, and a bilateral permission model.

### Workspace Sharing

**Status:** Roadmap, not yet designed.

Currently each workspace belongs to a single user. Workspace sharing would allow multiple users to interact with the same agent, sharing sessions, memory, and configuration. This introduces OS-level user isolation, permission management, and conflict resolution for concurrent access. Depends on inter-agent collaboration infrastructure being in place first.

---

## Distribution

### Package Managers

**Status:** [Planned](/guide/installation#package-managers-planned).

Native package manager installs will provide automatic updates, dependency management, and platform-standard installation paths. Three package managers are targeted:

| Manager | Platform | Install Command |
|---------|----------|----------------|
| Homebrew | macOS / Linux | `brew install openparallax/tap/openparallax` |
| Scoop | Windows | `scoop install openparallax` |
| winget | Windows | `winget install OpenParallax.OpenParallax` |

Until these ship, the [curl/PowerShell one-liners](/guide/installation) are the supported install path. The Homebrew tap and Scoop bucket repos need to be created and populated with release automation.

---

## Already Shipped

These cross-language wrappers are built, published, and documented:

| Package | Language | Transport |
|---------|----------|-----------|
| `openparallax-shield` | Python | JSON-RPC over stdin/stdout to Go bridge binary |
| `@openparallax/shield` | Node.js | JSON-RPC over stdin/stdout to Go bridge binary |
| `openparallax-audit` | Python | JSON-RPC over stdin/stdout to Go bridge binary |
| `@openparallax/audit` | Node.js | JSON-RPC over stdin/stdout to Go bridge binary |
| `openparallax-sandbox` | Python | JSON-RPC over stdin/stdout to Go bridge binary |
| `@openparallax/sandbox` | Node.js | JSON-RPC over stdin/stdout to Go bridge binary |
