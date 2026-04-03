// Package memory manages workspace memory files, FTS5 search indexing,
// daily action logging, and session summarization.
package memory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/openparallax/openparallax/internal/llm"
	"github.com/openparallax/openparallax/internal/storage"
	"github.com/openparallax/openparallax/internal/types"
)

// Store is the persistence interface Memory requires for FTS5 indexing.
type Store interface {
	// IndexMemoryFile indexes a memory file for FTS5 search.
	IndexMemoryFile(path string, content string)
	// ClearMemoryIndex removes all FTS5 entries.
	ClearMemoryIndex()
	// SearchMemory performs FTS5 search across indexed memory content.
	SearchMemory(query string, limit int) ([]storage.SearchResult, error)
}

// Manager handles memory file operations and search.
type Manager struct {
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
func (m *Manager) Read(fileType types.MemoryFileType) (string, error) {
	path := filepath.Join(m.workspace, string(fileType))
	data, err := os.ReadFile(path)
	if err != nil {
		return "", types.ErrMemoryFileNotFound
	}
	return string(data), nil
}

// Append adds content to the end of a memory file and reindexes it.
func (m *Manager) Append(fileType types.MemoryFileType, content string) error {
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
func (m *Manager) Search(query string, limit int) ([]storage.SearchResult, error) {
	if limit <= 0 {
		limit = 20
	}
	return m.store.SearchMemory(query, limit)
}

// LogAction appends one-line entries to the daily log after each action.
func (m *Manager) LogAction(actions []*types.ActionRequest, results []*types.ActionResult) {
	today := time.Now().Format("2006-01-02")
	logDir := filepath.Join(m.workspace, "memory")
	_ = os.MkdirAll(logDir, 0o755)
	logPath := filepath.Join(logDir, today+".md")

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()

	timeStr := time.Now().Format("15:04:05")
	for i, action := range actions {
		if i < len(results) {
			result := results[i]
			status := "OK"
			if !result.Success {
				status = "FAILED"
			}
			_, _ = fmt.Fprintf(f, "- %s > %s: %s > %s\n", timeStr, action.Type, result.Summary, status)
		}
	}
}

// SummarizeSession generates a 2-3 bullet summary and appends it to MEMORY.md.
func (m *Manager) SummarizeSession(ctx context.Context, sessionTitle string, messages []llm.ChatMessage) error {
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
		`Summarize this conversation in 2-3 concise bullet points. Focus on:
- Decisions made
- Facts learned about the user
- Tasks completed or still in progress

Be brief. Use "- " for each bullet point.

Conversation:
%s`, conv.String()), llm.WithMaxTokens(300))
	if err != nil {
		return err
	}

	header := fmt.Sprintf("\n## %s", time.Now().Format("2006-01-02"))
	if sessionTitle != "" {
		header += fmt.Sprintf(" — %s", sessionTitle)
	}
	header += "\n"

	return m.Append(types.MemoryMain, header+summary+"\n")
}

// ReindexAll rebuilds the FTS5 index from all memory files on disk.
func (m *Manager) ReindexAll() {
	m.store.ClearMemoryIndex()

	for _, ft := range types.AllMemoryFiles {
		content, err := m.Read(ft)
		if err == nil {
			m.store.IndexMemoryFile(string(ft), content)
		}
	}

	logDir := filepath.Join(m.workspace, "memory")
	entries, _ := os.ReadDir(logDir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".md") {
			content, err := os.ReadFile(filepath.Join(logDir, e.Name()))
			if err == nil {
				m.store.IndexMemoryFile("memory/"+e.Name(), string(content))
			}
		}
	}
}
