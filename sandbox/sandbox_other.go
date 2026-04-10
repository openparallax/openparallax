//go:build !linux && !darwin && !windows

package sandbox

import "os/exec"

// noopSandbox provides no kernel-level isolation.
// Used on platforms without supported sandboxing mechanisms.
type noopSandbox struct{}

// New returns a no-op sandbox for unsupported platforms.
func New() Sandbox { return &noopSandbox{} }

// Available always returns false.
func (s *noopSandbox) Available() bool { return false }

// Mode returns "none".
func (s *noopSandbox) Mode() string { return "none" }

// ApplySelf is a no-op.
func (s *noopSandbox) ApplySelf(_ Config) error { return nil }

// WrapCommand is a no-op.
func (s *noopSandbox) WrapCommand(_ *exec.Cmd, _ Config) error { return nil }

func unavailableReason() string {
	return "no supported sandbox mechanism on this platform"
}

func probeStatus(_ Sandbox) Status {
	return Status{Active: false, Mode: "none", Reason: unavailableReason()}
}
