//go:build darwin

package sandbox

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const profileTemplate = `(version 1)
(deny default)

; Allow reading the agent binary and system shared libraries
(allow file-read*
    (subpath "/usr/lib")
    (subpath "/System/Library")
    (subpath "/Library/Frameworks")
    (subpath "/private/etc/hosts")
    (subpath "/private/etc/resolv.conf")
    (literal "/dev/tty")
    (literal "/dev/null")
    (literal "/dev/urandom")
    (literal "/dev/stdin")
    (literal "/dev/stdout")
    (literal "/dev/stderr")
    (subpath "/usr/share/terminfo")
    (subpath "/usr/lib/terminfo")
    %EXTRA_READ%)

; Allow writing to tty and stdio
(allow file-write*
    (literal "/dev/tty")
    (literal "/dev/null")
    (literal "/dev/stdin")
    (literal "/dev/stdout")
    (literal "/dev/stderr"))

; Allow network connection to engine only
(allow network-outbound
    (remote tcp "%CONNECT_ADDR%"))

; Allow basic process operations
(allow process-exec (literal "%AGENT_BINARY%"))
(allow sysctl-read)
(allow mach-lookup
    (global-name "com.apple.system.logger")
    (global-name "com.apple.system.notification_center"))
`

// darwinSandbox uses macOS sandbox-exec to restrict the Agent process.
// sandbox-exec is deprecated by Apple but functional on all current macOS versions.
type darwinSandbox struct{}

// New returns the macOS sandbox-exec implementation.
func New() Sandbox { return &darwinSandbox{} }

// Available reports whether sandbox-exec is installed.
func (s *darwinSandbox) Available() bool {
	_, err := os.Stat("/usr/bin/sandbox-exec")
	return err == nil
}

// Mode returns "sandbox-exec".
func (s *darwinSandbox) Mode() string { return "sandbox-exec" }

// ApplySelf is unused on macOS; the Engine wraps the spawn via WrapCommand.
func (s *darwinSandbox) ApplySelf(_ Config) error { return nil }

// WrapCommand modifies cmd to run inside sandbox-exec.
func (s *darwinSandbox) WrapCommand(cmd *exec.Cmd, cfg Config) error {
	profile := generateProfile(cmd.Path, cfg)

	f, err := os.CreateTemp("", "openparallax-sandbox-*.sb")
	if err != nil {
		return fmt.Errorf("create sandbox profile: %w", err)
	}
	if _, err := f.WriteString(profile); err != nil {
		_ = f.Close()
		return fmt.Errorf("write sandbox profile: %w", err)
	}
	_ = f.Close()
	profilePath := f.Name()

	originalPath := cmd.Path
	originalArgs := cmd.Args

	cmd.Path = "/usr/bin/sandbox-exec"
	cmd.Args = make([]string, 0, len(originalArgs)+3)
	cmd.Args = append(cmd.Args, "sandbox-exec", "-f", profilePath)
	cmd.Args = append(cmd.Args, originalPath)
	cmd.Args = append(cmd.Args, originalArgs[1:]...)

	return nil
}

func generateProfile(binaryPath string, cfg Config) string {
	profile := profileTemplate

	// Agent binary
	profile = strings.ReplaceAll(profile, "%AGENT_BINARY%", binaryPath)

	// Network connect address
	connectAddr := "localhost:0"
	if len(cfg.AllowedTCPConnect) > 0 {
		connectAddr = cfg.AllowedTCPConnect[0]
	}
	// sandbox-exec needs "host:port" format with numeric port
	profile = strings.ReplaceAll(profile, "%CONNECT_ADDR%", connectAddr)

	// Extra read paths
	var extraRead strings.Builder
	for _, p := range cfg.AllowedReadPaths {
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		if info.IsDir() {
			fmt.Fprintf(&extraRead, "\n    (subpath %q)", p)
		} else {
			fmt.Fprintf(&extraRead, "\n    (literal %q)", p)
		}
	}
	profile = strings.ReplaceAll(profile, "%EXTRA_READ%", extraRead.String())

	return profile
}

func unavailableReason() string {
	return "sandbox-exec not found at /usr/bin/sandbox-exec"
}

func probeStatus(_ Sandbox) Status {
	return Status{
		Active:     true,
		Mode:       "sandbox-exec",
		Filesystem: true,
		Network:    true,
	}
}

