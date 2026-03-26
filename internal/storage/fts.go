package storage

import "strings"

// SearchResult is a single FTS5 search result.
type SearchResult struct {
	// Path is the memory file path.
	Path string `json:"path"`

	// Section is the markdown section header the match was found in.
	Section string `json:"section"`

	// Snippet is the highlighted match context.
	Snippet string `json:"snippet"`

	// Score is the FTS5 ranking score (lower is better).
	Score float64 `json:"score"`
}

// IndexMemoryFile indexes a memory file's content for FTS5 search.
// Content is split into sections by ## headers, then into paragraphs
// of approximately 500 characters each.
func (db *DB) IndexMemoryFile(path string, content string) {
	// Remove existing entries for this path.
	db.conn.Exec(`DELETE FROM memory_fts WHERE path = ?`, path) //nolint:errcheck // best-effort cleanup

	sections := splitSections(content)
	for _, section := range sections {
		paragraphs := splitParagraphs(section.content, 500)
		for _, para := range paragraphs {
			db.conn.Exec( //nolint:errcheck // best-effort indexing
				`INSERT INTO memory_fts (path, section, content) VALUES (?, ?, ?)`,
				path, section.header, para,
			)
		}
	}
}

// SearchMemory performs FTS5 search across all indexed memory content.
func (db *DB) SearchMemory(query string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 20
	}

	rows, err := db.conn.Query(
		`SELECT path, section, snippet(memory_fts, 2, '<b>', '</b>', '...', 30) as snippet, rank
		 FROM memory_fts
		 WHERE memory_fts MATCH ?
		 ORDER BY rank
		 LIMIT ?`, query, limit,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.Path, &r.Section, &r.Snippet, &r.Score); err != nil {
			continue
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// ClearMemoryIndex removes all FTS5 entries.
func (db *DB) ClearMemoryIndex() {
	db.conn.Exec(`DELETE FROM memory_fts`) //nolint:errcheck // best-effort cleanup
}

// sectionEntry holds a parsed markdown section.
type sectionEntry struct {
	header  string
	content string
}

// splitSections splits markdown content into sections by ## headers.
func splitSections(content string) []sectionEntry {
	lines := strings.Split(content, "\n")
	var sections []sectionEntry
	current := sectionEntry{header: "top"}
	var buf strings.Builder

	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			if buf.Len() > 0 {
				current.content = buf.String()
				sections = append(sections, current)
				buf.Reset()
			}
			current = sectionEntry{header: strings.TrimPrefix(line, "## ")}
		} else {
			buf.WriteString(line)
			buf.WriteString("\n")
		}
	}
	if buf.Len() > 0 {
		current.content = buf.String()
		sections = append(sections, current)
	}

	return sections
}

// splitParagraphs breaks text into chunks of approximately maxChars characters,
// splitting on paragraph boundaries (double newlines).
func splitParagraphs(content string, maxChars int) []string {
	paragraphs := strings.Split(content, "\n\n")
	var result []string
	var current strings.Builder

	for _, p := range paragraphs {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if current.Len()+len(p) > maxChars && current.Len() > 0 {
			result = append(result, current.String())
			current.Reset()
		}
		if current.Len() > 0 {
			current.WriteString(" ")
		}
		current.WriteString(p)
	}
	if current.Len() > 0 {
		result = append(result, current.String())
	}
	return result
}
