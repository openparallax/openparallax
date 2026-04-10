# Non-Negotiable Defenses

These security mechanisms are structurally fixed in the compiled binary. They cannot be disabled via `config.yaml`, environment variables, `/config set`, or any runtime API. The defense exists because its absence would undermine the architectural guarantee the system makes — and allowing it to be turned off, even by the user, creates a path an attacker (or a prompt injection masquerading as a user) could exploit.

If a mechanism appears on this page, the only way to disable it is to modify and recompile the source code. That is intentional.

## The mechanisms

- **Shield pipeline** — the 4-tier evaluation pipeline (Tier 0 policy, Tier 1 heuristic/classifier, Tier 2 LLM evaluator, Tier 3 human approval) cannot be bypassed. Individual tiers can be configured (policy presets, evaluator model) but the pipeline as a whole is always active.
- **Protection layer** — the hardcoded pre-Shield gate that enforces workspace boundaries, system path denylist, and file-level protection (SOUL.md read-only, config.yaml hard-blocked). Non-configurable.
- **Cross-platform default denylist** — the curated set of restricted/protected paths (`~/.ssh`, `~/.aws`, `/etc/shadow`, etc.) ships in the binary and is not user-extensible. The denylist only grows; it never shrinks.
- **Kernel sandbox** — the agent process is sandboxed via Landlock (Linux), sandbox-exec (macOS), or Job Objects (Windows). Best-effort: if the platform does not support sandboxing, the agent starts without it but logs the gap.
- **Hash verifier** — every action is hashed at proposal time and re-verified at execution time. Prevents TOCTOU (time-of-check-to-time-of-use) attacks.
- **Canary token enforcement** — the Tier 2 evaluator embeds a canary token in its prompt and verifies it in the response. Prevents prompt injection from forging evaluator verdicts.
- **Audit chain** — the append-only JSONL audit log with SHA-256 hash chain cannot be disabled. Every security-relevant event is logged; tampering breaks the chain and is detectable via `openparallax audit --verify`.
- **Agent authentication** — the agent process must present a per-spawn auth token when connecting to the engine over gRPC. Prevents rogue processes from impersonating the agent.
- **IFC subsystem** — the Information Flow Control subsystem is always active. The *policy* (which sources map to which sensitivity levels, which sinks are blocked) is user-tunable via `security/ifc/*.yaml`. The subsystem itself — the fact that data classification and flow checks happen — is not disableable.
- **Safe-command allowlist** — the curated list of commands that bypass Shield (git, npm, make, go, etc.) is compiled into the binary and is not user-extensible. Users cannot add their own commands to the fast path.

## Forbidden Config Keys

The following config paths must never appear in `SettableKeys` (the set of keys mutable via `/config set`). This list is enforced by CI — see `internal/config/security_invariant_test.go`.

```
shield.enabled
shield.tier0.enabled
shield.tier1.enabled
protection.enabled
sandbox.enabled
sandbox.required
audit.enabled
audit.chain_enabled
ifc.enabled
denylist.enabled
hash_verifier.enabled
canary.enforced
agent_auth.required
security.override_mode
```

## What you CAN tune

The distinction is between the *mechanism* (non-negotiable) and its *parameters* (tunable):

| Non-negotiable | Tunable parameter | Where |
|---|---|---|
| Shield pipeline exists | Policy preset (default/permissive/strict) | `security/shield/*.yaml` |
| Shield pipeline exists | Evaluator model and provider | `roles.shield` in `config.yaml` |
| Shield pipeline exists | Daily Tier 2 budget | `general.daily_budget` in `config.yaml` |
| IFC subsystem exists | IFC policy preset, source/sink rules, mode | `security/ifc/*.yaml` |
| IFC subsystem exists | Override mode (audit/enforce) | `security.override_mode` in `config.yaml` |
| Rate limiting exists | Actions per minute | `general.rate_limit` in `config.yaml` |

See [Hardening](hardening.md) for the full tuning guide.
