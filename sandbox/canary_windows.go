//go:build windows

package sandbox

import "os"

// runPlatformProbes runs Windows-specific canary probes.
// Job Objects enforce: process spawn blocking.
// Filesystem and network restrictions require admin elevation and are
// not available through Job Objects alone.
func runPlatformProbes() []ProbeResult {
	var probes []ProbeResult

	// Process spawn: try starting a child process (should be blocked).
	probes = append(probes, probeProcessSpawnWindows())

	// File read/write/network are not restricted by Job Objects.
	probes = append(probes, ProbeResult{
		Name:   "file_read",
		Status: "skipped",
		Error:  "not supported by Job Objects",
	})
	probes = append(probes, ProbeResult{
		Name:   "file_write",
		Status: "skipped",
		Error:  "not supported by Job Objects",
	})
	probes = append(probes, ProbeResult{
		Name:   "network",
		Status: "skipped",
		Error:  "not supported by Job Objects",
	})

	return probes
}

// probeProcessSpawnWindows tests whether spawning a child process is blocked.
func probeProcessSpawnWindows() ProbeResult {
	p, err := os.StartProcess("cmd.exe", []string{"cmd.exe"}, &os.ProcAttr{})
	if err != nil {
		return ProbeResult{Name: "process_spawn", Status: "blocked", Target: "cmd.exe"}
	}
	_, _ = p.Wait()
	return ProbeResult{Name: "process_spawn", Status: "failed", Target: "cmd.exe",
		Error: "sandbox did not block process spawn"}
}
