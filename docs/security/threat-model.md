# Threat Model

This document maps OpenParallax's defenses to specific threats from industry-standard security frameworks. For each threat, we identify which mechanisms address it and honestly acknowledge gaps.

## Referenced Frameworks

- **OWASP Top 10 (2021)** — web application security risks
- **OWASP Top 10 for LLM Applications (2025)** — risks specific to LLM-integrated systems
- **MITRE ATLAS** — adversarial threat landscape for AI systems
- **CWE** — common weakness enumeration (specific vulnerability types)

## OWASP Top 10 for LLM Applications

### LLM01: Prompt Injection

An attacker crafts input that causes the LLM to act as the attacker's proxy — executing unauthorized actions, leaking data, or overriding its instructions.

| Defense | How it helps |
|---|---|
| Cognitive-executive separation | Even a fully jailbroken LLM can only *propose* actions; it cannot execute them |
| Shield 4-tier pipeline | Every proposed action is independently evaluated; the LLM cannot bypass evaluation |
| Tier 2 inverted context | The evaluator LLM has no agent identity, no conversation history — it cannot be injection-chained |
| Canary token verification | Prevents prompt injection from forging evaluator verdicts |
| Output sanitization | Explicit data boundaries reduce indirect injection via tool results |
| IFC | Even if injection succeeds in proposing an action, data flow controls prevent credential exfiltration |

**Gap:** Social engineering of the human at Tier 3. If the attacker can craft a convincing approval prompt, the human may approve a harmful action. Mitigation: Tier 3 shows the raw action payload, not the LLM's summary.

### LLM02: Sensitive Information Disclosure

The LLM reveals sensitive data — credentials, personal information, internal system details — through its responses or actions.

| Defense | How it helps |
|---|---|
| IFC | Classifies data by sensitivity and blocks flows to external sinks |
| Kernel sandbox | Prevents the agent from reading files outside its workspace |
| Default denylist | Blocks access to credential directories (`~/.ssh`, `~/.aws`, etc.) |
| Secret redaction | Strips credential patterns from LLM output streams |
| OTR mode | Prevents sensitive conversations from persisting to disk |

### LLM06: Excessive Agency

The LLM takes actions beyond what the user intended — deleting files, modifying system config, sending unauthorized messages.

| Defense | How it helps |
|---|---|
| Cognitive-executive separation | The agent cannot act; it can only propose |
| Shield pipeline | Every proposal is evaluated before execution |
| Protection layer | Hard-blocks modifications to identity, config, audit, and security files |
| Default denylist | Prevents access to system-critical paths |
| Chronicle snapshots | Destructive actions can be rolled back |

### LLM10: Unbounded Consumption

The system consumes excessive resources — token spend, compute, network, storage — due to LLM loops, recursive delegation, or misconfiguration.

| Defense | How it helps |
|---|---|
| Rate limit | Caps actions per minute |
| Tier 2 daily budget | Caps expensive LLM evaluations per day |
| Sub-agent concurrency cap | Limits parallel delegation |
| Sub-agent timeout | Kills long-running sub-agents |
| Reasoning loop max rounds | Stops infinite tool-call loops |
| Crash restart budget | Prevents restart loops |

## OWASP Top 10 (2021) — Selected

### A01: Broken Access Control

| Defense | How it helps |
|---|---|
| Kernel sandbox | OS-level access control on the agent process |
| Default denylist | Path-level access control across the filesystem |
| Protection layer | File-level access control within the workspace |
| Agent auth token | Process-level authentication for gRPC connections |

### A03: Injection (CWE-77, CWE-78)

| Defense | How it helps |
|---|---|
| Shield Tier 1 heuristics | Pattern-matches command injection signatures |
| Git flag injection prevention | `--` separator stops flag smuggling |
| SSRF protection | Blocks internal IP injection via HTTP requests |

### A08: Software and Data Integrity Failures

| Defense | How it helps |
|---|---|
| Hash verifier | TOCTOU prevention via action hashing |
| Audit hash chain | Tamper-evident logging |
| Sealed security config | Digest-verified immutable config |
| Chronicle | Recoverable workspace state |

### A09: Security Logging and Monitoring Failures (CWE-778)

| Defense | How it helps |
|---|---|
| Audit chain | 28 event types covering the full pipeline lifecycle |
| Security dashboard | Real-time integrity canaries with alerting on non-zero values |
| Agent diagnostics | Exit code, signal, and stderr capture for crash investigation |

### A10: Server-Side Request Forgery (CWE-918)

| Defense | How it helps |
|---|---|
| SafeHTTPClient | Blocks private/loopback/link-local IPs at dial time |
| Redirect re-validation | Re-checks IP on every redirect hop |

## MITRE ATLAS — Selected Techniques

### T0051: LLM Prompt Injection

Covered by: cognitive-executive separation, Shield pipeline, Tier 2 inverted context, canary tokens, output sanitization. See LLM01 above.

### T0054: LLM Jailbreak

Covered by: same defenses as T0051. The key insight is that jailbreaking the agent's LLM is *insufficient* — the agent still cannot execute actions, and Shield evaluates independently.

### T0024: Exfiltration via ML Inference API

Covered by: kernel sandbox (network restrictions), IFC (blocks sensitive data → external sinks), SSRF protection (blocks internal IP access).

### T0048: External Harms

Covered by: cognitive-executive separation (the agent cannot cause harm because it cannot act), Shield pipeline (the engine evaluates before acting), Tier 3 (human approval for high-risk actions).

## Honest Gaps

- **Model-level data leakage.** If the LLM provider retains prompts for training, sensitive data in prompts could leak through the model. Mitigation: use providers with data retention policies; OTR mode reduces persistent data.
- **Social engineering at Tier 3.** A sophisticated attacker could craft an action that *looks* safe to a human reviewer but is harmful in context. Mitigation: Tier 3 shows raw payloads, not LLM summaries.
- **Side-channel timing attacks.** An attacker could infer information about Shield's evaluation by measuring response times. Not currently mitigated.
- **Supply chain attacks on the LLM itself.** If the LLM provider is compromised, the model could be instructed to generate harmful proposals at a rate that exhausts Shield's evaluation budget. Mitigation: budget limits, rate limits, and the structural impossibility of the agent executing directly.
