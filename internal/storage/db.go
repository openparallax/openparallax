// Package storage provides the SQLite persistence layer for sessions, memory
// search (FTS5), chronicle metadata, and audit indexing.
package storage

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite" // Pure Go SQLite driver — no CGo required.
)

// DB is the SQLite database connection.
type DB struct {
	conn *sql.DB
}

// Open opens (or creates) the SQLite database at the given path and runs
// all schema migrations. The database uses WAL journal mode for concurrent
// read performance and a 5-second busy timeout to handle lock contention.
func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := conn.Ping(); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	return db, nil
}

// migrate runs all schema creation statements. Each statement uses
// IF NOT EXISTS so migrations are idempotent.
func (db *DB) migrate() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			mode TEXT NOT NULL DEFAULT 'normal',
			title TEXT,
			created_at TEXT NOT NULL,
			last_message_at TEXT
		)`,

		`CREATE TABLE IF NOT EXISTS messages (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			thoughts_json TEXT,
			artifacts_json TEXT,
			timestamp TEXT NOT NULL,
			FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id)`,

		`CREATE VIRTUAL TABLE IF NOT EXISTS memory_fts USING fts5(
			path,
			section,
			content,
			tokenize='porter unicode61'
		)`,

		`CREATE TABLE IF NOT EXISTS snapshots (
			id TEXT PRIMARY KEY,
			transaction_id TEXT,
			timestamp TEXT NOT NULL,
			action_type TEXT NOT NULL,
			action_summary TEXT,
			files_json TEXT,
			hash TEXT NOT NULL,
			previous_hash TEXT
		)`,

		`CREATE TABLE IF NOT EXISTS transactions (
			id TEXT PRIMARY KEY,
			status TEXT NOT NULL DEFAULT 'active',
			started_at TEXT NOT NULL,
			finished_at TEXT
		)`,

		`CREATE TABLE IF NOT EXISTS audit_index (
			id TEXT PRIMARY KEY,
			event_type INTEGER NOT NULL,
			timestamp INTEGER NOT NULL,
			session_id TEXT,
			action_type TEXT,
			otr INTEGER DEFAULT 0
		)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON audit_index(timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_session ON audit_index(session_id)`,

		// Chunk-based memory index for hybrid search.
		`CREATE TABLE IF NOT EXISTS chunks (
			id TEXT PRIMARY KEY,
			path TEXT NOT NULL,
			start_line INTEGER,
			end_line INTEGER,
			text TEXT NOT NULL,
			hash TEXT NOT NULL,
			embedding BLOB,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_chunks_path ON chunks(path)`,
		`CREATE INDEX IF NOT EXISTS idx_chunks_hash ON chunks(hash)`,

		`CREATE VIRTUAL TABLE IF NOT EXISTS chunks_fts USING fts5(
			text,
			content='chunks',
			content_rowid='rowid'
		)`,

		// Embedding cache: avoid re-embedding unchanged content.
		`CREATE TABLE IF NOT EXISTS embedding_cache (
			content_hash TEXT PRIMARY KEY,
			embedding BLOB NOT NULL,
			model TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// File hash tracking for incremental indexing.
		`CREATE TABLE IF NOT EXISTS file_hashes (
			path TEXT PRIMARY KEY,
			hash TEXT NOT NULL,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
	}

	// Enable foreign key enforcement.
	if _, err := db.conn.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	for _, m := range migrations {
		if _, err := db.conn.Exec(m); err != nil {
			return fmt.Errorf("migration error: %w", err)
		}
	}

	return nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	return db.conn.Close()
}

// Conn returns the underlying *sql.DB for advanced queries.
func (db *DB) Conn() *sql.DB {
	return db.conn
}
