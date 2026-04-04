//go:build linux

package sandbox

import "os/exec"

// runPlatformProbes runs Linux-specific canary probes.
// Landlock enforces: file read, file write.
// seccomp-bpf enforces: process spawn blocking.
func runPlatformProbes() []ProbeResult {
	var probes []ProbeResult

	// File read: try reading /etc/shadow (should be blocked by Landlock).
	probes = append(probes, probeFileRead("/etc/shadow"))

	// File write: try writing to /tmp (should be blocked by Landlock).
	probes = append(probes, probeFileWrite("/tmp"))

	// Process spawn: try executing a child process (should be blocked by seccomp).
	probes = append(probes, probeProcessSpawn())

	return probes
}

// probeProcessSpawn tests whether spawning a child process is blocked.
func probeProcessSpawn() ProbeResult {
	cmd := exec.Command("/bin/true")
	err := cmd.Run()
	if err != nil {
		return ProbeResult{Name: "process_spawn", Status: "blocked", Target: "/bin/true"}
	}
	return ProbeResult{Name: "process_spawn", Status: "failed", Target: "/bin/true",
		Error: "sandbox did not block process spawn"}
}
