package storage

import (
	"encoding/json"
	"time"

	"github.com/openparallax/openparallax/internal/types"
)

// InsertSnapshot stores a Chronicle snapshot's metadata.
func (db *DB) InsertSnapshot(snap *types.SnapshotMetadata) error {
	filesJSON, _ := json.Marshal(snap.FilesBackedUp)
	_, err := db.conn.Exec(
		`INSERT INTO snapshots (id, transaction_id, timestamp, action_type, action_summary, files_json, hash, previous_hash)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		snap.ID, snap.TransactionID, snap.Timestamp.Format(time.RFC3339),
		snap.ActionType, snap.ActionSummary, string(filesJSON),
		snap.Hash, snap.PreviousHash,
	)
	return err
}

// GetSnapshot retrieves a snapshot by ID.
func (db *DB) GetSnapshot(id string) (*types.SnapshotMetadata, error) {
	row := db.conn.QueryRow(
		`SELECT id, transaction_id, timestamp, action_type, action_summary, files_json, hash, previous_hash
		 FROM snapshots WHERE id = ?`, id,
	)

	var snap types.SnapshotMetadata
	var tsStr, filesJSON string
	if err := row.Scan(
		&snap.ID, &snap.TransactionID, &tsStr,
		&snap.ActionType, &snap.ActionSummary, &filesJSON,
		&snap.Hash, &snap.PreviousHash,
	); err != nil {
		return nil, types.ErrSnapshotNotFound
	}

	snap.Timestamp, _ = time.Parse(time.RFC3339, tsStr)
	_ = json.Unmarshal([]byte(filesJSON), &snap.FilesBackedUp)
	return &snap, nil
}

// GetLastSnapshotHash returns the hash of the most recent snapshot.
// Returns an empty string if no snapshots exist (genesis state).
func (db *DB) GetLastSnapshotHash() string {
	var hash string
	err := db.conn.QueryRow(`SELECT hash FROM snapshots ORDER BY rowid DESC LIMIT 1`).Scan(&hash)
	if err != nil {
		return ""
	}
	return hash
}

// GetAllSnapshots returns all snapshots ordered by timestamp.
func (db *DB) GetAllSnapshots() []types.SnapshotMetadata {
	rows, err := db.conn.Query(
		`SELECT id, transaction_id, timestamp, action_type, action_summary, files_json, hash, previous_hash
		 FROM snapshots ORDER BY timestamp ASC`,
	)
	if err != nil {
		return nil
	}
	defer func() { _ = rows.Close() }()

	var snapshots []types.SnapshotMetadata
	for rows.Next() {
		var snap types.SnapshotMetadata
		var tsStr, filesJSON string
		if err := rows.Scan(
			&snap.ID, &snap.TransactionID, &tsStr,
			&snap.ActionType, &snap.ActionSummary, &filesJSON,
			&snap.Hash, &snap.PreviousHash,
		); err != nil {
			continue
		}
		snap.Timestamp, _ = time.Parse(time.RFC3339, tsStr)
		_ = json.Unmarshal([]byte(filesJSON), &snap.FilesBackedUp)
		snapshots = append(snapshots, snap)
	}
	return snapshots
}

// GetTransactionSnapshots returns all snapshots belonging to a transaction.
func (db *DB) GetTransactionSnapshots(txID string) []types.SnapshotMetadata {
	rows, err := db.conn.Query(
		`SELECT id, transaction_id, timestamp, action_type, action_summary, files_json, hash, previous_hash
		 FROM snapshots WHERE transaction_id = ? ORDER BY timestamp ASC`, txID,
	)
	if err != nil {
		return nil
	}
	defer func() { _ = rows.Close() }()

	var snapshots []types.SnapshotMetadata
	for rows.Next() {
		var snap types.SnapshotMetadata
		var tsStr, filesJSON string
		if err := rows.Scan(
			&snap.ID, &snap.TransactionID, &tsStr,
			&snap.ActionType, &snap.ActionSummary, &filesJSON,
			&snap.Hash, &snap.PreviousHash,
		); err != nil {
			continue
		}
		snap.Timestamp, _ = time.Parse(time.RFC3339, tsStr)
		_ = json.Unmarshal([]byte(filesJSON), &snap.FilesBackedUp)
		snapshots = append(snapshots, snap)
	}
	return snapshots
}

// SnapshotCount returns the total number of snapshots.
func (db *DB) SnapshotCount() (int, error) {
	var count int
	err := db.conn.QueryRow(`SELECT COUNT(*) FROM snapshots`).Scan(&count)
	return count, err
}

// PruneSnapshots removes old snapshots that exceed count or age limits.
// Snapshots that belong to active transactions are preserved.
func (db *DB) PruneSnapshots(maxCount int, maxAgeDays int) {
	// Prune by count: keep the most recent maxCount snapshots.
	_, _ = db.conn.Exec(
		`DELETE FROM snapshots WHERE id NOT IN (
			SELECT id FROM snapshots ORDER BY timestamp DESC LIMIT ?
		) AND (transaction_id IS NULL OR transaction_id NOT IN (
			SELECT id FROM transactions WHERE status = 'active'
		))`, maxCount,
	)

	// Prune by age: remove snapshots older than maxAgeDays.
	cutoff := time.Now().AddDate(0, 0, -maxAgeDays).Format(time.RFC3339)
	_, _ = db.conn.Exec(
		`DELETE FROM snapshots WHERE timestamp < ? AND (transaction_id IS NULL OR transaction_id NOT IN (
			SELECT id FROM transactions WHERE status = 'active'
		))`, cutoff,
	)
}

// InsertTransaction stores a new transaction record.
func (db *DB) InsertTransaction(tx *types.Transaction) error {
	_, err := db.conn.Exec(
		`INSERT INTO transactions (id, status, started_at) VALUES (?, ?, ?)`,
		tx.ID, tx.Status, tx.StartedAt.Format(time.RFC3339),
	)
	return err
}

// UpdateTransactionStatus updates a transaction's status and finished_at timestamp.
func (db *DB) UpdateTransactionStatus(txID string, status string) error {
	now := time.Now().Format(time.RFC3339)
	_, err := db.conn.Exec(
		`UPDATE transactions SET status = ?, finished_at = ? WHERE id = ?`,
		status, now, txID,
	)
	return err
}
