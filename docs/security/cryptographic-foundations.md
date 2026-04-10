# Cryptographic Foundations

These primitives underpin the integrity guarantees of higher-level mechanisms. They don't defend against threats directly — they provide the plumbing that makes tamper detection, authentication, and verification possible.

## Action Hashing

**Purpose:** TOCTOU prevention. Every action is hashed (SHA-256 of tool name + serialized arguments) at proposal time. The hash travels with the action through the pipeline and is re-verified before execution.

**Code:** `crypto/hash.go`

**Non-negotiable.**

## Canary Tokens

**Purpose:** Tier 2 evaluator integrity. A random token is generated during workspace initialization and embedded in the evaluator's prompt. The evaluator must include this token in its response. If it doesn't, the response is rejected — this catches prompt injection attacks that attempt to forge evaluator verdicts by echoing a plausible-looking response.

Also used by sandbox canary probes to verify that the kernel sandbox is actually enforcing restrictions (the probe attempts to read/write/spawn and verifies that the attempts are blocked).

**Code:** `crypto/canary.go`, `shield/tier2_*.go` (evaluator canary), `sandbox/canary_*.go` (sandbox probes)

**Non-negotiable.**

## Agent Auth Token

**Purpose:** Preventing unauthorized processes from impersonating the agent. Each time the agent process is spawned, the engine generates a random token and passes it via environment variable. The agent must present this token in its `AgentReady` message. If the token doesn't match, the engine rejects the connection.

This prevents a rogue process from connecting to the engine's gRPC port and submitting tool proposals that would be evaluated and executed as if they came from the legitimate agent.

**Code:** `crypto/` (token generation), `cmd/agent/internal_engine.go` (token lifecycle), `internal/engine/engine_pipeline.go` (token verification in `RunSession`)

**Non-negotiable.**

## Audit Hash Chain

**Purpose:** Tamper evidence. Each audit log entry includes a SHA-256 hash of the previous entry's content. Modifying or deleting any entry breaks the chain from that point forward. `openparallax audit --verify` walks the entire chain and reports the first break.

The chain serves incident investigation: if an attacker compromises the engine and tries to delete evidence of their actions from the audit log, the chain break is detectable even if the attacker successfully deletes the entries.

**Code:** `audit/` (logger, chain computation, verification)

**Non-negotiable.**
