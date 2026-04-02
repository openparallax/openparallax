//go:build darwin

package sandbox

// runPlatformProbes runs macOS-specific canary probes.
// sandbox-exec enforces: file read, file write, network.
// /proc does not exist on macOS.
func runPlatformProbes() []ProbeResult {
	var probes []ProbeResult

	// File read: try reading /etc/master.passwd (should be blocked).
	probes = append(probes, probeFileRead("/etc/master.passwd"))

	// File write: try writing to /tmp (should be blocked).
	probes = append(probes, probeFileWrite("/tmp"))

	// Network: try connecting to an external host (should be blocked).
	probes = append(probes, probeNetwork("1.1.1.1:443"))

	return probes
}
