# State Protection & Forensics

These mechanisms protect workspace state from irreversible corruption and provide the forensic trail needed for incident investigation.

## Chronicle (Copy-on-Write Snapshots)

**Threat:** A destructive action (file overwrite, deletion, directory restructuring) that cannot be undone after execution.

**Defense:** Before any state-mutating action, Chronicle creates a copy-on-write snapshot of affected files. Snapshots are stored in the workspace's `.openparallax/chronicle/` directory with configurable retention (`chronicle.max_snapshots`, `chronicle.max_age_days`). Users can roll back to any snapshot via `/chronicle rollback <id>` or the REST API.

A researcher analyzing experimental data can let the agent reorganize files knowing that any destructive change can be reversed. A legal team reviewing contracts can roll back if the agent's edits introduce errors.

**Relevant threats:**
- OWASP A08: Software and Data Integrity Failures

**Code:** `internal/chronicle/`, `chronicle/`

**Non-negotiable** (snapshots always run before mutations). **Tunable** (retention limits).

## Audit Chain

**Threat:** Security events going unrecorded, or recorded events being tampered with to hide malicious activity.

**Defense:** An append-only JSONL log with SHA-256 hash chain. Every security-relevant event (action proposed, evaluated, blocked, executed, failed, plus Shield errors, canary results, rate limit hits, IFC decisions, session lifecycle) is recorded. Each entry's hash includes the previous entry's hash, forming a tamper-evident chain. Verification via `openparallax audit --verify` walks the chain and reports breaks.

28 distinct event types cover the full pipeline lifecycle. The agent process cannot write to `audit.jsonl` — it lives under the `.openparallax/` directory, which is hard-blocked by the protection layer.

**Relevant threats:**
- OWASP A09: Security Logging and Monitoring Failures
- CWE-778: Insufficient Logging

**Code:** `audit/` (logger, types, verification), `internal/engine/` (event emission)

**Non-negotiable.**

## Memory Subsystem Isolation

**Threat:** The agent process writing to its own memory files directly, bypassing the engine's audit and IFC controls.

**Defense:** Memory writes flow through the engine, not the agent. The agent process can read memory files (MEMORY.md, USER.md) for context assembly — it needs this for intelligent responses. But all writes go through a `MemoryFlush` gRPC event from the agent to the engine, where they're evaluated by Shield and IFC before being persisted. The agent's kernel sandbox prevents direct filesystem writes.

**Relevant threats:**
- OWASP A01: Broken Access Control

**Code:** `memory/`, `internal/agent/loop.go` (memory flusher callback)

**Non-negotiable.**

## OTR Mode

**Threat:** Sensitive conversations being persisted to disk — session history, memory extractions, tool execution artifacts.

**Defense:** OTR (Off-The-Record) sessions filter tools at definition time (no filesystem writes, no memory persistence). Session data lives in `sync.Map` (never hits SQLite). Messages, tool results, and extracted memories from OTR sessions are never written to any persistent store. When the session ends, the data is gone.

A journalist using the agent to analyze leaked documents can work in OTR mode knowing that no trace of the conversation persists after the session.

**Relevant threats:**
- OWASP LLM02: Sensitive Information Disclosure
- CWE-312: Cleartext Storage of Sensitive Information

**Code:** `internal/session/otr.go`

**Configurable.** Users enable OTR mode per-session via `/otr` or the API.
