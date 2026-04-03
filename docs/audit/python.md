# Audit Python API

The Python wrapper provides native Python access to the audit module. It calls into the compiled Go library via a shared object, ensuring identical hash computation and chain behavior as the Go implementation.

## Installation

```bash
pip install openparallax-audit
```

Requires Python 3.9+. The package includes pre-built binaries for Linux (x86_64, arm64), macOS (x86_64, arm64), and Windows (x86_64).

## AuditLogger

```python
from openparallax_audit import AuditLogger

logger = AuditLogger(path: str)
```

Creates or opens an audit log at the given path. If the file already exists, the logger recovers the chain hash from the last entry.

### log

```python
logger.log(
    event_type: EventType,
    action_type: str = "",
    session_id: str = "",
    details: dict | str = None,
    otr: bool = False,
    source: str = "",
)
```

Appends an entry to the audit log with automatic hash chain computation.

| Parameter | Type | Description |
|---|---|---|
| `event_type` | `EventType` | The category of audit event |
| `action_type` | `str` | The action type, e.g. `"write_file"` |
| `session_id` | `str` | The session this event belongs to |
| `details` | `dict` or `str` | Event-specific metadata. If a dict, it is serialized to JSON. |
| `otr` | `bool` | Whether this is an Off-The-Record event |
| `source` | `str` | Origin identifier, e.g. `"pipeline"` |

```python
from openparallax_audit import AuditLogger, EventType

logger = AuditLogger("./audit.jsonl")

logger.log(
    event_type=EventType.ACTION_PROPOSED,
    action_type="run_command",
    session_id="sess-001",
    details={"command": "ls -la", "working_dir": "/workspace"},
)

logger.log(
    event_type=EventType.ACTION_BLOCKED,
    action_type="run_command",
    session_id="sess-001",
    details={"command": "rm -rf /", "reason": "denied by policy"},
)

logger.close()
```

### close

```python
logger.close()
```

Flushes and closes the audit log file. The logger should not be used after calling `close`.

### Context Manager

```python
with AuditLogger("./audit.jsonl") as logger:
    logger.log(event_type=EventType.ACTION_EXECUTED, action_type="read_file")
# Automatically closed on exit.
```

## EventType

```python
from openparallax_audit import EventType

EventType.ACTION_PROPOSED       # 1
EventType.ACTION_EVALUATED      # 2
EventType.ACTION_APPROVED       # 3
EventType.ACTION_BLOCKED        # 4
EventType.ACTION_EXECUTED       # 5
EventType.ACTION_FAILED         # 6
EventType.SHIELD_ERROR          # 7
EventType.CANARY_VERIFIED       # 8
EventType.CANARY_MISSING        # 9
EventType.RATE_LIMIT_HIT        # 10
EventType.BUDGET_EXHAUSTED      # 11
EventType.SELF_PROTECTION       # 12
EventType.SESSION_STARTED       # 17
EventType.SESSION_ENDED         # 18
```

## verify_integrity

```python
from openparallax_audit import verify_integrity

result = verify_integrity(path: str) -> VerifyResult
```

Reads the entire audit log and checks the hash chain. Returns a `VerifyResult` with:

| Field | Type | Description |
|---|---|---|
| `valid` | `bool` | Whether the chain is intact |
| `entries` | `int` | Total entries checked |
| `line` | `int` | Line number of first violation (0 if valid) |
| `error` | `str` | Description of the violation (empty if valid) |

```python
result = verify_integrity("./audit.jsonl")
if not result.valid:
    print(f"Tampered at line {result.line}: {result.error}")
```

## read_entries

```python
from openparallax_audit import read_entries, Query

entries = read_entries(path: str, query: Query = None) -> list[AuditEntry]
```

Reads audit entries from the JSONL file in reverse chronological order (most recent first).

### Query

```python
from openparallax_audit import Query

Query(
    session_id: str = "",       # Filter by session
    event_type: EventType = 0,  # Filter by event type
    limit: int = 0,             # Maximum entries to return
)
```

### AuditEntry

```python
@dataclass
class AuditEntry:
    id: str
    event_type: EventType
    timestamp: int          # Unix milliseconds
    session_id: str
    action_type: str
    details: dict           # Parsed from details_json
    previous_hash: str
    hash: str
    otr: bool
    source: str
```

### Examples

```python
# Most recent 10 entries.
entries = read_entries("./audit.jsonl", Query(limit=10))

# All blocked actions in a session.
blocked = read_entries("./audit.jsonl", Query(
    session_id="sess-001",
    event_type=EventType.ACTION_BLOCKED,
))

# Iterate over results.
for entry in entries:
    print(f"{entry.action_type}: {entry.details}")
```

## Error Handling

All functions raise `AuditError` on failure:

```python
from openparallax_audit import AuditError

try:
    logger = AuditLogger("/nonexistent/path/audit.jsonl")
except AuditError as e:
    print(f"Failed to create logger: {e}")
```
