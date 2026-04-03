// Package session manages the lifecycle of conversation sessions including
// creation, retrieval, listing, deletion, and OTR (Off the Record) mode.
package session

import (
	"sync"
	"time"
	"unicode/utf8"

	"github.com/openparallax/openparallax/crypto"
	"github.com/openparallax/openparallax/internal/llm"
	"github.com/openparallax/openparallax/internal/storage"
	"github.com/openparallax/openparallax/internal/types"
)

// Store manages session lifecycle. Normal sessions persist in SQLite.
// OTR sessions live in memory only and vanish on shutdown.
type Store struct {
	db          *storage.DB
	otrSessions sync.Map
}

// NewStore creates a session Store backed by the given database.
func NewStore(db *storage.DB) *Store {
	return &Store{db: db}
}

// Create creates a new session with the given mode.
// Normal sessions are persisted to SQLite. OTR sessions are stored in memory.
func (s *Store) Create(mode types.SessionMode) *types.Session {
	sess := &types.Session{
		ID:        crypto.NewID(),
		Mode:      mode,
		CreatedAt: time.Now(),
	}

	if mode == types.SessionOTR {
		s.otrSessions.Store(sess.ID, sess)
	} else {
		_ = s.db.InsertSession(sess)
	}

	return sess
}

// Get retrieves a session by ID. Checks OTR sessions first, then SQLite.
func (s *Store) Get(id string) (*types.Session, error) {
	if val, ok := s.otrSessions.Load(id); ok {
		sess, _ := val.(*types.Session)
		return sess, nil
	}
	return s.db.GetSession(id)
}

// List returns all Normal sessions ordered by most recent activity.
// OTR sessions are never included.
func (s *Store) List() ([]types.SessionMetadata, error) {
	return s.db.ListSessions()
}

// Delete removes a session and all its messages.
func (s *Store) Delete(id string) error {
	s.otrSessions.Delete(id)
	return s.db.DeleteSession(id)
}

// Rename updates a session's display title.
func (s *Store) Rename(id, title string) error {
	return s.db.UpdateSessionTitle(id, title)
}

// GetHistory returns the conversation history for a session as LLM messages.
func (s *Store) GetHistory(sessionID string) []llm.ChatMessage {
	messages, err := s.db.GetMessages(sessionID)
	if err != nil {
		return nil
	}
	result := make([]llm.ChatMessage, 0, len(messages))
	for _, m := range messages {
		result = append(result, llm.ChatMessage{Role: m.Role, Content: m.Content})
	}
	return result
}

// DestroyOTR removes all OTR sessions. Called on shutdown.
func (s *Store) DestroyOTR() {
	s.otrSessions.Range(func(key, _ any) bool {
		s.otrSessions.Delete(key)
		return true
	})
}

// AutoTitle generates a short title from the first user message.
// Truncates to 50 characters at a word boundary.
func AutoTitle(content string) string {
	if content == "" {
		return "New session"
	}
	maxLen := 50
	if utf8.RuneCountInString(content) <= maxLen {
		return content
	}
	runes := []rune(content)
	truncated := string(runes[:maxLen])
	// Find last space to avoid cutting mid-word.
	for i := len(truncated) - 1; i > 0; i-- {
		if truncated[i] == ' ' {
			return truncated[:i] + "..."
		}
	}
	return truncated + "..."
}
