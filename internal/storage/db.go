// Package storage provides the SQLite persistence layer for sessions, memory
// search (FTS5), chronicle metadata, and audit indexing.
package storage

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite" // Pure Go SQLite driver — no CGo required.
)

// DB is the SQLite database connection.
type DB struct {
	conn *sql.DB
}

// Open opens (or creates) the SQLite database at the given path and runs
// all schema migrations. The database uses WAL journal mode for concurrent
// read performance and a 5-second busy timeout to handle lock contention.
// Writers are wrapped in withRetry to handle SQLITE_BUSY_SNAPSHOT errors
// that the busy_timeout pragma cannot retry on its own.
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

// withRetry runs fn up to 5 times with exponential backoff on SQLITE_BUSY
// or SQLITE_BUSY_SNAPSHOT errors. These can occur on concurrent writers
// even with the busy_timeout pragma set, particularly when transactions
// race to upgrade from read to write in WAL mode.
func withRetry(fn func() error) error {
	const maxAttempts = 5
	var err error
	delay := 10 * time.Millisecond
	for i := 0; i < maxAttempts; i++ {
		err = fn()
		if err == nil {
			return nil
		}
		msg := err.Error()
		if !strings.Contains(msg, "SQLITE_BUSY") && !strings.Contains(msg, "database is locked") {
			return err
		}
		time.Sleep(delay)
		delay *= 2
	}
	return err
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

		// Per-message LLM token usage for cost and performance tracking.
		`CREATE TABLE IF NOT EXISTS llm_usage (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id TEXT NOT NULL,
			message_id TEXT NOT NULL,
			provider TEXT NOT NULL,
			model TEXT NOT NULL,
			input_tokens INTEGER NOT NULL DEFAULT 0,
			output_tokens INTEGER NOT NULL DEFAULT 0,
			cache_read_tokens INTEGER NOT NULL DEFAULT 0,
			cache_creation_tokens INTEGER NOT NULL DEFAULT 0,
			tool_def_tokens INTEGER NOT NULL DEFAULT 0,
			rounds INTEGER NOT NULL DEFAULT 0,
			duration_ms INTEGER NOT NULL DEFAULT 0,
			timestamp TEXT NOT NULL DEFAULT (datetime('now')),
			FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_llm_usage_session ON llm_usage(session_id)`,
		`CREATE INDEX IF NOT EXISTS idx_llm_usage_timestamp ON llm_usage(timestamp)`,

		// Daily aggregated metrics for dashboard views.
		`CREATE TABLE IF NOT EXISTS metrics_daily (
			date TEXT NOT NULL,
			metric TEXT NOT NULL,
			value INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (date, metric)
		)`,

		// Per-observation latency samples for percentile queries.
		// Mirrors the llm_usage.duration_ms approach but for events
		// (Shield evaluations etc.) that are not LLM calls.
		`CREATE TABLE IF NOT EXISTS metrics_latency (
			date TEXT NOT NULL,
			metric TEXT NOT NULL,
			latency_ms INTEGER NOT NULL,
			timestamp TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE INDEX IF NOT EXISTS idx_metrics_latency_date ON metrics_latency(date, metric)`,

		// OAuth2 token storage (encrypted at rest).
		`CREATE TABLE IF NOT EXISTS oauth_tokens (
			provider TEXT NOT NULL,
			account TEXT NOT NULL,
			access_token_enc BLOB NOT NULL,
			refresh_token_enc BLOB NOT NULL,
			expiry TEXT NOT NULL,
			scopes TEXT,
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now')),
			PRIMARY KEY (provider, account)
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
