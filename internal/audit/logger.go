// Package audit provides an append-only JSONL audit log with SHA-256 hash chain.
// Each entry includes the hash of the previous entry, making any tampering detectable.
package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/openparallax/openparallax/internal/crypto"
	"github.com/openparallax/openparallax/internal/types"
)

// Entry is the input for logging an audit event.
type Entry struct {
	EventType  types.AuditEventType
	ActionType string
	SessionID  string
	Details    string
	OTR        bool
	Source     string
}

// Logger writes audit entries to an append-only JSONL file with hash chain.
type Logger struct {
	file     *os.File
	lastHash string
	mu       sync.Mutex
}

// NewLogger creates an audit logger at the given path.
// Reads the last entry to recover the chain hash on startup.
func NewLogger(path string) (*Logger, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}

	lastHash := readLastHash(path)

	return &Logger{file: f, lastHash: lastHash}, nil
}

// Log writes an entry to the audit log. Thread-safe.
func (l *Logger) Log(entry Entry) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	auditEntry := types.AuditEntry{
		ID:           crypto.NewID(),
		EventType:    entry.EventType,
		Timestamp:    time.Now().UnixMilli(),
		SessionID:    entry.SessionID,
		ActionType:   entry.ActionType,
		DetailsJSON:  entry.Details,
		PreviousHash: l.lastHash,
		OTR:          entry.OTR,
		Source:       entry.Source,
	}

	canonical, err := crypto.Canonicalize(auditEntry)
	if err != nil {
		return fmt.Errorf("failed to canonicalize audit entry: %w", err)
	}
	auditEntry.Hash = crypto.SHA256Hex(canonical)

	data, err := json.Marshal(auditEntry)
	if err != nil {
		return fmt.Errorf("failed to marshal audit entry: %w", err)
	}
	if _, err := fmt.Fprintf(l.file, "%s\n", data); err != nil {
		return fmt.Errorf("failed to write audit entry: %w", err)
	}

	l.lastHash = auditEntry.Hash
	return nil
}

// Close flushes and closes the audit log.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.file.Close()
}

// readLastHash reads the last line of the JSONL file and extracts the hash.
func readLastHash(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 0 {
		return ""
	}
	var entry types.AuditEntry
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &entry); err != nil {
		return ""
	}
	return entry.Hash
}
