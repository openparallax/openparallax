//go:build darwin

package sandbox

import (
	"net"
	"os/exec"
	"time"
)

// runPlatformProbes runs macOS-specific canary probes.
// sandbox-exec enforces: file read, file write, network, process spawn.
func runPlatformProbes() []ProbeResult {
	var probes []ProbeResult

	// File read: try reading /etc/master.passwd (should be blocked).
	probes = append(probes, probeFileRead("/etc/master.passwd"))

	// File write: try writing to /tmp (should be blocked).
	probes = append(probes, probeFileWrite("/tmp"))

	// Network: try connecting to an external host (should be blocked).
	probes = append(probes, probeNetwork("1.1.1.1:443"))

	// Process spawn: try executing a child process (should be blocked).
	probes = append(probes, probeProcessSpawn())

	return probes
}

// probeNetwork tests whether outbound TCP to an external host is blocked.
func probeNetwork(host string) ProbeResult {
	conn, err := net.DialTimeout("tcp", host, 2*time.Second)
	if err != nil {
		return ProbeResult{Name: "network", Status: "blocked", Target: host}
	}
	_ = conn.Close()
	return ProbeResult{Name: "network", Status: "failed", Target: host,
		Error: "sandbox did not block outbound connection"}
}

// probeProcessSpawn tests whether spawning a child process is blocked.
func probeProcessSpawn() ProbeResult {
	cmd := exec.Command("/usr/bin/true")
	err := cmd.Run()
	if err != nil {
		return ProbeResult{Name: "process_spawn", Status: "blocked", Target: "/usr/bin/true"}
	}
	return ProbeResult{Name: "process_spawn", Status: "failed", Target: "/usr/bin/true",
		Error: "sandbox did not block process spawn"}
}
