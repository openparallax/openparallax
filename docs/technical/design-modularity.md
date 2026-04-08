# Modularity: Standalone Infrastructure Modules

OpenParallax is a personal AI agent built from standalone infrastructure modules. Shield, Memory, Audit, Sandbox, Chronicle, and IFC are independently importable Go packages with no dependency on the engine or agent. This page covers the module structure and how the pieces compose.

## Independently Importable Modules

Agent frameworks typically implement security, memory, and audit as internal components tied to their orchestrator. OpenParallax's modules are designed as standalone packages. Each module is a self-contained Go package at the repository root:

```
shield/       4-tier AI security pipeline
memory/       FTS5 + vector semantic search
audit/        Tamper-evident JSONL logging
sandbox/      Kernel-level process isolation
chronicle/    Copy-on-write workspace snapshots
ifc/          Information flow control
llm/          Multi-provider LLM abstraction
crypto/       ID generation, hash chains, canary tokens
mcp/          MCP client integration
channels/     Multi-platform messaging adapters
```

Each package has its own types, its own configuration, and zero imports from `internal/`. The Engine — the OpenParallax-specific orchestrator in `internal/engine/` — imports these modules and wires them together. The modules know nothing about the Engine.

Import Shield into your own project:

```go
import "github.com/openparallax/openparallax/shield"

gw := shield.NewGateway(shield.GatewayConfig{
    Policy:     policyEngine,
    Classifier: dualClassifier,
    Evaluator:  evaluator,
    FailClosed: true,
})

verdict := gw.Evaluate(ctx, &shield.ActionRequest{
    Type:    "shell_exec",
    Command: userProvidedCommand,
})
```

No engine. No agent. No gRPC. Just the security pipeline, running in your process, evaluating your actions. The same applies to every other module.

## Bridge Binaries for Cross-Language Support

Go packages cannot be imported from Python or Node.js. But a subprocess that speaks JSON-RPC over stdin/stdout can be called from any language.

The 5 bridge binaries are thin Go processes that expose module functionality via JSON-RPC:

```
cmd/shield-bridge/      → Shield evaluation pipeline
cmd/audit-bridge/       → Audit trail read/write/verify
cmd/memory-bridge/      → Semantic search and indexing
cmd/sandbox-bridge/     → Sandbox application and canary verification
cmd/channels-bridge/    → Channel adapter management
```

The Python wrapper spawns the bridge and provides an idiomatic API:

```python
from openparallax_shield import Shield, ShieldConfig

shield = Shield(ShieldConfig(
    policy_path="policy.yaml",
    fail_closed=True,
))

verdict = shield.evaluate(action_type="shell_exec", command=cmd)
if verdict.decision == "BLOCK":
    raise SecurityError(verdict.reason)
```

Behind the scenes, this serializes the request as JSON, sends it to `shield-bridge` via stdin, and reads the response from stdout. The bridge is a compiled Go binary — the full Shield pipeline (YAML policy + ONNX classifier + LLM evaluator) runs in the bridge process, not in Python.

The Node.js wrapper follows the same pattern:

```typescript
import { Shield } from 'openparallax-shield';

const shield = new Shield({ policyPath: 'policy.yaml', failClosed: true });
const verdict = await shield.evaluate({ type: 'shell_exec', command: cmd });
```

This architecture means the Python and Node.js wrappers are thin — they manage the bridge subprocess lifecycle and serialize/deserialize JSON. All logic runs in Go. Updates to the Shield pipeline automatically propagate to all language wrappers when the bridge binary is rebuilt.

## Zero CGo

The entire project compiles to a single static binary with `CGO_ENABLED=0`. This constraint shapes every dependency choice:

- **SQLite**: `modernc.org/sqlite` — a pure Go transpilation of the SQLite C source. No `libsqlite3`, no shared libraries, no build-time C compiler.
- **ONNX inference**: `onnxruntime-purego` — a Go-to-C bridge using `purego` (runtime dlopen) instead of CGo. The ONNX runtime shared library is loaded at runtime, not linked at build time.
- **Protobuf**: Pure Go generated code. No C++ runtime.

The result:

```bash
# Cross-compile from any platform to any target
GOOS=linux GOARCH=arm64 go build -o openparallax-linux-arm64
GOOS=darwin GOARCH=amd64 go build -o openparallax-darwin-amd64
GOOS=windows GOARCH=amd64 go build -o openparallax-windows-amd64.exe

# Deploy by copying one file
scp openparallax-linux-arm64 server:/usr/local/bin/openparallax
```

No shared library dependencies. No platform-specific compilation. No build matrix. One `go build` produces a self-contained binary that runs on the target platform.

This is particularly important for the bridge binaries, which need to run on any machine where the Python or Node.js wrapper is installed. Requiring a C toolchain on the user's machine — or shipping platform-specific shared libraries — would make the wrappers impractical. A single static binary per platform is a straightforward distribution model.

## Type Hierarchy: ifc/ to shield/ to internal/types/

The module packages form a dependency tree where each layer adds types without duplicating them:

```
ifc/
  ActionType          (base: "file_read", "shell_exec", ...)
  SensitivityLevel    (public, internal, confidential, restricted)
       │
       ▼
shield/
  re-exports ActionType, SensitivityLevel (type alias)
  adds: Verdict, VerdictDecision, ActionRequest, Gateway
       │
       ▼
internal/types/
  re-exports everything from shield/
  adds: 40+ action types used by engine executors
  adds: Config, SessionInfo, PipelineEvent, ...
```

This layering serves the modularity goal:

- **Import `ifc/`**: Get data classification primitives. No security pipeline, no engine types. Useful for tagging data sensitivity in any application.
- **Import `shield/`**: Get the full security evaluation pipeline plus IFC types. No engine, no agent, no executor types. Useful for adding AI security to any LLM application.
- **Import `internal/types/`**: Engine-internal. Gets everything. Not meant for external consumption.

Each layer re-exports via type aliases, not wrapper types. `shield.ActionType` is literally `ifc.ActionType` — no conversion, no mapping, no impedance mismatch. Code that works with `ifc.ActionType` works with `shield.ActionType` because they are the same type.

## Proto as Wire Format, Not Source of Truth

The protobuf definitions in `proto/openparallax/v1/` define the gRPC wire protocol — how the Engine, Agent, and clients communicate over the network. The Go types used in application logic come from the module packages.

This separation exists because:

1. **Modules work without gRPC.** Importing `shield/` to evaluate actions does not require protobuf, gRPC, or any network protocol. The Go types are the API. The proto is for serialization.

2. **Proto types are constrained by wire format.** Protobuf has limited type expressiveness — no interfaces, no methods, no custom serialization. The Go types have full language support: methods, interfaces, validation logic, custom JSON marshaling.

3. **Different consumers need different interfaces.** A library user imports Go types. A distributed system uses gRPC. A Python wrapper uses JSON-RPC. Each has its own serialization format; none should dictate the in-memory representation.

The Engine translates between proto types and Go types at the gRPC boundary. This is a small amount of mapping code, but it keeps the module packages clean — they never import `google.golang.org/protobuf` or any generated code.

## OTR Mode

Not every conversation should be remembered. Medical questions, financial details, personal matters — users need a way to interact with the agent without creating a permanent record.

OTR (Off-The-Record) mode is privacy as a first-class feature:

- **Storage**: `sync.Map` in memory. Never touches SQLite. Destroyed on process shutdown.
- **Tools**: Write-capable groups (`files`, `shell`, `git`) are disabled at the definition level via `DisableGroups`. The tools are not filtered from responses — they are never loaded.
- **Memory**: No memory logging, no embedding indexing.
- **Audit**: OTR sessions are recorded in the audit trail (the fact that they happened) but message content is not logged.
- **Visual**: The web UI switches all `--accent-*` CSS tokens from cyan to amber via the `.otr` class on the root element. The color change is persistent and unmissable — the user always knows they are in ephemeral mode.

```css
/* Normal mode: cyan accents */
:root {
  --accent-base: rgba(0, 220, 255, 1);
  --accent-glow: rgba(0, 220, 255, 0.4);
}

/* OTR mode: amber accents */
.otr {
  --accent-base: rgba(255, 191, 0, 1);
  --accent-glow: rgba(255, 191, 0, 0.4);
}
```

OTR is activated via the `/otr` slash command in both the web UI and CLI. The session is marked as OTR at creation time — there is no way to convert a normal session to OTR or vice versa. This prevents accidental data persistence after the user thought they were in private mode.

## Unified Channel Entry Point

Every channel — CLI, web UI, Telegram, Discord, Slack, WhatsApp, Signal, Teams — enters the engine through the same function:

```go
engine.ProcessMessageForWeb(ctx, sender, sessionID, messageID, content, mode)
```

The `sender` implements a single interface:

```go
type EventSender interface {
    SendEvent(event *PipelineEvent) error
}
```

Eight event types cover the entire pipeline: `llm_token`, `action_started`, `shield_verdict`, `action_completed`, `action_artifact`, `response_complete`, `otr_blocked`, `error`. Each channel adapter implements `EventSender` to translate pipeline events into its platform's format — WebSocket JSON for the web UI, gRPC streaming for the CLI, Telegram API calls for Telegram.

This design means adding a new channel requires:

1. Implement `EventSender` for the transport
2. Handle incoming messages — create or look up a session, call `ProcessMessageForWeb`
3. Translate the 8 event types into the platform's output format

The security pipeline, tool execution, memory, audit, and chronicle are completely channel-agnostic. A message from Telegram goes through the same Shield evaluation as a message from the CLI. There is no "trusted channel" concept — every channel is untrusted input.

## Intelligence-Agnostic Principles

The Parallax principles are *Intelligence-Agnostic*: they evaluate actions, not reasoning. Whether the agent uses autoregressive language models, reinforcement learning, or architectures not yet developed, Shield evaluates the same interface — action type, target, payload, and data flow. The security properties hold across AI architectures because the invariant is universal: autonomous systems propose actions with consequences.

See [The Ecosystem](/technical/ecosystem) for the full module map and dependency graph.
