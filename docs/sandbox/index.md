# Sandbox

Sandbox provides kernel-level process isolation for AI agents using platform-native mechanisms. On Linux it uses Landlock LSM, on macOS sandbox-exec, on Windows Job Objects. All implementations are pure Go with zero CGo.

The sandbox restricts what the Agent process can do at the operating system level: which files it can read, which files it can write, which network connections it can make, and whether it can spawn child processes. These restrictions are enforced by the kernel, not by application-level checks. A compromised agent cannot bypass them.

## Why Sandbox Exists

Shield evaluates every tool call and blocks dangerous actions. But Shield is software running in the same process as the agent. If an attacker finds a way to bypass Shield (e.g., by exploiting a bug in the evaluation logic, manipulating memory, or calling system calls directly), the agent has unrestricted access to the operating system.

Sandbox is defense in depth. Even if Shield is completely bypassed, the kernel prevents the agent from:

- Reading files outside its workspace
- Writing files outside its workspace
- Making network connections to arbitrary hosts
- Spawning child processes

These restrictions are enforced by the operating system kernel. They are not bypassable from userspace.

## Platform Implementations

| Platform | Mechanism | Filesystem | Network | Process Spawn | Method |
|---|---|---|---|---|---|
| Linux 5.13+ | Landlock LSM | Restricted | Restricted (v4+, kernel 6.7+) | Restricted | Self-sandbox via `ApplySelf()` |
| macOS | sandbox-exec | Restricted | Restricted | Restricted | Engine wraps spawn via `WrapCommand()` |
| Windows | Job Objects | Not restricted | Not restricted | Restricted | Engine wraps spawn via `WrapCommand()` |

### Linux: Landlock LSM

The strongest sandbox. Landlock is a Linux Security Module available since kernel 5.13. It allows unprivileged processes to restrict themselves. The Agent calls `ApplySelf()` on startup, and the restrictions are irreversible for the lifetime of the process. No root required. No CGo. Kernel 6.7+ adds network restriction (Landlock ABI v4).

### macOS: sandbox-exec

The Engine generates a Seatbelt profile and spawns the Agent inside `sandbox-exec`. The profile denies all access by default, then explicitly allows reading system libraries, TLS certificates, and the workspace. Network access is limited to the Engine's gRPC address and the LLM API host.

### Windows: Job Objects

The Engine creates a Job Object that prevents the Agent from spawning child processes. Filesystem and network restrictions are not available through Job Objects without admin elevation. This is the weakest sandbox of the three platforms.

## Best-Effort Design

Sandbox availability depends on the platform and kernel version. The agent never fails to start because of sandboxing. If the sandbox mechanism is unavailable (old kernel, unsupported platform, missing `sandbox-exec`), the agent starts normally without isolation.

However, the **canary probe system** verifies whether the sandbox is actually working. After the Agent applies its sandbox, it runs a series of probes that attempt operations the sandbox should block (reading `/etc/shadow`, writing to `/tmp`, connecting to an external host). If the probes succeed when they should have been blocked, the canary result indicates a sandbox failure.

## How It Works

### Restrict Filesystem Access

The sandbox configuration specifies which paths the agent can read and write:

```go
cfg := sandbox.Config{
    AllowedReadPaths:  []string{"/workspace", "/usr/lib"},
    AllowedWritePaths: []string{"/workspace/.openparallax"},
}
```

All other filesystem access is blocked by the kernel. Attempts to read `/etc/shadow`, write to `/tmp`, or access any path outside the allowed list fail with a permission error.

### Restrict Network Access

The sandbox configuration specifies which TCP endpoints the agent can connect to:

```go
cfg := sandbox.Config{
    AllowedTCPConnect: []string{"127.0.0.1:50051", "api.anthropic.com:443"},
}
```

All other outbound connections are blocked. The agent cannot exfiltrate data to arbitrary servers.

### Restrict Process Spawning

The agent is not allowed to spawn child processes. This prevents an attacker from breaking out of the sandbox by executing a new process without restrictions.

```go
cfg := sandbox.Config{
    AllowProcessSpawn: false,
}
```

## Canary Probe Verification

After the sandbox is applied, the Agent runs canary probes to verify the restrictions are working. Each probe attempts an operation that should be blocked:

- **file_read**: Attempt to read a protected system file
- **file_write**: Attempt to write to a directory outside the workspace
- **network**: Attempt an outbound TCP connection to an external host
- **process_spawn** (Windows): Attempt to start a child process

If all probes are blocked, the sandbox is verified. If any probe succeeds, the sandbox is only partially effective or not working at all.

See [Canary Probes](canary.md) for the full specification.

## Integration with OpenParallax

The Engine spawns the Agent and configures the sandbox:

1. **Linux**: The Engine passes sandbox configuration via environment variables or gRPC. The Agent calls `ApplySelf()` on startup, restricting itself irreversibly. The Engine cannot apply Landlock to a child process from the outside.

2. **macOS**: The Engine generates a Seatbelt profile with the allowed paths and network endpoints, writes it to a temp file, and spawns the Agent via `sandbox-exec -f profile.sb ./agent`. The Agent runs inside the sandbox from its first instruction.

3. **Windows**: The Engine creates a Job Object with `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` and `JOB_OBJECT_LIMIT_ACTIVE_PROCESS` (set to 1). After spawning the Agent, the Engine assigns the process to the Job Object via `PostStart()`.

## Status Reporting

The sandbox status is available through:

- `GET /api/status` includes a `sandbox` field with the current state
- `openparallax doctor` reports sandbox capabilities and verification results
- The canary result is written to `.openparallax/sandbox.status` as JSON
