//go:build darwin

package sandbox

// requiredProbes returns the set of probes that must pass for the agent to start.
// On macOS: all probes are required — sandbox-exec enforces everything.
func requiredProbes() map[string]bool {
	return map[string]bool{
		"file_read":     true,
		"file_write":    true,
		"network":       true,
		"process_spawn": true,
	}
}
