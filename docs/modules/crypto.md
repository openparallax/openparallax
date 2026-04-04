# Cryptographic Primitives

The `crypto` package provides four cryptographic capabilities used throughout OpenParallax: ID generation, action hashing, hash chains, and canary tokens. All implementations use Go's standard library (`crypto/rand`, `crypto/sha256`, `crypto/subtle`) with zero external cryptographic dependencies.

Package: `github.com/openparallax/openparallax/crypto`

## 1. ID Generation

OpenParallax needs unique identifiers for sessions, messages, actions, audit entries, and snapshots. These IDs must be globally unique, safe for use in URLs and filenames, and generated without coordination between processes.

### NewID

```go
func NewID() string
```

Generates a UUID v4 string using `github.com/google/uuid`, which reads from `crypto/rand`. UUID v4 provides 122 bits of randomness, making collisions effectively impossible.

```go
id := crypto.NewID()
// "f47ac10b-58cc-4372-a567-0e02b2c3d479"
```

Used for:
- Session IDs
- Message IDs
- Action request IDs
- Audit entry IDs
- Chronicle snapshot IDs
- Client IDs (web, CLI)

### RandomHex

```go
func RandomHex(n int) (string, error)
```

Generates `n` random bytes from `crypto/rand` and returns them as a hex-encoded string (2n characters long). Used for tokens and secrets where UUID format is not needed.

```go
hex, err := crypto.RandomHex(16)
// "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6" (32 hex chars from 16 bytes)

hex, err = crypto.RandomHex(32)
// "a1b2c3d4..." (64 hex chars from 32 bytes -- same length as a canary token)
```

Used for:
- Canary token generation (via `GenerateCanary`)
- Session tokens
- Encryption nonces

## 2. Action Hashing

Action hashing provides TOCTOU (time-of-check-to-time-of-use) defense. When the LLM proposes a tool call, the Engine computes a hash of the action. This hash is verified again immediately before execution. If the action was modified between these two points -- by a race condition, a bug, or an attack -- the hash will not match and execution is blocked.

### HashAction

```go
func HashAction(actionType string, payload map[string]any) (string, error)
```

Computes the SHA-256 hash of a canonicalized action. The action is represented as a JSON object with `type` and `payload` fields:

```go
hash, err := crypto.HashAction("write_file", map[string]any{
    "path":    "/workspace/main.go",
    "content": "package main\nfunc main() {}",
})
// hash: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
```

Internally, `HashAction` works by:

1. Constructing a map: `{"type": actionType, "payload": payload}`
2. Canonicalizing the map (sorting keys at every nesting level)
3. Computing SHA-256 of the canonical JSON bytes

The hash covers both the action type and the full payload. Changing the action type, any argument, or adding/removing fields produces a different hash.

### Canonicalize

```go
func Canonicalize(v any) ([]byte, error)
```

Produces a deterministic JSON representation of any value. All map keys are sorted alphabetically at every nesting level. This is critical because Go's `json.Marshal` does not guarantee key ordering for maps, meaning the same logical content could produce different JSON bytes on different runs, different machines, or different Go versions.

```go
canonical, err := crypto.Canonicalize(map[string]any{
    "z_key": "last",
    "a_key": "first",
    "nested": map[string]any{
        "b": 2,
        "a": 1,
    },
})
// Result: {"a_key":"first","nested":{"a":1,"b":2},"z_key":"last"}
```

The implementation:

1. Marshals the input to JSON via `json.Marshal` (normalizing Go types)
2. Unmarshals back to `any` (converting to `map[string]any` and `[]any`)
3. Recursively serializes via `marshalSorted`:
   - `map[string]any`: Sort keys alphabetically, recursively serialize values
   - `[]any`: Serialize elements in order
   - Everything else: Use `json.Marshal` directly

### SHA256Hex

```go
func SHA256Hex(data []byte) string
```

Computes the SHA-256 hash of the input bytes and returns it as a lowercase hex string (64 characters).

```go
hash := crypto.SHA256Hex([]byte("hello world"))
// "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
```

Used by action hashing, audit hash chains, Chronicle snapshot chains, and file integrity verification.

### Verification Flow

In the pipeline, action hashing prevents TOCTOU attacks:

```
1. Agent proposes tool call with arguments
         │
2. Engine computes hash: crypto.HashAction(toolName, args)
   Engine stores hash on ActionRequest
         │
3. Shield evaluation (Tier 0 → 1 → 2)
         │
4. Before execution: Verifier.Verify(action)
   - Recomputes hash from action.Type and action.Payload
   - Compares with stored action.Hash
   - Returns ErrHashMismatch if different → execution blocked
         │
5. Execute the action
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

Note: any error in hash computation is treated as a mismatch (fail-closed).

## 3. Hash Chain

The audit log and Chronicle snapshots use hash chains for tamper detection. Each entry includes the SHA-256 hash of the previous entry, creating a linked chain. If any entry in the chain is modified, deleted, or reordered, all subsequent hashes break.

### How It Works

```
Entry 1 (genesis)       Entry 2                 Entry 3
┌──────────────────┐    ┌──────────────────┐    ┌──────────────────┐
│ PreviousHash: "" │    │ PreviousHash: H1 │    │ PreviousHash: H2 │
│ Hash: H1         │◄───│ Hash: H2         │◄───│ Hash: H3         │
│ (all other fields)│    │ (all other fields)│    │ (all other fields)│
└──────────────────┘    └──────────────────┘    └──────────────────┘
```

The genesis entry (first entry) has an empty string for `PreviousHash`.

### Entry Hash Computation

When writing an entry:

1. Set `PreviousHash` to the `Hash` of the last written entry (empty string for the first entry)
2. Set `Hash` to the empty string
3. Canonicalize the entry using `crypto.Canonicalize()` (sorted keys at all levels)
4. Compute SHA-256 of the canonical form using `crypto.SHA256Hex()`
5. Set `Hash` to the computed value
6. Write the entry (JSONL line for audit, SQLite row for Chronicle)
7. Store the hash as `lastHash` for the next entry

### Chain Verification

Verification reads every entry in order and checks two properties:

1. **Chain link**: Each entry's `PreviousHash` matches the `Hash` of the preceding entry
2. **Entry integrity**: Each entry's `Hash` matches the recomputed hash of its contents

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

### Audit Entry Structure

```go
type AuditEntry struct {
    ID           string
    EventType    types.AuditEventType
    Timestamp    int64
    SessionID    string
    ActionType   string
    DetailsJSON  string
    PreviousHash string   // Hash of the previous entry
    Hash         string   // Hash of this entry
    OTR          bool
}
```

### Recovery on Startup

The audit logger reads the last line of the JSONL file on startup to recover the chain hash:

```go
lastHash := readLastHash(path)
```

This allows the chain to continue after a restart without re-reading the entire file.

### CLI and Web Verification

Both the CLI and web UI can verify the hash chain:

```bash
openparallax audit --verify
```

The web UI audit panel shows chain integrity status. If verification fails, the exact entry where the chain breaks is reported.

## 4. Canary Tokens

Canary tokens detect prompt injection attacks against the Shield Tier 2 LLM evaluator. A random token is generated during workspace initialization and embedded in the evaluator prompt. When the LLM evaluator responds, the response must contain the canary token. If it does not, the evaluation is considered compromised and the action is automatically blocked.

### GenerateCanary

```go
func GenerateCanary() (string, error)
```

Creates a cryptographically random 64-character hex token (32 bytes of randomness):

```go
canary, err := crypto.GenerateCanary()
// "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6"
```

The token is stored at `<workspace>/canary.token` and is in the `FullBlock` protection list -- the agent cannot read or write it. This prevents a compromised agent from learning the canary token.

### VerifyCanary

```go
func VerifyCanary(response string, expected string) bool
```

Checks if the expected canary token appears anywhere in the LLM evaluator's response. Uses constant-time comparison at each candidate position to prevent timing side-channel attacks.

```go
ok := crypto.VerifyCanary(evaluatorResponse, canaryToken)
if !ok {
    // Canary missing -- evaluator may have been prompt-injected.
    // Automatic BLOCK with confidence 1.0.
}
```

The implementation:

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

1. Rejects tokens that are not exactly 64 characters
2. Scans the response byte-by-byte using a sliding window of 64 bytes
3. At each position, uses `subtle.ConstantTimeCompare` for comparison
4. Returns `true` if a match is found at any position

Constant-time comparison prevents an attacker from using response timing to guess the canary token one character at a time.

### Attack Model

The canary defends against this specific attack:

1. An adversary crafts malicious content (e.g., a file, a web page, a prompt injection in retrieved context)
2. This content is included in the Shield Tier 2 evaluation prompt (as part of the action being evaluated)
3. The malicious content contains an injection that causes the LLM evaluator to return ALLOW for a harmful action
4. Because the malicious content does not know the canary token (it is randomly generated per workspace and stored in a protected file), the injected response cannot include the canary
5. The canary verification detects the missing token and returns BLOCK with confidence 1.0

Properties that make this defense effective:

- The token is randomly generated per workspace (not predictable)
- The token is not in any LLM training data (generated after training cutoff, unique per workspace)
- The token is stored in a protected file the agent cannot access
- Missing canary = automatic BLOCK, no exceptions

### Canary Tokens vs. Sandbox Canary Probes

These are different mechanisms with similar names:

| | Canary Token | Canary Probe |
|---|---|---|
| **Location** | `crypto/canary.go` | `internal/sandbox/canary.go` |
| **Purpose** | Detect prompt injection in Tier 2 evaluator responses | Verify kernel sandbox is actually enforcing restrictions |
| **How it works** | Random string embedded in evaluator prompt, verified in response | Platform-specific tests (file access, network, spawn) that should fail if sandbox is working |
| **Used by** | Shield Tier 2 | Sandbox subsystem, `openparallax doctor` |

## Encryption

The crypto package also provides AES-256-GCM encryption for protecting sensitive data at rest, such as OAuth tokens for channel adapters.

### DeriveKey

```go
func DeriveKey(canaryHex, info string) ([]byte, error)
```

Derives a 256-bit AES key from a canary token hex string using HKDF-SHA256. The `info` parameter provides domain separation, ensuring different uses produce different keys from the same input material.

```go
key, err := crypto.DeriveKey(canaryToken, "openparallax-oauth-encryption")
```

Requires at least 16 bytes (32 hex characters) of input material.

### Encrypt / Decrypt

```go
func Encrypt(key, plaintext []byte) ([]byte, error)
func Decrypt(key, ciphertextWithNonce []byte) ([]byte, error)
```

AES-256-GCM authenticated encryption. `Encrypt` returns `nonce || ciphertext` (the 12-byte nonce is prepended). `Decrypt` expects this format and returns the original plaintext, or `ErrDecryptionFailed` if the key is wrong or the ciphertext has been tampered with.

```go
key, _ := crypto.DeriveKey(canary, "my-domain")
ciphertext, err := crypto.Encrypt(key, []byte("secret data"))
plaintext, err := crypto.Decrypt(key, ciphertext)
```

## Usage Summary

| Primitive | Used By | Purpose |
|---|---|---|
| `NewID()` | Session, message, audit, snapshot creation | Globally unique identifiers |
| `SHA256Hex()` | Audit log, Chronicle, file integrity | Deterministic hashing |
| `Canonicalize()` | Audit entries, snapshot metadata | Deterministic JSON for reproducible hashes |
| `HashAction()` | Engine pipeline (proposal + verification) | TOCTOU prevention |
| `GenerateCanary()` | Workspace initialization | Evaluator prompt injection detection |
| `VerifyCanary()` | Shield Tier 2 | Verify evaluator response integrity |
| `DeriveKey()` | Channel adapters | Key derivation for OAuth token encryption |
| `Encrypt()`/`Decrypt()` | Channel adapters | Protecting OAuth tokens at rest |
| `RandomHex()` | Various | General-purpose random hex strings |

## Key Source Files

| File | Purpose |
|---|---|
| `crypto/random.go` | NewID (UUID v4), RandomHex |
| `crypto/hash.go` | SHA256Hex, Canonicalize, HashAction, marshalSorted |
| `crypto/canary.go` | GenerateCanary, VerifyCanary |
| `crypto/encrypt.go` | DeriveKey, Encrypt, Decrypt |
| `audit/logger.go` | Audit logger with hash chain |
| `audit/integrity.go` | VerifyIntegrity for chain verification |
| `internal/engine/verifier.go` | Verifier for TOCTOU hash checks |
