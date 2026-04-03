# Sandbox Quick Start

Kernel-level process isolation in a few lines of Go.

## Self-Sandboxing (Linux)

On Linux, a process can restrict itself using Landlock. Restrictions are irreversible once applied.

```go
package main

import (
    "fmt"
    "log"
    "os"

    "github.com/openparallax/openparallax/internal/sandbox"
)

func main() {
    sb := sandbox.New()

    // Check if the sandbox mechanism is available.
    if !sb.Available() {
        fmt.Println("Sandbox not available on this platform/kernel")
        os.Exit(1)
    }
    fmt.Printf("Sandbox mechanism: %s\n", sb.Mode())

    // Define what this process is allowed to do.
    cfg := sandbox.Config{
        AllowedReadPaths:  []string{"/workspace", "/usr/lib", "/etc/hosts"},
        AllowedWritePaths: []string{"/workspace/output"},
        AllowedTCPConnect: []string{"api.anthropic.com:443"},
        AllowProcessSpawn: false,
    }

    // Apply restrictions. This is irreversible.
    if err := sb.ApplySelf(cfg); err != nil {
        log.Fatalf("Failed to apply sandbox: %s", err)
    }
    fmt.Println("Sandbox applied. Restrictions are now in effect.")

    // Verify with canary probes.
    result := sandbox.VerifyCanary()
    fmt.Printf("Status: %s (%d/%d probes blocked)\n",
        result.Status, result.Blocked(), result.Blocked()+result.Failed())

    // From here on, the process cannot:
    // - Read files outside /workspace, /usr/lib, /etc/hosts
    // - Write files outside /workspace/output
    // - Connect to any host except api.anthropic.com:443
    // - Spawn child processes

    // This will succeed:
    data, err := os.ReadFile("/workspace/config.yaml")
    if err != nil {
        fmt.Printf("Read allowed file: error (unexpected): %s\n", err)
    } else {
        fmt.Printf("Read allowed file: OK (%d bytes)\n", len(data))
    }

    // This will fail with a permission error:
    _, err = os.ReadFile("/etc/shadow")
    if err != nil {
        fmt.Printf("Read blocked file: blocked (expected): %s\n", err)
    }
}
```

## Sandboxing a Child Process (macOS)

On macOS, the parent process wraps the child spawn with `sandbox-exec`:

```go
package main

import (
    "fmt"
    "log"
    "os/exec"

    "github.com/openparallax/openparallax/internal/sandbox"
)

func main() {
    sb := sandbox.New()
    if !sb.Available() {
        log.Fatal("Sandbox not available")
    }

    // Prepare the child command.
    cmd := exec.Command("./my-agent", "--workspace", "/workspace")

    // Define restrictions.
    cfg := sandbox.Config{
        AllowedReadPaths:  []string{"/workspace", "/usr/lib"},
        AllowedWritePaths: []string{"/workspace/.openparallax"},
        AllowedTCPConnect: []string{"127.0.0.1:50051"},
    }

    // Wrap the command to run inside the sandbox.
    // On macOS, this rewrites cmd.Path to sandbox-exec and prepends
    // the profile file argument.
    if err := sb.WrapCommand(cmd, cfg); err != nil {
        log.Fatalf("Failed to wrap command: %s", err)
    }

    // Start the sandboxed child.
    if err := cmd.Start(); err != nil {
        log.Fatalf("Failed to start: %s", err)
    }

    fmt.Printf("Child process started in sandbox (PID: %d)\n", cmd.Process.Pid)
    _ = cmd.Wait()
}
```

## Checking Platform Capabilities

```go
package main

import (
    "encoding/json"
    "fmt"

    "github.com/openparallax/openparallax/internal/sandbox"
)

func main() {
    status := sandbox.Probe()
    data, _ := json.MarshalIndent(status, "", "  ")
    fmt.Println(string(data))
}
```

Output on Linux with kernel 6.8:

```json
{
  "active": true,
  "mode": "landlock",
  "version": 4,
  "filesystem": true,
  "network": true
}
```

Output on macOS:

```json
{
  "active": true,
  "mode": "sandbox-exec",
  "filesystem": true,
  "network": true
}
```

Output on Windows:

```json
{
  "active": true,
  "mode": "job-object",
  "filesystem": false,
  "network": false
}
```
