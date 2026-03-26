package storage

import (
	"encoding/json"
	"time"

	"github.com/openparallax/openparallax/internal/types"
)

// InsertSession creates a new session record.
func (db *DB) InsertSession(s *types.Session) error {
	_, err := db.conn.Exec(
		`INSERT INTO sessions (id, mode, title, created_at) VALUES (?, ?, ?, ?)`,
		s.ID, s.Mode, s.Title, s.CreatedAt.Format(time.RFC3339),
	)
	return err
}

// GetSession retrieves a session by ID.
func (db *DB) GetSession(id string) (*types.Session, error) {
	row := db.conn.QueryRow(
		`SELECT id, mode, title, created_at, last_message_at FROM sessions WHERE id = ?`, id,
	)

	var s types.Session
	var createdStr string
	var lastMsgStr *string
	if err := row.Scan(&s.ID, &s.Mode, &s.Title, &createdStr, &lastMsgStr); err != nil {
		return nil, types.ErrSessionNotFound
	}

	s.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)
	if lastMsgStr != nil {
		t, _ := time.Parse(time.RFC3339, *lastMsgStr)
		s.LastMsgAt = &t
	}

	return &s, nil
}

// ListSessions returns all non-OTR sessions ordered by most recent activity.
func (db *DB) ListSessions() ([]types.SessionMetadata, error) {
	rows, err := db.conn.Query(
		`SELECT s.id, s.mode, s.title, s.created_at, s.last_message_at,
			(SELECT content FROM messages WHERE session_id = s.id ORDER BY timestamp DESC LIMIT 1) as preview
		 FROM sessions s
		 WHERE s.mode = 'normal'
		 ORDER BY COALESCE(s.last_message_at, s.created_at) DESC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var sessions []types.SessionMetadata
	for rows.Next() {
		var s types.SessionMetadata
		var title, lastMsg, preview *string
		if err := rows.Scan(&s.ID, &s.Mode, &title, &s.CreatedAt, &lastMsg, &preview); err != nil {
			continue
		}
		if title != nil {
			s.Title = *title
		}
		if lastMsg != nil {
			s.LastMsgAt = *lastMsg
		}
		if preview != nil {
			s.Preview = *preview
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

// InsertMessage stores a message in a session and updates the session's
// last_message_at timestamp.
func (db *DB) InsertMessage(m *types.Message) error {
	thoughtsJSON, _ := json.Marshal(m.Thoughts)
	artifactsJSON, _ := json.Marshal(m.Artifacts)

	ts := m.Timestamp.Format(time.RFC3339)
	_, err := db.conn.Exec(
		`INSERT INTO messages (id, session_id, role, content, thoughts_json, artifacts_json, timestamp) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		m.ID, m.SessionID, m.Role, m.Content, string(thoughtsJSON), string(artifactsJSON), ts,
	)
	if err != nil {
		return err
	}

	_, err = db.conn.Exec(`UPDATE sessions SET last_message_at = ? WHERE id = ?`, ts, m.SessionID)
	return err
}

// GetMessages returns all messages in a session ordered by timestamp.
func (db *DB) GetMessages(sessionID string) ([]types.Message, error) {
	rows, err := db.conn.Query(
		`SELECT id, session_id, role, content, thoughts_json, artifacts_json, timestamp
		 FROM messages WHERE session_id = ? ORDER BY timestamp ASC`, sessionID,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var messages []types.Message
	for rows.Next() {
		var m types.Message
		var thoughtsJSON, artifactsJSON, tsStr string
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &thoughtsJSON, &artifactsJSON, &tsStr); err != nil {
			continue
		}
		m.Timestamp, _ = time.Parse(time.RFC3339, tsStr)
		_ = json.Unmarshal([]byte(thoughtsJSON), &m.Thoughts)
		_ = json.Unmarshal([]byte(artifactsJSON), &m.Artifacts)
		messages = append(messages, m)
	}
	return messages, rows.Err()
}

// DeleteSession removes a session and all its messages (via CASCADE).
func (db *DB) DeleteSession(id string) error {
	// Manually delete messages first because SQLite CGo-free driver
	// may not enforce foreign key cascades in all configurations.
	if _, err := db.conn.Exec(`DELETE FROM messages WHERE session_id = ?`, id); err != nil {
		return err
	}
	_, err := db.conn.Exec(`DELETE FROM sessions WHERE id = ?`, id)
	return err
}

// UpdateSessionTitle renames a session.
func (db *DB) UpdateSessionTitle(id, title string) error {
	_, err := db.conn.Exec(`UPDATE sessions SET title = ? WHERE id = ?`, title, id)
	return err
}

// SessionCount returns the total number of sessions.
func (db *DB) SessionCount() (int, error) {
	var count int
	err := db.conn.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&count)
	return count, err
}
