package storage

import (
	"database/sql"
	"fmt"

	"github.com/openparallax/openparallax/internal/types"
)

// InsertArtifact persists artifact metadata to the artifacts table.
func (db *DB) InsertArtifact(a *types.Artifact) error {
	_, err := db.conn.Exec(
		`INSERT OR REPLACE INTO artifacts (id, session_id, type, title, path, language, size_bytes, preview_type, storage_path)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.SessionID, a.Type, a.Title, a.Path, a.Language, a.SizeBytes, a.PreviewType, a.StoragePath,
	)
	if err != nil {
		return fmt.Errorf("insert artifact: %w", err)
	}
	return nil
}

// GetArtifact retrieves a single artifact by ID.
func (db *DB) GetArtifact(id string) (*types.Artifact, error) {
	row := db.conn.QueryRow(
		`SELECT id, session_id, type, title, COALESCE(path,''), COALESCE(language,''), size_bytes, COALESCE(preview_type,''), storage_path
		 FROM artifacts WHERE id = ?`, id,
	)

	var a types.Artifact
	err := row.Scan(&a.ID, &a.SessionID, &a.Type, &a.Title, &a.Path, &a.Language, &a.SizeBytes, &a.PreviewType, &a.StoragePath)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("artifact not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("get artifact: %w", err)
	}
	return &a, nil
}

// ListPersistedArtifacts returns all artifacts from the dedicated artifacts table.
// ListSessionArtifacts returns all artifacts for a session.
func (db *DB) ListSessionArtifacts(sessionID string) ([]types.Artifact, error) {
	rows, err := db.conn.Query(
		`SELECT id, session_id, type, title, COALESCE(path,''), COALESCE(language,''), size_bytes, COALESCE(preview_type,''), storage_path
		 FROM artifacts WHERE session_id = ? ORDER BY created_at ASC`, sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("list session artifacts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []types.Artifact
	for rows.Next() {
		var a types.Artifact
		if err := rows.Scan(&a.ID, &a.SessionID, &a.Type, &a.Title, &a.Path, &a.Language, &a.SizeBytes, &a.PreviewType, &a.StoragePath); err != nil {
			continue
		}
		result = append(result, a)
	}
	return result, rows.Err()
}

// ListPersistedArtifacts returns all persisted artifacts across all sessions.
func (db *DB) ListPersistedArtifacts() ([]types.Artifact, error) {
	rows, err := db.conn.Query(
		`SELECT id, session_id, type, title, COALESCE(path,''), COALESCE(language,''), size_bytes, COALESCE(preview_type,''), storage_path
		 FROM artifacts ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list persisted artifacts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []types.Artifact
	for rows.Next() {
		var a types.Artifact
		if err := rows.Scan(&a.ID, &a.SessionID, &a.Type, &a.Title, &a.Path, &a.Language, &a.SizeBytes, &a.PreviewType, &a.StoragePath); err != nil {
			continue
		}
		result = append(result, a)
	}
	return result, rows.Err()
}
