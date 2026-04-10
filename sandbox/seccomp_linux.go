//go:build linux

package sandbox

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/unix"
)

// applySeccompSpawnBlock installs a seccomp-bpf filter that blocks process
// creation syscalls (fork, vfork, execve, execveat, clone3). The clone syscall
// is allowed when CLONE_THREAD is set (Go runtime needs this for goroutines)
// but blocked otherwise (which would create a new process).
//
// The filter returns EPERM for blocked syscalls rather than killing the process,
// so callers get a clean error instead of a crash.
func applySeccompSpawnBlock() error {
	// PR_SET_NO_NEW_PRIVS is required before installing a seccomp filter
	// without CAP_SYS_ADMIN.
	if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
		return fmt.Errorf("prctl NO_NEW_PRIVS: %w", err)
	}

	filter := buildSpawnFilter()
	prog := unix.SockFprog{
		Len:    uint16(len(filter)),
		Filter: &filter[0],
	}

	if _, _, errno := unix.Syscall(
		unix.SYS_SECCOMP,
		uintptr(unix.SECCOMP_SET_MODE_FILTER),
		0,
		uintptr(unsafe.Pointer(&prog)),
	); errno != 0 {
		return fmt.Errorf("seccomp: %w", errno)
	}

	return nil
}

// buildSpawnFilter constructs a BPF program that blocks process creation.
//
// Filter logic:
//  1. Verify architecture matches (reject mismatched syscall ABIs)
//  2. Load syscall number
//  3. For each blocked syscall: if match → return EPERM
//  4. For clone: check flags argument — allow if CLONE_THREAD is set
//  5. Default: allow
func buildSpawnFilter() []unix.SockFilter {
	var insns []unix.SockFilter

	// Load architecture from seccomp_data.arch (offset 4).
	insns = append(insns, bpfStmt(unix.BPF_LD|unix.BPF_W|unix.BPF_ABS, 4))
	// If arch doesn't match, skip to ALLOW (permissive on unknown arch).
	insns = append(insns, bpfJump(unix.BPF_JMP|unix.BPF_JEQ|unix.BPF_K, seccompAuditArch, 1, 0))
	insns = append(insns, bpfStmt(unix.BPF_RET|unix.BPF_K, unix.SECCOMP_RET_ALLOW))

	// Load syscall number from seccomp_data.nr (offset 0).
	insns = append(insns, bpfStmt(unix.BPF_LD|unix.BPF_W|unix.BPF_ABS, 0))

	// Block each spawn syscall unconditionally.
	for _, nr := range blockedSpawnSyscalls {
		insns = append(insns, bpfJump(unix.BPF_JMP|unix.BPF_JEQ|unix.BPF_K, nr, 0, 1))
		insns = append(insns, bpfStmt(unix.BPF_RET|unix.BPF_K, unix.SECCOMP_RET_ERRNO|1)) // EPERM
	}

	// clone: allow if CLONE_THREAD is set (Go goroutines), block otherwise.
	// clone's first argument (flags) is in seccomp_data.args[0] (offset 16).
	insns = append(insns, bpfJump(unix.BPF_JMP|unix.BPF_JEQ|unix.BPF_K, unix.SYS_CLONE, 0, 3))
	insns = append(insns, bpfStmt(unix.BPF_LD|unix.BPF_W|unix.BPF_ABS, 16))                        // load args[0]
	insns = append(insns, bpfJump(unix.BPF_JMP|unix.BPF_JSET|unix.BPF_K, unix.CLONE_THREAD, 1, 0)) // CLONE_THREAD set?
	insns = append(insns, bpfStmt(unix.BPF_RET|unix.BPF_K, unix.SECCOMP_RET_ERRNO|1))              // no → EPERM

	// Default: allow.
	insns = append(insns, bpfStmt(unix.BPF_RET|unix.BPF_K, unix.SECCOMP_RET_ALLOW))

	return insns
}

func bpfStmt(code uint16, k uint32) unix.SockFilter {
	return unix.SockFilter{Code: code, K: k}
}

func bpfJump(code uint16, k uint32, jt, jf uint8) unix.SockFilter {
	return unix.SockFilter{Code: code, Jt: jt, Jf: jf, K: k}
}
