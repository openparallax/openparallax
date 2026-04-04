//go:build linux && arm64

package sandbox

import "golang.org/x/sys/unix"

const seccompAuditArch = unix.AUDIT_ARCH_AARCH64

// blockedSpawnSyscalls are syscalls that create new processes on arm64.
// arm64 has no fork/vfork — it uses clone exclusively.
var blockedSpawnSyscalls = []uint32{
	unix.SYS_EXECVE,
	unix.SYS_EXECVEAT,
	unix.SYS_CLONE3,
}
