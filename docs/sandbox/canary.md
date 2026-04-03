# Canary Probe Verification

After the sandbox is applied, the Agent runs canary probes to verify the restrictions are actually working. Each probe attempts an operation that should be blocked. The results are aggregated into a `CanaryResult` that reports the overall sandbox status.

## Why Canary Probes

Applying a sandbox configuration is not the same as verifying it works. The sandbox could fail silently due to kernel bugs, misconfiguration, or race conditions. Canary probes provide empirical verification: they try the things that should be blocked and confirm they are actually blocked.

## CanaryResult

```go
type CanaryResult struct {
    Verified  bool          `json:"verified"`
    Status    string        `json:"status"`
    Platform  string        `json:"platform"`
    Mechanism string        `json:"mechanism"`
    Probes    []ProbeResult `json:"probes"`
    Summary   string        `json:"summary"`
    Timestamp time.Time     `json:"timestamp"`
}
```

| Field | Description |
|---|---|
| `Verified` | `true` when all applicable probes were blocked |
| `Status` | Overall status: `"sandboxed"`, `"partial"`, `"unsandboxed"`, or `"unavailable"` |
| `Platform` | The runtime OS (`"linux"`, `"darwin"`, `"windows"`) |
| `Mechanism` | The sandbox type: `"landlock"`, `"sandbox-exec"`, `"job-object"`, `"none"` |
| `Probes` | Per-probe results |
| `Summary` | Human-readable one-liner, e.g. `"Sandbox verified: 3/3 probes blocked (file_read, file_write, network)."` |
| `Timestamp` | When the probes ran |

### Helper Methods

```go
result.Blocked()  // Number of probes that were blocked (expected)
result.Failed()   // Number of probes that failed (sandbox didn't block)
result.Skipped()  // Number of probes that were skipped (not applicable)
```

## ProbeResult

```go
type ProbeResult struct {
    Name   string `json:"name"`
    Status string `json:"status"`
    Target string `json:"target,omitempty"`
    Error  string `json:"error,omitempty"`
}
```

| Field | Description |
|---|---|
| `Name` | Probe identifier: `"file_read"`, `"file_write"`, `"network"`, `"process_spawn"` |
| `Status` | `"blocked"` (sandbox worked), `"failed"` (sandbox did not block), or `"skipped"` (not applicable) |
| `Target` | What was probed: a file path, a `host:port`, or a process name |
| `Error` | Set when the probe result is unexpected, describing the failure |

### Status Values

- **`blocked`**: The operation was denied. This is the expected result when the sandbox is working. A `blocked` status is assigned when the operation returns a permission error or any error that is consistent with sandbox enforcement.
- **`failed`**: The operation succeeded when it should have been blocked. This means the sandbox is not enforcing the expected restriction. The `Error` field explains what happened (e.g., `"sandbox did not block read access"`).
- **`skipped`**: The probe was not applicable on this platform or kernel version. For example, the network probe is skipped on Linux with Landlock v1-v3 (pre-6.7 kernels) because Landlock cannot restrict network access in those versions.

## Platform-Specific Probes

### Linux

| Probe | Target | What It Tests |
|---|---|---|
| `file_read` | `/etc/shadow` | Reads a protected system file. Should be blocked by Landlock filesystem rules. |
| `file_write` | `/tmp` | Writes `.openparallax-canary-probe` to `/tmp`. Should be blocked unless `/tmp` is in `AllowedWritePaths`. |
| `network` | `1.1.1.1:443` | Outbound TCP connection to Cloudflare DNS. Should be blocked by Landlock v4+ network rules. Skipped on pre-6.7 kernels. |

### macOS

| Probe | Target | What It Tests |
|---|---|---|
| `file_read` | `/etc/master.passwd` | Reads the macOS password database. Should be blocked by the Seatbelt profile's deny-default policy. |
| `file_write` | `/tmp` | Writes to `/tmp`. Should be blocked because the profile only allows writing to stdio devices. |
| `network` | `1.1.1.1:443` | Outbound TCP connection. Should be blocked because the profile only allows connections to specific `host:port` pairs. |

### Windows

| Probe | Target | What It Tests |
|---|---|---|
| `process_spawn` | `cmd.exe` | Starts a child process. Should be blocked by the Job Object's active process limit of 1. |
| `file_read` | (skipped) | Not supported by Job Objects |
| `file_write` | (skipped) | Not supported by Job Objects |
| `network` | (skipped) | Not supported by Job Objects |

## Status Computation

The overall `Status` is computed from the probe results:

```
applicable = blocked + failed  (probes that actually ran)

if applicable == 0:
    status = "unavailable"     (no probes ran at all)
    verified = false

elif failed == 0:
    status = "sandboxed"       (all probes were blocked)
    verified = true

elif blocked > 0:
    status = "partial"         (some blocked, some failed)
    verified = false

else:
    status = "unsandboxed"     (all probes failed)
    verified = false
```

Only the `"sandboxed"` status sets `Verified = true`. All other statuses mean the sandbox is not fully effective.

## Running Canary Probes

```go
result := sandbox.VerifyCanary()
fmt.Printf("Status: %s\n", result.Status)
fmt.Printf("Summary: %s\n", result.Summary)
```

Probes must be run **after** `ApplySelf()` (on Linux) or inside the sandboxed process (on macOS/Windows). Running them before the sandbox is applied will produce `"unsandboxed"` results.

## Persistence

The Agent writes the canary result to `.openparallax/sandbox.status` as JSON so the Engine can read it:

```go
// Agent writes after probes.
sandbox.WriteCanaryResult(workspace, result)

// Engine reads to check Agent sandbox status.
result := sandbox.ReadCanaryResult(workspace)
if !result.Verified {
    log.Warn("Agent sandbox not verified", "status", result.Status)
}
```

If the file does not exist or cannot be parsed, `ReadCanaryResult` returns a result with `Status: "unknown"`.

## Example Output

### Fully Sandboxed (Linux, kernel 6.8)

```json
{
  "verified": true,
  "status": "sandboxed",
  "platform": "linux",
  "mechanism": "landlock",
  "probes": [
    {"name": "file_read", "status": "blocked", "target": "/etc/shadow"},
    {"name": "file_write", "status": "blocked", "target": "/tmp"},
    {"name": "network", "status": "blocked", "target": "1.1.1.1:443"}
  ],
  "summary": "Sandbox verified: 3/3 probes blocked (file_read, file_write, network).",
  "timestamp": "2026-04-03T10:30:00Z"
}
```

### Partial (Linux, kernel 5.15 -- no network restriction)

```json
{
  "verified": true,
  "status": "sandboxed",
  "platform": "linux",
  "mechanism": "landlock",
  "probes": [
    {"name": "file_read", "status": "blocked", "target": "/etc/shadow"},
    {"name": "file_write", "status": "blocked", "target": "/tmp"},
    {"name": "network", "status": "skipped", "error": "requires Landlock v4+ (kernel 6.7+)"}
  ],
  "summary": "Sandbox verified: 2/2 probes blocked (file_read, file_write). Skipped: network.",
  "timestamp": "2026-04-03T10:30:00Z"
}
```

Note: This still shows `verified: true` because skipped probes do not count against verification. Only failed probes do.

### Unsandboxed (sandbox not applied)

```json
{
  "verified": false,
  "status": "unsandboxed",
  "platform": "linux",
  "mechanism": "landlock",
  "probes": [
    {"name": "file_read", "status": "failed", "target": "/etc/shadow", "error": "sandbox did not block read access"},
    {"name": "file_write", "status": "failed", "target": "/tmp", "error": "sandbox did not block write access"},
    {"name": "network", "status": "failed", "target": "1.1.1.1:443", "error": "sandbox did not block outbound connection"}
  ],
  "summary": "Sandbox verified: 0/3 probes blocked. Failed: file_read, file_write, network.",
  "timestamp": "2026-04-03T10:30:00Z"
}
```
