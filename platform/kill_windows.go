//go:build windows

package platform

import (
	"fmt"
	"os/exec"
)

// killProcessTreePlatform kills a process tree on Windows.
// taskkill /T terminates the process and all child processes.
// taskkill /F forces termination without cleanup.
func killProcessTreePlatform(pid int) error {
	cmd := exec.Command("taskkill", "/T", "/F", "/PID", fmt.Sprintf("%d", pid))
	return cmd.Run()
}
