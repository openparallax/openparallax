//go:build linux && amd64

package sandbox

import "golang.org/x/sys/unix"

const seccompAuditArch = unix.AUDIT_ARCH_X86_64

// blockedSpawnSyscalls are syscalls that create new processes on amd64.
// clone is NOT blocked because Go's runtime uses it for goroutines (with CLONE_THREAD).
// We block clone only when CLONE_THREAD is not set (see seccomp filter).
var blockedSpawnSyscalls = []uint32{
	unix.SYS_FORK,
	unix.SYS_VFORK,
	unix.SYS_EXECVE,
	unix.SYS_EXECVEAT,
	unix.SYS_CLONE3,
}
