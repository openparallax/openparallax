# File Protection

The Engine enforces hardcoded file protection levels that cannot be overridden by policy, configuration, or user request. `CheckProtection` is the first check in the tool execution pipeline -- it runs before OTR checks, before Shield evaluation, before audit logging, before everything.

## Protection Levels

Four protection levels are defined in `internal/engine/protection.go`:

| Level | Constant | Behavior |
|---|---|---|
| ReadOnly | `ReadOnly` | Agent can read the file; all writes are hard-blocked. |
| EscalateTier2 | `EscalateTier2` | Writes are allowed but require Tier 2 LLM evaluation. Deletes are hard-blocked. |
| WriteTier1Min | `WriteTier1Min` | Writes are allowed but require at least Tier 1 evaluation (heuristic classifier). |
| FullBlock | `FullBlock` | Agent cannot read or write the file. Access is completely denied. |

`Unprotected` (zero value) means no hardcoded protection applies; the action proceeds to normal Shield evaluation.

## Protected Files

### ReadOnly Files

| File | Purpose |
|---|---|
| `SOUL.md` | Core guardrails -- non-negotiable constraints |
| `IDENTITY.md` | Agent identity, name, role, communication style |

These define the agent's fundamental identity and safety constraints. If the agent could modify them, it could alter its own guardrails -- a critical safety violation. The protection message explains why: "it defines the agent's core identity and guardrails. Edit it manually if you need to change it."

### EscalateTier2 Files

| File | Purpose |
|---|---|
| `AGENTS.md` | Sub-agent definitions |
| `HEARTBEAT.md` | Scheduled task definitions |

These are configuration files with significant operational impact. The agent can modify them, but every write requires a full Tier 2 LLM evaluation to verify the changes are safe and intentional. Deletes are hard-blocked -- these files cannot be removed by the agent.

When `EscalateTier2` protection is detected, the action's `MinTier` is set to 2:

```go
case EscalateTier2:
    action.MinTier = 2
```

This forces the Shield pipeline to evaluate through all three tiers regardless of Tier 0 or Tier 1 results.

### WriteTier1Min Files

| File | Purpose |
|---|---|
| `MEMORY.md` | Long-term memory, facts from conversations |
| `USER.md` | User profile, preferences |

These are files the agent regularly writes to as part of normal operation (memory persistence, learning user preferences). Writes are allowed, but the heuristic classifier must check for prompt injection or other attacks. The `MinTier` is set to 1.

### FullBlock Files

| File | Purpose |
|---|---|
| `config.yaml` | Workspace configuration (LLM keys, Shield settings) |
| `canary.token` | Canary token for Shield Tier 2 verification |
| `audit.jsonl` | Append-only audit log with hash chain |
| `openparallax.db` | SQLite database |
| `evaluator-v1.md` | Shield Tier 2 evaluator prompt |

These are system-critical files. The agent cannot read or write them. Exposing `config.yaml` would leak API keys. Exposing `canary.token` would compromise the Shield's prompt injection detection. Modifying `audit.jsonl` would break the hash chain. Modifying `evaluator-v1.md` would compromise the security evaluator.

### FullBlock Directories

| Directory | Purpose |
|---|---|
| `.openparallax/` | Internal system directory (database, logs, audit chain, sandbox status, **config backups**) |
| `policies/` | Shield YAML policy files |

All files within these directories are blocked from agent access.

### ReadOnly Directories

| Directory | Purpose |
|---|---|
| `skills/` | Custom skill definitions (SKILL.md files) |

The agent can read skills (it does so to load skill bodies) but cannot modify them. Skills are created and maintained by the user.

### Tier1 Directories

| Directory | Purpose |
|---|---|
| `memory/` | Memory sub-files (daily logs, embeddings) |

Files in the memory directory follow the same protection as `MEMORY.md` -- writes require Tier 1 minimum.

## How CheckProtection Works

```go
func CheckProtection(action *types.ActionRequest, workspacePath string) (bool, ProtectionLevel, string)
```

Returns three values:
- `allowed`: Whether the action can proceed.
- `protection`: The highest protection level encountered.
- `reason`: Human-readable explanation if blocked.

### Processing Steps

1. **Shell commands**: For `execute_command` actions, extract only write targets from the command string using regex patterns. Read-only commands (`cat`, `grep`, `head`) are allowed. Write patterns detected: redirects (`>`), `tee`, `cp`, `mv`, `rm`, and Windows equivalents.

2. **Directory operations**: For `copy_dir` or `move_dir`, check if any protected files would be overwritten at the destination.

3. **Path extraction**: Extract all filesystem paths from the action payload (fields: `path`, `source`, `destination`, `dir`, `file`, `target`).

4. **For each path**:
   - Resolve to absolute path using workspace as base.
   - Resolve symlinks via `filepath.EvalSymlinks` to detect symlink bypass attacks.
   - Check against hard-blocked files and directories.
   - Check against read-only directories (if the action is a write).
   - Check against Tier 1 directories (if the action is a write).
   - Check the basename against `protectedFiles` map.

5. **Protection escalation**: The function tracks the highest protection level across all paths. If any path is `FullBlock` or `ReadOnly`, the action is immediately blocked. If `EscalateTier2` or `WriteTier1Min` is detected, the action proceeds but with an elevated `MinTier`.

### Shell Command Analysis

The protection system parses shell commands to identify write targets:

```go
// Unix patterns
redirectRe = regexp.MustCompile(`>{1,2}\s*([^\s;|&]+)`)       // > file, >> file
teeRe      = regexp.MustCompile(`\btee\s+(?:-a\s+)?([^\s;|&]+)`) // tee file
cpMvRe     = regexp.MustCompile(`\b(?:cp|mv)\s+...`)           // cp/mv src dst
rmRe       = regexp.MustCompile(`\brm\s+...`)                   // rm file

// Windows patterns
winCopyRe, winMoveRe, winDelRe, psItemRe, psWriteRe
```

Only write targets are checked. A command like `cat SOUL.md` passes protection (reading is allowed for ReadOnly files). But `echo "new content" > SOUL.md` is blocked because the redirect target is a ReadOnly file.

### Symlink Protection

All paths are resolved through `filepath.EvalSymlinks` before checking:

```go
if realPath, err := filepath.EvalSymlinks(resolved); err == nil {
    resolved = realPath
}
```

This prevents attacks where a symlink in an unprotected location points to a protected file.

## Integration in the Pipeline

`CheckProtection` runs in `handleToolProposal` before any other check:

```go
allowed, protection, protReason := CheckProtection(action, e.cfg.Workspace)
if !allowed {
    // Hard-blocked -- return error immediately
    return &pb.ToolResultDelivery{Content: "Blocked: " + protReason, IsError: true}
}
switch protection {
case EscalateTier2:
    action.MinTier = 2
case WriteTier1Min:
    action.MinTier = 1
}
```

If the action is blocked, no audit entry is written, no Shield evaluation runs, and the tool result is returned immediately with an error.

If the action is allowed but protected, the `MinTier` on the `ActionRequest` is elevated. The Shield pipeline respects this: even if Tier 0 returns ALLOW, the action continues through higher tiers until the minimum is reached.

## SSRF Protection

The Engine blocks requests to private and reserved IP ranges to prevent server-side request forgery (SSRF) attacks. This applies to both the HTTP executor (`http_request`) and the browser executor (`browser_navigate`, `browser_extract`).

### Blocked Ranges

| Range | Description |
|---|---|
| `127.0.0.0/8` | IPv4 loopback |
| `10.0.0.0/8` | Private (RFC 1918) |
| `172.16.0.0/12` | Private (RFC 1918) |
| `192.168.0.0/16` | Private (RFC 1918) |
| `169.254.0.0/16` | Link-local |
| `::1` | IPv6 loopback |
| `fc00::/7` | IPv6 unique local |

Before any outbound HTTP request or browser navigation, the target hostname is resolved to an IP address and checked against these ranges. If the resolved IP falls within a blocked range, the request is rejected before any connection is made. This prevents the agent from being tricked into accessing internal services, cloud metadata endpoints (e.g., `169.254.169.254`), or localhost-bound admin interfaces.

## Key Source Files

| File | Purpose |
|---|---|
| `internal/engine/protection.go` | CheckProtection, protection level definitions, path resolution, shell command parsing |
| `internal/engine/protection_test.go` | Tests for all protection scenarios |
| `internal/engine/engine.go` | handleToolProposal (where CheckProtection is called) |
