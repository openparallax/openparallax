# Audit

Audit is a tamper-evident, append-only logging system that records every security-relevant event in an AI agent's lifecycle. Every tool proposal, every Shield verdict, every execution result is written to a JSONL file with a SHA-256 hash chain that makes any modification detectable.

Audit runs inside OpenParallax as the compliance and forensics backbone, but it is also a standalone module. You can drop it into any AI agent, any tool-calling pipeline, any system that needs cryptographic proof of what happened and when.

## Why Audit Exists

AI agents execute actions autonomously. When an agent reads files, runs shell commands, sends emails, or modifies databases, there must be an immutable record of what it did, what it was asked to do, what was blocked, and why.

Standard application logging is not sufficient. Log files can be edited, rotated, or deleted. An attacker who compromises an agent can cover their tracks by modifying logs after the fact. Audit solves this with a hash chain: each entry includes the SHA-256 hash of the previous entry. Modifying any entry breaks the chain, and verification detects the tampering.

## How the Hash Chain Works

Every audit entry contains two critical fields:

- **`previous_hash`** -- the SHA-256 hash of the preceding entry
- **`hash`** -- the SHA-256 hash of this entry (computed over a canonical JSON representation with the `hash` field zeroed)

The first entry in the log has an empty `previous_hash` (the genesis entry). Every subsequent entry chains to the one before it.

```
Entry 0:  previous_hash=""         hash=sha256(canonical(entry0))
Entry 1:  previous_hash=entry0.hash  hash=sha256(canonical(entry1))
Entry 2:  previous_hash=entry1.hash  hash=sha256(canonical(entry2))
...
```

To verify the chain, you recompute every hash from scratch. If any entry was modified, its hash will not match. If any entry was deleted or inserted, the `previous_hash` link will be broken.

### Canonicalization

Before hashing, the entry is converted to a canonical JSON form with deterministically sorted keys at every nesting level. This ensures that the same logical content always produces the same hash regardless of how JSON serialization orders keys.

## What Gets Logged

Every audit entry has an `event_type` that identifies what happened:

| Event Type | Value | Description |
|---|---|---|
| `ACTION_PROPOSED` | 1 | Agent proposed a tool call |
| `ACTION_EVALUATED` | 2 | Shield evaluated the action |
| `ACTION_APPROVED` | 3 | Action was approved |
| `ACTION_BLOCKED` | 4 | Action was blocked by Shield |
| `ACTION_EXECUTED` | 5 | Action was executed successfully |
| `ACTION_FAILED` | 6 | Action execution failed |
| `SHIELD_ERROR` | 7 | Shield evaluation error |
| `CANARY_VERIFIED` | 8 | Canary token verified in evaluator response |
| `CANARY_MISSING` | 9 | Canary token missing from evaluator response |
| `RATE_LIMIT_HIT` | 10 | Rate limit triggered |
| `BUDGET_EXHAUSTED` | 11 | Daily evaluation budget exhausted |
| `SELF_PROTECTION` | 12 | Self-protection rule triggered |
| `TRANSACTION_BEGIN` | 13 | Transaction started |
| `TRANSACTION_COMMIT` | 14 | Transaction committed |
| `TRANSACTION_ROLLBACK` | 15 | Transaction rolled back |
| `INTEGRITY_VIOLATION` | 16 | Integrity chain violation detected |
| `SESSION_STARTED` | 17 | Session started |
| `SESSION_ENDED` | 18 | Session ended |

## Entry Structure

Each JSONL line is a JSON object with these fields:

```json
{
  "id": "a3f1c2e4-5b6d-7890-abcd-ef1234567890",
  "event_type": 5,
  "timestamp": 1711929600000,
  "session_id": "sess-abc123",
  "action_type": "write_file",
  "details_json": "{\"path\":\"/workspace/main.go\",\"size\":1024}",
  "previous_hash": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
  "hash": "abc123def456...",
  "otr": false,
  "source": "pipeline"
}
```

The `details_json` field contains event-specific metadata as a JSON string. The `otr` flag indicates whether the event occurred in an Off-The-Record session.

## JSONL Format

The audit log is a plain text file with one JSON object per line. No framing, no schema versioning, no binary encoding. You can read it with `cat`, search it with `grep`, parse it with `jq`, or load it into any data pipeline.

```bash
# View the last 5 entries
tail -5 .openparallax/audit.jsonl | jq .

# Count blocked actions
grep '"event_type":4' .openparallax/audit.jsonl | wc -l

# Extract all shell commands
grep '"action_type":"run_command"' .openparallax/audit.jsonl | jq -r '.details_json' | jq '.command'
```

## Verification

### CLI

```bash
openparallax audit --verify
```

This recomputes the entire hash chain and reports the first violation found (if any).

### Programmatic

```go
err := audit.VerifyIntegrity("/path/to/audit.jsonl")
if err != nil {
    // Chain is broken -- log has been tampered with
    fmt.Printf("Integrity violation: %s\n", err)
}
```

## Chain Recovery

When the audit logger starts, it reads the last line of the existing log file and extracts the hash. New entries chain from that hash. This means the logger can be stopped and restarted without breaking the chain.

If the log file does not exist or is empty, the first entry becomes the genesis entry with an empty `previous_hash`.

## Dual Storage

Audit entries are written to two locations:

1. **JSONL file** -- the append-only, hash-chained primary record
2. **SQLite index** (optional) -- a secondary index for fast queries by session, event type, or time range

The JSONL file is the source of truth. The SQLite index is a convenience layer that can be rebuilt from the JSONL file at any time.

## Thread Safety

The audit logger is safe for concurrent use. All writes are serialized through a mutex. The hash chain is maintained atomically -- no interleaving of entries from concurrent goroutines can break the chain.

## Standalone Value

The audit module has minimal dependencies. You can use it independently of the rest of OpenParallax:

- **Go**: Import `github.com/openparallax/openparallax/internal/audit` directly
- **Python**: `pip install openparallax-audit` (wrapper around the Go library via shared object)
- **Node.js**: `npm install @openparallax/audit` (wrapper around the Go library via N-API)

The Go implementation is the canonical source. Python and Node.js wrappers call into the same compiled Go code, ensuring identical hash computation and chain behavior across all languages.
