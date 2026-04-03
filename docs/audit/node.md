# Audit Node.js API

The Node.js wrapper provides native JavaScript/TypeScript access to the audit module. It calls into the compiled Go library via N-API, ensuring identical hash computation and chain behavior as the Go implementation.

## Installation

```bash
npm install @openparallax/audit
```

Requires Node.js 18+. The package includes pre-built binaries for Linux (x86_64, arm64), macOS (x86_64, arm64), and Windows (x86_64).

## AuditLogger

```typescript
import { AuditLogger } from '@openparallax/audit';

const logger = new AuditLogger(path: string);
```

Creates or opens an audit log at the given path. If the file already exists, the logger recovers the chain hash from the last entry.

### log

```typescript
await logger.log(entry: LogEntry): Promise<void>
```

Appends an entry to the audit log with automatic hash chain computation.

```typescript
interface LogEntry {
  eventType: EventType;
  actionType?: string;
  sessionId?: string;
  details?: Record<string, unknown> | string;
  otr?: boolean;
  source?: string;
}
```

```typescript
import { AuditLogger, EventType } from '@openparallax/audit';

const logger = new AuditLogger('./audit.jsonl');

await logger.log({
  eventType: EventType.ACTION_PROPOSED,
  actionType: 'run_command',
  sessionId: 'sess-001',
  details: { command: 'ls -la', working_dir: '/workspace' },
});

await logger.log({
  eventType: EventType.ACTION_EXECUTED,
  actionType: 'run_command',
  sessionId: 'sess-001',
  details: { command: 'ls -la', exit_code: 0 },
});

await logger.close();
```

### close

```typescript
await logger.close(): Promise<void>
```

Flushes and closes the audit log file. The logger should not be used after calling `close`.

## EventType

```typescript
import { EventType } from '@openparallax/audit';

EventType.ACTION_PROPOSED       // 1
EventType.ACTION_EVALUATED      // 2
EventType.ACTION_APPROVED       // 3
EventType.ACTION_BLOCKED        // 4
EventType.ACTION_EXECUTED       // 5
EventType.ACTION_FAILED         // 6
EventType.SHIELD_ERROR          // 7
EventType.CANARY_VERIFIED       // 8
EventType.CANARY_MISSING        // 9
EventType.RATE_LIMIT_HIT        // 10
EventType.BUDGET_EXHAUSTED      // 11
EventType.SELF_PROTECTION       // 12
EventType.SESSION_STARTED       // 17
EventType.SESSION_ENDED         // 18
```

## verifyIntegrity

```typescript
import { verifyIntegrity } from '@openparallax/audit';

const result = await verifyIntegrity(path: string): Promise<VerifyResult>
```

Reads the entire audit log and checks the hash chain.

```typescript
interface VerifyResult {
  valid: boolean;
  entries: number;     // Total entries checked
  line: number;        // Line of first violation (0 if valid)
  error: string;       // Description of violation (empty if valid)
}
```

```typescript
const result = await verifyIntegrity('./audit.jsonl');
if (!result.valid) {
  console.error(`Tampered at line ${result.line}: ${result.error}`);
}
```

## readEntries

```typescript
import { readEntries } from '@openparallax/audit';

const entries = await readEntries(path: string, query?: Query): Promise<AuditEntry[]>
```

Reads audit entries from the JSONL file in reverse chronological order (most recent first).

### Query

```typescript
interface Query {
  sessionId?: string;       // Filter by session
  eventType?: EventType;    // Filter by event type
  limit?: number;           // Maximum entries to return
}
```

### AuditEntry

```typescript
interface AuditEntry {
  id: string;
  eventType: EventType;
  timestamp: number;        // Unix milliseconds
  sessionId: string;
  actionType: string;
  details: Record<string, unknown>;  // Parsed from details_json
  previousHash: string;
  hash: string;
  otr: boolean;
  source: string;
}
```

### Examples

```typescript
// Most recent 10 entries.
const entries = await readEntries('./audit.jsonl', { limit: 10 });

// All blocked actions in a session.
const blocked = await readEntries('./audit.jsonl', {
  sessionId: 'sess-001',
  eventType: EventType.ACTION_BLOCKED,
});

// Iterate over results.
for (const entry of entries) {
  console.log(`${entry.actionType}: ${JSON.stringify(entry.details)}`);
}
```

## Error Handling

All functions throw `AuditError` on failure:

```typescript
import { AuditError } from '@openparallax/audit';

try {
  const logger = new AuditLogger('/nonexistent/path/audit.jsonl');
} catch (err) {
  if (err instanceof AuditError) {
    console.error(`Audit error: ${err.message}`);
  }
}
```

## TypeScript Support

The package ships with full TypeScript declarations. All types are exported from the main entry point:

```typescript
import type { AuditEntry, LogEntry, Query, VerifyResult } from '@openparallax/audit';
```
