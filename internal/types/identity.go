package types

// AgentIdentity is parsed from IDENTITY.md.
type AgentIdentity struct {
	// Name is the agent's display name.
	Name string `json:"name"`

	// Role describes the agent's purpose.
	Role string `json:"role"`

	// CommunicationStyle describes how the agent communicates.
	CommunicationStyle string `json:"communication_style"`
}

// UserProfile is parsed from USER.md.
type UserProfile struct {
	// Name is the user's display name.
	Name string `json:"name,omitempty"`

	// Timezone is the user's timezone (e.g., "America/New_York").
	Timezone string `json:"timezone,omitempty"`

	// Language is the user's preferred language (e.g., "en").
	Language string `json:"language,omitempty"`

	// Preferences is a set of user-defined key-value preferences.
	Preferences map[string]string `json:"preferences,omitempty"`
}

// DefaultIdentity is used when IDENTITY.md doesn't exist.
var DefaultIdentity = AgentIdentity{
	Name:               "Atlas",
	Role:               "Personal AI Agent",
	CommunicationStyle: "Direct, concise, helpful",
}
