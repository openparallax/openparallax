//go:build linux

package sandbox

import "github.com/shoenig/go-landlock"

// runPlatformProbes runs Linux-specific canary probes.
// Landlock enforces: file read, file write, network (v4+/kernel 6.7+).
func runPlatformProbes() []ProbeResult {
	var probes []ProbeResult

	// File read: try reading /etc/shadow (should be blocked).
	probes = append(probes, probeFileRead("/etc/shadow"))

	// File write: try writing to /tmp (should be blocked if not in AllowedWritePaths).
	probes = append(probes, probeFileWrite("/tmp"))

	// Network: try connecting to an external host.
	// Only applicable on Landlock v4+ (kernel 6.7+).
	version, _ := landlock.Detect()
	if version >= 4 {
		probes = append(probes, probeNetwork("1.1.1.1:443"))
	} else {
		probes = append(probes, ProbeResult{
			Name:   "network",
			Status: "skipped",
			Error:  "requires Landlock v4+ (kernel 6.7+)",
		})
	}

	return probes
}
