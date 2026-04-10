//go:build linux

package sandbox

// requiredProbes returns the set of probes that must pass for the agent to start.
// On Linux: filesystem enforcement is required, process spawn is advisory.
func requiredProbes() map[string]bool {
	return map[string]bool{
		"file_read":  true,
		"file_write": true,
	}
}
