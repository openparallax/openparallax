//go:build linux && arm64

package sandbox

import "golang.org/x/sys/unix"

const seccompAuditArch = unix.AUDIT_ARCH_AARCH64

// blockedSpawnSyscalls are syscalls that create new processes on arm64.
// arm64 has no fork/vfork — it uses clone exclusively.
//
// clone3 is NOT blocked: classic BPF cannot dereference clone3's struct
// pointer to inspect flags, and Go 1.22+ routes thread creation through
// clone3 by default. Blocking it unconditionally causes the runtime to
// SIGABRT the moment it needs another OS thread. The defense against
// process spawn is enforced by blocking execve/execveat — even if a child
// were spawned via clone3, it cannot load any code we did not start with.
var blockedSpawnSyscalls = []uint32{
	unix.SYS_EXECVE,
	unix.SYS_EXECVEAT,
}
