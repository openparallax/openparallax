//go:build !linux && !darwin && !windows

package sandbox

// requiredProbes returns the set of probes that must pass for the agent to start.
// On unsupported platforms: nothing is required.
func requiredProbes() map[string]bool {
	return map[string]bool{}
}
