// Package sandbox provides kernel-level process isolation using platform-native
// mechanisms. On Linux it uses Landlock LSM, on macOS sandbox-exec, on Windows
// Job Objects. All implementations are pure Go with zero CGo.
package sandbox

import "os/exec"

// Sandbox restricts the current process or a child process using
// platform-native kernel isolation.
type Sandbox interface {
	// Available reports whether this sandbox mechanism is supported
	// on the current system.
	Available() bool

	// Mode returns the sandbox mechanism name (e.g. "landlock", "sandbox-exec").
	Mode() string

	// ApplySelf restricts the current process. Once applied, restrictions
	// are irreversible. Used by the Agent on Linux to self-sandbox.
	ApplySelf(cfg Config) error

	// WrapCommand modifies cmd to run inside a sandbox. Used by the Engine
	// on macOS and Windows to spawn a sandboxed Agent.
	WrapCommand(cmd *exec.Cmd, cfg Config) error
}

// Config specifies sandbox restrictions.
type Config struct {
	// AllowedReadPaths are filesystem paths the process can read.
	// Empty means no filesystem read access beyond shared libraries.
	AllowedReadPaths []string

	// AllowedWritePaths are filesystem paths the process can write.
	// Typically empty for the Agent.
	AllowedWritePaths []string

	// AllowedTCPConnect are host:port pairs the process can connect to.
	// For the Agent: only the Engine's gRPC address.
	AllowedTCPConnect []string

	// AllowProcessSpawn controls whether the process can create children.
	// False for the Agent.
	AllowProcessSpawn bool
}

// Status reports the current sandbox state for API responses and doctor checks.
type Status struct {
	Active     bool   `json:"active"`
	Mode       string `json:"mode"`
	Version    int    `json:"version,omitempty"`
	Filesystem bool   `json:"filesystem"`
	Network    bool   `json:"network"`
	Reason     string `json:"reason,omitempty"`
}

// Probe checks what sandbox mechanism is available on this platform and
// returns the expected status. The Agent applies the sandbox on startup;
// this reports what the Engine expects the Agent to have.
func Probe() Status {
	sb := New()
	if !sb.Available() {
		return Status{Active: false, Mode: "none", Reason: unavailableReason()}
	}
	return probeStatus(sb)
}
