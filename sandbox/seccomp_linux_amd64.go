//go:build linux && amd64

package sandbox

import "golang.org/x/sys/unix"

const seccompAuditArch = unix.AUDIT_ARCH_X86_64

// blockedSpawnSyscalls are syscalls that create new processes on amd64.
// clone is NOT blocked because Go's runtime uses it for goroutines (with
// CLONE_THREAD); the seccomp filter inspects clone's flags argument and only
// blocks the call when CLONE_THREAD is unset.
//
// clone3 is also NOT blocked, even though it can create both processes and
// threads. The reason: clone3 takes a struct pointer instead of flat args,
// so classic BPF cannot dereference it to inspect flags. Go 1.22+ routes
// thread creation through clone3 by default, so blocking it unconditionally
// causes the runtime to SIGABRT the moment it needs another OS thread (e.g.
// during JSON marshaling on a continuation LLM call).
//
// The architectural defense against process spawn is enforced by blocking
// execve and execveat: even if a child were spawned via clone3, it cannot
// load any code we did not start with, so it has no useful capability.
var blockedSpawnSyscalls = []uint32{
	unix.SYS_FORK,
	unix.SYS_VFORK,
	unix.SYS_EXECVE,
	unix.SYS_EXECVEAT,
}
