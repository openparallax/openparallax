# Linux: Landlock LSM

Landlock is a Linux Security Module that allows unprivileged processes to restrict their own access rights. It has been available since kernel 5.13 (July 2021) and requires no root privileges, no CGo, and no external tools.

## How Landlock Works

Landlock uses a ruleset model. A process creates a ruleset, adds rules specifying what access is allowed, and then enforces the ruleset. Once enforced, restrictions are irreversible for the lifetime of the process and all its descendants.

The key properties that make Landlock ideal for agent sandboxing:

- **Unprivileged**: No root, no capabilities, no special permissions required
- **Irreversible**: Once applied, the process cannot remove or weaken restrictions
- **Inheritable**: Child processes inherit the same restrictions
- **Stackable**: Multiple rulesets can be applied, each adding further restrictions

## Kernel Version Requirements

| Landlock ABI Version | Kernel Version | Capabilities |
|---|---|---|
| v1 | 5.13+ | Filesystem access control |
| v2 | 5.19+ | File renaming/linking restrictions |
| v3 | 6.2+ | Truncation restrictions |
| v4 | 6.7+ | TCP network access control |

OpenParallax detects the Landlock ABI version at runtime and uses available features. Filesystem isolation is available on all Landlock-capable kernels. Network isolation requires kernel 6.7+ (ABI v4).

## Implementation

OpenParallax uses the `github.com/shoenig/go-landlock` library for a pure Go Landlock interface.

### Default Allowed Paths

Every sandbox configuration automatically includes:

| Rule | Purpose |
|---|---|
| `landlock.Shared()` | Dynamic linker, shared libraries |
| `landlock.Stdio()` | `/dev/null`, `/dev/zero`, `/dev/urandom`, `/proc/self/cmdline` |
| `landlock.TTY()` | `/dev/tty`, terminfo (required for the bubbletea TUI) |
| `landlock.DNS()` | `/etc/hosts`, `/etc/resolv.conf` (for localhost resolution) |

These are required for the process to function at all. They cannot be removed.

### Custom Path Rules

Additional paths from `Config.AllowedReadPaths` and `Config.AllowedWritePaths` are mapped to Landlock rules:

- **Directories** get `landlock.Dir(path, "r")` or `landlock.Dir(path, "rw")`
- **Files** get `landlock.File(path, "r")` or `landlock.File(path, "rw")`
- **Non-existent paths** are silently skipped (the path must exist at sandbox application time)

### Enforcement

```go
locker := landlock.New(paths...)
err := locker.Lock(landlock.OnlyAvailable)
```

The `OnlyAvailable` flag means: enforce restrictions when Landlock is detected, silently skip on older kernels. An error is returned only when Landlock should work but the locking call itself fails.

## Self-Sandboxing

On Linux, the Agent self-sandboxes. The Engine does not wrap the Agent spawn with any external tool. Instead, the Agent calls `ApplySelf()` early in its startup sequence, before making any gRPC calls or processing user input.

```go
sb := sandbox.New()
if sb.Available() {
    err := sb.ApplySelf(sandbox.Config{
        AllowedReadPaths:  []string{workspace, configDir},
        AllowedWritePaths: []string{dotDir},
        AllowedTCPConnect: []string{engineAddr},
    })
    if err != nil {
        log.Fatalf("sandbox: %s", err)
    }
}
```

After `ApplySelf` returns successfully, the Agent process is permanently restricted. Even if a prompt injection attack gains arbitrary code execution within the Agent, the code is constrained by Landlock rules enforced by the kernel.

## Capabilities Restricted

Once the sandbox is applied, the Agent cannot:

| Operation | Result |
|---|---|
| Read `/etc/shadow` | Permission denied |
| Read `~/.ssh/id_rsa` | Permission denied |
| Write to `/tmp` | Permission denied |
| Write to `/etc/` | Permission denied |
| Connect to arbitrary hosts (v4+) | Connection refused |
| Read files outside workspace | Permission denied |

The Agent can:

| Operation | Result |
|---|---|
| Read workspace files | Allowed |
| Read shared libraries | Allowed |
| Read `/dev/tty` (for TUI) | Allowed |
| Connect to Engine gRPC | Allowed |
| Connect to LLM API | Allowed |

## Network Restriction (ABI v4+)

On kernel 6.7+ with Landlock ABI v4, TCP connections are restricted. The sandbox configuration specifies allowed `host:port` pairs. The Agent typically needs:

- The Engine's gRPC address (e.g., `127.0.0.1:50051`)
- The LLM API host (e.g., `api.anthropic.com:443`)

All other outbound connections are blocked by the kernel. This prevents data exfiltration to attacker-controlled servers even if the Agent is fully compromised.

On older kernels (pre-6.7), network restriction is not available and the canary network probe is skipped.

## Verifying Landlock is Active

```go
version, err := landlock.Detect()
if err != nil {
    fmt.Printf("Landlock not available: %s\n", err)
} else {
    fmt.Printf("Landlock v%d available\n", version)
}
```

The `openparallax doctor` command reports the Landlock version and capabilities:

```
Sandbox:      landlock v4 (filesystem + network)
```

## Limitations

- **Path existence**: Paths must exist at sandbox application time. Paths created after `ApplySelf()` are not covered by read/write rules unless they are inside an allowed directory.
- **No network restriction on pre-6.7 kernels**: Only filesystem isolation is available on kernels 5.13 through 6.6.
- **No UDP restriction**: Landlock v4 restricts TCP connections only. UDP traffic is not restricted.
- **Containers**: Landlock works inside containers (Docker, Podman) as long as the kernel supports it. No additional container configuration is required.
