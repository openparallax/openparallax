//go:build linux

package sandbox

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/shoenig/go-landlock"
)

// linuxSandbox uses Landlock LSM to restrict the Agent process.
// Landlock is available since kernel 5.13 (July 2021).
// No root required. No CGo. Restrictions are irreversible.
type linuxSandbox struct{}

// New returns the Linux Landlock sandbox implementation.
func New() Sandbox { return &linuxSandbox{} }

// Available reports whether Landlock is supported on this kernel.
func (s *linuxSandbox) Available() bool { return landlock.Available() }

// Mode returns "landlock".
func (s *linuxSandbox) Mode() string { return "landlock" }

// ApplySelf restricts the current process using Landlock.
// Must be called before any untrusted operations.
func (s *linuxSandbox) ApplySelf(cfg Config) error {
	paths := []*landlock.Path{
		landlock.Shared(), // dynamic linker, shared libraries
		landlock.Stdio(),  // /dev/null, /dev/zero, /dev/urandom, /proc/self/cmdline
		landlock.TTY(),    // /dev/tty, terminfo (needed for bubbletea TUI)
		landlock.DNS(),    // /etc/hosts, /etc/resolv.conf (for localhost resolution)
	}

	for _, p := range cfg.AllowedReadPaths {
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		if info.IsDir() {
			paths = append(paths, landlock.Dir(p, "r"))
		} else {
			paths = append(paths, landlock.File(p, "r"))
		}
	}

	for _, p := range cfg.AllowedWritePaths {
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		if info.IsDir() {
			paths = append(paths, landlock.Dir(p, "rw"))
		} else {
			paths = append(paths, landlock.File(p, "rw"))
		}
	}

	locker := landlock.New(paths...)

	// OnlyAvailable: enforce when Landlock is detected, silently skip on
	// older kernels or non-Linux. Returns error only when Landlock should
	// work but the locking call itself failed.
	if err := locker.Lock(landlock.OnlyAvailable); err != nil {
		return fmt.Errorf("landlock: %w", err)
	}

	return nil
}

// WrapCommand is unused on Linux; the Agent self-sandboxes via ApplySelf.
func (s *linuxSandbox) WrapCommand(_ *exec.Cmd, _ Config) error { return nil }

func unavailableReason() string {
	if _, err := landlock.Detect(); err != nil {
		return fmt.Sprintf("Landlock not supported: %s", err)
	}
	return "Landlock not available"
}

func probeStatus(_ Sandbox) Status {
	version, err := landlock.Detect()
	if err != nil {
		return Status{Active: false, Mode: "landlock", Reason: err.Error()}
	}
	return Status{
		Active:     true,
		Mode:       "landlock",
		Version:    version,
		Filesystem: true,
		Network:    version >= 4,
	}
}
