//go:build windows

package sandbox

import (
	"fmt"
	"os/exec"
	"unsafe"

	"golang.org/x/sys/windows"
)

// windowsSandbox uses Windows Job Objects to limit the Agent process.
// No admin privileges required. Pure Go via golang.org/x/sys/windows.
//
// Limitations compared to Linux/macOS:
//   - No per-process filesystem restriction without admin
//   - No per-process network restriction without admin (WFP requires elevation)
//   - Primary restrictions: no child processes, kill-on-job-close
type windowsSandbox struct {
	job windows.Handle
}

// New returns the Windows Job Object sandbox implementation.
func New() Sandbox { return &windowsSandbox{} }

// Available always returns true on Windows; Job Objects are always available.
func (s *windowsSandbox) Available() bool { return true }

// Mode returns "job-object".
func (s *windowsSandbox) Mode() string { return "job-object" }

// ApplySelf is unused on Windows; the Engine wraps the spawn via WrapCommand.
func (s *windowsSandbox) ApplySelf(_ Config) error { return nil }

// WrapCommand creates a Job Object and stores it for assignment after process start.
// The caller must invoke PostStart after cmd.Start() to assign the process.
func (s *windowsSandbox) WrapCommand(cmd *exec.Cmd, cfg Config) error {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return fmt.Errorf("create job object: %w", err)
	}

	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags =
		windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE |
			windows.JOB_OBJECT_LIMIT_ACTIVE_PROCESS
	info.BasicLimitInformation.ActiveProcessLimit = 1

	_, err = windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	)
	if err != nil {
		_ = windows.CloseHandle(job)
		return fmt.Errorf("set job limits: %w", err)
	}

	s.job = job
	return nil
}

// PostStart assigns the running process to the Job Object.
// Must be called after cmd.Start().
func (s *windowsSandbox) PostStart(pid int) error {
	if s.job == 0 {
		return nil
	}

	handle, err := windows.OpenProcess(
		windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE,
		false,
		uint32(pid),
	)
	if err != nil {
		return fmt.Errorf("open process: %w", err)
	}
	defer func() { _ = windows.CloseHandle(handle) }()

	return windows.AssignProcessToJobObject(s.job, handle)
}

// Close releases the Job Object handle.
func (s *windowsSandbox) Close() {
	if s.job != 0 {
		_ = windows.CloseHandle(s.job)
		s.job = 0
	}
}

func unavailableReason() string {
	return "Windows Job Objects are always available"
}

func probeStatus(_ Sandbox) Status {
	return Status{
		Active:     true,
		Mode:       "job-object",
		Filesystem: false,
		Network:    false,
	}
}
