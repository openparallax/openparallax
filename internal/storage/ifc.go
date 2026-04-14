package storage

import (
	"os"
	"path/filepath"
)

// IFCActivityEntry represents a classified write destination in the activity table.
type IFCActivityEntry struct {
	Path        string `json:"path"`
	Sensitivity int    `json:"sensitivity"`
	SourcePath  string `json:"source_path"`
	CreatedAt   string `json:"created_at"`
	SessionID   string `json:"session_id"`
}

// IFCSweepEntry represents an entry removed by the sweep command.
type IFCSweepEntry struct {
	Path        string `json:"path"`
	Sensitivity int    `json:"sensitivity"`
	CreatedAt   string `json:"created_at"`
}

// RecordIFCWrite stores or upgrades the classification for a written path.
// Only the highest sensitivity wins — a path classified as Critical stays
// Critical even if later written with Confidential data. This is the
// mechanism that enables cross-session IFC enforcement: once classified
// data is written to a path, that path carries the classification until
// the file is deleted and the activity table is swept.
func (db *DB) RecordIFCWrite(path string, sensitivity int, sourcePath, sessionID string) {
	_, _ = db.conn.Exec(`INSERT INTO ifc_activity (path, sensitivity, source_path, session_id)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			sensitivity = MAX(sensitivity, excluded.sensitivity),
			source_path = CASE WHEN excluded.sensitivity > sensitivity THEN excluded.source_path ELSE source_path END,
			session_id = excluded.session_id`,
		path, sensitivity, sourcePath, sessionID,
	)
}

// LookupIFCClassification returns the sensitivity level for a path from
// the activity table, or -1 if the path is not tracked. Called during
// IFC classification to supplement the policy's source rules with
// persistent taint from previous writes.
func (db *DB) LookupIFCClassification(path string) int {
	var sensitivity int
	err := db.conn.QueryRow(
		`SELECT sensitivity FROM ifc_activity WHERE path = ?`, path,
	).Scan(&sensitivity)
	if err != nil {
		return -1
	}
	return sensitivity
}

// SweepIFCActivity removes entries where the file no longer exists on disk.
// Returns the list of removed entries for display. The basePath is the
// workspace root — relative paths in the table are resolved against it.
func (db *DB) SweepIFCActivity(basePath string) ([]IFCSweepEntry, error) {
	rows, err := db.conn.Query(`SELECT path, sensitivity, created_at FROM ifc_activity`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var stale []IFCSweepEntry
	for rows.Next() {
		var e IFCSweepEntry
		if rows.Scan(&e.Path, &e.Sensitivity, &e.CreatedAt) != nil {
			continue
		}
		absPath := e.Path
		if !filepath.IsAbs(absPath) {
			absPath = filepath.Join(basePath, absPath)
		}
		if _, statErr := os.Stat(absPath); os.IsNotExist(statErr) {
			stale = append(stale, e)
		}
	}

	for _, e := range stale {
		_, _ = db.conn.Exec(`DELETE FROM ifc_activity WHERE path = ?`, e.Path)
	}

	return stale, nil
}

// ListIFCActivity returns all tracked paths with their classifications,
// ordered by sensitivity (highest first).
func (db *DB) ListIFCActivity() ([]IFCActivityEntry, error) {
	rows, err := db.conn.Query(
		`SELECT path, sensitivity, source_path, created_at, COALESCE(session_id, '')
		 FROM ifc_activity ORDER BY sensitivity DESC, created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var entries []IFCActivityEntry
	for rows.Next() {
		var e IFCActivityEntry
		if rows.Scan(&e.Path, &e.Sensitivity, &e.SourcePath, &e.CreatedAt, &e.SessionID) == nil {
			entries = append(entries, e)
		}
	}
	return entries, nil
}
