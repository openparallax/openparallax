package sandbox

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// CanaryResult holds the outcome of a sandbox canary probe.
type CanaryResult struct {
	// Verified is true if the sandbox was confirmed active via canary probe.
	Verified bool `json:"verified"`
	// Status is "sandboxed", "unsandboxed", or "inconclusive".
	Status string `json:"status"`
	// CanaryPath is the file path that was probed.
	CanaryPath string `json:"canary_path"`
	// CanaryBlocked is true if the canary open was denied.
	CanaryBlocked bool `json:"canary_blocked"`
	// Platform is the runtime OS.
	Platform string `json:"platform"`
	// Mechanism is the sandbox type (landlock, sandbox-exec, job-object, none).
	Mechanism string `json:"mechanism"`
	// Error is set for inconclusive results.
	Error string `json:"error,omitempty"`
	// Timestamp is when the probe ran.
	Timestamp time.Time `json:"timestamp"`
}

// VerifyCanary runs a canary probe to verify the sandbox is actually applied.
// It attempts to open a system file that should be blocked by the sandbox.
// Returns the probe result. This must be called AFTER ApplySelf and BEFORE
// any gRPC connections or LLM calls.
func VerifyCanary() CanaryResult {
	path := canaryPath()
	mechanism := New().Mode()

	f, err := os.Open(path)
	if err != nil {
		if os.IsPermission(err) {
			// Permission denied = sandbox is active and working.
			return CanaryResult{
				Verified:      true,
				Status:        "sandboxed",
				CanaryPath:    path,
				CanaryBlocked: true,
				Platform:      runtime.GOOS,
				Mechanism:     mechanism,
				Timestamp:     time.Now(),
			}
		}
		// Other error (file not found, etc.) = inconclusive.
		return CanaryResult{
			Verified:   false,
			Status:     "inconclusive",
			CanaryPath: path,
			Platform:   runtime.GOOS,
			Mechanism:  mechanism,
			Error:      fmt.Sprintf("canary probe inconclusive: %s", err),
			Timestamp:  time.Now(),
		}
	}
	// Open succeeded = sandbox is NOT active.
	_ = f.Close()
	return CanaryResult{
		Verified:      false,
		Status:        "unsandboxed",
		CanaryPath:    path,
		CanaryBlocked: false,
		Platform:      runtime.GOOS,
		Mechanism:     mechanism,
		Timestamp:     time.Now(),
	}
}

// canaryPath returns a system file path that should always exist and should be
// blocked by the sandbox. Platform-specific.
func canaryPath() string {
	switch runtime.GOOS {
	case "windows":
		root := os.Getenv("SYSTEMROOT")
		if root == "" {
			root = `C:\Windows`
		}
		return filepath.Join(root, "System32", "config", "SAM")
	default: // linux, darwin
		return "/etc/passwd"
	}
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
