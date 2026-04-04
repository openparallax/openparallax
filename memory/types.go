package memory

import "errors"

// SearchResult is a single FTS5 search result from memory files.
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

// ChunkSearchResult is a search result from the chunks index.
type ChunkSearchResult struct {
	// ID is the unique chunk identifier.
	ID string
	// Path is the source file path.
	Path string
	// StartLine is the first line of the chunk in the source file.
	StartLine int
	// EndLine is the last line of the chunk in the source file.
	EndLine int
	// Text is the chunk content.
	Text string
	// Score is the relevance score.
	Score float64
	// Source indicates how the result was found ("keyword", "vector", or "hybrid").
	Source string
}

// ChunkEmbedding holds a chunk ID and its embedding vector.
type ChunkEmbedding struct {
	// ID is the unique chunk identifier.
	ID string
	// Embedding is the float32 vector for the chunk.
	Embedding []float32
}

// SessionMessage is a minimal message representation for session indexing.
type SessionMessage struct {
	// Role is the message sender role (e.g. "user", "assistant").
	Role string
	// Content is the message text.
	Content string
}

// FileType identifies workspace memory files.
type FileType string

const (
	// MemorySoul defines the agent's core values, personality, and guardrails.
	MemorySoul FileType = "SOUL.md"
	// MemoryIdentity defines the agent's name, role, and communication style.
	MemoryIdentity FileType = "IDENTITY.md"
	// MemoryUser defines the user's profile and preferences.
	MemoryUser FileType = "USER.md"
	// MemoryMain is the accumulated knowledge and session summaries.
	MemoryMain FileType = "MEMORY.md"
	// MemoryHeartbeat defines cron schedules for proactive tasks.
	MemoryHeartbeat FileType = "HEARTBEAT.md"
	// MemoryAgents defines the multi-agent roster.
	MemoryAgents FileType = "AGENTS.md"
	// MemoryTools defines available tools and capabilities.
	MemoryTools FileType = "TOOLS.md"
	// MemoryBoot defines the startup checklist.
	MemoryBoot FileType = "BOOT.md"
)

// AllFiles is the complete list of workspace memory file types.
var AllFiles = []FileType{
	MemorySoul, MemoryIdentity, MemoryUser, MemoryMain,
	MemoryHeartbeat, MemoryAgents, MemoryTools, MemoryBoot,
}

// ErrFileNotFound indicates a memory file does not exist in the workspace.
var ErrFileNotFound = errors.New("memory file not found")

// ActionEntry is the minimal action data needed for daily log entries.
type ActionEntry struct {
	// Type is the action type identifier (e.g. "read_file", "execute_command").
	Type string
}

// ResultEntry is the minimal result data needed for daily log entries.
type ResultEntry struct {
	// Success indicates whether the action succeeded.
	Success bool
	// Summary is a one-line description of the result.
	Summary string
}
