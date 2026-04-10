---
description: Defense-in-depth security for AI agents — 4-tier Shield pipeline, kernel sandboxing, information flow control, and tamper-evident audit logging.
---

# Security

OpenParallax uses a defense-in-depth approach: a 4-tier Shield pipeline evaluates every tool call, kernel sandboxing restricts the agent process at the OS level, and a tamper-evident audit log records everything.

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
│   Tier 2    │──── BLOCK ───────────► BLOCK
│    (LLM     │──── ALLOW ───────────► ALLOW
│  Evaluator) │──── ESCALATE ─────────► continue
└─────────────┘
     │
     ▼
┌─────────────┐
│   Tier 3    │──── User approves ───► ALLOW
│  (Human-in- │──── User denies ─────► BLOCK
│  the-Loop)  │──── Timeout ─────────► BLOCK (default-deny)
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

**Heuristic rules** (always available): Pattern matching against ~79 known-dangerous patterns. Detects prompt injection attempts, suspicious shell commands, credential access patterns, and other high-risk indicators.

**ONNX classifier** (recommended, downloaded separately): A fine-tuned DeBERTa-v3 model trained on prompt injection data. Runs in-process using pure Go ONNX inference. The `init` wizard offers to download it; without it, Tier 1 falls back to heuristic-only mode (lower attack detection on encoding/obfuscation/multi-agent categories). The model card and training data are public on [HuggingFace](https://huggingface.co/openparallax/shield-classifier-v1).

The default config bypasses ONNX for a few action types where the trained model over-fires on benign payloads (`write_file`, `delete_file`, `move_file`, `copy_file`, `send_email`, `send_message`, `http_request`). These types are escalated to Tier 2 instead so the LLM evaluator handles content judgment. The full story is in [eval reports/03-classifier-optimization](https://github.com/openparallax/openparallax/blob/main/eval-results/reports/03-classifier-optimization.md). For the configuration knobs, see [Tier 1 — Per-Action-Type Skip List](/shield/tier1#per-action-type-onnx-skip-list).

The ONNX confidence threshold is configurable via `shield.onnx_threshold` (default: 0.85).

### Tier 2: LLM Evaluator

Tier 2 sends the action to an LLM (the Shield evaluator configured in `shield.evaluator`) for a full security review. The evaluator receives only the action type and payload as raw data — no agent identity, no conversation history, no tools. Its single job is to decide whether the action is safe.

The evaluator returns one of three verdicts:

- **ALLOW** — clearly safe to execute
- **BLOCK** — clearly malicious or in violation of a guardrail
- **ESCALATE** — genuinely ambiguous, needs a human decision (routes to Tier 3)

A **canary token** is embedded in the evaluator prompt and must appear in the response. If the canary is missing, the evaluator was manipulated by injection in the action payload — the verdict is rejected and the action is blocked.

Tier 2 is rate-limited by `general.daily_budget` (default: 100 evaluations per day) to control costs. Verdicts are cached for `general.verdict_ttl_seconds` (default: 60 seconds) to avoid redundant evaluations.

**Best practice:** Use a different model for Shield evaluation than for conversation. Cross-model evaluation is harder to manipulate because a prompt injection crafted for one model is less likely to work on another.

### Tier 3: Human-in-the-Loop Approval

When Tier 2 returns ESCALATE, Shield broadcasts an approval request to **every connected channel adapter simultaneously** — web UI, CLI, Telegram, Discord, Signal, iMessage. The user sees the tool name, the action arguments, and Shield's reasoning, then approves or denies.

Tier 3 is the fallback for actions that are ambiguous to the LLM evaluator: irreversible side effects without obvious malice (force-pushing to main, dropping a database table, sending a wire transfer email), actions whose intent depends on business context the evaluator cannot know, or operations that should never run autonomously regardless of model confidence.

Three guarantees:

- **First response wins.** If you approve in the web UI before Telegram delivers the message, the Telegram prompt resolves automatically.
- **Default-deny on timeout.** If no response arrives within the configured window (default 300 seconds), the action is blocked.
- **Rate-limited.** `shield.tier3.max_per_hour` (default 10) caps how many approval requests Shield will issue per hour. This prevents an attack from hammering the user with prompts until they click "approve" out of fatigue.

Configure with `shield.tier3.timeout_seconds` and `shield.tier3.max_per_hour`. See the [Tier 3 reference](/shield/tier3) for the full approval flow and the channel adapter integration details.

## Policy Files

Policy files define Tier 0 rules. Three files are included in every workspace:

### default.yaml

Balanced security suitable for most use cases.

**Blocked:**
- File operations on sensitive system paths (`~/.ssh/**`, `~/.aws/**`, `~/.gnupg/**`, `~/.kube/**`, `/etc/shadow`, `/etc/sudoers`)
- Deletion of identity files (SOUL.md, IDENTITY.md, AGENTS.md, HEARTBEAT.md)

::: info Platform-specific sensitive paths
The paths above (`/etc/shadow`, `/etc/sudoers`) are Linux-specific examples. The Shield policy engine works with whatever paths are relevant to your platform:
- **macOS**: `/etc/master.passwd`, `~/Library/Keychains/`
- **Windows**: `C:\Windows\System32\config\SAM`, `%APPDATA%\Microsoft\Credentials\`

Customize your policy file to block the sensitive paths for your operating system.
:::

**Evaluated at higher tiers:**
- Shell commands (Tier 2)
- External communication: messages, emails, HTTP requests (Tier 2)
- File writes (Tier 2)
- Destructive file operations: delete, move (Tier 2)
- Identity file modification: SOUL.md, IDENTITY.md (Tier 2)
- Agent config modification: AGENTS.md, HEARTBEAT.md (Tier 2)
- User data modification: USER.md, MEMORY.md (Tier 1)
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
  policy_file: security/shield/strict.yaml
```

Restart the agent for the change to take effect.

### Writing Custom Policies

Create a new YAML file in the `security/shield/` directory. The format:

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

## Default Denylist

OpenParallax ships with a curated denylist that applies to **any path the agent touches, anywhere on disk** — not just paths inside the workspace. The denylist is fixed in the binary and is not user-extensible. If you want the agent to access something on the list, you relocate the data to a path that is not on the list. Moving the file is the explicit consent action.

The denylist has two protection levels:

**Restricted (no read, no write).** Reading the path is the attack — the content is the secret. The agent cannot `read_file`, `write_file`, `delete_file`, `cat`, `cp`, or any other operation against these paths.

- Credential directories: `~/.ssh`, `~/.aws`, `~/.gnupg`, `~/.docker`, `~/.kube`, `~/.password-store`, `~/.azure`, `~/.config/gcloud`, `~/.config/op` (1Password CLI)
- Linux: `/etc/shadow`, `/etc/sudoers`, `/root`, `/etc/sudoers.d`, `/etc/ssh`
- macOS: `~/Library/Keychains`, `~/Library/Cookies`, browser credential dirs (Chrome, Firefox, Safari)
- Windows: `C:\Windows\System32\config`, `%APPDATA%\Microsoft\Credentials`, `%LOCALAPPDATA%\Microsoft\Credentials`
- Filename patterns matched anywhere on disk:
  - SSH keys: `id_rsa`, `id_dsa`, `id_ecdsa`, `id_ed25519`
  - Cert/key files: `*.pem`, `*.key`, `*.p12`, `*.pfx`, `*.keystore`, `*.jks`, `*.asc`
  - Env files: `.env`, `.env.local`, `.env.production`
  - Credential files: `credentials`, `credentials.json`, `secrets.{yaml,yml,json}`, `token.json`, `service-account.json`, `.pgpass`, `.my.cnf`

**Protected (read OK, write/delete blocked).** Reading the file is useful and safe; modifying it is a persistence vector or destabilises the host. The agent can `read_file` and `cat` these freely, but every write is blocked.

- Shell rc files: `.bashrc`, `.bash_profile`, `.zshrc`, `.zprofile`, `.profile`, fish config
- VCS configs: `.gitconfig`, `.gitignore_global`, `.npmrc`, `.yarnrc`, pip config, cargo config
- Editor configs: `.vimrc`, nvim init files, `.tmux.conf`, `.inputrc`
- Linux system reference files: `/etc/hosts`, `/etc/passwd`, `/etc/group`, `/etc/fstab`, `/etc/resolv.conf`, `/etc/crontab`, `/etc/environment`
- Linux package manager and service dirs: `/etc/cron.{d,daily,weekly,monthly,hourly}`, `/etc/systemd`, `/etc/init.d`, `/etc/apt`, `/etc/yum.repos.d`, `/etc/dnf`, `/etc/pacman.d`
- macOS: `/etc/hosts`
- Windows: `C:\Windows\System32\drivers\etc\hosts`

The denylist runs after symlink resolution. A symlink in `/tmp/safe.txt` pointing at `~/.ssh/id_rsa` resolves to `~/.ssh/id_rsa` and is blocked.

## Workspace File Protection Levels

In addition to the cross-platform default denylist, files inside the agent's own workspace have their own protection levels. These apply only to paths inside `cfg.Workspace`:

| Protection Level | Files | What Happens |
|-----------------|-------|-------------|
| **FullBlock** | `config.yaml`, `canary.token`, `audit.jsonl`, `openparallax.db`, `.openparallax/`, `security/` | Always blocked, cannot be read or written. The evaluator prompt is compiled into the binary and is not a file on disk. |
| **ReadOnly** | SOUL.md, IDENTITY.md, `skills/` | Read OK, write/delete blocked |
| **EscalateTier2** | AGENTS.md, HEARTBEAT.md | Writes proceed but require Tier 2 LLM evaluation |
| **WriteTier1Min** | USER.md, MEMORY.md, `memory/` | Writes proceed but require Tier 1 minimum (heuristic/ONNX check) |

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

23 event types are defined. The most common action-lifecycle types:

| Type | Description |
|------|-------------|
| `ACTION_PROPOSED` | A tool call was proposed by the agent |
| `ACTION_EVALUATED` | Shield evaluated the action |
| `ACTION_APPROVED` | Shield allowed the action |
| `ACTION_BLOCKED` | Shield blocked the action |
| `ACTION_EXECUTED` | The action was executed successfully |
| `ACTION_FAILED` | The action execution failed |

See [Audit Event Types](/audit/#event-types) for the complete list of all 23 types including Shield errors, canary verification, transactions, sessions, config changes, IFC, chronicle, and sandbox events.

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

## OAuth Token Encryption

OAuth2 tokens (Google, Microsoft) are encrypted at rest in the SQLite database using AES-256-GCM. The encryption key is derived from the workspace canary token via HKDF-SHA256 (`crypto.DeriveKey`). Tokens are auto-refreshed when they expire within 5 minutes of use. Revoked or expired refresh tokens return `ErrTokenRevoked`.

## Security Best Practices

1. **Use cross-model evaluation** — set a different LLM provider/model for `shield.evaluator` than for `llm`. This makes injection attacks significantly harder.
2. **Start with default.yaml** — it provides good security without excessive friction. Switch to strict.yaml if you handle sensitive data.
3. **Keep fail_closed true** — never set `general.fail_closed` to false. This ensures Shield errors result in blocked actions.
4. **Tier 1 runs 79 heuristic rules by default** — ML-based classification via sidecar is the immediate next item on the [roadmap](/project/roadmap#immediate-next-steps).
5. **Review the audit log** — periodically check `openparallax audit` for unexpected actions or blocked attempts.
6. **Restrict channel access** — use `allowed_users` and `allowed_numbers` in channel configs to control who can interact with the agent.
7. **Run doctor regularly** — `openparallax doctor` checks sandbox status, audit chain integrity, and overall system health.
8. **Input validation is defense-in-depth** — all tool parameters from the LLM are validated before execution. Archive extraction rejects symlinks and enforces cumulative size limits. Git parameters are validated as safe identifiers. Email headers reject CRLF injection. Config changes validate provider names against the supported set.

## Next Steps

- [Tools](/guide/tools) — which tools trigger which evaluation tiers
- [Configuration](/guide/configuration) — Shield and general security settings
- [Troubleshooting](/guide/troubleshooting) — common security-related issues
