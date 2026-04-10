//go:build !windows

package platform

import (
	"os/exec"
	"syscall"
)

// ApplyDaemonProcAttr detaches the spawned process from the controlling
// terminal so the child survives after the parent shell exits. Setsid is
// a Unix-specific syscall; the Windows variant of this file uses
// CREATE_NEW_PROCESS_GROUP + DETACHED_PROCESS to achieve the equivalent.
func ApplyDaemonProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
