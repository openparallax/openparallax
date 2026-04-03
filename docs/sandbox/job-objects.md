# Windows: Job Objects

On Windows, OpenParallax uses Job Objects to restrict the Agent process. Job Objects are a Windows kernel mechanism for grouping processes and applying resource limits. No administrator privileges are required.

## How Job Objects Work

A Job Object is a kernel object that can be associated with one or more processes. Limits set on the Job Object apply to all processes assigned to it. The limits are enforced by the kernel and cannot be bypassed by the managed processes.

OpenParallax uses two specific Job Object limits:

- **`JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`**: When the Job Object handle is closed (e.g., when the Engine exits), all processes in the job are terminated. This ensures the Agent cannot outlive the Engine.
- **`JOB_OBJECT_LIMIT_ACTIVE_PROCESS`**: Limits the number of active processes in the job to 1. This prevents the Agent from spawning child processes.

## Implementation

The Windows sandbox implementation uses `golang.org/x/sys/windows` for pure Go access to the Windows API. No CGo.

### WrapCommand

```go
func (s *windowsSandbox) WrapCommand(cmd *exec.Cmd, cfg Config) error
```

Creates a Job Object and configures it with the limits described above. The Job Object handle is stored on the sandbox struct for later assignment.

### PostStart

```go
func (s *windowsSandbox) PostStart(pid int) error
```

Assigns the running process to the Job Object. This must be called after `cmd.Start()` because the process must be running before it can be assigned.

```go
sb := sandbox.New()
ws := sb.(*windowsSandbox)

cmd := exec.Command("./agent.exe", "--workspace", "C:\\workspace")
_ = sb.WrapCommand(cmd, sandbox.Config{})
_ = cmd.Start()
_ = ws.PostStart(cmd.Process.Pid)

// When done:
defer ws.Close()
```

### Close

```go
func (s *windowsSandbox) Close()
```

Releases the Job Object handle. If `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` is set, this also terminates the Agent process.

## JOB_OBJECT_LIMIT Flags

| Flag | Effect |
|---|---|
| `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` | All processes in the job are killed when the Job Object handle is closed |
| `JOB_OBJECT_LIMIT_ACTIVE_PROCESS` | Limits the maximum number of active processes. Set to 1 to prevent child spawning. |

These flags are set via `SetInformationJobObject` with the `JOBOBJECT_EXTENDED_LIMIT_INFORMATION` structure.

## Current Limitations

Job Objects on Windows do not provide the same level of isolation as Landlock on Linux or sandbox-exec on macOS:

| Capability | Linux (Landlock) | macOS (sandbox-exec) | Windows (Job Objects) |
|---|---|---|---|
| Filesystem restriction | Yes | Yes | **No** |
| Network restriction | Yes (kernel 6.7+) | Yes | **No** |
| Process spawn restriction | Yes | Yes | Yes |
| Kill on parent exit | Inherent (prctl) | Inherent | Yes (via flag) |

### No Filesystem Restriction

Job Objects cannot restrict which files a process can read or write. Windows provides other mechanisms for this (e.g., file system minifilters, integrity levels), but they require administrator privileges or kernel-mode drivers. OpenParallax does not use these because it must run without elevation.

### No Network Restriction

Job Objects cannot restrict network access. Windows Filtering Platform (WFP) can restrict per-process network access, but it requires administrator elevation. OpenParallax does not use WFP.

### Implications

On Windows, the Agent relies entirely on Shield (the application-level security pipeline) for access control. The kernel sandbox only prevents the Agent from spawning child processes and ensures it is terminated when the Engine exits.

The canary probe system reflects this: on Windows, only the `process_spawn` probe runs. File read, file write, and network probes are skipped because Job Objects cannot enforce these restrictions.

## Canary Probes

On Windows, a single probe runs:

- **process_spawn**: Attempts to start `cmd.exe` as a child process. The Job Object's active process limit of 1 should block this.

The remaining probes are skipped:

```json
{
  "probes": [
    {"name": "process_spawn", "status": "blocked", "target": "cmd.exe"},
    {"name": "file_read", "status": "skipped", "error": "not supported by Job Objects"},
    {"name": "file_write", "status": "skipped", "error": "not supported by Job Objects"},
    {"name": "network", "status": "skipped", "error": "not supported by Job Objects"}
  ]
}
```

## Availability

Job Objects are available on all supported Windows versions (Windows 10+, Windows Server 2016+). The `Available()` method always returns `true` on Windows.
