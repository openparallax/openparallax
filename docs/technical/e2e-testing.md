---
description: End-to-end testing for OpenParallax — boot a real engine, connect a real agent, run through the full pipeline with mock or real LLM providers.
---

# E2E Testing

The E2E test suite boots a full engine process, connects a sandboxed agent, and runs tests through the complete pipeline: message → tool proposal → Shield evaluation → execution → response. It catches integration regressions that unit tests miss — gRPC protocol changes, WebSocket event ordering, Shield + IFC interaction, sub-agent lifecycle, and audit chain integrity.

E2E tests are build-tagged `//go:build e2e` so they never leak into `make test` or production builds.

## Quick Start

```bash
# Build the binary first
make build

# Run with mock LLM (default, no API key needed)
E2E_LLM=mock go test -tags e2e -timeout 300s ./e2e/...

# Verbose output showing each test
E2E_LLM=mock go test -tags e2e -v -timeout 300s ./e2e/...
```

## LLM Modes

The same tests run against three different LLM backends. The mode is selected via the `E2E_LLM` environment variable.

### Mock (default, CI)

```bash
E2E_LLM=mock go test -tags e2e -timeout 300s ./e2e/...
```

A local HTTP server that implements the OpenAI Chat Completions SSE protocol. Returns deterministic, pattern-matched responses. No API key, no network, no cost. This is what CI runs on every push.

The mock matches on the last user message content and the conversation state (assistant turn count, tool results present). Patterns include:
- First message → `load_tools` tool call
- `/etc/shadow` in message → `read_file` (triggers Shield block)
- "read file" → `read_file` on a workspace file
- "write" + "test.txt" → `write_file`
- "delegate" → `create_agent` (sub-agent spawn)
- After any tool result → text summary ("Done.")

### Ollama (local real model)

```bash
E2E_LLM=ollama OLLAMA_MODEL=llama3.2 go test -tags e2e -v ./e2e/...
```

Uses a locally running Ollama instance. Tests real LLM behavior — streaming, tool call parsing, multi-turn reasoning. Skips if Ollama is not reachable on `localhost:11434`.

Useful for validating:
- Tool call JSON parsing with a real model (models sometimes emit malformed JSON)
- Streaming chunk boundaries
- Context window behavior under real token counts

### Cloud (real provider)

```bash
# Anthropic
E2E_LLM=cloud E2E_PROVIDER=anthropic go test -tags e2e -v ./e2e/...

# OpenAI
E2E_LLM=cloud E2E_PROVIDER=openai go test -tags e2e -v ./e2e/...

# Google
E2E_LLM=cloud E2E_PROVIDER=google go test -tags e2e -v ./e2e/...

# OpenAI-compatible (Groq, Together, any base_url)
E2E_LLM=cloud E2E_PROVIDER=openai \
  E2E_BASE_URL=https://api.groq.com/openai/v1 \
  E2E_MODEL=llama-3.3-70b \
  OPENAI_API_KEY=gsk-... \
  go test -tags e2e -v ./e2e/...
```

Uses real API calls. Tests the full stack including Shield Tier 2 evaluation (real LLM evaluator). Requires the corresponding API key in the environment (`ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `GOOGLE_AI_API_KEY`). Skips if the key is not set.

Useful for validating:
- Provider-specific streaming behavior
- Real Shield Tier 2 verdicts
- Context assembly and compaction with real token counts
- Model-specific tool call format quirks

## When to Use Which Mode

| You changed... | Recommended mode |
|---|---|
| Engine pipeline, Shield, IFC, executors, gRPC | `mock` — deterministic, fast |
| LLM provider code (`llm/`), streaming, tool call parsing | `ollama` or `cloud` — mock can't catch streaming edge cases |
| Context assembly, compaction, history (`internal/agent/`) | `cloud` — prompt regressions need a real model |
| Sub-agent orchestration | `mock` — sub-agent lifecycle is protocol-level |
| Frontend only | Skip E2E entirely, run `cd web && npm test` |
| Pre-commit sanity check | `mock` — 10 seconds, catches most regressions |
| Pre-release validation | `cloud` with your production provider — full confidence |

## Test Inventory

The suite covers the core integration paths:

| Test | What it validates |
|---|---|
| `TestEngineBootAndHealth` | Engine starts, `/api/status` returns 200, sandbox info present |
| `TestSendMessageGetResponse` | Full message round-trip: send → LLM stream → `response_complete` |
| `TestToolCallReadFile` | Tool call pipeline: `load_tools` → `read_file` → Shield ALLOW → content returned |
| `TestShieldBlocksDangerousCommand` | Shield Tier 0 blocks `/etc/shadow` read, verdict = BLOCK |
| `TestOTRBlocksWrites` | OTR mode filters write tools, file not created |
| `TestSandboxActive` | Sandbox is active on Linux (Landlock) |
| `TestSubAgentSpawnAndComplete` | Sub-agent spawns, executes, completes, result returned |
| `TestAgentSurvivesBlockedActionSequence` | Agent stays alive after a Shield block — second message in same session succeeds |
| `TestSessionLifecycle` | Create, list, delete sessions via REST API |
| `TestChronicleRollback` | Chronicle snapshot taken before write, rollback restores |
| `TestSlashCommands` | `/status`, `/doctor`, `/history` return results |
| `TestDynamicToolLoading` | `load_tools` meta-tool works, response completes |
| `TestAuditChainIntegrity` | Audit chain is valid after multiple actions |

## Architecture

The E2E harness (`e2e/harness.go`) manages the full lifecycle:

1. **Setup** — creates a temp workspace, writes config + policy files, starts the mock LLM server (if mock mode), builds the binary
2. **Start** — spawns the engine process, reads the gRPC port from stdout, polls `/api/status` until the agent is connected and responsive
3. **Tests** — each test creates sessions via REST, connects via WebSocket, sends messages, and collects events until `response_complete`
4. **Teardown** — SIGTERM to engine, wait for exit, remove temp workspace

The engine runs as a real OS process (not in-process) — the same binary that ships to users. The agent is a real sandboxed child process. Tests exercise the actual gRPC protocol, WebSocket events, and HTTP REST API.

## Writing New E2E Tests

```go
//go:build e2e

package e2e

func TestMyFeature(t *testing.T) {
    te := SharedEngine
    sid := te.CreateSession(t, "")
    ws := te.WS(t)

    require.NoError(t, ws.SendMessage(sid, "your prompt here"))

    events, err := ws.CollectUntil("response_complete", 120*time.Second)
    require.NoError(t, err)

    rc := FindEvent(events, "response_complete")
    require.NotNil(t, rc)
    // ... assert on events
}
```

Key helpers:
- `SharedEngine` — the single engine instance shared across all tests
- `te.CreateSession(t, mode)` — creates a session via REST (`""` = normal, `"otr"` = OTR)
- `te.WS(t)` — connects a WebSocket client, auto-closed on test cleanup
- `ws.SendMessage(sid, content)` — sends a chat message
- `ws.SendCommand(sid, command)` — sends a slash command
- `ws.CollectUntil(eventType, timeout)` — reads events until the specified type appears
- `FindEvent(events, type)` — finds the first event of a given type
- `CountEvents(events, type)` — counts events of a given type

## CI Integration

The E2E suite runs in CI on every push to `main` and on every release tag:

```yaml
# From .github/workflows/ci.yml
- name: Run E2E tests
  env:
    E2E_LLM: mock
  run: go test -tags e2e -count=1 -timeout 300s ./e2e/...
```

Mock mode only in CI — no API keys, no external dependencies, deterministic results.
