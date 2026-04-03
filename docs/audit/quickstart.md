# Audit Quick Start

Get tamper-evident logging running in under five minutes.

## Go

### Install

```bash
go get github.com/openparallax/openparallax
```

### Create a Log, Append Entries, Verify

```go
package main

import (
    "fmt"
    "log"

    "github.com/openparallax/openparallax/internal/audit"
    "github.com/openparallax/openparallax/internal/types"
)

func main() {
    // Create or open an audit log. If the file already exists,
    // the logger recovers the last hash to continue the chain.
    logger, err := audit.NewLogger("./audit.jsonl")
    if err != nil {
        log.Fatal(err)
    }
    defer logger.Close()

    // Log a proposed action.
    err = logger.Log(audit.Entry{
        EventType:  types.AuditActionProposed,
        ActionType: "write_file",
        SessionID:  "session-001",
        Details:    `{"path":"/workspace/main.go","content":"package main"}`,
    })
    if err != nil {
        log.Fatal(err)
    }

    // Log the execution result.
    err = logger.Log(audit.Entry{
        EventType:  types.AuditActionExecuted,
        ActionType: "write_file",
        SessionID:  "session-001",
        Details:    `{"path":"/workspace/main.go","bytes_written":27}`,
    })
    if err != nil {
        log.Fatal(err)
    }

    // Log a blocked action.
    err = logger.Log(audit.Entry{
        EventType:  types.AuditActionBlocked,
        ActionType: "run_command",
        SessionID:  "session-001",
        Details:    `{"command":"rm -rf /","reason":"denied by policy"}`,
    })
    if err != nil {
        log.Fatal(err)
    }

    // Close the logger to flush.
    logger.Close()

    // Verify the hash chain.
    if err := audit.VerifyIntegrity("./audit.jsonl"); err != nil {
        fmt.Printf("INTEGRITY VIOLATION: %s\n", err)
    } else {
        fmt.Println("Audit log integrity verified.")
    }
}
```

### Query Entries

```go
// Read the 10 most recent entries for a specific session.
entries, err := audit.ReadEntries("./audit.jsonl", audit.Query{
    SessionID: "session-001",
    Limit:     10,
})
if err != nil {
    log.Fatal(err)
}

for _, e := range entries {
    fmt.Printf("[%d] %s: %s\n", e.EventType, e.ActionType, e.DetailsJSON)
}
```

### Filter by Event Type

```go
// Read only blocked actions.
blocked, err := audit.ReadEntries("./audit.jsonl", audit.Query{
    EventType: types.AuditActionBlocked,
    Limit:     50,
})
```

## Python

### Install

```bash
pip install openparallax-audit
```

### Basic Usage

```python
from openparallax_audit import AuditLogger, EventType

# Create or open an audit log.
logger = AuditLogger("./audit.jsonl")

# Log events.
logger.log(
    event_type=EventType.ACTION_PROPOSED,
    action_type="write_file",
    session_id="session-001",
    details={"path": "/workspace/main.go"},
)

logger.log(
    event_type=EventType.ACTION_EXECUTED,
    action_type="write_file",
    session_id="session-001",
    details={"path": "/workspace/main.go", "bytes_written": 27},
)

logger.close()

# Verify the chain.
from openparallax_audit import verify_integrity

result = verify_integrity("./audit.jsonl")
if result.valid:
    print("Audit log integrity verified.")
else:
    print(f"INTEGRITY VIOLATION at line {result.line}: {result.error}")
```

### Query Entries

```python
from openparallax_audit import read_entries, Query

entries = read_entries("./audit.jsonl", Query(
    session_id="session-001",
    limit=10,
))

for entry in entries:
    print(f"[{entry.event_type}] {entry.action_type}: {entry.details}")
```

## Node.js

### Install

```bash
npm install @openparallax/audit
```

### Basic Usage

```typescript
import { AuditLogger, EventType, verifyIntegrity } from '@openparallax/audit';

// Create or open an audit log.
const logger = new AuditLogger('./audit.jsonl');

// Log events.
await logger.log({
  eventType: EventType.ACTION_PROPOSED,
  actionType: 'write_file',
  sessionId: 'session-001',
  details: { path: '/workspace/main.go' },
});

await logger.log({
  eventType: EventType.ACTION_EXECUTED,
  actionType: 'write_file',
  sessionId: 'session-001',
  details: { path: '/workspace/main.go', bytesWritten: 27 },
});

await logger.close();

// Verify the chain.
const result = await verifyIntegrity('./audit.jsonl');
if (result.valid) {
  console.log('Audit log integrity verified.');
} else {
  console.error(`INTEGRITY VIOLATION at line ${result.line}: ${result.error}`);
}
```

### Query Entries

```typescript
import { readEntries } from '@openparallax/audit';

const entries = await readEntries('./audit.jsonl', {
  sessionId: 'session-001',
  limit: 10,
});

for (const entry of entries) {
  console.log(`[${entry.eventType}] ${entry.actionType}: ${JSON.stringify(entry.details)}`);
}
```
