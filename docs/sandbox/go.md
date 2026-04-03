# Sandbox Go API

The sandbox package provides kernel-level process isolation using platform-native mechanisms. All implementations are pure Go with zero CGo. Build tags select the correct implementation at compile time.

Package: `github.com/openparallax/openparallax/internal/sandbox`

## Sandbox Interface

```go
type Sandbox interface {
    Available() bool
    Mode() string
    ApplySelf(cfg Config) error
    WrapCommand(cmd *exec.Cmd, cfg Config) error
}
```

### Available

```go
func (s *Sandbox) Available() bool
```

Reports whether the sandbox mechanism is supported on the current system. On Linux, this checks if Landlock is available in the kernel. On macOS, this checks if `/usr/bin/sandbox-exec` exists. On Windows, this always returns `true` (Job Objects are always available).

### Mode

```go
func (s *Sandbox) Mode() string
```

Returns the sandbox mechanism name:

- `"landlock"` on Linux
- `"sandbox-exec"` on macOS
- `"job-object"` on Windows
- `"none"` on unsupported platforms

### ApplySelf

```go
func (s *Sandbox) ApplySelf(cfg Config) error
```

Restricts the current process. Once applied, restrictions are irreversible for the lifetime of the process. Used by the Agent on Linux to self-sandbox via Landlock.

On macOS and Windows, `ApplySelf` is a no-op. These platforms sandbox via `WrapCommand` instead, because the parent process wraps the child spawn.

```go
sb := sandbox.New()
err := sb.ApplySelf(sandbox.Config{
    AllowedReadPaths:  []string{"/workspace"},
    AllowedWritePaths: []string{"/workspace/.openparallax"},
    AllowedTCPConnect: []string{"127.0.0.1:50051"},
})
```

### WrapCommand

```go
func (s *Sandbox) WrapCommand(cmd *exec.Cmd, cfg Config) error
```

Modifies `cmd` to run inside a sandbox. Used by the Engine on macOS and Windows to spawn a sandboxed Agent.

On macOS, this rewrites `cmd.Path` to `/usr/bin/sandbox-exec`, generates a Seatbelt profile, writes it to a temp file, and prepends the `-f profile.sb` arguments.

On Windows, this creates a Job Object with process spawn limits. The caller must call `PostStart(pid)` after `cmd.Start()` to assign the process to the Job Object.

On Linux, `WrapCommand` is a no-op. The Agent self-sandboxes via `ApplySelf`.

```go
cmd := exec.Command("./agent", "--workspace", "/workspace")
sb := sandbox.New()
err := sb.WrapCommand(cmd, sandbox.Config{
    AllowedReadPaths:  []string{"/workspace"},
    AllowedTCPConnect: []string{"127.0.0.1:50051"},
})
if err != nil {
    return err
}
err = cmd.Start()
```

## New

```go
func New() Sandbox
```

Returns the platform-appropriate sandbox implementation. Selected at compile time via build tags (`sandbox_linux.go`, `sandbox_darwin.go`, `sandbox_windows.go`, `sandbox_other.go`).

## Config

```go
type Config struct {
    AllowedReadPaths  []string
    AllowedWritePaths []string
    AllowedTCPConnect []string
    AllowProcessSpawn bool
}
```

| Field | Description |
|---|---|
| `AllowedReadPaths` | Filesystem paths the process can read. Empty means no filesystem read access beyond system libraries. Directories are recursive. |
| `AllowedWritePaths` | Filesystem paths the process can write. Typically empty for the Agent (or limited to the `.openparallax` directory). |
| `AllowedTCPConnect` | `host:port` pairs the process can connect to. For the Agent: only the Engine's gRPC address and the LLM API host. |
| `AllowProcessSpawn` | Whether the process can create children. `false` for the Agent. |

System paths (shared libraries, dynamic linker, `/dev/null`, `/dev/urandom`, TTY, DNS resolution files) are always allowed regardless of the configuration.

## Status

```go
type Status struct {
    Active     bool   `json:"active"`
    Mode       string `json:"mode"`
    Version    int    `json:"version,omitempty"`
    Filesystem bool   `json:"filesystem"`
    Network    bool   `json:"network"`
    Reason     string `json:"reason,omitempty"`
}
```

Reports the current sandbox state. Used by `GET /api/status` and `openparallax doctor`.

| Field | Description |
|---|---|
| `Active` | Whether the sandbox mechanism is available |
| `Mode` | Mechanism name: `"landlock"`, `"sandbox-exec"`, `"job-object"`, `"none"` |
| `Version` | Landlock ABI version (Linux only) |
| `Filesystem` | Whether filesystem isolation is supported |
| `Network` | Whether network isolation is supported |
| `Reason` | Explanation when the sandbox is unavailable |

## Probe

```go
func Probe() Status
```

Checks what sandbox mechanism is available on this platform and returns the expected status. Does not apply any sandbox. The Engine uses this to report what the Agent should have.

```go
status := sandbox.Probe()
if status.Active {
    fmt.Printf("Sandbox available: %s (fs=%v, net=%v)\n",
        status.Mode, status.Filesystem, status.Network)
} else {
    fmt.Printf("Sandbox unavailable: %s\n", status.Reason)
}
```

## Platform Detection

The correct implementation is selected at compile time via Go build tags:

| File | Build Tag | Platform |
|---|---|---|
| `sandbox_linux.go` | `//go:build linux` | Linux (Landlock) |
| `sandbox_darwin.go` | `//go:build darwin` | macOS (sandbox-exec) |
| `sandbox_windows.go` | `//go:build windows` | Windows (Job Objects) |
| `sandbox_other.go` | `//go:build !linux && !darwin && !windows` | Unsupported (no-op) |

The canary probes have matching platform-specific files:

| File | Build Tag | Platform |
|---|---|---|
| `canary_linux.go` | `//go:build linux` | Tests file_read, file_write, network |
| `canary_darwin.go` | `//go:build darwin` | Tests file_read, file_write, network |
| `canary_windows.go` | `//go:build windows` | Tests process_spawn |
| `canary_other.go` | `//go:build !linux && !darwin && !windows` | No probes |

## Windows PostStart

On Windows, the Job Object must be assigned to the process after it starts:

```go
sb := sandbox.New()
ws, ok := sb.(*windowsSandbox)
if !ok {
    return
}

cmd := exec.Command("./agent")
_ = sb.WrapCommand(cmd, cfg)
_ = cmd.Start()

// Assign the running process to the Job Object.
err := ws.PostStart(cmd.Process.Pid)

// Close releases the Job Object when done.
defer ws.Close()
```
