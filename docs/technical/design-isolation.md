# Process Isolation: Sandbox & Chronicle

OpenParallax separates thinking from acting at the OS process level. The Agent proposes; the Engine decides and executes. Kernel sandboxing constrains the Agent, copy-on-write snapshots protect the filesystem, and a tamper-evident audit trail records everything. This page covers each layer and the threat it addresses.

## Kernel Sandboxing

The Agent is a process on the user's machine, accessing their files, talking to their LLM provider, running in their terminal. The threat model is: **what if the LLM convinces the agent to do something dangerous?** The agent talks to external LLM APIs. Those APIs can be manipulated through prompt injection. If the agent has unrestricted filesystem and network access, a successful injection can read SSH keys, exfiltrate data, or install malware — and the agent would execute those actions thinking it is being helpful.

Kernel sandboxing restricts what the agent process can do at the OS level:

| Platform | Mechanism | Filesystem | Network | Process Spawn |
|----------|-----------|-----------|---------|---------------|
| Linux 5.13+ | Landlock LSM | Restricted to workspace | Blocked (6.7+) | Blocked via seccomp |
| macOS | sandbox-exec | Restricted to workspace | Blocked | Blocked |
| Windows | Job Objects | Unrestricted | Unrestricted | Blocked |

On Linux, the agent self-sandboxes by calling `ApplySelf` before making any gRPC calls to the engine:

```go
sb := sandbox.New()
sbErr := sb.ApplySelf(sandbox.Config{
    WorkspacePath: workspacePath,
    AllowedPaths:  []string{configDir, cacheDir},
    AllowNetwork:  []string{engineAddr},
})
```

This is critical: the sandbox is applied from *within* the process itself. Even if the agent binary is compromised, the kernel enforces the restrictions. There is no daemon to kill, no container to escape, no configuration file to edit. The restrictions are baked into the process's security context by the kernel.

No root required. No container runtime. No daemon. The sandbox is a syscall.

## Canary Probes

Requesting a sandbox is not the same as having one. On Linux, Landlock requires kernel 5.13+. Network restriction requires 6.7+. On macOS, `sandbox-exec` is deprecated and may be removed. On Windows, Job Objects restrict process creation but not filesystem or network access.

The canary probe system verifies that the sandbox is actually applied by attempting prohibited operations and confirming they fail:

```go
func VerifyCanary() CanaryResult {
    mechanism := New().Mode()
    probes := runPlatformProbes()
    // Each probe attempts a blocked operation:
    //   file_read:  read /etc/shadow (or equivalent)
    //   file_write: create a file in /tmp
    //   network:    connect to an external host
    //   spawn:      exec a child process
    // A "blocked" result means the sandbox is working.
    // A "failed" result means the operation succeeded — the sandbox is broken.
}
```

The result is a structured report:

```json
{
  "verified": true,
  "status": "sandboxed",
  "mechanism": "landlock",
  "probes": [
    {"name": "file_read", "status": "blocked", "target": "/etc/shadow"},
    {"name": "file_write", "status": "blocked", "target": "/tmp/canary-test"},
    {"name": "network", "status": "blocked", "target": "1.1.1.1:443"},
    {"name": "spawn", "status": "blocked", "target": "/bin/echo"}
  ]
}
```

This is verification, not assumption. The agent reports its sandbox status to the engine via gRPC. The engine includes it in `GET /api/status`. The `openparallax doctor` command shows sandbox capabilities as part of the health check.

If the sandbox is unavailable (old kernel, unsupported platform), the agent starts normally. Sandboxing is defense in depth — its absence does not disable other security layers. Shield still evaluates every action. The audit trail still records everything. Chronicle still takes snapshots.

## Copy-on-Write Chronicle

Shield prevents dangerous actions. But what if something slips through? A classifier false negative. A novel attack pattern. A legitimate action with unintended consequences. Prevention is necessary but not sufficient.

Chronicle creates a snapshot of affected files BEFORE every write, delete, or move operation:

```
User: "Refactor the auth module"

Agent proposes: file_write auth.go (new content)
  → Chronicle: snapshot auth.go (current content, SHA-256 hash)
  → Shield: evaluate → ALLOW
  → Executor: write auth.go
  → Audit: log execution with snapshot reference

Agent proposes: file_delete auth_old.go
  → Chronicle: snapshot auth_old.go (full content)
  → Shield: evaluate → ALLOW
  → Executor: delete auth_old.go
  → Audit: log execution with snapshot reference
```

If the refactoring is wrong — or if the action was malicious — the snapshot chain allows rollback to the exact state before the operation. The snapshot chain is hash-linked: each snapshot's hash includes the previous snapshot's hash, making the chain tamper-evident. Modifying a snapshot breaks the chain from that point forward, just like the audit trail.

Together, Shield and Chronicle form a **prevent-and-revert** loop. Shield stops the vast majority of dangerous actions. Chronicle handles the fraction that gets through. The gap is data exfiltration — once data leaves the system, it cannot be un-sent. That is where kernel-level network restrictions and [SSRF protection](/technical/protection) close the gap.

The `chronicle.max_snapshots` configuration controls rollback budget. When the limit is reached, the oldest snapshots are pruned. This prevents unbounded disk usage while maintaining recent rollback capability.

## Append-Only Audit with Hash Chains

Every action proposal, Shield evaluation, execution result, and failure is recorded in a JSONL file. Each entry's SHA-256 hash includes the previous entry's hash:

```
entry[0].hash = SHA-256(entry[0].data)
entry[1].hash = SHA-256(entry[1].data + entry[0].hash)
entry[2].hash = SHA-256(entry[2].data + entry[1].hash)
...
```

Modifying any entry changes its hash, which breaks every subsequent hash in the chain. Verification walks the entire chain and reports the first break:

```bash
openparallax audit --verify
# ✓ 1,247 entries verified, chain intact
# or
# ✗ Chain broken at entry 892 (hash mismatch)
```

The audit log lives in `.openparallax/` — a hard-blocked directory that the sandboxed agent process cannot read or write. Even a fully compromised agent cannot tamper with the audit trail because it physically cannot access the file.

This is not logging. It is a cryptographic proof of what happened, in what order, with what security decisions. When something goes wrong, the audit trail provides a verifiable record.

## The 3-Process Model

The core architectural principle: **the entity that thinks must never be the entity that acts.**

```
Process Manager (openparallax start)
  └── Engine (privileged, unsandboxed)
        ├── Shield pipeline
        ├── Executors (file, shell, git, browser, ...)
        ├── Audit trail
        ├── Chronicle snapshots
        └── Agent (sandboxed, restricted)
              └── LLM reasoning loop
```

The Engine and Agent run as separate OS processes communicating over gRPC. The Engine holds the database, audit log, Shield pipeline, and executor registry. The Agent holds the LLM session and proposes actions. The Agent physically cannot execute anything — it sends proposals to the Engine, which evaluates and executes them.

This separation means a fully compromised agent — total prompt injection, complete jailbreak, adversarial control of the LLM — still cannot:

1. **Bypass Shield** — Shield runs in the Engine process, not the Agent
2. **Execute actions directly** — executors are in the Engine, behind gRPC
3. **Read the audit trail** — the audit file is in a directory the sandbox blocks
4. **Modify Chronicle snapshots** — same directory restriction
5. **Access the database** — SQLite file is Engine-side, not accessible to the Agent
6. **Spawn child processes** — blocked by kernel sandbox
7. **Exfiltrate data over the network** — blocked by kernel sandbox (Linux 6.7+)

The gRPC boundary is authenticated with ephemeral tokens — each agent process receives a unique 128-bit random token via environment variable, validated on the first stream message. This prevents unauthorized processes from impersonating agents on localhost. See [Process Model](/technical/process-model) for the lifecycle details.

## Best-Effort Sandboxing

The sandbox is best-effort: if the kernel does not support it (Linux < 5.13, old macOS, Windows filesystem), the agent starts normally. This is a deliberate choice.

The alternative — refusing to start without a sandbox — would make OpenParallax unusable on older systems, in CI environments, in development containers, and on Windows.

Best-effort means: apply every restriction the platform supports, verify what was actually applied via canary probes, report the result, and continue. The sandbox is one layer of defense in depth. Its absence weakens the system but does not break it. Shield, Chronicle, audit, and the process boundary all function independently of the sandbox.

The `openparallax doctor` command reports the sandbox status so users know exactly what protection level their platform provides. On a modern Linux kernel, the answer is: full filesystem, network, and process isolation. On Windows, the answer is: process spawn restriction only. The user can make informed decisions about their deployment environment.
