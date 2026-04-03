# Tier 0 -- YAML Policy

Tier 0 is the first evaluation tier in the Shield pipeline. It is a deterministic, rule-based policy engine that matches actions against YAML rules using glob patterns. It executes in microseconds with zero external dependencies -- no ML models, no LLM calls, no network requests.

## How It Works

When an action arrives, the policy engine checks it against three rule lists in order:

1. **Deny rules** -- checked first. If any deny rule matches, the action is immediately BLOCKED. No further evaluation occurs.
2. **Verify rules** -- checked second. If a verify rule matches, the action is ESCALATED to the tier specified by `tier_override`. The action continues to Tier 1 or Tier 2 for further evaluation.
3. **Allow rules** -- checked last. If an allow rule matches, the action is ALLOWED (subject to `min_tier` overrides from other sources).

If no rule matches, the result is `NoMatch` and the action proceeds to Tier 1.

```
Action arrives
    │
    ▼
Deny rules ──── match? ──── YES ──→ BLOCK (immediately)
    │ NO
    ▼
Verify rules ── match? ──── YES ──→ ESCALATE (to tier_override)
    │ NO
    ▼
Allow rules ─── match? ──── YES ──→ ALLOW (if minTier ≤ 0)
    │ NO
    ▼
NoMatch ──────────────────────────→ Proceed to Tier 1
```

## Policy File Structure

A policy file is a YAML document with three top-level keys:

```yaml
deny:
  - name: rule_name
    action_types: [...]
    paths: [...]
    tier_override: N

verify:
  - name: rule_name
    action_types: [...]
    paths: [...]
    tier_override: N

allow:
  - name: rule_name
    action_types: [...]
    paths: [...]
```

### Rule Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | yes | Unique identifier for the rule. Appears in verdicts and audit logs. |
| `action_types` | string[] | no | List of action type strings to match. If omitted, matches all action types. |
| `paths` | string[] | no | List of glob patterns to match against file paths in the action payload. If omitted, matches regardless of path. |
| `tier_override` | int | no | For verify rules: the minimum tier to escalate to (1 or 2). Defaults to 1. |

A rule matches when **all** specified criteria match simultaneously. If a rule has both `action_types` and `paths`, the action must match at least one action type AND at least one path pattern.

## Pattern Matching

### Action Types

Action types are matched by exact string comparison. Shield defines 50+ action types. Common ones:

| Action Type | Description |
|-------------|-------------|
| `read_file` | Read a file |
| `write_file` | Write/create a file |
| `delete_file` | Delete a file |
| `copy_file` | Copy a file |
| `move_file` | Move/rename a file |
| `list_directory` | List directory contents |
| `create_directory` | Create a directory |
| `delete_directory` | Delete a directory |
| `execute_command` | Execute a shell command |
| `git_commit` | Git commit |
| `git_push` | Git push |
| `send_email` | Send an email |
| `send_message` | Send a chat message |
| `http_request` | Make an HTTP request |
| `memory_write` | Write to memory |
| `memory_search` | Search memory |
| `browser_navigate` | Navigate browser |
| `canvas_create` | Create a canvas artifact |

### Path Patterns

Path patterns use glob syntax powered by the `gobwas/glob` library. The separator is `/` (forward slash on all platforms). Paths are normalized before matching: tilde (`~`) is expanded to the user's home directory and backslashes are converted to forward slashes.

| Pattern | Matches | Does Not Match |
|---------|---------|----------------|
| `~/.ssh/**` | `~/.ssh/id_rsa`, `~/.ssh/config`, `~/.ssh/keys/deploy` | `~/.ssh` (directory itself) |
| `~/.ssh/*` | `~/.ssh/id_rsa`, `~/.ssh/config` | `~/.ssh/keys/deploy` (nested) |
| `~/.ssh/id_*` | `~/.ssh/id_rsa`, `~/.ssh/id_ed25519` | `~/.ssh/config` |
| `/etc/shadow` | `/etc/shadow` only | `/etc/shadow.bak` |
| `**/SOUL.md` | `./SOUL.md`, `/home/user/workspace/SOUL.md` | `SOUL.md.bak` |
| `C:\\Windows\\System32\\config\\**` | Any file under `C:\Windows\System32\config\` | Files elsewhere |

**Path extraction**: Shield extracts paths from multiple payload fields -- `path`, `source`, `destination`, `dir`, `file`, and `target`. This ensures that copy/move operations are caught by both source and destination paths. A rule blocking `~/.ssh/**` will catch:

- `read_file` with `path: ~/.ssh/id_rsa`
- `copy_file` with `source: ~/.ssh/id_rsa` or `destination: ~/.ssh/id_rsa`
- `move_file` with `source: ~/.ssh/config` or `destination: ~/.ssh/config`

### Content Patterns

Content patterns match against the text content within action payloads. They are particularly useful for verify rules that need to inspect what is being written or what command is being executed, beyond just the path or action type.

## Examples

### Block Access to Sensitive System Paths

```yaml
deny:
  - name: block_sensitive_system_paths
    action_types:
      - read_file
      - write_file
      - delete_file
      - copy_file
      - move_file
    paths:
      - "~/.ssh/**"
      - "~/.aws/**"
      - "~/.gnupg/**"
      - "~/.kube/**"
      - "/etc/shadow"
      - "/etc/sudoers"
      - "C:\\Windows\\System32\\config\\**"
```

This rule blocks any file operation on SSH keys, AWS credentials, GPG keys, Kubernetes config, shadow passwords, sudoers, and Windows security configuration. The action is blocked at Tier 0 -- no ML or LLM evaluation is needed.

### Protect Identity Files from Deletion

```yaml
deny:
  - name: block_identity_deletion
    action_types:
      - delete_file
      - delete_directory
    paths:
      - "**/SOUL.md"
      - "**/IDENTITY.md"
      - "**/TOOLS.md"
      - "**/BOOT.md"
```

Identity files define the agent's personality and capabilities. This rule prevents deletion regardless of where they are located (the `**/` prefix matches any directory depth).

### Require Tier 1 for Shell Commands

```yaml
verify:
  - name: evaluate_shell_commands
    action_types:
      - execute_command
    tier_override: 1
```

Shell commands are powerful and unpredictable. This rule ensures every shell command passes through the Tier 1 classifier (ONNX + heuristic) before execution. If the classifier detects prompt injection or a dangerous pattern, it blocks the command. If it is uncertain, it escalates to Tier 2.

### Require Tier 2 for Identity Modification

```yaml
verify:
  - name: evaluate_soul_modification
    action_types:
      - write_file
      - copy_file
      - move_file
    paths:
      - "**/SOUL.md"
      - "**/IDENTITY.md"
      - "**/TOOLS.md"
      - "**/BOOT.md"
    tier_override: 2
```

Modifying identity files is the highest-risk operation -- an attacker who changes the agent's identity can make it do anything. This rule sends identity modifications directly to the LLM evaluator (skipping Tier 1) for deep contextual analysis.

### Allow Read-Only Operations

```yaml
allow:
  - name: allow_workspace_reads
    action_types:
      - read_file
      - list_directory
      - search_files

  - name: allow_memory_search
    action_types:
      - memory_search

  - name: allow_git_readonly
    action_types:
      - git_status
      - git_diff
      - git_log
```

Read-only operations that do not modify state are generally safe. These rules allow them at Tier 0 without further evaluation, keeping latency low for the most common operations.

### Allow Everything Except External Communication (Permissive Mode)

```yaml
deny:
  - name: block_critical_system_paths
    action_types:
      - read_file
      - write_file
      - delete_file
    paths:
      - "~/.ssh/id_*"
      - "~/.aws/credentials"
      - "/etc/shadow"

verify:
  - name: evaluate_external_sends
    action_types:
      - send_message
      - send_email
      - http_request
    tier_override: 1

allow:
  - name: allow_all_file_ops
    action_types:
      - read_file
      - write_file
      - delete_file
      - move_file
      - copy_file
      - create_directory
      - list_directory
      - search_files

  - name: allow_shell
    action_types:
      - execute_command
```

This permissive policy trusts the LLM for local operations but evaluates anything that communicates externally. Suitable for trusted development environments where speed matters more than maximum security.

## Rule Ordering

Within each section (`deny`, `verify`, `allow`), rules are evaluated in the order they appear in the YAML file. **First match wins** within each section. Across sections, the order is always: deny first, then verify, then allow.

This means you can create specific exceptions before broad rules:

```yaml
deny:
  # Specific: block all SSH keys
  - name: block_ssh_keys
    action_types: [read_file]
    paths: ["~/.ssh/**"]

  # Broad: block all dotfiles
  - name: block_dotfiles
    action_types: [read_file, write_file]
    paths: ["~/.*/**"]

allow:
  # But allow reading .gitconfig
  # This will never be reached for ~/.ssh/** because deny runs first!
  - name: allow_gitconfig
    action_types: [read_file]
    paths: ["~/.gitconfig"]
```

::: warning
An allow rule cannot override a deny rule. If a deny rule matches, the action is blocked immediately and allow rules are never checked. To create an exception within a deny category, restructure your deny rules to be more specific.
:::

## Default Policy

Shield ships with three built-in policies:

| Policy | File | Description |
|--------|------|-------------|
| Default | `policies/default.yaml` | Balanced security. Blocks sensitive paths, evaluates shell commands and external communication, allows workspace reads. |
| Strict | `policies/strict.yaml` | Maximum security. Evaluates all writes at Tier 2, blocks destructive operations and browser interactions. |
| Permissive | `policies/permissive.yaml` | Minimal friction. Allows most operations, only evaluates external communication. |

See [Policy Syntax](/shield/policies) for the full reference and [Configuration](/shield/configuration) for how to select a policy.

## Loading Policies

The policy engine loads the YAML file once at initialization and compiles all glob patterns. Patterns that fail to compile are silently skipped (this is logged when a logger is configured).

```go
engine, err := tier0.NewPolicyEngine("policies/default.yaml")
if err != nil {
    // File not found, YAML parse error, etc.
    // In fail-closed mode, Shield blocks everything when policy loading fails.
    log.Fatal(err)
}

result := engine.Evaluate(&types.ActionRequest{
    Type:    "read_file",
    Payload: map[string]any{"path": "/home/user/.ssh/id_rsa"},
})
// result.Decision == Deny
// result.Reason == "block_sensitive_system_paths"
```

## Next Steps

- [Tier 1 -- Classifier](/shield/tier1) -- what happens when Tier 0 escalates
- [Policy Syntax](/shield/policies) -- full reference for all three shipped policies
- [Configuration](/shield/configuration) -- how to configure the policy file path
