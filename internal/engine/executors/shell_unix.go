//go:build !windows

package executors

import (
	"os/exec"
	"syscall"
)

// setProcGroup configures the command to run in its own process group on Unix.
// This allows killing the entire process tree on timeout via negative PID signal.
func setProcGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}
