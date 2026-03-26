package audit

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/openparallax/openparallax/internal/types"
)

// Query filters for reading audit entries.
type Query struct {
	SessionID string
	EventType types.AuditEventType
	Limit     int
}

// ReadEntries loads audit entries from the JSONL file, applying optional filters.
func ReadEntries(path string, q Query) ([]types.AuditEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	var entries []types.AuditEntry

	// Read in reverse order (most recent first).
	for i := len(lines) - 1; i >= 0; i-- {
		if lines[i] == "" {
			continue
		}
		var entry types.AuditEntry
		if err := json.Unmarshal([]byte(lines[i]), &entry); err != nil {
			continue
		}
		if q.SessionID != "" && entry.SessionID != q.SessionID {
			continue
		}
		if q.EventType != 0 && entry.EventType != q.EventType {
			continue
		}
		entries = append(entries, entry)
		if q.Limit > 0 && len(entries) >= q.Limit {
			break
		}
	}

	return entries, nil
}
