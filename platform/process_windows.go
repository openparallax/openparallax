//go:build windows

package platform

import (
	"os/exec"
	"syscall"
)

// detachedProcess is the Windows process creation flag that prevents the
// child from inheriting the parent's console. Defined here because the
// Go standard library's syscall package on Windows does not export it as
// a named constant despite documenting it.
const detachedProcess = 0x00000008

// ApplyDaemonProcAttr detaches the spawned process from the parent
// console on Windows. CREATE_NEW_PROCESS_GROUP makes the child its own
// process group leader so it does not receive Ctrl+C from the parent
// console; DETACHED_PROCESS prevents the child from inheriting the
// parent's console at all.
func ApplyDaemonProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | detachedProcess,
	}
}
