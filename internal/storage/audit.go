package storage

import "github.com/openparallax/openparallax/internal/types"

// InsertAuditEntry indexes an audit entry in SQLite for fast querying.
// The full audit entry (with hash chain) is stored in the JSONL file;
// this table provides indexed access for filtering and counting.
func (db *DB) InsertAuditEntry(entry *types.AuditEntry) error {
	otr := 0
	if entry.OTR {
		otr = 1
	}
	_, err := db.conn.Exec(
		`INSERT INTO audit_index (id, event_type, timestamp, session_id, action_type, otr)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		entry.ID, int(entry.EventType), entry.Timestamp,
		entry.SessionID, entry.ActionType, otr,
	)
	return err
}

// QueryAuditEntries returns audit index entries matching the given filters.
// Pass empty strings or zero values to skip a filter.
func (db *DB) QueryAuditEntries(sessionID string, eventType int, limit int) ([]types.AuditEntry, error) {
	if limit <= 0 {
		limit = 100
	}

	query := `SELECT id, event_type, timestamp, session_id, action_type, otr FROM audit_index WHERE 1=1`
	var args []any

	if sessionID != "" {
		query += ` AND session_id = ?`
		args = append(args, sessionID)
	}
	if eventType > 0 {
		query += ` AND event_type = ?`
		args = append(args, eventType)
	}

	query += ` ORDER BY timestamp DESC LIMIT ?`
	args = append(args, limit)

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var entries []types.AuditEntry
	for rows.Next() {
		var e types.AuditEntry
		var otr int
		if err := rows.Scan(&e.ID, &e.EventType, &e.Timestamp, &e.SessionID, &e.ActionType, &otr); err != nil {
			continue
		}
		e.OTR = otr != 0
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// CountAuditToday returns the number of audit entries recorded today.
func (db *DB) CountAuditToday() (int, error) {
	var count int
	// Audit timestamps are Unix milliseconds. Compute today's start in ms.
	err := db.conn.QueryRow(
		`SELECT COUNT(*) FROM audit_index WHERE timestamp >= strftime('%s', 'now', 'start of day') * 1000`,
	).Scan(&count)
	return count, err
}

// AuditEntryCount returns the total number of audit entries.
func (db *DB) AuditEntryCount() (int, error) {
	var count int
	err := db.conn.QueryRow(`SELECT COUNT(*) FROM audit_index`).Scan(&count)
	return count, err
}
