// Package chronicle provides copy-on-write state versioning for workspace files.
// Before each destructive action, affected files are backed up to a snapshot
// directory. Snapshots form an integrity hash chain and support rollback.
package chronicle

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/openparallax/openparallax/internal/crypto"
	"github.com/openparallax/openparallax/internal/storage"
	"github.com/openparallax/openparallax/internal/types"
)

// Chronicle manages state versioning with COW snapshots.
type Chronicle struct {
	workspace     string
	snapshotDir   string
	db            *storage.DB
	retentionMax  int
	retentionDays int
}

// New creates a Chronicle instance for the given workspace.
func New(workspace string, cfg types.ChronicleConfig, db *storage.DB) (*Chronicle, error) {
	snapshotDir := filepath.Join(workspace, ".openparallax", "chronicle", "snapshots")
	if err := os.MkdirAll(snapshotDir, 0o755); err != nil {
		return nil, fmt.Errorf("create snapshot directory: %w", err)
	}

	return &Chronicle{
		workspace:     workspace,
		snapshotDir:   snapshotDir,
		db:            db,
		retentionMax:  cfg.MaxSnapshots,
		retentionDays: cfg.MaxAgeDays,
	}, nil
}

// Snapshot creates a pre-execution backup of files affected by the action.
// Returns nil metadata if no files need backing up (e.g., read actions).
func (c *Chronicle) Snapshot(action *types.ActionRequest) (*types.SnapshotMetadata, error) {
	files := c.affectedFiles(action)
	if len(files) == 0 {
		return nil, nil
	}

	snap := &types.SnapshotMetadata{
		ID:            crypto.NewID(),
		Timestamp:     time.Now(),
		ActionType:    string(action.Type),
		ActionSummary: fmt.Sprintf("%s: %v", action.Type, action.Payload["path"]),
	}

	snapDir := filepath.Join(c.snapshotDir, snap.ID)
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		return nil, fmt.Errorf("create snapshot directory: %w", err)
	}

	for _, path := range files {
		if err := copyFile(path, filepath.Join(snapDir, filepath.Base(path))); err != nil {
			continue
		}
		snap.FilesBackedUp = append(snap.FilesBackedUp, path)
	}

	snap.PreviousHash = c.db.GetLastSnapshotHash()
	canonical, _ := crypto.Canonicalize(snap)
	snap.Hash = crypto.SHA256Hex(canonical)

	if err := c.db.InsertSnapshot(snap); err != nil {
		return nil, fmt.Errorf("store snapshot metadata: %w", err)
	}

	c.db.PruneSnapshots(c.retentionMax, c.retentionDays)

	return snap, nil
}

// Rollback restores files from a specific snapshot to their pre-action state.
func (c *Chronicle) Rollback(snapshotID string) error {
	snap, err := c.db.GetSnapshot(snapshotID)
	if err != nil {
		return types.ErrSnapshotNotFound
	}

	snapDir := filepath.Join(c.snapshotDir, snap.ID)
	for _, path := range snap.FilesBackedUp {
		backupPath := filepath.Join(snapDir, filepath.Base(path))
		if err := copyFile(backupPath, path); err != nil {
			return fmt.Errorf("rollback %s: %w", path, err)
		}
	}

	return nil
}

// Diff computes changes between a snapshot and the current file state.
func (c *Chronicle) Diff(snapshotID string) (*types.Diff, error) {
	snap, err := c.db.GetSnapshot(snapshotID)
	if err != nil {
		return nil, types.ErrSnapshotNotFound
	}

	diff := &types.Diff{
		FromSnapshot: snapshotID,
		Timestamp:    time.Now(),
	}

	snapDir := filepath.Join(c.snapshotDir, snap.ID)
	for _, path := range snap.FilesBackedUp {
		backupPath := filepath.Join(snapDir, filepath.Base(path))
		backupHash := hashFile(backupPath)
		currentHash := hashFile(path)

		if backupHash != currentHash {
			changeType := "modified"
			if currentHash == "" {
				changeType = "deleted"
			}
			diff.Changes = append(diff.Changes, types.FileChange{
				Path:       path,
				ChangeType: changeType,
				BeforeHash: backupHash,
				AfterHash:  currentHash,
			})
		}
	}

	return diff, nil
}

// VerifyIntegrity checks the hash chain of all snapshots.
func (c *Chronicle) VerifyIntegrity() error {
	snapshots := c.db.GetAllSnapshots()
	prevHash := ""
	for _, snap := range snapshots {
		if snap.PreviousHash != prevHash {
			return fmt.Errorf("%w: snapshot %s has previous_hash %q but expected %q",
				types.ErrIntegrityViolation, snap.ID, snap.PreviousHash, prevHash)
		}
		prevHash = snap.Hash
	}
	return nil
}

// List returns all snapshots ordered by timestamp.
func (c *Chronicle) List() []types.SnapshotMetadata {
	return c.db.GetAllSnapshots()
}

// Close is a no-op — Chronicle has no resources to release beyond the shared DB.
func (c *Chronicle) Close() error {
	return nil
}

// affectedFiles returns the list of files that will be modified by this action.
func (c *Chronicle) affectedFiles(action *types.ActionRequest) []string {
	switch action.Type {
	case types.ActionWriteFile, types.ActionDeleteFile:
		if path, ok := action.Payload["path"].(string); ok && path != "" {
			resolved := c.resolvePath(path)
			if fileExists(resolved) {
				return []string{resolved}
			}
		}
	case types.ActionMoveFile:
		if src, ok := action.Payload["source"].(string); ok && src != "" {
			resolved := c.resolvePath(src)
			if fileExists(resolved) {
				return []string{resolved}
			}
		}
	}
	return nil
}

func (c *Chronicle) resolvePath(path string) string {
	if !filepath.IsAbs(path) {
		return filepath.Join(c.workspace, path)
	}
	return filepath.Clean(path)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

func hashFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return crypto.SHA256Hex(data)
}
