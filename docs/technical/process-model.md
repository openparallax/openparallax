# Process Model

OpenParallax runs as three OS processes with a strict privilege hierarchy. The Process Manager spawns the Engine, which in turn spawns the sandboxed Agent. Each process boundary enforces isolation: the Agent cannot access files, network, or child processes outside its allowed scope.

```
openparallax start              (Process Manager)
  |
  +-- internal-engine           (Engine: privileged, unsandboxed)
        |
        +-- internal-agent      (Agent: sandboxed, headless)
```

## Process Manager

The Process Manager is the entry point (`cmd/agent/start.go`). It is a `processManager` struct that:

1. Resolves the config path from `--config`, agent name, or workspace discovery.
2. Validates no other instance is running via `registry.IsRunning(workspace)`.
3. Spawns the Engine as a child process.
4. Monitors the Engine for crashes and restarts.
5. Handles shutdown signals (SIGTERM, SIGINT).

### Spawning the Engine

The Process Manager calls `os.Executable()` to find its own binary, then spawns it with the `internal-engine` subcommand:

```go
cmd = exec.CommandContext(ctx, executable, "internal-engine", "--config", configPath)
```

In daemon mode (`--daemon`), the child is detached from the terminal via `Setsid: true` in `SysProcAttr`.

### Stdout Protocol

The Engine communicates startup status to the Process Manager through a line-based protocol on stdout. The Process Manager reads lines with a 30-second timeout:

| Line format | Meaning |
|---|---|
| `PORT:<grpc_port>` | gRPC server is listening on this port |
| `WEB:<web_port>` | HTTP/WebSocket server is ready |
| `WEB_FAILED:<port>:<error>` | Web server failed to bind |
| `WEB_DISABLED` | Web server is not configured |

The Process Manager parses these lines sequentially. `PORT:` always comes first, followed by one of the `WEB*` lines. Once both are received, the Engine is considered started.

```go
type engineStartResult struct {
    grpcPort int
    web      webStatus
}
```

### Crash Recovery

The `monitor()` method runs in a goroutine and calls `cmd.Wait()` in a loop. When the Engine exits:

- **Exit code 0**: Clean shutdown. Process Manager exits.
- **Exit code 75**: Restart requested (not a crash). Respawn immediately.
- **Any other exit**: Crash. Apply the restart budget.

The restart budget allows a maximum of 5 crashes within a 60-second sliding window. Each crash is timestamped, and crashes older than 60 seconds are ignored when counting:

```go
cutoff := now.Add(-60 * time.Second)
recentCrashes := 0
for _, t := range pm.crashes {
    if t.After(cutoff) {
        recentCrashes++
    }
}
if recentCrashes >= 5 {
    // Give up
}
```

After a crash (but within budget), the Process Manager waits 1 second before respawning to avoid tight crash loops.

### Restart Protocol (Exit Code 75)

Exit code 75 is the restart signal. It is not counted against the crash budget. Two sources trigger it:

1. **`/restart` slash command** in the CLI or web UI.
2. **`POST /api/restart`** REST endpoint, which calls `os.Exit(75)` in a goroutine.

The Process Manager detects exit code 75 in the monitor loop:

```go
if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 75 {
    // Not a crash — respawn immediately
    pm.spawnEngine(ctx)
    continue
}
```

### Signal Handling

The Process Manager registers for SIGTERM and SIGINT. On receipt:

1. Cancels the context (stops the monitor goroutine).
2. Calls `stopEngine()`, which sends SIGTERM to the Engine.
3. Waits up to 5 seconds for the Engine to exit.
4. If the Engine does not exit within 5 seconds, sends SIGKILL.
5. Removes the PID file.

## Engine Process

The Engine (`cmd/agent/internal_engine.go`, `runInternalEngine`) is the privileged process. It:

1. Loads config from the `--config` flag.
2. Creates the `engine.Engine` instance (gRPC server, Shield pipeline, executors, audit, chronicle, memory).
3. Starts the gRPC server on a dynamic port (or configured port).
4. Writes `PORT:<port>` to stdout.
5. Probes sandbox capability via `sandbox.Probe()`.
6. Starts the web server (if configured) and writes `WEB:<port>` or `WEB_FAILED` to stdout.
7. Starts channel adapters (Telegram, WhatsApp, Discord, Signal).
8. Spawns the Agent as a sandboxed child process.
9. Reads the canary verification result from the Agent (after a 2-second delay).
10. Waits for the Agent to exit or a shutdown signal.

### Agent Spawning

The Engine uses an `agentManager` struct to spawn and supervise the Agent:

```go
cmd = exec.Command(executable, "internal-agent",
    "--grpc", grpcAddr,
    "--name", agentName,
    "--workspace", workspace)
```

The Agent is headless -- stdout goes to `/dev/null`. Stderr goes to the Engine's stderr for crash diagnostics.

Before starting the Agent, the Engine applies sandbox wrapping:

```go
sb := sandbox.New()
if sb.Available() {
    sb.WrapCommand(cmd, sandbox.Config{
        AllowedReadPaths:  []string{executable, workspace},
        AllowedWritePaths: []string{},
        AllowedTCPConnect: []string{grpcAddr, llmHost},
        AllowProcessSpawn: false,
    })
}
```

On macOS, `WrapCommand` prepends `sandbox-exec` with a generated profile. On Windows, it configures a Job Object. On Linux, the Agent self-sandboxes (the Engine does not wrap it).

The Agent has its own crash budget (5 crashes in 60 seconds), independent of the Engine's budget. If the Agent crashes but the web server is running, the Engine continues serving the web UI for diagnostics.

## Agent Process

The Agent (`cmd/agent/internal_agent.go`, `runInternalAgent`) is the sandboxed, headless process. It:

1. Loads config from the workspace `config.yaml`.
2. Applies the kernel sandbox via `ApplySelf()` (Linux only -- macOS/Windows use `WrapCommand` from the Engine).
3. Runs canary probes to verify the sandbox is enforced.
4. Writes the canary result to `.openparallax/sandbox.status` for the Engine to read.
5. Creates the LLM provider.
6. Opens a read-only database connection for history access.
7. Creates the `agent.Agent` instance.
8. Connects to the Engine via gRPC.
9. Opens a bidirectional `RunSession` stream.
10. Sends `AgentReady` to signal initialization is complete.
11. Enters the directive loop, waiting for `ProcessRequest` or `ShutdownDirective` from the Engine.

### Sandbox Application Order

The sandbox is applied before any untrusted operations, before the gRPC connection, before loading any data:

```go
sb := sandbox.New()
if sb.Available() {
    sb.ApplySelf(sandbox.Config{
        AllowedReadPaths:  []string{workspace},
        AllowedWritePaths: []string{},
        AllowedTCPConnect: []string{grpcAddr, llmHost},
        AllowProcessSpawn: false,
    })
}
```

On Linux with Landlock, `ApplySelf()` is irreversible -- restrictions cannot be relaxed after application. The Agent can read workspace files and connect to the Engine's gRPC port and the LLM API host, but cannot write to the filesystem, connect to arbitrary network hosts, or spawn child processes.

### Canary Verification

After applying the sandbox, the Agent runs platform-specific probes:

| Platform | Probes |
|---|---|
| Linux | `file_read` (/etc/shadow), `file_write` (/tmp), `network` (1.1.1.1:443, Landlock v4+ only) |
| macOS | `file_read` (/etc/master.passwd), `file_write` (/tmp), `network` (1.1.1.1:443) |
| Windows | `process_spawn` (cmd.exe); file/network probes skipped (not supported by Job Objects) |

The result is one of:
- `sandboxed`: All applicable probes were blocked. Proceed normally.
- `partial`: Some probes were blocked, others were not. Agent refuses to start.
- `unsandboxed`: No probes were blocked. Agent refuses to start.
- `unavailable`: No sandbox mechanism available. Agent starts with a warning.

The fail-closed behavior means the Agent will not start in a partially sandboxed state. It either has full sandbox enforcement or none (with a warning on unsupported platforms).

### Directive Loop

The Agent's main loop reads `EngineDirective` messages from the gRPC stream:

```go
for {
    directive, err := stream.Recv()
    switch d := directive.Directive.(type) {
    case *pb.EngineDirective_Process:
        processMessage(ctx, stream, loopCfg, d.Process, db)
    case *pb.EngineDirective_Shutdown:
        return nil
    }
}
```

Each `ProcessRequest` triggers the LLM reasoning loop (`agent.RunLoop`). During processing, a separate goroutine reads `ToolResultDelivery` and `ToolDefsDelivery` directives from the stream and feeds them into the result channel consumed by the reasoning loop.

## Lifecycle Diagram

```
[Process Manager]
     |
     | spawn (internal-engine --config ...)
     v
[Engine]
     | 1. Create engine, start gRPC
     | 2. Write PORT: to stdout
     | 3. Probe sandbox, start web
     | 4. Write WEB: to stdout
     | 5. Start channel adapters
     |
     | spawn (internal-agent --grpc ... --workspace ...)
     v
[Agent]
     | 1. Load config
     | 2. ApplySelf sandbox (Linux)
     | 3. Run canary probes
     | 4. Write sandbox.status
     | 5. Connect gRPC
     | 6. Send AgentReady
     | 7. Wait for ProcessRequest directives
     |
     |<-- gRPC bidirectional stream -->|
     |                                  |
     | EngineDirective (Process, ToolResult, Shutdown)
     | AgentEvent (Ready, Token, ToolProposal, Complete)
```

## Key Source Files

| File | Purpose |
|---|---|
| `cmd/agent/start.go` | Process Manager, `processManager` struct |
| `cmd/agent/internal_engine.go` | Engine process entry point, `agentManager` struct |
| `cmd/agent/internal_agent.go` | Agent process entry point, directive loop |
| `internal/sandbox/sandbox.go` | Sandbox interface, Config, Probe |
| `internal/sandbox/canary.go` | Canary probes, VerifyCanary, result I/O |
| `internal/sandbox/sandbox_linux.go` | Landlock implementation |
| `internal/sandbox/sandbox_darwin.go` | sandbox-exec implementation |
| `internal/sandbox/sandbox_windows.go` | Job Objects implementation |
