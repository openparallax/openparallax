# Audit Verification

The hash chain makes every modification to the audit log detectable. This page explains how the chain works, what verification catches, and how to use it for forensic analysis.

## How the Chain Works

### Genesis Entry

The first entry in the audit log is the genesis entry. Its `previous_hash` is an empty string because there is no preceding entry:

```json
{
  "id": "a1b2c3d4-...",
  "event_type": 17,
  "timestamp": 1711929600000,
  "previous_hash": "",
  "hash": "e3b0c44298fc1c149afbf4c8996fb924..."
}
```

### Subsequent Entries

Every entry after the genesis includes the hash of the entry immediately before it:

```json
{
  "id": "e5f6g7h8-...",
  "event_type": 1,
  "timestamp": 1711929600123,
  "previous_hash": "e3b0c44298fc1c149afbf4c8996fb924...",
  "hash": "7d793037a076849b8379bfa9f37ccb8f..."
}
```

### Hash Computation

For each entry, the hash is computed as follows:

1. Set the `hash` field to its zero value (empty string)
2. Canonicalize the entry: serialize to JSON with all keys sorted alphabetically at every nesting level
3. Compute SHA-256 of the canonical bytes
4. Store the hex-encoded hash in the `hash` field

The `previous_hash` field is included in the hash computation. This creates a dependency chain where changing any entry invalidates the hash of every subsequent entry.

```
Entry N:
  canonical = json_sorted({
    "id": "...",
    "event_type": 5,
    "timestamp": 1711929600456,
    "previous_hash": hash(Entry N-1),
    "hash": "",               <-- zeroed for computation
    ...
  })
  hash = sha256(canonical)
```

## What Verification Detects

### Modified Entry

If an attacker changes any field in any entry (e.g., changing `event_type` from `ACTION_BLOCKED` to `ACTION_APPROVED`), the recomputed hash will not match the stored hash.

```
Verification output:
  line 42: hash mismatch: stored "abc123..." computed "def456..."
```

### Deleted Entry

If an entry is deleted from the middle of the log, the next entry's `previous_hash` will not match the hash of the entry now preceding it.

```
Verification output:
  line 43: chain broken: previous_hash "abc123..." does not match expected "xyz789..."
```

### Inserted Entry

If an entry is inserted into the middle of the log, the chain will break at the insertion point because the subsequent entry's `previous_hash` will point to the wrong predecessor.

### Truncated Log

If entries are removed from the end of the log, verification will pass for the remaining entries. The hash chain cannot detect tail truncation on its own. To detect this, compare the entry count or the last known hash against an external reference.

### Reordered Entries

If entries are reordered, the `previous_hash` links will not match.

## CLI Verification

```bash
openparallax audit --verify
```

This reads the entire audit log, recomputes every hash, and checks every chain link. Output:

```
Verifying audit log... 1,247 entries checked.
Audit log integrity verified.
```

Or on failure:

```
Verifying audit log...
INTEGRITY VIOLATION at line 892:
  chain broken: previous_hash "a1b2c3..." does not match expected "d4e5f6..."
```

### Additional CLI Options

```bash
# Verify and show summary statistics.
openparallax audit --verify --session sess-001

# Query specific event types.
openparallax audit --type blocked --session sess-001

# Show recent entries.
openparallax audit --lines 20
```

## Programmatic Verification

### Go

```go
import "github.com/openparallax/openparallax/internal/audit"

err := audit.VerifyIntegrity("/workspace/.openparallax/audit.jsonl")
if err != nil {
    // err contains the line number and nature of the violation
    log.Printf("Audit tampering detected: %s", err)
}
```

The function reads the file line by line, parsing each entry and checking:

1. `entry.PreviousHash == prevHash` (chain link)
2. `sha256(canonical(entry with hash="")) == entry.Hash` (entry integrity)

It returns `nil` for a valid chain, or a descriptive error for the first violation found.

### Python

```python
from openparallax_audit import verify_integrity

result = verify_integrity("./audit.jsonl")
if not result.valid:
    print(f"Line {result.line}: {result.error}")
```

### Node.js

```typescript
import { verifyIntegrity } from '@openparallax/audit';

const result = await verifyIntegrity('./audit.jsonl');
if (!result.valid) {
    console.error(`Line ${result.line}: ${result.error}`);
}
```

## Forensic Analysis

### Identifying the Scope of Tampering

When verification fails, the error tells you where the chain breaks. But the actual tampering may have started earlier. To identify the scope:

1. **Find the break point**: The verification error gives you the first line where the chain is inconsistent.
2. **Check entries before the break**: Entries before the break point have a valid chain relative to each other, but an attacker could have modified the entire prefix and recomputed all hashes from the genesis.
3. **Cross-reference external records**: Compare entry IDs and timestamps against external systems (SQLite index, monitoring logs, Shield verdicts) to identify which entries were modified.

### Detecting Full-Chain Rewrite

A sophisticated attacker could modify entries and recompute the entire chain from scratch. The resulting log would pass hash chain verification. To defend against this:

- **External anchoring**: Periodically write the latest hash to an external system (a separate database, a monitoring service, a blockchain). Compare the anchored hash against the log's hash at that point.
- **SQLite cross-reference**: The audit module optionally indexes entries in SQLite. Cross-reference the two stores to detect divergence.
- **Entry count tracking**: Track the expected number of entries externally. A rewritten log with different entry counts is detectable.

### Timeline Reconstruction

Use the audit log to reconstruct what the agent did during a specific time window:

```go
entries, _ := audit.ReadEntries(path, audit.Query{
    SessionID: "sess-001",
})

for _, e := range entries {
    ts := time.UnixMilli(e.Timestamp).Format(time.RFC3339)
    fmt.Printf("%s [%d] %s: %s\n", ts, e.EventType, e.ActionType, e.DetailsJSON)
}
```

### Shell-Based Analysis

The JSONL format makes the audit log accessible to standard Unix tools:

```bash
# Timeline of all actions in a session
grep '"session_id":"sess-001"' audit.jsonl | jq -r '[.timestamp, .event_type, .action_type] | @tsv'

# All unique action types that were blocked
grep '"event_type":4' audit.jsonl | jq -r '.action_type' | sort -u

# Hash chain spot check: verify entry N links to entry N-1
sed -n '100p' audit.jsonl | jq '.previous_hash'
sed -n '99p' audit.jsonl | jq '.hash'
# These two values should match.
```
