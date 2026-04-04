# Cryptographic Primitives

The `crypto` package provides cryptographic utilities for ID generation, action hashing, audit chain integrity, canary token management, and encryption. All implementations use Go's `crypto/rand` for randomness and `crypto/sha256` for hashing. Zero external cryptographic dependencies beyond the standard library and `golang.org/x/crypto/bcrypt` (used in web auth, not in this package).

## ID Generation

```go
func NewID() string
```

Generates a UUID v4 string using `github.com/google/uuid`. Used throughout the system for message IDs, session IDs, action request IDs, audit entry IDs, and client IDs.

Example output: `f47ac10b-58cc-4372-a567-0e02b2c3d479`

```go
func RandomHex(n int) (string, error)
```

Generates `n` random bytes and returns them as a hex string. Used for session tokens, canary tokens, and other secrets where UUID format is not needed.

## Action Hashing

Action hashing provides TOCTOU (time-of-check-to-time-of-use) defense. The hash is computed when the action is proposed and verified before execution. If the action was modified between these points, the hash will not match and execution is blocked.

### HashAction

```go
func HashAction(actionType string, payload map[string]any) (string, error)
```

Computes the SHA-256 hash of a canonicalized action. The action is represented as:

```json
{
  "type": "write_file",
  "payload": {"content": "...", "path": "/home/user/file.txt"}
}
```

This is canonicalized (keys sorted at every nesting level) and hashed:

```go
func HashAction(actionType string, payload map[string]any) (string, error) {
    obj := map[string]any{
        "type":    actionType,
        "payload": payload,
    }
    canonical, err := Canonicalize(obj)
    if err != nil {
        return "", err
    }
    return SHA256Hex(canonical), nil
}
```

### Canonicalize

```go
func Canonicalize(v any) ([]byte, error)
```

Produces a deterministic JSON representation of any value. Keys are sorted alphabetically at every nesting level. This ensures the same logical content always produces the same hash regardless of key ordering in the original JSON.

The implementation:
1. Marshals the input to JSON (using `json.Marshal`).
2. Unmarshals back to `any` (normalizing types).
3. Recursively serializes with sorted keys via `marshalSorted`.

`marshalSorted` handles three cases:
- `map[string]any`: Sort keys, recursively serialize values.
- `[]any`: Serialize elements in order.
- Everything else: Use `json.Marshal` directly.

### Verification Flow

In the pipeline:

```
1. Agent proposes tool call with arguments
2. Engine computes hash: crypto.HashAction(toolName, args)
3. Engine stores hash on ActionRequest
4. ... Shield evaluation, OTR check, etc. ...
5. Before execution: Verifier.Verify(action)
   - Recomputes hash from action.Type and action.Payload
   - Compares with stored action.Hash
   - Returns ErrHashMismatch if different
```

The `Verifier` in `internal/engine/verifier.go`:

```go
func (v *Verifier) Verify(action *types.ActionRequest) error {
    computed, err := crypto.HashAction(string(action.Type), action.Payload)
    if err != nil {
        return types.ErrHashMismatch
    }
    if computed != action.Hash {
        return types.ErrHashMismatch
    }
    return nil
}
```

## SHA-256 Utilities

```go
func SHA256Hex(data []byte) string
```

Computes SHA-256 and returns the hex-encoded result. Used by both action hashing and audit chain integrity.

## Audit Hash Chain

Each audit entry includes the hash of the previous entry, forming a chain. Any modification to a past entry changes its hash, which breaks the chain at the next entry.

### Entry Structure

```go
type AuditEntry struct {
    ID           string
    EventType    types.AuditEventType
    Timestamp    int64
    SessionID    string
    ActionType   string
    DetailsJSON  string
    PreviousHash string   // Hash of the previous entry
    Hash         string   // Hash of this entry (computed from all other fields)
    OTR          bool
}
```

### Hash Computation

When writing an entry:

1. Set `PreviousHash` to the `Hash` of the last written entry (empty string for the first entry).
2. Set `Hash` to empty string.
3. Canonicalize the entry (sorted keys at all levels).
4. Compute SHA-256 of the canonical form.
5. Set `Hash` to the computed value.
6. Write the entry as a JSONL line.
7. Store the hash as `lastHash` for the next entry.

### Chain Verification

`audit.VerifyIntegrity(path)` reads the entire audit log and verifies:

1. Each entry's `PreviousHash` matches the `Hash` of the preceding entry.
2. Each entry's `Hash` matches the recomputed hash of its contents.

```go
for i, line := range lines {
    var entry types.AuditEntry
    json.Unmarshal([]byte(line), &entry)

    // Verify chain link
    if entry.PreviousHash != prevHash {
        return fmt.Errorf("chain broken at line %d", i+1)
    }

    // Verify entry integrity
    storedHash := entry.Hash
    entry.Hash = ""
    canonical, _ := crypto.Canonicalize(entry)
    computed := crypto.SHA256Hex(canonical)
    if computed != storedHash {
        return fmt.Errorf("hash mismatch at line %d", i+1)
    }

    prevHash = storedHash
}
```

The `openparallax audit --verify` CLI command and the web UI audit viewer both use this function.

### Recovery on Startup

The audit logger reads the last line of the JSONL file on startup to recover the chain hash:

```go
lastHash := readLastHash(path)
```

This allows the chain to continue after a restart without re-reading the entire file.

## Canary Tokens

Canary tokens detect prompt injection in Shield Tier 2 evaluations. A random token is generated during workspace initialization and embedded in the evaluator prompt. When the LLM evaluator responds, the response must contain the canary token. If it does not, the evaluation is considered compromised.

### Generation

```go
func GenerateCanary() (string, error)
```

Creates a cryptographically random 64-character hex token (32 bytes of randomness):

```go
b := make([]byte, 32)
rand.Read(b)
return hex.EncodeToString(b), nil
```

The token is stored at `<workspace>/canary.token` and is in the `FullBlock` protection list -- the agent cannot read or write it.

### Verification

```go
func VerifyCanary(response string, expected string) bool
```

Checks if the expected canary token appears anywhere in the LLM evaluator's response. Uses constant-time comparison at each candidate position to prevent timing side-channel attacks:

```go
func VerifyCanary(response string, expected string) bool {
    if len(expected) != 64 || len(response) < 64 {
        return false
    }
    expectedBytes := []byte(expected)
    responseBytes := []byte(response)

    for i := 0; i <= len(responseBytes)-64; i++ {
        if subtle.ConstantTimeCompare(responseBytes[i:i+64], expectedBytes) == 1 {
            return true
        }
    }
    return false
}
```

### Attack Model

The canary defends against a specific attack: an adversary crafts content (e.g., a malicious file) that, when included in the Shield Tier 2 evaluation prompt, causes the LLM evaluator to return ALLOW for a harmful action. The canary token is not in any training data and is randomly generated per workspace, making it impossible for an adversary to predict and include in their injection payload. If the evaluator's response does not include the canary, the response was likely generated by injected content rather than the legitimate evaluator prompt.

### Canary Token vs. Sandbox Canary Probes

These are different mechanisms with similar names:

- **Canary token** (`crypto/canary.go`): A random string embedded in the Shield evaluator prompt and verified in responses. Detects prompt injection.
- **Canary probes** (`internal/sandbox/canary.go`): Platform-specific tests that verify the kernel sandbox is actually enforced. Detects sandbox failures.

## Key Source Files

| File | Purpose |
|---|---|
| `crypto/random.go` | NewID (UUID v4), RandomHex |
| `crypto/hash.go` | SHA256Hex, Canonicalize, HashAction |
| `crypto/canary.go` | GenerateCanary, VerifyCanary |
| `crypto/encrypt.go` | Encryption utilities |
| `audit/logger.go` | Audit logger with hash chain |
| `audit/integrity.go` | VerifyIntegrity for chain verification |
| `internal/engine/verifier.go` | Verifier for TOCTOU hash checks |
