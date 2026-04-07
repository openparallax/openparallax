# Audit Go API

The Go audit package is the canonical implementation. All other language wrappers call into the compiled Go library.

Package: `github.com/openparallax/openparallax/audit`

## NewLogger

```go
func NewLogger(path string) (*Logger, error)
```

Creates an audit logger that appends to the JSONL file at `path`. If the file already exists, the logger reads the last entry to recover the chain hash. If the file does not exist, it is created. The parent directory is created if needed.

The returned `Logger` is safe for concurrent use from multiple goroutines.

```go
logger, err := audit.NewLogger("/workspace/.openparallax/audit.jsonl")
if err != nil {
    return fmt.Errorf("create audit logger: %w", err)
}
defer logger.Close()
```

## Entry

```go
type Entry struct {
    EventType  audit.AuditEventType
    ActionType string
    SessionID  string
    Details    string
    OTR        bool
    Source     string
}
```

`Entry` is the input type for logging. It contains the event metadata without the hash chain fields (those are computed automatically).

| Field | Description |
|---|---|
| `EventType` | The category of audit event (see Event Types below) |
| `ActionType` | The action type string, e.g. `"write_file"`, `"run_command"` |
| `SessionID` | The session this event belongs to |
| `Details` | A JSON string with event-specific metadata |
| `OTR` | Whether this event occurred in an Off-The-Record session |
| `Source` | Where the event originated, e.g. `"pipeline"`, `"shield"` |

## Log

```go
func (l *Logger) Log(entry Entry) error
```

Appends an entry to the audit log. The method:

1. Generates a unique ID for the entry
2. Sets the timestamp to the current time
3. Sets `previous_hash` to the hash of the last entry
4. Canonicalizes the entry (deterministic JSON with sorted keys)
5. Computes the SHA-256 hash
6. Marshals the entry to JSON and appends it to the file
7. Optionally indexes the entry in SQLite

Thread-safe. Concurrent calls are serialized through a mutex.

```go
err := logger.Log(audit.Entry{
    EventType:  audit.AuditActionProposed,
    ActionType: "run_command",
    SessionID:  "sess-123",
    Details:    `{"command":"ls -la","working_dir":"/workspace"}`,
    Source:     "pipeline",
})
```

## SetDB

```go
func (l *Logger) SetDB(db DBIndexer)
```

Attaches a SQLite database for secondary indexing. When set, every `Log` call also inserts the entry into the database for fast queries. The JSONL file remains the primary record.

```go
logger.SetDB(storageDB)
```

The `DBIndexer` interface requires a single method:

```go
type DBIndexer interface {
    InsertAuditEntry(entry *audit.AuditEntry) error
}
```

## Close

```go
func (l *Logger) Close() error
```

Flushes and closes the underlying file. After `Close`, no further `Log` calls should be made.

## VerifyIntegrity

```go
func VerifyIntegrity(path string) error
```

Reads the entire audit log and verifies the hash chain. Returns `nil` if the chain is valid, or an error describing the first violation found.

Verification checks two things for every entry:

1. **Chain continuity**: The entry's `previous_hash` matches the hash of the preceding entry
2. **Entry integrity**: The entry's `hash` matches the recomputed SHA-256 of its canonical form

```go
if err := audit.VerifyIntegrity("/workspace/.openparallax/audit.jsonl"); err != nil {
    log.Printf("Audit log tampered: %s", err)
    // Take remedial action
}
```

### Error Messages

- `line N: invalid JSON: ...` -- the entry on line N is not valid JSON
- `line N: chain broken: previous_hash "X" does not match expected "Y"` -- the chain link is broken
- `line N: hash mismatch: stored "X", computed "Y"` -- the entry content was modified

An empty or nonexistent file passes verification (no entries to verify).

## ReadEntries

```go
func ReadEntries(path string, q Query) ([]audit.AuditEntry, error)
```

Reads audit entries from the JSONL file, applying optional filters. Returns entries in reverse chronological order (most recent first).

```go
type Query struct {
    SessionID string
    EventType audit.AuditEventType
    Limit     int
}
```

| Field | Description |
|---|---|
| `SessionID` | Filter to entries matching this session ID. Empty means all sessions. |
| `EventType` | Filter to entries matching this event type. Zero means all types. |
| `Limit` | Maximum number of entries to return. Zero means no limit. |

```go
// Get the 20 most recent entries across all sessions.
entries, err := audit.ReadEntries(path, audit.Query{Limit: 20})

// Get all blocked actions in a specific session.
blocked, err := audit.ReadEntries(path, audit.Query{
    SessionID: "sess-123",
    EventType: audit.AuditActionBlocked,
})

// Get the 5 most recent Shield errors.
errors, err := audit.ReadEntries(path, audit.Query{
    EventType: audit.AuditShieldError,
    Limit:     5,
})
```

## Hash Chain Internals

### Entry Lifecycle

When `Log` is called, the following happens internally:

```go
// 1. Build the AuditEntry with chain fields
auditEntry := audit.AuditEntry{
    ID:           crypto.NewID(),        // UUID v4
    EventType:    entry.EventType,
    Timestamp:    time.Now().UnixMilli(),
    SessionID:    entry.SessionID,
    ActionType:   entry.ActionType,
    DetailsJSON:  entry.Details,
    PreviousHash: l.lastHash,            // chain link
    OTR:          entry.OTR,
    Source:       entry.Source,
}

// 2. Canonicalize (sorted keys at all levels)
canonical, _ := crypto.Canonicalize(auditEntry)

// 3. Hash
auditEntry.Hash = crypto.SHA256Hex(canonical)

// 4. Serialize and append
data, _ := json.Marshal(auditEntry)
fmt.Fprintf(l.file, "%s\n", data)

// 5. Update chain state
l.lastHash = auditEntry.Hash
```

### Hash Computation

The hash is computed over the canonical JSON of the entry with the `hash` field set to its zero value (empty string). This means the hash covers all other fields including `previous_hash`, making the chain tamper-evident.

The canonicalization function (`crypto.Canonicalize`) produces deterministic JSON by sorting all map keys alphabetically at every nesting level. This is critical because Go's `json.Marshal` does not guarantee key ordering.

### Chain Recovery on Restart

When `NewLogger` is called on an existing file, it reads the last line, parses the `hash` field, and uses it as `lastHash`. New entries chain from there seamlessly.

```go
func readLastHash(path string) string {
    data, _ := os.ReadFile(path)
    lines := strings.Split(strings.TrimSpace(string(data)), "\n")
    if len(lines) == 0 {
        return ""
    }
    var entry audit.AuditEntry
    json.Unmarshal([]byte(lines[len(lines)-1]), &entry)
    return entry.Hash
}
```

## AuditEntry Type

The full `AuditEntry` struct (defined in `types` package):

```go
type AuditEntry struct {
    ID           string         `json:"id"`
    EventType    AuditEventType `json:"event_type"`
    Timestamp    int64          `json:"timestamp"`
    SessionID    string         `json:"session_id,omitempty"`
    ActionType   string         `json:"action_type,omitempty"`
    DetailsJSON  string         `json:"details_json,omitempty"`
    PreviousHash string         `json:"previous_hash"`
    Hash         string         `json:"hash"`
    OTR          bool           `json:"otr"`
    Source       string         `json:"source,omitempty"`
}
```

## Event Types

```go
const (
    AuditActionProposed      AuditEventType = 1
    AuditActionEvaluated     AuditEventType = 2
    AuditActionApproved      AuditEventType = 3
    AuditActionBlocked       AuditEventType = 4
    AuditActionExecuted      AuditEventType = 5
    AuditActionFailed        AuditEventType = 6
    AuditShieldError         AuditEventType = 7
    AuditCanaryVerified      AuditEventType = 8
    AuditCanaryMissing       AuditEventType = 9
    AuditRateLimitHit        AuditEventType = 10
    AuditBudgetExhausted     AuditEventType = 11
    AuditSelfProtection      AuditEventType = 12
    AuditTransactionBegin    AuditEventType = 13
    AuditTransactionCommit   AuditEventType = 14
    AuditTransactionRollback AuditEventType = 15
    AuditIntegrityViolation  AuditEventType = 16
    AuditSessionStarted          AuditEventType = 17
    AuditSessionEnded            AuditEventType = 18
    AuditConfigChanged           AuditEventType = 19
    AuditIFCClassified           AuditEventType = 20
    AuditChronicleSnapshot       AuditEventType = 21
    AuditChronicleSnapshotFailed AuditEventType = 22
    AuditSandboxCanaryResult     AuditEventType = 23
)
```
