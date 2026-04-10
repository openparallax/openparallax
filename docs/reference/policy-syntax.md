# Policy Syntax Reference

Shield policies are YAML files that define rules for Tier 0 evaluation. Each rule matches against action properties and determines whether the action should be allowed, blocked, or escalated to a higher tier.

## File Location

Policies live in the workspace's `security/shield/` directory. The active policy is specified in `config.yaml`:

```yaml
shield:
  policy_file: security/shield/default.yaml
```

OpenParallax ships with three policy presets:

| Policy | File | Philosophy |
|--------|------|-----------|
| **Default** | `security/shield/default.yaml` | Balanced — allows reads, requires evaluation for writes and commands |
| **Permissive** | `security/shield/permissive.yaml` | Minimal friction — allows most actions, blocks only dangerous paths |
| **Strict** | `security/shield/strict.yaml` | Maximum security — requires Tier 2 for any write or command |

## Schema

```yaml
# Top-level structure
version: 1                    # Policy format version (currently always 1)
description: "Policy name"    # Human-readable description

# Default behavior when no rule matches
default:
  decision: ESCALATE          # ALLOW | BLOCK | ESCALATE
  min_tier: 1                 # Minimum tier required (0, 1, or 2)

# Rules are evaluated in order — first match wins
rules:
  - name: rule-name           # Unique identifier (lowercase, hyphens)
    description: "Why"        # Human-readable explanation
    action_types:             # List of action types to match
      - read_file
      - list_directory
    path_patterns:            # Glob patterns for file paths (optional)
      - "src/**"
      - "*.go"
    path_deny_patterns:       # Glob patterns to exclude (optional)
      - "/etc/**"
      - "~/.ssh/**"
    content_patterns:         # Regex patterns matching action content (optional)
      - "rm -rf"
      - "DROP TABLE"
    decision: ALLOW           # ALLOW | BLOCK | ESCALATE
    min_tier: 0               # Minimum tier before this decision applies
```

## Field Reference

### `version`

Always `1`. Reserved for future schema evolution.

### `description`

Human-readable description of the policy's purpose. Shown in `openparallax status` and the web UI.

### `default`

The fallback rule when no explicit rule matches an action.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `decision` | string | yes | What to do with unmatched actions. Usually `ESCALATE` (send to higher tiers for evaluation). |
| `min_tier` | int | no | Minimum tier required before the default decision applies. Default `0`. |

::: warning Fail-Closed Default
If you set `default.decision: ALLOW`, unmatched actions skip all security evaluation. This is dangerous. The recommended default is `ESCALATE` with `min_tier: 1` — unmatched actions go through at least the heuristic/classifier before being allowed.
:::

### `rules`

An ordered list of rules. **First match wins** — once an action matches a rule, no further rules are evaluated. Order your rules from most specific to most general.

### Rule Fields

#### `name` (required)

Unique identifier for the rule. Lowercase letters, numbers, and hyphens. Used in audit logs and Shield verdicts to identify which rule matched.

#### `description` (optional)

Human-readable explanation of why this rule exists. Included in Shield verdict messages.

#### `action_types` (required)

List of action types this rule applies to. Must match values from the [Action Types](/reference/actions) reference.

```yaml
# Match specific actions
action_types:
  - read_file
  - list_directory
  - search_files

# Match all actions (use with caution)
action_types:
  - "*"
```

The wildcard `"*"` matches all action types. Use it for catch-all rules (typically the last rule before the default).

#### `path_patterns` (optional)

Glob patterns that match against the action's file path. If specified, the rule only matches when the action's path matches at least one pattern.

```yaml
# Match Go source files
path_patterns:
  - "**/*.go"
  - "**/*.mod"

# Match everything in the workspace
path_patterns:
  - "**"

# Match specific directories
path_patterns:
  - "src/**"
  - "internal/**"
```

Glob syntax:
- `*` — matches any characters within a single path segment
- `**` — matches any number of path segments (including zero)
- `?` — matches a single character
- `[abc]` — matches any character in the set
- `{go,rs}` — matches any of the alternatives

#### `path_deny_patterns` (optional)

Glob patterns that exclude paths from matching. If an action's path matches a deny pattern, the rule does not match — even if it matches a `path_patterns` entry.

```yaml
# Allow reads everywhere except sensitive paths
path_patterns:
  - "**"
path_deny_patterns:
  - "/etc/shadow"
  - "/etc/passwd"
  - "~/.ssh/**"
  - "**/.env"
  - "**/.env.*"
  - "**/credentials*"
```

Deny patterns are evaluated after allow patterns. Think of it as: "match these paths, but NOT these paths."

#### `content_patterns` (optional)

Regular expressions that match against the action's content (arguments, command text, file content being written). If specified, the rule only matches when at least one pattern matches the content.

```yaml
# Block destructive commands
content_patterns:
  - "rm\\s+-rf"
  - "DROP\\s+TABLE"
  - "DELETE\\s+FROM.*WHERE\\s+1\\s*=\\s*1"
  - "mkfs"
  - "dd\\s+if="
  - "> /dev/sd"

# Block credential exfiltration attempts
content_patterns:
  - "curl.*\\$\\{?[A-Z_]*KEY"
  - "wget.*api[_-]?key"
  - "echo.*\\$\\{?[A-Z_]*SECRET"
```

Content patterns use Go's `regexp` syntax (RE2). They are case-sensitive by default. Use `(?i)` for case-insensitive matching:

```yaml
content_patterns:
  - "(?i)drop\\s+table"   # matches DROP TABLE, Drop Table, etc.
```

#### `decision` (required)

What Shield should do when this rule matches:

| Decision | Effect |
|----------|--------|
| `ALLOW` | Action is permitted. Skips higher tiers (subject to `min_tier`). |
| `BLOCK` | Action is denied. No execution, no higher tier evaluation. Immediate and final. |
| `ESCALATE` | Pass to the next tier for further evaluation. |

#### `min_tier` (optional)

Minimum tier that must have evaluated the action before this rule's decision applies. Default `0` (Tier 0 can make the decision).

| Value | Meaning |
|-------|---------|
| `0` | Tier 0 (this policy rule) can make the final decision. |
| `1` | Even if this rule matches, the action must also pass Tier 1 (classifier + heuristic) before the decision is applied. If this rule says ALLOW with min_tier 1, the action is escalated to Tier 1 first. |
| `2` | Action must pass Tier 2 (LLM evaluator) regardless of this rule's decision. |

This is crucial for defense in depth. Even if a policy rule allows an action, `min_tier: 1` ensures the classifier still checks for prompt injection:

```yaml
# Allow shell commands, but require classifier check
- name: allow-shell-with-check
  action_types: [run_command]
  decision: ALLOW
  min_tier: 1    # Classifier must also approve
```

## Evaluation Flow

```
Action arrives at Tier 0
  ↓
For each rule (in order):
  1. Does action_type match? No → next rule
  2. Does path match path_patterns? No → next rule
  3. Does path match path_deny_patterns? Yes → next rule
  4. Does content match content_patterns? No → next rule
  5. Match found!
     ↓
  Is decision BLOCK? → BLOCK immediately
  Is decision ALLOW and min_tier ≤ 0? → ALLOW immediately
  Is decision ALLOW but min_tier > 0? → ESCALATE to next tier
  Is decision ESCALATE? → ESCALATE to next tier
  ↓
No rule matched → apply default
```

## Examples

### Default Policy (Balanced)

```yaml
version: 1
description: "Balanced security — allows reads, evaluates writes and commands"

default:
  decision: ESCALATE
  min_tier: 1

rules:
  # Read operations are always allowed
  - name: allow-reads
    description: "File reads and searches are safe"
    action_types:
      - read_file
      - list_directory
      - search_files
      - git_status
      - git_diff
      - git_log
      - memory_search
      - read_email
      - search_email
      - read_calendar
      - list_tasks
    decision: ALLOW
    min_tier: 0

  # Block access to sensitive system paths
  - name: block-system-paths
    description: "Never access system-critical files"
    action_types: ["*"]
    path_patterns:
      - "/etc/shadow"
      - "/etc/passwd"
      - "~/.ssh/**"
      - "**/.env"
      - "**/.env.*"
      - "**/credentials*"
      - "**/*secret*"
    decision: BLOCK

  # Block destructive commands
  - name: block-destructive
    description: "Block known-dangerous commands"
    action_types: [run_command]
    content_patterns:
      - "rm\\s+-rf\\s+/"
      - "mkfs"
      - "dd\\s+if="
      - "> /dev/sd"
      - "chmod\\s+777"
      - "curl.*\\|.*sh"
    decision: BLOCK

  # Browser navigation — allow with classifier check
  - name: allow-browser
    action_types:
      - browser_navigate
      - browser_extract
    decision: ALLOW
    min_tier: 1

  # Shell commands — require classifier
  - name: shell-needs-classifier
    action_types: [run_command]
    decision: ALLOW
    min_tier: 1

  # File writes — require classifier
  - name: writes-need-classifier
    action_types:
      - write_file
      - edit_file
      - delete_file
      - move_file
    decision: ALLOW
    min_tier: 1

  # Email sending — require Tier 2 (LLM evaluator)
  - name: email-needs-evaluator
    description: "Sending email is high-risk — require human-level evaluation"
    action_types: [send_email]
    decision: ALLOW
    min_tier: 2
```

### Strict Policy

```yaml
version: 1
description: "Maximum security — Tier 2 for all writes and commands"

default:
  decision: BLOCK

rules:
  - name: allow-reads
    action_types:
      - read_file
      - list_directory
      - search_files
      - git_status
      - git_diff
      - git_log
      - memory_search
    decision: ALLOW
    min_tier: 0

  - name: all-writes-tier2
    action_types: ["*"]
    decision: ALLOW
    min_tier: 2
```

### Permissive Policy

```yaml
version: 1
description: "Minimal friction — allows most actions, blocks only dangerous paths"

default:
  decision: ALLOW
  min_tier: 1

rules:
  - name: allow-reads
    action_types:
      - read_file
      - list_directory
      - search_files
      - git_status
      - git_diff
      - git_log
    decision: ALLOW
    min_tier: 0

  - name: block-system
    action_types: ["*"]
    path_patterns:
      - "/etc/**"
      - "~/.ssh/**"
    decision: BLOCK
```

## Testing Policies

You can test a policy without running the full agent:

```bash
# Dry-run: check what the policy would do for a specific action
openparallax shield test \
  --policy security/shield/default.yaml \
  --action read_file \
  --path src/main.go

# Output: ALLOW (rule: allow-reads, tier: 0)
```

```bash
# Test a dangerous action
openparallax shield test \
  --policy security/shield/default.yaml \
  --action run_command \
  --content "rm -rf /"

# Output: BLOCK (rule: block-destructive)
```

## Best Practices

1. **Start with the default policy.** It's balanced and covers common cases. Customize from there.
2. **Put BLOCK rules before ALLOW rules.** First match wins — block dangerous paths before allowing general reads.
3. **Use `min_tier` for defense in depth.** Even if a rule allows an action, requiring Tier 1 ensures the classifier checks for prompt injection.
4. **Be specific with `path_deny_patterns`.** Broad deny patterns can accidentally block legitimate workspace files.
5. **Use content patterns for commands.** Path patterns don't help with `run_command` — the danger is in the command text, not a file path.
6. **Test after changes.** Use `openparallax shield test` to verify your rules match as expected.
7. **Keep the default as ESCALATE.** Setting `default.decision: ALLOW` bypasses security for unmatched actions. Only use in development.
