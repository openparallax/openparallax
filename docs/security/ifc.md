# Information Flow Control

IFC prevents sensitive data from flowing to destinations where it shouldn't go. When the agent reads a credential file, that data is tagged with a sensitivity level. When a subsequent action attempts to send that data via email or HTTP, IFC blocks the flow — even if Shield approved the action.

## How It Works

1. **Classify.** When the agent proposes an action, IFC classifies the data source (file path) using the policy's source rules. The first matching rule wins.

2. **Decide.** IFC looks up the action type in the policy's sink categories, then checks the decision matrix: for this sensitivity level × this sink category, is the decision `allow`, `block`, or `escalate`?

3. **Enforce or audit.** In `enforce` mode, blocks are real — the action is stopped and logged. In `audit` mode, blocks are logged but the action proceeds. Escalations route to Shield Tier 3 (human approval).

## Schema

IFC policies live in `security/ifc/` as YAML files. The schema has four sections:

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
  workspace_write:
    - write_file
    - memory_write
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

**Unknown action types** (MCP tools, future built-ins) that don't appear in any sink category are always allowed. This denylist design ensures new tools work without policy updates.

### rules

The decision matrix. For each sensitivity level × sink category: `allow`, `block`, or `escalate`.

```yaml
rules:
  public:
    external: allow
    exec: allow
    workspace_write: allow
    workspace_read: allow
  critical:
    external: block
    exec: block
    workspace_write: block
    workspace_read: block
```

`escalate` routes the action to Shield Tier 2 (LLM evaluation) and potentially Tier 3 (human approval).

## Worked Examples

### Example 1: Reading a credential file, then trying to email it

A financial analyst asks the agent to "read the API keys from `.env` and email them to the team."

1. Agent proposes `read_file(.env)` — IFC classifies `.env` as **critical** (source rule: `env_files`). Shield evaluates and blocks at Tier 0 (denylist).
2. Even if the read somehow succeeded, the subsequent `send_email` with critical data → external sink → **block** in the decision matrix.

### Example 2: Writing a Go test file that mentions DROP TABLE

A developer asks the agent to write a database test helper.

1. Agent proposes `write_file(testhelpers/db.go, "... DROP TABLE users ...")`.
2. IFC classifies `testhelpers/db.go` as **public** (no source rule matches the path).
3. Public → workspace_write → **allow**. The file is written.
4. The old enricher would have flagged "DROP TABLE" in the content as destructive. The policy-driven system doesn't scan file content — it classifies by path, not payload.

### Example 3: Handling patient records in a healthcare setting

A clinic's agent is asked to "summarize the patient intake form and add it to the report."

With the **strict** preset:
1. Agent reads `patient-intake-2024.pdf` — classified as **restricted** (basename contains "patient").
2. Agent proposes `write_file(report.md, ...)` — restricted → workspace_write → **block** (strict preset blocks restricted writes).
3. The action is blocked. The agent reports that the data is too sensitive to write without human approval.

With the **default** preset:
1. Same classification: restricted.
2. restricted → workspace_write → **escalate** (default preset escalates restricted writes to human approval).
3. The human approves, and the report is written.

### Example 4: Drafting a contract email

A legal team's agent is asked to "draft an email summarizing the NDA terms."

With the **strict** preset:
1. Agent reads `nda-acme-2024.pdf` — classified as **restricted** (basename contains "nda").
2. Agent proposes `send_email(...)` — restricted → external → **block**.
3. The agent can summarize the NDA in chat but cannot send it externally.

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
    workspace_write: allow # was: escalate
    workspace_read: allow
```

**Preview changes with audit mode:**
```yaml
mode: audit    # log but don't block
```

## Related Documentation

- [Security Architecture](index.md) — how IFC fits into the full defense map
- [Non-Negotiable Defenses](non-negotiable.md) — the IFC *subsystem* is non-negotiable; the *policy* is tunable
- [Hardening Guide](hardening.md) — which preset to use when
- [Threat Model](threat-model.md) — OWASP/MITRE mapping for IFC-addressed threats
