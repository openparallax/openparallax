package mcp

// ServerConfig configures a single MCP server connection.
type ServerConfig struct {
	// Name is the server identifier used for tool namespacing.
	Name string `yaml:"name" json:"name"`
	// Command is the executable command for stdio transport.
	Command string `yaml:"command" json:"command"`
	// Args are the command arguments.
	Args []string `yaml:"args,omitempty" json:"args,omitempty"`
	// Env provides additional environment variables.
	Env map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
	// IdleTimeout is the idle timeout in seconds before auto-shutdown (default 300).
	IdleTimeout int `yaml:"idle_timeout,omitempty" json:"idle_timeout,omitempty"`
}
