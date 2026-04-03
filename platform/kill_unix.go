//go:build !windows

package platform

import "syscall"

// killProcessTreePlatform kills a process group on Unix.
// A negative PID sends the signal to the entire process group.
func killProcessTreePlatform(pid int) error {
	return syscall.Kill(-pid, syscall.SIGKILL)
}
