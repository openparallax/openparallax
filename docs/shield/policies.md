# Policy Writing Guide

Shield policies are YAML files that define deterministic rules for Tier 0 evaluation. This page extends the [Tier 0 reference](/shield/tier0) with practical guidance on designing, testing, and migrating policies.

::: tip Curated safe-command fast path
Before any policy rule runs, the gateway checks `execute_command` actions against a curated allowlist of known-safe shell command first-tokens (`git`, `npm`, `make`, `go`, `cargo`, `docker`, `kubectl`, `pwd`, `whoami`, `date`, etc., plus their cmd.exe equivalents on Windows). A single-statement command whose first token is in the allowlist is fast-tracked to ALLOW with confidence 1.0, bypassing all four tiers. Single-statement only — any command containing `;`, `&`, `|`, `>`, `<`, `` ` ``, or `$(...)` falls through to normal evaluation. The allowlist is curated and ships in the binary; it is not user-extensible. See [Safe Command Fast Path](#safe-command-fast-path) below.
:::

## How to Think About Policy Design

A Shield policy answers three questions for every tool call:

1. **Should this be blocked immediately?** Put it in `deny`.
2. **Should this be evaluated by a higher tier?** Put it in `verify`.
3. **Should this be allowed without further evaluation?** Put it in `allow`.

If no rule matches, the action falls through to Tier 1 (the classifier). This is the "safety net" -- actions you did not anticipate get evaluated by the ML classifier rather than silently passing through.

### The Policy Design Spectrum

```
STRICT                                                    PERMISSIVE
  │                                                          │
  │  Block most things.         Allow most things.           │
  │  Verify everything else.    Only block the critical.     │
  │  Allow only reads.          Verify only external sends.  │
  │                                                          │
  │  High latency.              Low latency.                 │
  │  High security.             Lower security.              │
  │  More false positives.      Fewer false positives.       │
  │  Higher Tier 2 costs.       Lower Tier 2 costs.          │
```

Most deployments should start with the `default.yaml` policy and customize from there.

## YAML Structure

A policy file has three top-level sections, each containing an ordered list of rules:

```yaml
deny:
  - name: ...
    action_types: [...]
    paths: [...]

verify:
  - name: ...
    action_types: [...]
    paths: [...]
    tier_override: N

allow:
  - name: ...
    action_types: [...]
    paths: [...]
```

**Evaluation order**: deny -> verify -> allow -> NoMatch. Within each section, rules are evaluated top-to-bottom. First match wins within each section. Across sections, the order is always: deny first, then verify, then allow.

## Rule Fields

### `name` (required)

A unique string identifier for the rule. Appears in verdict reasoning and audit logs. Use descriptive names that make audit log entries readable.

Naming conventions:
- `block_*` for deny rules
- `evaluate_*` for verify rules
- `allow_*` for allow rules
- Lowercase with underscores

### `action_types` (optional)

A list of action type strings. The rule matches if the action's type is in this list. If omitted, the rule matches **all** action types -- use this sparingly.

See the [full action type list](/shield/tier0#action-types) for all 50+ available types.

### `paths` (optional)

A list of glob patterns matched against file paths in the action payload. If omitted, the rule matches regardless of path. Shield extracts paths from multiple payload fields: `path`, `source`, `destination`, `dir`, `file`, and `target`.

Patterns use glob syntax via `gobwas/glob`:

| Pattern | Meaning |
|---------|---------|
| `*` | Any characters except `/` |
| `**` | Any characters including `/` (recursive) |
| `?` | Exactly one character except `/` |
| `[abc]` | One of `a`, `b`, `c` |
| `{foo,bar}` | Match `foo` or `bar` |

Paths are normalized before matching: `~` is expanded to the home directory, backslashes become forward slashes.

### `tier_override` (verify rules only)

The minimum tier to escalate to. Defaults to 1 if omitted.

| Value | Meaning |
|-------|---------|
| `1` | Escalate to Tier 1 (DualClassifier: ONNX + heuristic) |
| `2` | Escalate to Tier 2 (LLM evaluator), skipping Tier 1 decision authority |

### Rule Matching Logic

A rule matches when **all** specified criteria match simultaneously:

```
IF action_types is specified:
    action.Type MUST be in the list
IF paths is specified:
    At least ONE path field in the action payload
    MUST match at least ONE glob pattern
```

If both `action_types` and `paths` are specified, both must match. If neither is specified, the rule matches everything (catch-all).

## Start from the Default, Customize Incrementally

The safest approach to policy design:

1. Start with `default.yaml` (the shipped balanced policy)
2. Run with it for a few days, reviewing the audit log
3. Identify actions that are too aggressively blocked (false positives)
4. Identify actions that should be more carefully evaluated
5. Add or modify rules one at a time
6. Test each change before deploying

```bash
cp security/shield/default.yaml security/shield/my-policy.yaml
# Edit my-policy.yaml
# Update config.yaml: shield.policy_file: security/shield/my-policy.yaml
```

## Common Patterns

### Workspace-Scoped Access

Allow reads and writes only within the workspace directory. Block everything outside.

```yaml
deny:
  # Block writes outside workspace
  - name: block_writes_outside_workspace
    action_types:
      - write_file
      - delete_file
      - move_file
      - copy_file
      - create_directory
      - delete_directory
    # Note: This blocks ALL writes. The allow rule below
    # re-allows writes within the workspace.

allow:
  # Allow workspace operations
  - name: allow_workspace_reads
    action_types:
      - read_file
      - list_directory
      - search_files
    paths:
      - "/home/user/workspace/**"

  - name: allow_workspace_writes
    action_types:
      - write_file
      - delete_file
      - copy_file
      - move_file
    paths:
      - "/home/user/workspace/**"
```

::: warning
Remember: deny rules run before allow rules. If a deny rule matches, the action is blocked immediately and allow rules are never checked. The example above blocks ALL writes first, which means the allow rule for workspace writes will never be reached. To implement workspace scoping, use verify rules instead of deny rules, or restructure the deny rules to only block specific paths outside the workspace.
:::

A better approach using verify rules:

```yaml
verify:
  # Escalate writes outside workspace
  - name: evaluate_writes_outside_workspace
    action_types:
      - write_file
      - delete_file
      - move_file
      - copy_file
    tier_override: 2

allow:
  - name: allow_workspace_writes
    action_types:
      - write_file
      - delete_file
      - copy_file
      - move_file
    paths:
      - "/home/user/workspace/**"
```

This way, writes inside the workspace are allowed at Tier 0, while writes outside go to the LLM evaluator.

### Sensitive File Protection

Block access to credential files, SSH keys, and other sensitive paths:

```yaml
deny:
  - name: block_credential_files
    action_types:
      - read_file
      - write_file
      - delete_file
      - copy_file
      - move_file
    paths:
      - "**/.env"
      - "**/.env.*"
      - "**/credentials"
      - "**/credentials.json"
      - "**/secrets.yaml"
      - "**/secrets.json"
      - "**/*.pem"
      - "**/*.key"

  - name: block_ssh_directory
    paths:
      - "~/.ssh/**"

  - name: block_cloud_credentials
    paths:
      - "~/.aws/**"
      - "~/.gnupg/**"
      - "~/.kube/**"
      - "~/.docker/**"
```

The `block_ssh_directory` rule omits `action_types`, so it blocks ALL operations on `~/.ssh/**` -- reads, writes, copies, anything.

### Command Whitelisting

Allow specific shell commands, block or escalate everything else:

```yaml
verify:
  # All shell commands go to Tier 1 by default
  - name: evaluate_all_commands
    action_types:
      - execute_command
    tier_override: 1
```

For stricter control, you can block shell commands entirely and only allow specific exceptions:

```yaml
deny:
  - name: block_dangerous_commands
    action_types:
      - execute_command

allow:
  # But allow git and test commands in the workspace
  - name: allow_git_commands
    action_types:
      - git_status
      - git_diff
      - git_log
      - git_commit
```

::: warning
Remember the deny-before-allow rule. If you deny `execute_command`, the allow rule for git commands will not help because git commands have different action types (`git_status`, `git_diff`, etc.). However, if you want to allow `execute_command` for specific paths, the deny rule would block it before the allow rule runs. Use verify rules for this pattern instead.
:::

### Network Restriction

Block or escalate external network access:

```yaml
deny:
  # Block all external communication
  - name: block_external_communication
    action_types:
      - send_email
      - send_message
      - http_request
```

Or, for a less restrictive approach:

```yaml
verify:
  # Evaluate all external communication at Tier 2
  - name: evaluate_external_sends
    action_types:
      - send_email
      - send_message
    tier_override: 2

  # HTTP requests go to Tier 1
  - name: evaluate_http
    action_types:
      - http_request
    tier_override: 1
```

### Per-Tool Escalation

Different risk levels for different operations:

```yaml
verify:
  # Identity file modification → Tier 2 (highest scrutiny)
  - name: evaluate_identity_modification
    action_types:
      - write_file
      - copy_file
      - move_file
    paths:
      - "**/SOUL.md"
      - "**/IDENTITY.md"
    tier_override: 2

  # Shell commands → Tier 1 (classifier)
  - name: evaluate_shell_commands
    action_types:
      - execute_command
    tier_override: 1

  # File deletions → Tier 1
  - name: evaluate_destructive_ops
    action_types:
      - delete_file
      - delete_directory
    tier_override: 1

  # Git push → Tier 1
  - name: evaluate_git_push
    action_types:
      - git_push
    tier_override: 1

  # External communication → Tier 1
  - name: evaluate_external
    action_types:
      - send_email
      - send_message
      - http_request
    tier_override: 1
```

### Project-Specific Rules

Protect production config and deployment files:

```yaml
deny:
  - name: block_production_configs
    paths:
      - "**/production.env"
      - "**/prod.env"
      - "**/.env.production"
      - "**/deploy/secrets/**"

verify:
  - name: evaluate_deployment_files
    action_types:
      - write_file
      - delete_file
    paths:
      - "**/Dockerfile"
      - "**/.github/workflows/**"
      - "**/terraform/**"
      - "**/k8s/**"
    tier_override: 2
```

## Shipped Policies

Shield ships with three policies:

### `default.yaml` -- Balanced Security

The default policy balances security and usability:

**Deny:** File operations on `~/.ssh/**`, `~/.aws/**`, `~/.gnupg/**`, `~/.kube/**`, `/etc/shadow`, `/etc/sudoers`. Deletion of identity files.

**Verify:** Shell commands (Tier 1). External communication (Tier 1). Identity file modification (Tier 2). Agent config modification (Tier 2). Destructive file ops (Tier 1). Git push (Tier 1).

**Allow:** Workspace reads. Memory search. Git read-only. Calendar reads. Schedule listing.

### `strict.yaml` -- Maximum Security

**Deny (additions beyond default):** `~/.docker/**`, `/etc/ssh/**`, `/root/**`. All file deletions. Git force push. Browser interactions.

**Verify:** All write operations (Tier 2). Shell commands (Tier 2). All communication (Tier 2). Git operations (Tier 1). Browser navigation (Tier 1).

**Allow:** Read-only operations only.

### `permissive.yaml` -- Minimal Friction

**Deny:** Only the most critical: `~/.ssh/id_*`, `~/.aws/credentials`, `~/.gnupg/**`, `/etc/shadow`.

**Verify:** External sends (Tier 1). Outbound HTTP (Tier 1).

**Allow:** All file operations, shell commands, memory, git, browser, schedules, calendar, canvas.

## Testing Policies

### Command-Line Testing

Test specific actions against your policy:

```bash
# Test reading an SSH key
openparallax-shield evaluate \
  --action-type read_file \
  --payload '{"path": "/home/user/.ssh/id_rsa"}' \
  --policy security/shield/my-policy.yaml
# Expected: BLOCK (deny rule matches)

# Test writing to workspace
openparallax-shield evaluate \
  --action-type write_file \
  --payload '{"path": "/home/user/workspace/main.go", "content": "package main"}' \
  --policy security/shield/my-policy.yaml
# Expected: ALLOW or ESCALATE depending on your policy

# Test shell command
openparallax-shield evaluate \
  --action-type execute_command \
  --payload '{"command": "ls -la"}' \
  --policy security/shield/my-policy.yaml
# Expected: ESCALATE to Tier 1
```

### Unit Testing with Go

```go
func TestMyPolicy(t *testing.T) {
    pe, err := tier0.NewPolicyEngine("my-policy.yaml")
    require.NoError(t, err)

    // Verify deny rules block SSH keys.
    result := pe.Evaluate(&types.ActionRequest{
        Type:    types.ActionReadFile,
        Payload: map[string]any{"path": "/home/user/.ssh/id_rsa"},
    })
    assert.Equal(t, tier0.Deny, result.Decision)

    // Verify allow rules pass workspace reads.
    result = pe.Evaluate(&types.ActionRequest{
        Type:    types.ActionReadFile,
        Payload: map[string]any{"path": "/home/user/workspace/main.go"},
    })
    assert.Equal(t, tier0.Allow, result.Decision)

    // Verify verify rules escalate shell commands.
    result = pe.Evaluate(&types.ActionRequest{
        Type:    types.ActionExecCommand,
        Payload: map[string]any{"command": "go test ./..."},
    })
    assert.Equal(t, tier0.Escalate, result.Decision)
    assert.Equal(t, 1, result.TierOverride)
}
```

### Testing Checklist

When testing a new or modified policy:

1. **Verify deny rules**: Test each deny rule with a matching action. Confirm BLOCK.
2. **Verify deny boundaries**: Test actions that are close to matching but should not. Confirm no false blocks.
3. **Verify escalations**: Test each verify rule. Confirm ESCALATE with the correct `tier_override`.
4. **Verify allows**: Test each allow rule. Confirm ALLOW.
5. **Verify NoMatch behavior**: Test an action that matches no rule. Confirm it falls through to Tier 1.
6. **Test path normalization**: Test with `~`, backslashes, relative paths. Confirm patterns still match.

## Debugging: Why Was My Action Blocked?

When an action is unexpectedly blocked, check the audit log:

```bash
openparallax audit --type PROPOSED --lines 20
```

Each audit entry for a PROPOSED action includes:

- The action type and payload
- The Shield verdict (ALLOW, BLOCK, ESCALATE)
- The tier that made the decision (0, 1, or 2)
- The reasoning (which rule matched, classifier score, or evaluator explanation)

### Common Causes

| Symptom | Likely Cause |
|---------|-------------|
| Action blocked at Tier 0 | A deny rule matched. Check the `reasoning` field for the rule name. |
| Action blocked at Tier 1 | The classifier or heuristic engine detected a risky pattern. Check the confidence score. |
| Action blocked at Tier 2 | The LLM evaluator decided the action is unsafe in context. Check the evaluator reasoning. |
| Action allowed but should be blocked | No deny rule matches. Add a deny or verify rule. |
| Action escalated unnecessarily | A verify rule is too broad. Make it more specific with `action_types` and `paths`. |

### Audit Log Fields

```json
{
  "event_type": "PROPOSED",
  "action_type": "write_file",
  "details": {
    "verdict": "BLOCK",
    "tier": 0,
    "reasoning": "block_sensitive_system_paths: path ~/.ssh/id_rsa matches deny rule",
    "confidence": 1.0
  }
}
```

## Migration: Moving from Permissive to Strict

If you are running with `permissive.yaml` and want to tighten security:

### Phase 1: Audit Only

Run with `permissive.yaml` but review the audit log daily. Identify which actions are common and which are unusual.

```bash
# See all actions from the last 24 hours
openparallax audit --lines 100
```

### Phase 2: Add Verify Rules

Start escalating risky operations without blocking them. This lets the classifier and evaluator learn your patterns while you observe the verdicts:

```yaml
# Add to your policy
verify:
  - name: evaluate_shell_commands
    action_types: [execute_command]
    tier_override: 1
  - name: evaluate_external_sends
    action_types: [send_email, send_message, http_request]
    tier_override: 1
```

### Phase 3: Add Deny Rules

Once you understand the traffic patterns, add deny rules for operations that should never happen:

```yaml
deny:
  - name: block_credential_access
    paths:
      - "~/.ssh/**"
      - "~/.aws/**"
      - "**/.env"
```

### Phase 4: Restrict Allow Rules

Narrow the allow rules to only the operations the agent actually needs:

```yaml
allow:
  - name: allow_workspace_only
    action_types: [read_file, write_file, list_directory, search_files]
    paths:
      - "/home/user/workspace/**"
```

### Phase 5: Switch to Default or Strict

Once your custom rules are working, consider switching the base policy to `default.yaml` or `strict.yaml` and layering your customizations on top.

## Best Practices

1. **Deny rules first.** Put the most critical blocks at the top of the deny section. They are checked first and cannot be overridden by allow rules.

2. **Be specific with action types.** A rule with no `action_types` matches everything -- use this only for catch-all deny rules on critical paths.

3. **Use `**` for recursive paths.** `~/.ssh/*` only matches direct children; `~/.ssh/**` matches all descendants including nested directories.

4. **Test your policy.** Use the Shield evaluate command or Go unit tests to verify each rule before deploying.

5. **Review NoMatch actions.** Actions that match no rule fall through to Tier 1. If too many actions are falling through, add more specific allow/deny/verify rules to keep Tier 1 and Tier 2 costs under control.

6. **Do not rely on allow rules for security.** Allow rules do not override deny rules. Use deny rules for security boundaries.

7. **Name rules descriptively.** Rule names appear in audit logs. `block_ssh_keys` is better than `rule_1`.

8. **Keep policies under version control.** Treat policy files like code -- track changes, review diffs, and test before deploying.

## Safe Command Fast Path

Before any policy rule runs, the gateway runs a curated allowlist check on `execute_command` actions. The allowlist contains the first-tokens of common dev workflow commands whose safety is determined by the command itself (it does not take arbitrary path arguments) or by its working-directory operation pattern.

A command qualifies for the fast path when:

1. It is a single statement. Any command containing `;`, `&`, `|`, `>`, `<`, `` ` ``, or `$(...)` is rejected from the fast path and falls through to normal Shield evaluation.
2. After stripping an optional `cd <absolute-path> && ` prefix, the first whitespace-separated token (lowercased and stripped of `.exe` on Windows) is in the platform-appropriate allowlist.

When a command qualifies, the gateway returns ALLOW with confidence 1.0. No Tier 0 policy check, no Tier 1 classifier, no Tier 2 evaluator. The user wins back the latency and tokens of an LLM call on every routine `git status`, `npm install`, `make build`, `go test`, `pwd`, `whoami`.

The allowlist intentionally excludes commands that take arbitrary path arguments (`cat`, `ls`, `head`, `tail`, `grep`, `find`, `rm`, `cp`, `mv` on Unix; `type`, `dir`, `findstr` on Windows). Those commands go through Shield's normal pipeline so the heuristic and Tier 2 layers can evaluate the actual targets.

The allowlist is curated and ships in the binary. It is not user-extensible. Adding to it requires a code change and a release.

A representative subset of the Unix allowlist:

```
git, hg, svn
npm, pnpm, yarn, npx, bun, deno, node
pip, pip3, poetry, python, python3
cargo, rustc, rustup
go, gofmt
make, cmake, ninja, bazel
mvn, gradle, java, javac
docker, docker-compose, kubectl, helm, podman
pwd, whoami, hostname, date, id, uname, echo, printf
df, du, free, ps, top, lsof, netstat, ss
which, whereis
```

Windows includes the same external tools (`git`, `npm`, etc.) plus cmd.exe builtins like `tasklist`, `ipconfig`, `systeminfo`, `where`. PowerShell is not exposed as a separate shell tool — `powershell.exe` as a first token is not in the allowlist and falls through to normal evaluation, which is the correct behavior since PowerShell scripts are arbitrary code.

Caller-imposed `MinTier` overrides take precedence: if a caller has set `MinTier > 0`, the fast path is skipped and the action goes through normal evaluation regardless of whether the command would otherwise qualify.

## Next Steps

- [Tier 0 -- Policy](/shield/tier0) -- how the policy engine evaluates rules
- [Configuration](/shield/configuration) -- how to set the policy file path
- [Go library](/shield/go) -- using the policy engine directly
