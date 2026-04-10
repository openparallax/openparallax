# Hardening Guide

This guide covers the tunable security surfaces in OpenParallax — what knobs exist, what each preset means, and when to use which. For the list of mechanisms that *cannot* be tuned, see [Non-Negotiable Defenses](non-negotiable.md).

## Shield Policy Presets

Shield Tier 0 uses a YAML policy file to make deny/verify/allow decisions. Three presets ship in `security/shield/`:

| Preset | File | Philosophy |
|---|---|---|
| **default** | `security/shield/default.yaml` | Balanced. Blocks sensitive path access, requires evaluation for writes and commands, allows reads of workspace files. Suitable for most users. |
| **permissive** | `security/shield/permissive.yaml` | Minimal friction. Allows most actions, blocks only dangerous paths. The agent can work freely with occasional Shield escalations. Best for trusted single-user setups where productivity matters more than compartmentalization. |
| **strict** | `security/shield/strict.yaml` | Maximum security. Requires Tier 2 LLM evaluation for any write or command. Generates frequent escalations. Best for regulated environments where the cost of a false negative (harmful action executed) far outweighs the cost of a false positive (action blocked). |

**Switch presets:**
```bash
openparallax policy set strict
openparallax restart
```

**Create a custom policy:**
```bash
cp security/shield/default.yaml security/shield/my-policy.yaml
# Edit my-policy.yaml
openparallax policy set my-policy
openparallax restart
```

See [Policy Syntax](/reference/policy-syntax) for the full rule reference.

## IFC Policy Presets

Information Flow Control uses a YAML policy to classify data sources and control flows to sinks. Three presets ship in `security/ifc/`:

| Preset | File | Philosophy |
|---|---|---|
| **default** | `security/ifc/default.yaml` | Blocks credentials from external sinks (HTTP, email, messages). Escalates restricted data to human approval. Allows workspace reads, writes, and memory for most levels. |
| **permissive** | `security/ifc/permissive.yaml` | Only critical data (SSH keys, `.env` files, cloud credentials) is blocked. Everything else flows freely. |
| **strict** | `security/ifc/strict.yaml` | Includes medical, legal, and financial document patterns. Confidential data requires human approval for writes. Restricted data is hard-blocked from writes and execution. |

**How to choose:**

- **Individual developer, personal workstation:** permissive IFC + default Shield. The agent works freely; credentials are still protected.
- **Team environment, shared infrastructure:** default IFC + default Shield. Balanced protection without friction.
- **Healthcare, finance, legal:** strict IFC + strict Shield. Maximum compartmentalization for regulated data.
- **Penetration testing, security research:** permissive IFC + permissive Shield. You know what you're doing and accept the risk.

**Switch IFC presets** by editing `config.yaml`:
```yaml
security:
  ifc_policy: security/ifc/permissive.yaml
```
Then restart the engine.

## Audit Mode (Shadow Rollout)

Both Shield and IFC support a shadow mode where violations are logged but not enforced:

```yaml
# In config.yaml — overrides the IFC policy's declared mode
security:
  override_mode: audit
```

In audit mode:
- The IFC subsystem classifies data and evaluates flows as usual
- Violations are logged as `IFCAuditWouldBlock` events in the audit chain
- The `ifc_audit_would_block` metric increments in the dashboard
- **The action is allowed to proceed**

Use this to preview what a stricter policy would block before enabling enforcement:

1. Set `override_mode: audit` and switch to the strict preset
2. Work normally for a day
3. Check the audit log: `openparallax audit` — look for `IFCAuditWouldBlock` events
4. Adjust the policy (add exclusions, tune source rules)
5. When satisfied, remove `override_mode` (or set to `enforce`) and restart

## Rate Limits and Budgets

| Setting | Default | What it controls |
|---|---|---|
| `general.rate_limit` | 30 | Actions per minute (global) |
| `general.daily_budget` | 100 | Tier 2 LLM evaluator calls per day |
| `general.verdict_ttl_seconds` | 60 | How long a Shield verdict is cached |
| `shield.tier3.max_per_hour` | — | Human approval requests per hour |

**When to increase `rate_limit`:** Heavy sub-agent fan-outs or rapid tool-call sequences. If you see frequent "rate limit exceeded" in the dashboard, consider raising to 60.

**When to increase `daily_budget`:** Workspaces with strict policies that escalate many actions to Tier 2. The budget prevents unbounded LLM spend; raise it if you're comfortable with the cost.

## Security Dashboard

The web UI's Security tab shows real-time metrics across all defense layers:

1. **Security Integrity** — canary counters that should always be zero. Non-zero = investigate immediately.
2. **Shield Decisions** — allow/block/escalate distribution.
3. **Defense Activity** — protection layer blocks, Tier 3 requests, sub-agent cap hits.
4. **IFC** — classification and flow-control activity.
5. **Tool Execution** — total calls, success rate.

The Integrity panel uses red borders on non-zero values. If you see any red, check the audit trail immediately.

## Verifying Your Setup

```bash
# Health check — includes sandbox status, Shield config, IFC policy
openparallax doctor

# Verify audit chain integrity
openparallax audit --verify

# List active Shield policy
openparallax policy list

# Show active Shield policy rules
openparallax policy show
```

## Related Documentation

- [Shield Pipeline](/shield/) — detailed Tier 0–3 documentation
- [Policy Syntax](/reference/policy-syntax) — Shield policy rule reference
- [IFC Reference](ifc.md) — full IFC schema specification
- [Non-Negotiable Defenses](non-negotiable.md) — what cannot be configured
- [Practical Security Guide](/guide/hardening) — workflow-focused hardening
