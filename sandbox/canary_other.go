//go:build !linux && !darwin && !windows

package sandbox

// runPlatformProbes returns no probes on unsupported platforms.
func runPlatformProbes() []ProbeResult {
	return nil
}
