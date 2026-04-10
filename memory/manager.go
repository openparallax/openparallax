// Package memory manages workspace memory files, FTS5 search indexing,
// and session summarization.
package memory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"sync"

	"github.com/openparallax/openparallax/llm"
)

// summarizeMaxTokens is the LLM token cap for session summarization calls.
const summarizeMaxTokens = 500

// Store is the persistence interface Memory requires for FTS5 indexing.
type Store interface {
	// IndexMemoryFile indexes a memory file for FTS5 search.
	IndexMemoryFile(path string, content string)
	// ClearMemoryIndex removes all FTS5 entries.
	ClearMemoryIndex()
	// SearchMemory performs FTS5 search across indexed memory content.
	SearchMemory(query string, limit int) ([]SearchResult, error)
}

// Manager handles memory file operations and search.
type Manager struct {
	mu        sync.RWMutex
	workspace string
	store     Store
	llm       llm.Provider
}

// NewManager creates a memory Manager and indexes all memory files on startup.
func NewManager(workspace string, store Store, provider llm.Provider) *Manager {
	m := &Manager{workspace: workspace, store: store, llm: provider}
	m.ReindexAll()
	return m
}

// Read reads a memory file by type.
func (m *Manager) Read(fileType FileType) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.readLocked(fileType)
}

func (m *Manager) readLocked(fileType FileType) (string, error) {
	path := filepath.Join(m.workspace, string(fileType))
	data, err := os.ReadFile(path)
	if err != nil {
		return "", ErrFileNotFound
	}
	return string(data), nil
}

// Append adds content to the end of a memory file and reindexes it.
func (m *Manager) Append(fileType FileType, content string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.appendLocked(fileType, content)
}

func (m *Manager) appendLocked(fileType FileType, content string) error {
	path := filepath.Join(m.workspace, string(fileType))

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	if _, err := f.WriteString(content); err != nil {
		return err
	}

	fullContent, _ := os.ReadFile(path)
	m.store.IndexMemoryFile(string(fileType), string(fullContent))

	return nil
}

// Search performs full-text search across all indexed memory content.
func (m *Manager) Search(query string, limit int) ([]SearchResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if limit <= 0 {
		limit = 20
	}
	return m.store.SearchMemory(query, limit)
}

// SummarizeSession generates a 2-3 bullet summary and appends it to MEMORY.md.
func (m *Manager) SummarizeSession(ctx context.Context, sessionTitle string, messages []llm.ChatMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(messages) < 2 {
		return nil
	}

	recent := messages
	if len(recent) > 10 {
		recent = recent[len(recent)-10:]
	}

	var conv strings.Builder
	for _, msg := range recent {
		fmt.Fprintf(&conv, "%s: %s\n", msg.Role, msg.Content)
	}

	summary, err := m.llm.Complete(ctx, fmt.Sprintf(
		`Extract the most useful facts from this conversation as 2-5 bullet points.

Capture SPECIFIC details that would be valuable in future conversations:
- Names, URLs, file paths, project names, API keys (masked), versions mentioned
- User preferences and decisions ("prefers X over Y", "chose to use Z")
- Technical facts ("database is PostgreSQL 16", "API rate limit is 100/min")
- Tasks started but not finished, with enough context to resume
- Corrections the user made ("don't do X because Y")

Rules:
- Be concrete. "Set up project" is useless. "Created Next.js app 'acme-dashboard' at ~/projects/" is useful.
- Skip greetings, small talk, and anything obvious from the code itself.
- If nothing specific is worth remembering, respond with exactly "NONE".
- Use "- " for each bullet point.

Conversation:
%s`, conv.String()), llm.WithMaxTokens(summarizeMaxTokens))
	if err != nil {
		return err
	}

	trimmed := strings.TrimSpace(summary)
	if trimmed == "" || strings.EqualFold(trimmed, "NONE") {
		return nil
	}

	header := fmt.Sprintf("\n## %s", time.Now().Format("2006-01-02"))
	if sessionTitle != "" {
		header += fmt.Sprintf(" — %s", sessionTitle)
	}
	header += "\n"

	return m.appendLocked(MemoryMain, header+summary+"\n")
}

// ReindexAll rebuilds the FTS5 index from workspace memory files.
func (m *Manager) ReindexAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.store.ClearMemoryIndex()

	for _, ft := range AllFiles {
		content, err := m.readLocked(ft)
		if err == nil {
			m.store.IndexMemoryFile(string(ft), content)
		}
	}
}

// SearchRelevant returns memory chunks relevant to a query plus the most
// recent entries, deduplicated. Used by the context assembler to inject
// only pertinent memory into the system prompt instead of the full file.
func (m *Manager) SearchRelevant(query string, relevantK, recentK int) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	seen := make(map[string]bool)
	var chunks []string

	// FTS5 relevance search.
	if query != "" {
		if results, err := m.store.SearchMemory(query, relevantK); err == nil {
			for _, r := range results {
				key := r.Section + "|" + r.Snippet
				if !seen[key] {
					seen[key] = true
					entry := r.Snippet
					if r.Section != "" {
						entry = r.Section + "\n" + r.Snippet
					}
					chunks = append(chunks, strings.TrimSpace(entry))
				}
			}
		}
	}

	// Recent entries: read MEMORY.md and take the last N sections.
	if content, err := m.readLocked(MemoryMain); err == nil {
		sections := splitMemorySections(content)
		start := len(sections) - recentK
		if start < 0 {
			start = 0
		}
		for _, s := range sections[start:] {
			trimmed := strings.TrimSpace(s)
			if trimmed == "" {
				continue
			}
			if !seen[trimmed] {
				seen[trimmed] = true
				chunks = append(chunks, trimmed)
			}
		}
	}

	return chunks
}

// splitMemorySections splits MEMORY.md content by ## headers.
func splitMemorySections(content string) []string {
	var sections []string
	var current strings.Builder
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "## ") && current.Len() > 0 {
			sections = append(sections, current.String())
			current.Reset()
		}
		current.WriteString(line)
		current.WriteByte('\n')
	}
	if current.Len() > 0 {
		sections = append(sections, current.String())
	}
	return sections
}
