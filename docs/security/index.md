# Security Architecture

OpenParallax implements the Parallax security paradigm: **the system that reasons cannot execute, the system that executes cannot reason, and an independent validator sits between them.** This is structural enforcement, not behavioral — it does not depend on the model's safety training, the prompt, or runtime checks the model could bypass.

This section documents every security mechanism in the system, organized by what category of threat each one mitigates.

## Defense Mechanism Map

| Group | Mechanisms | What it protects against |
|---|---|---|
| [Structural Isolation](structural-isolation.md) | Cognitive-executive separation, kernel sandbox, sub-agent recursion guard | A compromised LLM directly executing harmful actions |
| [Action Validation](action-validation.md) | Shield (4 tiers), safe-command fast path, protection layer, denylist, IFC, hash verifier | Harmful tool calls proposed by the LLM reaching execution |
| [State Protection](state-protection.md) | Chronicle, audit chain, memory isolation, OTR mode | Unrecoverable state corruption, evidence tampering, data persistence leaks |
| [Input/Output Safety](input-output-safety.md) | Output sanitization, redaction, SSRF protection, HTTP header denylist, git flag injection prevention, tool call ID sanitization, identity validation, Ollama loopback restriction, setup wizard allowlist | Prompt injection, data exfiltration, credential leakage |
| [Resource Limiting](resource-limiting.md) | Rate limit, Tier 2 budget, verdict TTL, Tier 3 hourly cap, crash budget, sub-agent caps, reasoning loop rounds | Resource exhaustion, runaway loops, denial of service |
| [Cryptographic Foundations](cryptographic-foundations.md) | Action hashing, canary tokens, agent auth token, audit hash chain | Tampering, impersonation, replay attacks |

## Configurable vs Non-Negotiable

Some mechanisms are structurally fixed in the compiled binary — they cannot be disabled via configuration, environment variables, or any runtime API. Others have tunable parameters (policy presets, rate limits, budgets) while the mechanism itself remains active.

See [Non-Negotiable Defenses](non-negotiable.md) for the full inventory of fixed mechanisms, and [Hardening](hardening.md) for the tuning guide.

## Threat Model

The [Threat Model](threat-model.md) maps each defense to specific threats from OWASP Top 10, OWASP Top 10 for LLM Applications, MITRE ATLAS, and CWE.

## Information Flow Control

The [IFC subsystem](ifc.md) prevents sensitive data from crossing sensitivity boundaries through three mechanisms: per-action classification from policy rules and a persistent activity table, session taint that propagates the highest sensitivity seen to all subsequent actions, and content sensitivity tags that carry classification metadata through the LLM turn. IFC also gates memory writes — data classified at configurable sensitivity levels cannot persist to agent memory, preventing cross-session data laundering via the memory system.

## Related Documentation

- [Shield Pipeline](/shield/) — detailed Tier 0–3 documentation
- [Sandbox](/sandbox/) — kernel-level process isolation
- [Audit](/audit/) — append-only hash-chained logging
- [Hardening Guide](/guide/hardening) — practical tuning workflows
