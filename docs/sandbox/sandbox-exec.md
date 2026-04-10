# macOS: sandbox-exec

On macOS, OpenParallax uses `sandbox-exec` with a generated Seatbelt profile to restrict the Agent process. The Engine generates the profile at spawn time based on the sandbox configuration, writes it to a temporary file, and launches the Agent through `sandbox-exec -f profile.sb`.

## How sandbox-exec Works

`sandbox-exec` is a macOS system utility that runs a process inside a sandbox defined by a Seatbelt profile. The profile is a Lisp-like DSL that specifies what the process is allowed and denied.

The default policy is `(deny default)` -- everything is denied unless explicitly allowed. The profile then adds specific allow rules for the operations the Agent needs.

Apple has deprecated `sandbox-exec` in favor of App Sandbox entitlements, but it remains functional on all current macOS versions and is the only mechanism available to non-App Store command-line tools.

## Profile Format

OpenParallax generates profiles from a template:

```scheme
(version 1)
(deny default)

; Allow reading the agent binary, system libraries, and TLS certs
(allow file-read*
    (subpath "/usr/lib")
    (subpath "/System/Library")
    (subpath "/Library/Frameworks")
    (subpath "/private/etc/hosts")
    (subpath "/private/etc/resolv.conf")
    (subpath "/private/etc/ssl")
    (literal "/dev/null")
    (literal "/dev/urandom")
    (literal "/dev/stdin")
    (literal "/dev/stdout")
    (literal "/dev/stderr")
    ; Additional read paths from config
)

; Allow writing to stdio only
(allow file-write*
    (literal "/dev/null")
    (literal "/dev/stdin")
    (literal "/dev/stdout")
    (literal "/dev/stderr"))

; Allow network connections to engine and LLM API
(allow network-outbound
    ; Allowed TCP connections from config
)

; Allow basic process operations
(allow process-exec (literal "/path/to/agent"))
(allow sysctl-read)
(allow mach-lookup
    (global-name "com.apple.system.logger")
    (global-name "com.apple.system.notification_center"))
```

## Profile Generation

The Engine calls `WrapCommand()` which:

1. Starts from the profile template
2. Substitutes the agent binary path into the `process-exec` rule
3. Adds `Config.AllowedReadPaths` as `file-read*` rules (directories use `subpath`, files use `literal`)
4. Adds `Config.AllowedTCPConnect` as `network-outbound` rules
5. Writes the profile to a temporary file
6. Rewrites `cmd.Path` to `/usr/bin/sandbox-exec` and prepends `-f profile.sb` to the arguments

```go
sb := sandbox.New()
cmd := exec.Command("./agent", "--workspace", "/workspace")
err := sb.WrapCommand(cmd, sandbox.Config{
    AllowedReadPaths:  []string{"/workspace", "/usr/local/lib"},
    AllowedTCPConnect: []string{"127.0.0.1:50051", "api.anthropic.com:443"},
})
// cmd.Path is now /usr/bin/sandbox-exec
// cmd.Args is ["sandbox-exec", "-f", "/tmp/openparallax-sandbox-XXXX.sb", "./agent", "--workspace", "/workspace"]
```

## Filesystem Restrictions

### Default Read Access

The profile always allows reading:

- `/usr/lib` -- system libraries and dylibs
- `/System/Library` -- macOS frameworks
- `/Library/Frameworks` -- third-party frameworks
- `/private/etc/hosts` and `/private/etc/resolv.conf` -- DNS resolution
- `/private/etc/ssl` -- TLS certificates (required for HTTPS to LLM APIs)
- Standard I/O devices (`/dev/null`, `/dev/urandom`, `/dev/stdin`, `/dev/stdout`, `/dev/stderr`)

### Custom Read Paths

Paths from `Config.AllowedReadPaths` are added as read rules. Directories get `(subpath ...)` for recursive access, files get `(literal ...)` for exact match.

### Write Access

By default, the Agent can only write to standard I/O devices. The template does not add custom write paths from the config because the Agent communicates results through gRPC, not by writing files directly.

## Network Restrictions

The `(allow network-outbound)` section permits TCP connections to specific `host:port` pairs from `Config.AllowedTCPConnect`. All other outbound connections are denied.

The Agent typically needs:

- `127.0.0.1:<gRPC port>` -- communication with the Engine
- `api.anthropic.com:443` (or the appropriate LLM API host) -- LLM calls

### TLS Certificate Access

The profile allows reading `/private/etc/ssl` so the Go TLS stack can load system root certificates. Without this, HTTPS connections to the LLM API would fail with certificate verification errors.

## Mach Service Access

The profile allows `mach-lookup` for two system services:

- `com.apple.system.logger` -- system logging
- `com.apple.system.notification_center` -- system notifications

These are required for basic process operation on macOS.

## Canary Probes

On macOS, the canary system runs three probes:

1. **file_read**: Attempts to read `/etc/master.passwd` (the macOS equivalent of `/etc/shadow`)
2. **file_write**: Attempts to write to `/tmp`
3. **network**: Attempts an outbound TCP connection to `1.1.1.1:443`

All three should be blocked by the sandbox. If any succeed, the sandbox is only partially effective.

## Limitations

- **Deprecated API**: Apple has deprecated `sandbox-exec` and the Seatbelt profile language. It continues to work on all current macOS versions, but Apple could remove it in a future release.
- **No write path configuration**: The current profile template does not support custom write paths. The Agent communicates through gRPC only.
- **Profile visibility**: The profile is written to a temporary file that is readable by other processes on the system. It does not contain secrets, but it reveals the sandbox configuration.
