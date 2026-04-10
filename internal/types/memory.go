package types

// MemoryFileType identifies workspace memory files.
type MemoryFileType string

const (
	// MemorySoul defines the agent's core values, personality, and guardrails.
	MemorySoul MemoryFileType = "SOUL.md"
	// MemoryIdentity defines the agent's name, role, and communication style.
	MemoryIdentity MemoryFileType = "IDENTITY.md"
	// MemoryUser defines the user's profile and preferences.
	MemoryUser MemoryFileType = "USER.md"
	// MemoryMain is the accumulated knowledge and session summaries.
	MemoryMain MemoryFileType = "MEMORY.md"
	// MemoryHeartbeat defines cron schedules for proactive tasks.
	MemoryHeartbeat MemoryFileType = "HEARTBEAT.md"
	// MemoryAgents defines the multi-agent roster.
	MemoryAgents MemoryFileType = "AGENTS.md"
)

// AllMemoryFiles is the complete list of workspace memory files.
var AllMemoryFiles = []MemoryFileType{
	MemorySoul, MemoryIdentity, MemoryUser, MemoryMain,
	MemoryHeartbeat, MemoryAgents,
}
