# Chronicle

Chronicle provides copy-on-write workspace snapshots. Before every destructive action (file write, file delete, file move), Chronicle backs up the affected files to a snapshot directory. Snapshots form an integrity hash chain and support rollback to any previous state.

## Why Chronicle Exists

AI agents modify files. When an agent writes to a file, the previous content is gone. If the agent makes a mistake -- writes incorrect code, corrupts a config file, deletes something important -- there is no built-in way to recover.

Chronicle creates automatic backups before every write operation. Every snapshot records what files were backed up, what action triggered the backup, and includes a hash chain for tamper detection. You can roll back to any snapshot to restore files to their pre-action state.

This is not version control. Git tracks commits you explicitly create. Chronicle tracks every individual write operation the agent performs, whether or not those writes ever become git commits. It operates below the level of version control, capturing the state before each atomic modification.

## How It Works

### Snapshot Lifecycle

```
Agent proposes: write_file main.go
         │
         ▼
Engine calls Chronicle.Snapshot(action)
         │
         ▼
Chronicle checks: does main.go exist?
         │
    YES  │  NO (new file, nothing to back up)
         │  └──→ return nil (no snapshot)
         ▼
Copy main.go → .openparallax/chronicle/snapshots/<id>/main.go
         │
         ▼
Compute hash chain (previous hash + canonical metadata)
         │
         ▼
Store metadata in SQLite
         │
         ▼
Prune old snapshots if retention limits exceeded
         │
         ▼
Return SnapshotMetadata to engine
         │
         ▼
Engine executes the write_file action
```

1. The engine prepares an action (e.g., `write_file` to `/workspace/main.go`)
2. Chronicle checks if the action will modify existing files
3. If yes, it copies the current version of affected files to a snapshot directory
4. The snapshot metadata (ID, timestamp, action type, file list, hash chain) is stored in SQLite
5. The action executes
6. If something goes wrong, the user can roll back to the snapshot

### Copy-on-Write

Chronicle does not snapshot the entire workspace. It only copies files that are about to be modified by the current action. This keeps snapshot storage proportional to the number of changes, not the workspace size.

A workspace with 10,000 files where the agent modifies 5 files produces 5 file copies -- not 10,000. Over 100 actions modifying an average of 2 files each, Chronicle stores roughly 200 file copies.

### Hash Chain

Like the audit log, snapshots form a hash chain. Each snapshot includes the hash of the previous snapshot. This detects tampering with the snapshot history.

```
Snapshot 1          Snapshot 2          Snapshot 3
┌──────────────┐    ┌──────────────┐    ┌──────────────┐
│ PreviousHash │    │ PreviousHash │    │ PreviousHash │
│   = ""       │◄───│   = Hash(1)  │◄───│   = Hash(2)  │
│ Hash = H1    │    │ Hash = H2    │    │ Hash = H3    │
└──────────────┘    └──────────────┘    └──────────────┘
```

If any snapshot in the chain is modified or deleted, `VerifyIntegrity()` detects the break.

## Affected Files Detection

Chronicle determines which files to back up based on the action type:

| Action Type | Files Backed Up |
|---|---|
| `write_file` | The file at `payload.path` (if it exists) |
| `delete_file` | The file at `payload.path` (if it exists) |
| `move_file` | The source file at `payload.source` (if it exists) |
| All other actions | None (no snapshot created) |

New files being created for the first time do not trigger a snapshot because there is nothing to back up. Read operations (`read_file`, `list_directory`, `search_files`) never produce snapshots.

Relative paths are resolved against the workspace root. Absolute paths are used as-is after cleaning.

## How OpenParallax Uses Chronicle

The Engine calls `Chronicle.Snapshot()` in the pipeline, after Shield evaluation and hash verification but before executing the action:

```
Shield.Evaluate() → Verify hash → Chronicle.Snapshot() → IFC check → Execute
```

This placement means:

- The snapshot captures the file **before** the write, not after
- If Shield blocks the action, no snapshot is created (no wasted storage)
- If the write fails after the snapshot, the snapshot still exists for debugging
- In OTR mode, snapshots are skipped entirely (OTR sessions leave no trace)

When an action produces a bad result, the user can ask the agent to rollback:

```
User: "That last edit broke the tests. Roll it back."
Agent: [calls rollback with the snapshot ID]
```

The web UI displays snapshot history in the audit panel, and each snapshot can be inspected with `Diff()` to see what changed since the backup was taken.

## Configuration

Chronicle is configured in the `chronicle` section of `config.yaml`:

```yaml
chronicle:
  max_snapshots: 100    # Maximum number of snapshots to retain
  max_age_days: 30      # Maximum age in days for retained snapshots
```

### Configuration Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `max_snapshots` | int | `50` | Maximum number of snapshots to retain. When a new snapshot is created and the count exceeds this limit, the oldest snapshots are pruned. |
| `max_age_days` | int | `30` | Maximum age in days. Snapshots older than this are pruned regardless of the count limit. |

### Retention Policy

Snapshots are pruned after each new snapshot based on two thresholds. Both limits are enforced independently -- a snapshot is pruned if it exceeds **either** threshold:

- **`max_snapshots`**: Oldest snapshots beyond this count are deleted.
- **`max_age_days`**: Snapshots older than this are deleted.

Pruning removes both the SQLite metadata record and the filesystem directory containing the backed-up files.

## Storage Format

Snapshots are stored in the filesystem and indexed in SQLite:

```
<workspace>/.openparallax/chronicle/snapshots/
    a1b2c3d4-e5f6-7890-abcd-ef1234567890/
        main.go          # Backed-up copy of main.go before write
    b2c3d4e5-f6a7-8901-bcde-f12345678901/
        config.yaml      # Backed-up copy of config.yaml before delete
        main.go          # Backed-up copy of main.go before write
```

Snapshot directories are named by UUID. Each directory contains copies of the files that were about to be modified, using the base filename (not the full path). The full original paths are recorded in the SQLite metadata.

The SQLite `snapshots` table stores: ID, timestamp, action type, action summary, file list (JSON array of original paths), hash, and previous hash.

## Go API

Package: `github.com/openparallax/openparallax/internal/chronicle`

### New

```go
func New(workspace string, cfg types.ChronicleConfig, db *storage.DB) (*Chronicle, error)
```

Creates a Chronicle instance for the given workspace. The snapshot directory is created at `<workspace>/.openparallax/chronicle/snapshots/`. Returns an error if the directory cannot be created.

```go
chron, err := chronicle.New("/home/user/workspace", types.ChronicleConfig{
    MaxSnapshots: 100,
    MaxAgeDays:   30,
}, db)
if err != nil {
    log.Fatal(err)
}
```

### Snapshot

```go
func (c *Chronicle) Snapshot(action *types.ActionRequest) (*types.SnapshotMetadata, error)
```

Creates a pre-execution backup of files affected by the action. Returns `nil` metadata if no files need backing up (e.g., read-only actions or writes to files that do not yet exist).

The method:

1. Determines which files will be affected by the action
2. Creates a snapshot directory named by UUID
3. Copies each affected file to the snapshot directory
4. Computes the hash chain (previous hash + canonical metadata)
5. Stores the metadata in SQLite
6. Prunes old snapshots according to retention policy

Files that cannot be copied (e.g., permission denied) are silently skipped -- the snapshot records only the files that were successfully backed up.

```go
snap, err := chron.Snapshot(&types.ActionRequest{
    Type: types.ActionWriteFile,
    Payload: map[string]any{
        "path":    "main.go",
        "content": "package main\n...",
    },
})
if snap != nil {
    fmt.Printf("Snapshot %s: backed up %d files\n", snap.ID, len(snap.FilesBackedUp))
}
// snap is nil for read operations or new file creation
```

### Rollback

```go
func (c *Chronicle) Rollback(snapshotID string) error
```

Restores files from a specific snapshot to their pre-action state. Copies each backed-up file from the snapshot directory back to its original path, overwriting the current content.

```go
err := chron.Rollback("a1b2c3d4-e5f6-7890-abcd-ef1234567890")
// Files are restored to their state before the action that triggered this snapshot
```

Returns `types.ErrSnapshotNotFound` if the snapshot ID does not exist in the database.

::: warning
Rollback overwrites the current file contents. If the current state has been modified since the snapshot was taken, those modifications are lost. Consider creating a new snapshot before rolling back if the current state may be valuable.
:::

### Diff

```go
func (c *Chronicle) Diff(snapshotID string) (*types.Diff, error)
```

Computes the changes between a snapshot and the current file state. For each backed-up file, it compares the SHA-256 hash of the snapshot copy with the current file on disk. Returns a list of changes with their type.

```go
diff, err := chron.Diff(snapshotID)
for _, change := range diff.Changes {
    fmt.Printf("%s: %s (before: %s, after: %s)\n",
        change.ChangeType, change.Path, change.BeforeHash, change.AfterHash)
}
```

Change types:

| ChangeType | Meaning |
|------------|---------|
| `"modified"` | File exists but its content differs from the snapshot |
| `"deleted"` | File no longer exists on disk |

### VerifyIntegrity

```go
func (c *Chronicle) VerifyIntegrity() error
```

Checks the hash chain of all snapshots. Iterates through every snapshot in chronological order, verifying that each snapshot's `PreviousHash` matches the `Hash` of the preceding snapshot.

```go
if err := chron.VerifyIntegrity(); err != nil {
    log.Printf("Chronicle integrity violation: %s", err)
    // The snapshot chain has been tampered with
}
```

Returns `nil` if the chain is valid, or an error wrapping `types.ErrIntegrityViolation` if any link in the chain is broken.

### List

```go
func (c *Chronicle) List() []types.SnapshotMetadata
```

Returns all snapshots ordered by timestamp, oldest first.

```go
snapshots := chron.List()
for _, s := range snapshots {
    fmt.Printf("%s [%s] %s: %d files\n",
        s.ID, s.Timestamp.Format(time.RFC3339), s.ActionSummary, len(s.FilesBackedUp))
}
```

### Close

```go
func (c *Chronicle) Close() error
```

No-op. Chronicle has no resources to release beyond the shared database connection.

## Types

### SnapshotMetadata

```go
type SnapshotMetadata struct {
    ID            string    // Unique snapshot identifier (UUID v4)
    Timestamp     time.Time // When the snapshot was created
    ActionType    string    // The action that triggered this snapshot (e.g., "write_file")
    ActionSummary string    // Human-readable summary (e.g., "write_file: main.go")
    FilesBackedUp []string  // Original paths of files that were backed up
    PreviousHash  string    // Hash of the preceding snapshot in the chain
    Hash          string    // SHA-256 hash of this snapshot's canonical metadata
}
```

### Diff

```go
type Diff struct {
    FromSnapshot string       // Starting snapshot ID
    ToSnapshot   string       // Ending snapshot ID (empty for current state)
    Changes      []FileChange // List of file changes
    Timestamp    time.Time    // When the diff was computed
}
```

### FileChange

```go
type FileChange struct {
    Path       string // File path that changed
    ChangeType string // "created", "modified", or "deleted"
    BeforeHash string // SHA-256 hash before the change
    AfterHash  string // SHA-256 hash after the change (empty if deleted)
    SizeBytes  int64  // File size after the change
}
```

## Standalone Usage

Chronicle can be used independently of the full OpenParallax system. You need a `storage.DB` instance (SQLite) for metadata persistence:

```go
package main

import (
    "fmt"
    "log"

    "github.com/openparallax/openparallax/internal/chronicle"
    "github.com/openparallax/openparallax/internal/storage"
    "github.com/openparallax/openparallax/internal/types"
)

func main() {
    db, err := storage.Open("/tmp/myapp/state.db")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    chron, err := chronicle.New("/tmp/myapp/workspace", types.ChronicleConfig{
        MaxSnapshots: 50,
        MaxAgeDays:   7,
    }, db)
    if err != nil {
        log.Fatal(err)
    }

    // Take a snapshot before modifying a file.
    snap, err := chron.Snapshot(&types.ActionRequest{
        Type:    types.ActionWriteFile,
        Payload: map[string]any{"path": "config.yaml"},
    })
    if err != nil {
        log.Fatal(err)
    }
    if snap != nil {
        fmt.Printf("Created snapshot %s\n", snap.ID)
    }

    // ... perform the write ...

    // Check what changed since the snapshot.
    diff, err := chron.Diff(snap.ID)
    if err != nil {
        log.Fatal(err)
    }
    for _, c := range diff.Changes {
        fmt.Printf("  %s: %s\n", c.ChangeType, c.Path)
    }

    // Roll back if needed.
    if err := chron.Rollback(snap.ID); err != nil {
        log.Fatal(err)
    }
    fmt.Println("Rolled back successfully")
}
```

## Key Source Files

| File | Purpose |
|---|---|
| `internal/chronicle/chronicle.go` | Chronicle struct, Snapshot, Rollback, Diff, VerifyIntegrity, List |
| `internal/chronicle/chronicle_test.go` | Integration tests |
| `internal/types/chronicle.go` | SnapshotMetadata, FileChange, Diff type definitions |
| `internal/types/config.go` | ChronicleConfig struct |
