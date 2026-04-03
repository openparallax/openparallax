# Security

OpenParallax uses a defense-in-depth approach: a 3-tier Shield pipeline evaluates every tool call, kernel sandboxing restricts the agent process at the OS level, and a tamper-evident audit log records everything.

## Shield Overview

Shield is the security pipeline that evaluates every tool call before execution. When the agent proposes an action (write a file, run a command, send an email), Shield determines whether to allow, block, or escalate the action.

The pipeline is **fail-closed**: if any part of the evaluation fails (network error, timeout, misconfiguration), the action is blocked. This is controlled by `general.fail_closed: true` in config.yaml and should never be set to false.

### Evaluation Flow

```
Tool Call Proposed
     │
     ▼
┌─────────────┐
│   Tier 0    │──── DENY match ──────► BLOCK
│  (Policy)   │──── ALLOW match ──────► ALLOW (if minTier ≤ 0)
│             │──── No match / ESCALATE ──► continue
└─────────────┘
     │
     ▼
┌─────────────┐
│   Tier 1    │──── BLOCK ───────────► BLOCK
│ (Heuristic  │──── ALLOW ───────────► ALLOW (if minTier ≤ 1)
│  + ONNX)    │──── Otherwise ────────► continue
└─────────────┘
     │
     ▼
┌─────────────┐
│   Tier 2    │──── Final decision ──► ALLOW or BLOCK
│   (LLM)    │    (rate-limited, daily budget)
└─────────────┘
```

### Tier 0: Policy Matching

Tier 0 is a fast, deterministic YAML-based policy engine. It matches action types and file path patterns against rules.

Three rule types:

- **deny** — immediately block the action. No further evaluation.
- **allow** — immediately allow the action, unless a `tier_override` forces higher-tier evaluation.
- **verify** — escalate to a specific tier for further evaluation. The `tier_override` field sets the minimum tier.

Policy evaluation is instant (microseconds) and requires no network calls.

### Tier 1: Heuristic + ONNX Classifier

Tier 1 combines two detection methods:

**Heuristic rules** (always available): Pattern matching against known-dangerous patterns. Detects prompt injection attempts, suspicious shell commands, credential access patterns, and other high-risk indicators.

**ONNX classifier** (optional): A DeBERTa-v3 model trained on prompt injection datasets. Runs in-process using pure Go ONNX inference. Provides 98.8% accuracy on injection detection.

Install the classifier with `openparallax get-classifier`. Without it, Tier 1 runs in heuristic-only mode.

The ONNX confidence threshold is configurable via `shield.onnx_threshold` (default: 0.85).

### Tier 2: LLM Evaluator

Tier 2 sends the action to an LLM (the Shield evaluator configured in `shield.evaluator`) for a full security review. The evaluator receives:

- The action type and parameters
- The conversation context
- The policy rules
- A canary token for verification (ensures the evaluator is not being manipulated)

Tier 2 is rate-limited by `general.daily_budget` (default: 100 evaluations per day) to control costs. Verdicts are cached for `general.verdict_ttl_seconds` (default: 60 seconds) to avoid redundant evaluations.

**Best practice:** Use a different model for Shield evaluation than for conversation. Cross-model evaluation is harder to manipulate because a prompt injection crafted for one model is less likely to work on another.

## Policy Files

Policy files define Tier 0 rules. Three files are included in every workspace:

### default.yaml

Balanced security suitable for most use cases.

**Blocked:**
- File operations on sensitive system paths (`~/.ssh/**`, `~/.aws/**`, `~/.gnupg/**`, `~/.kube/**`, `/etc/shadow`, `/etc/sudoers`)
- Deletion of identity files (SOUL.md, IDENTITY.md, TOOLS.md, BOOT.md, AGENTS.md, HEARTBEAT.md)

::: info Platform-specific sensitive paths
The paths above (`/etc/shadow`, `/etc/sudoers`) are Linux-specific examples. The Shield policy engine works with whatever paths are relevant to your platform:
- **macOS**: `/etc/master.passwd`, `~/Library/Keychains/`
- **Windows**: `C:\Windows\System32\config\SAM`, `%APPDATA%\Microsoft\Credentials\`

Customize your policy file to block the sensitive paths for your operating system.
:::

**Evaluated at higher tiers:**
- Shell commands (Tier 1)
- External communication: messages, emails, HTTP requests (Tier 1)
- Identity file modification: SOUL.md, IDENTITY.md, TOOLS.md, BOOT.md (Tier 2)
- Agent config modification: AGENTS.md, HEARTBEAT.md (Tier 2)
- User data modification: USER.md, MEMORY.md (Tier 1)
- Destructive file operations: delete, move (Tier 1)
- Git push (Tier 1)

**Allowed without evaluation:**
- Workspace reads: `read_file`, `list_directory`, `search_files`
- Memory search
- Read-only git: `git_status`, `git_diff`, `git_log`
- Calendar reads
- Schedule listing

### permissive.yaml

Minimal friction for trusted development environments.

**Blocked:**
- Only the most critical system paths (`~/.ssh/id_*`, `~/.ssh/authorized_keys`, `~/.aws/credentials`, `~/.gnupg/**`, `/etc/shadow`)

**Evaluated at higher tiers:**
- External sends: messages, emails (Tier 1)
- Outbound HTTP requests (Tier 1)

**Allowed without evaluation:**
- All file operations, shell commands, git operations, browser, calendar, canvas, memory, schedules

### strict.yaml

Maximum security for environments handling sensitive data.

**Blocked:**
- Broad set of sensitive system paths (includes `~/.docker/**`, `/etc/ssh/**`, `/root/**`)
- All file deletions
- Git push
- Browser interactions (click, type)
- Identity file deletion

**Evaluated at higher tiers:**
- All file writes, moves, copies, directory operations, memory writes, canvas (Tier 2)
- Shell commands (Tier 2)
- All external communication (Tier 2)
- Git operations: commit, pull, branch, checkout (Tier 1)
- Schedule changes (Tier 2)
- Calendar changes (Tier 2)
- Browser navigation, extract, screenshot (Tier 1)

**Allowed without evaluation:**
- Read-only operations only: `read_file`, `list_directory`, `search_files`, `memory_search`, `git_status`, `git_diff`, `git_log`, `read_calendar`, `list_schedules`

### Switching Policies

Change the active policy in config.yaml:

```yaml
shield:
  policy_file: policies/strict.yaml
```

Restart the agent for the change to take effect.

### Writing Custom Policies

Create a new YAML file in the `policies/` directory. The format:

```yaml
deny:
  - name: rule_name           # Descriptive rule name
    action_types:              # List of action types to match
      - write_file
      - delete_file
    paths:                     # Optional: file path patterns (glob)
      - "~/.ssh/**"
      - "/etc/**"

verify:
  - name: rule_name
    action_types:
      - execute_command
    tier_override: 2           # Minimum tier for evaluation

allow:
  - name: rule_name
    action_types:
      - read_file
      - list_directory
```

**Rule evaluation order:** deny rules are checked first, then verify rules, then allow rules. The first matching rule determines the outcome.

**Action types** use snake_case names corresponding to the tool names: `read_file`, `write_file`, `delete_file`, `execute_command`, `git_commit`, `send_email`, `http_request`, `browser_navigate`, `memory_write`, etc.

**Path patterns** use glob syntax with `**` for recursive matching and `*` for single-level matching. The `~` prefix expands to the user's home directory.

## File Protection Levels

Different files in the workspace have different protection levels based on the default policy:

| Protection Level | Files | What Happens |
|-----------------|-------|-------------|
| **FullBlock** | `~/.ssh/**`, `~/.aws/**`, `~/.gnupg/**`, `/etc/shadow` | Always blocked, cannot be read or written |
| **EscalateTier2** | SOUL.md, IDENTITY.md, TOOLS.md, BOOT.md, AGENTS.md, HEARTBEAT.md | Modifications sent to LLM evaluator for approval |
| **WriteTier1Min** | USER.md, MEMORY.md, destructive file ops | Must pass heuristic/ONNX check |
| **ReadOnly** | All workspace reads, git status/diff/log | Allowed immediately |

## Kernel Sandboxing

In addition to Shield, the agent process is kernel-sandboxed at the OS level. This provides defense in depth — even if Shield has a bug, the kernel prevents unauthorized access.

| Platform | Mechanism | Filesystem | Network | Process Spawn |
|----------|-----------|-----------|---------|--------------|
| Linux 5.13+ | Landlock LSM | Restricted | Restricted (6.7+) | Restricted |
| macOS | sandbox-exec | Restricted | Restricted | Restricted |
| Windows | Job Objects | Not restricted | Not restricted | Restricted |

### How Sandboxing Works

- **Linux:** The agent process calls `ApplySelf()` on startup, applying Landlock restrictions before making any gRPC calls. Only the workspace directory and necessary system paths are accessible.
- **macOS:** The engine wraps the agent spawn with `WrapCommand()`, applying a sandbox-exec profile that restricts file, network, and process operations.
- **Windows:** The engine creates a Job Object that prevents the agent from spawning child processes.

Sandboxing is best-effort. If the kernel does not support it (old kernel, unsupported platform), the agent starts normally. Run `openparallax doctor` to check sandbox status.

### Verifying Sandbox Status

```bash
openparallax doctor
```

The Sandbox check reports:

- **Mode** — Landlock, sandbox-exec, or Job Objects
- **Version** — Landlock ABI version (Linux)
- **Capabilities** — filesystem isolation, network isolation, process limits

The web UI also reports sandbox status via `GET /api/status` and the settings panel.

## Audit Log

Every tool call is recorded in a tamper-evident audit log at `<workspace>/.openparallax/audit.jsonl`. Each entry includes:

- Timestamp
- Session ID
- Action type and parameters
- Shield verdict (tier, decision, confidence, reasoning)
- SHA-256 hash
- Previous entry's hash (forming a chain)

### Hash Chain Verification

Each audit entry contains a hash of its contents and the hash of the previous entry, forming a chain. If any entry is modified or deleted, the chain breaks.

Verify chain integrity:

```bash
openparallax audit --verify
```

The `openparallax doctor` command also checks chain integrity as part of its health check.

### Audit Entry Types

| Type | Description |
|------|-------------|
| `ACTION_PROPOSED` | A tool call was proposed by the agent |
| `ACTION_EVALUATED` | Shield evaluated the action |
| `ACTION_APPROVED` | Shield allowed the action |
| `ACTION_BLOCKED` | Shield blocked the action |
| `ACTION_EXECUTED` | The action was executed successfully |
| `ACTION_FAILED` | The action execution failed |

### Querying the Audit Log

```bash
# Recent entries
openparallax audit

# Filter by session
openparallax audit --session "sess_abc123"

# Filter by type
openparallax audit --type ACTION_BLOCKED

# Verify integrity
openparallax audit --verify
```

## Canary Tokens

Each workspace contains a canary token at `<workspace>/.openparallax/canary.token`. This token is used during Tier 2 evaluation:

1. The canary is included in the evaluation prompt
2. The evaluator must echo the canary in its response
3. If the canary is missing from the response, the evaluation is considered compromised and the action is blocked

This prevents prompt injection attacks that attempt to manipulate the Shield evaluator.

## Security Best Practices

1. **Use cross-model evaluation** — set a different LLM provider/model for `shield.evaluator` than for `llm`. This makes injection attacks significantly harder.
2. **Start with default.yaml** — it provides good security without excessive friction. Switch to strict.yaml if you handle sensitive data.
3. **Keep fail_closed true** — never set `general.fail_closed` to false. This ensures Shield errors result in blocked actions.
4. **Install the ONNX classifier** — `openparallax get-classifier` adds ML-based injection detection alongside heuristic rules.
5. **Review the audit log** — periodically check `openparallax audit` for unexpected actions or blocked attempts.
6. **Restrict channel access** — use `allowed_users` and `allowed_numbers` in channel configs to control who can interact with the agent.
7. **Run doctor regularly** — `openparallax doctor` checks sandbox status, audit chain integrity, and overall system health.

## Next Steps

- [Tools](/guide/tools) — which tools trigger which evaluation tiers
- [Configuration](/guide/configuration) — Shield and general security settings
- [Troubleshooting](/guide/troubleshooting) — common security-related issues
