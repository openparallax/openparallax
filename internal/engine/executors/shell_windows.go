//go:build windows

package executors

import "os/exec"

// setProcGroup is a no-op on Windows. Process tree termination is handled
// by taskkill /T in platform.KillProcessTree.
func setProcGroup(cmd *exec.Cmd) {}
