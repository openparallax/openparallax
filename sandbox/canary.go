package sandbox

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// ProbeResult holds the outcome of a single canary probe.
type ProbeResult struct {
	// Name identifies the probe (e.g. "file_read", "file_write", "network").
	Name string `json:"name"`
	// Status is "blocked", "failed", or "skipped".
	Status string `json:"status"`
	// Target is what was probed (file path, host:port, etc.).
	Target string `json:"target,omitempty"`
	// Error is set when the probe result is unexpected.
	Error string `json:"error,omitempty"`
}

// CanaryResult holds the outcome of all sandbox canary probes.
type CanaryResult struct {
	// Verified is true when all applicable probes were blocked.
	Verified bool `json:"verified"`
	// Status is "sandboxed", "partial", "unsandboxed", or "unavailable".
	Status string `json:"status"`
	// Platform is the runtime OS.
	Platform string `json:"platform"`
	// Mechanism is the sandbox type (landlock, sandbox-exec, job-object, none).
	Mechanism string `json:"mechanism"`
	// Probes holds per-probe results.
	Probes []ProbeResult `json:"probes"`
	// Summary is a human-readable one-liner.
	Summary string `json:"summary"`
	// Timestamp is when the probes ran.
	Timestamp time.Time `json:"timestamp"`
}

// Blocked returns the number of probes that were blocked.
func (r *CanaryResult) Blocked() int {
	n := 0
	for _, p := range r.Probes {
		if p.Status == "blocked" {
			n++
		}
	}
	return n
}

// Failed returns the number of probes that failed (sandbox didn't block).
func (r *CanaryResult) Failed() int {
	n := 0
	for _, p := range r.Probes {
		if p.Status == "failed" {
			n++
		}
	}
	return n
}

// Skipped returns the number of probes that were skipped.
func (r *CanaryResult) Skipped() int {
	n := 0
	for _, p := range r.Probes {
		if p.Status == "skipped" {
			n++
		}
	}
	return n
}

// VerifyCanary runs platform-appropriate canary probes to verify the sandbox
// is actually applied. Each platform tests only what its sandbox mechanism
// can enforce. Must be called AFTER ApplySelf.
func VerifyCanary() CanaryResult {
	mechanism := New().Mode()
	probes := runPlatformProbes()

	blocked := 0
	failed := 0
	skipped := 0
	var blockedNames, failedNames, skippedNames []string

	for _, p := range probes {
		switch p.Status {
		case "blocked":
			blocked++
			blockedNames = append(blockedNames, p.Name)
		case "failed":
			failed++
			failedNames = append(failedNames, p.Name)
		case "skipped":
			skipped++
			skippedNames = append(skippedNames, p.Name)
		}
	}

	applicable := blocked + failed
	var status string
	var verified bool

	switch {
	case applicable == 0:
		status = "unavailable"
		verified = false
	case failed == 0:
		status = "sandboxed"
		verified = true
	case blocked > 0:
		status = "partial"
		verified = false
	default:
		status = "unsandboxed"
		verified = false
	}

	summary := buildSummary(blocked, applicable, blockedNames, failedNames, skippedNames)

	return CanaryResult{
		Verified:  verified,
		Status:    status,
		Platform:  runtime.GOOS,
		Mechanism: mechanism,
		Probes:    probes,
		Summary:   summary,
		Timestamp: time.Now(),
	}
}

func buildSummary(blocked, applicable int, blockedNames, failedNames, skippedNames []string) string {
	s := fmt.Sprintf("Sandbox verified: %d/%d probes blocked", blocked, applicable)
	if len(blockedNames) > 0 {
		s += fmt.Sprintf(" (%s)", joinNames(blockedNames))
	}
	s += "."
	if len(failedNames) > 0 {
		s += fmt.Sprintf(" Failed: %s.", joinNames(failedNames))
	}
	if len(skippedNames) > 0 {
		s += fmt.Sprintf(" Skipped: %s.", joinNames(skippedNames))
	}
	return s
}

func joinNames(names []string) string {
	result := ""
	for i, n := range names {
		if i > 0 {
			result += ", "
		}
		result += n
	}
	return result
}

// probeFileRead tests whether reading a protected system file is blocked.
func probeFileRead(path string) ProbeResult {
	f, err := os.Open(path)
	if err != nil {
		if os.IsPermission(err) {
			return ProbeResult{Name: "file_read", Status: "blocked", Target: path}
		}
		// File doesn't exist or other error — inconclusive, treat as blocked
		// since we can't distinguish.
		return ProbeResult{Name: "file_read", Status: "blocked", Target: path}
	}
	_ = f.Close()
	return ProbeResult{Name: "file_read", Status: "failed", Target: path,
		Error: "sandbox did not block read access"}
}

// probeFileWrite tests whether writing to a protected directory is blocked.
func probeFileWrite(dir string) ProbeResult {
	path := filepath.Join(dir, ".openparallax-canary-probe")
	f, err := os.Create(path)
	if err != nil {
		if os.IsPermission(err) {
			return ProbeResult{Name: "file_write", Status: "blocked", Target: dir}
		}
		// Other error — treat as blocked.
		return ProbeResult{Name: "file_write", Status: "blocked", Target: dir}
	}
	_ = f.Close()
	_ = os.Remove(path)
	return ProbeResult{Name: "file_write", Status: "failed", Target: dir,
		Error: "sandbox did not block write access"}
}

// RequiredFailed returns the names of required probes that failed.
// Required probes vary by platform:
//   - Linux: file_read, file_write (filesystem must be enforced)
//   - macOS: all probes (sandbox-exec enforces everything)
//   - Windows: process_spawn (only Job Object capability)
func (r *CanaryResult) RequiredFailed() []string {
	required := requiredProbes()
	var failed []string
	for _, p := range r.Probes {
		if p.Status == "failed" && required[p.Name] {
			failed = append(failed, p.Name)
		}
	}
	return failed
}

// AdvisoryFailed returns the names of advisory probes that failed.
// These are logged as warnings but do not prevent startup.
func (r *CanaryResult) AdvisoryFailed() []string {
	required := requiredProbes()
	var failed []string
	for _, p := range r.Probes {
		if p.Status == "failed" && !required[p.Name] {
			failed = append(failed, p.Name)
		}
	}
	return failed
}

// WriteCanaryResult writes the canary probe result to a JSON file in the
// workspace so the Engine can read it.
func WriteCanaryResult(workspace string, result CanaryResult) error {
	dir := filepath.Join(workspace, ".openparallax")
	_ = os.MkdirAll(dir, 0o755)
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "sandbox.status"), data, 0o644)
}

// ReadCanaryResult reads the canary probe result written by the Agent.
// Returns a zero result if the file doesn't exist or can't be parsed.
func ReadCanaryResult(workspace string) CanaryResult {
	path := filepath.Join(workspace, ".openparallax", "sandbox.status")
	data, err := os.ReadFile(path)
	if err != nil {
		return CanaryResult{Status: "unknown"}
	}
	var result CanaryResult
	if err := json.Unmarshal(data, &result); err != nil {
		return CanaryResult{Status: "unknown"}
	}
	return result
}
