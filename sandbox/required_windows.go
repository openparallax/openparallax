//go:build windows

package sandbox

// requiredProbes returns the set of probes that must pass for the agent to start.
// On Windows: only process spawn is enforced via Job Objects.
func requiredProbes() map[string]bool {
	return map[string]bool{
		"process_spawn": true,
	}
}
