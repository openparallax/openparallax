# Action Validation

These mechanisms evaluate every tool call proposed by the agent before it reaches execution. They form the gateway between "the LLM wants to do X" and "X actually happens."

## Shield Pipeline (4 Tiers)

**Threat:** The LLM proposing harmful tool calls — reading sensitive files, executing destructive commands, exfiltrating data via HTTP, modifying system configuration.

**Defense:** Every tool call passes through a 4-tier evaluation pipeline. Tiers are structurally heterogeneous — defeating one does not defeat the others.

| Tier | Name | Mechanism | Speed | Configurable |
|---|---|---|---|---|
| 0 | Policy Engine | YAML pattern matching (deny/verify/allow rules) | <1ms | Yes — `security/shield/*.yaml` presets |
| 1 | Heuristic Classifier | 79-rule regex engine + optional ONNX DeBERTa model | <5ms | Partially — ONNX opt-in, heuristics always on |
| 2 | LLM Evaluator | Independent LLM with no agent state; sees action as JSON data | 1–3s | Yes — model, budget, provider |
| 3 | Human Approval | Broadcasts to all connected channels; first response wins | Minutes | Yes — hourly cap, timeout |

A financial analyst's agent proposing to read `~/tax_returns/2024.pdf` passes through Tier 0 (no deny rule), Tier 1 (heuristic finds no destructive pattern), and proceeds. The same agent proposing to email that file triggers a Tier 2 evaluation and potentially Tier 3 human approval.

**Relevant threats:**
- OWASP LLM01: Prompt Injection (attacker tricks agent into proposing harmful actions)
- OWASP LLM06: Excessive Agency
- MITRE ATLAS T0051: LLM Prompt Injection
- CWE-77: Command Injection

**Code:** `shield/` (full pipeline), `shield/gateway.go` (orchestrator), `shield/tier0_policy.go`, `shield/tier1_*.go`, `shield/tier2_*.go`

**Non-negotiable** (the pipeline exists and runs). **Tunable** (policy presets, evaluator model, budget). See [Shield documentation](/shield/) for full reference.

## Safe-Command Fast Path

**Threat:** Unnecessary latency on known-safe developer commands (git status, npm install, make, go build) that waste Tier 2 LLM tokens.

**Defense:** A curated, compiled-in allowlist of command prefixes bypasses all four Shield tiers and returns ALLOW with confidence 1.0. Single-statement commands only — anything with shell metacharacters (`;`, `&`, `|`, `>`, `<`, `` ` ``, `$(...)`) falls through to normal evaluation. The allowlist excludes commands that take arbitrary path arguments (`cat`, `ls`, `rm`, `cp`, `mv`).

**Relevant threats:** Performance, not security — the allowlist only *permits*, never blocks.

**Code:** `shield/safe_commands.go`, `platform/safe_commands_{unix,windows}.go`

**Non-negotiable.** The allowlist is compiled into the binary and is not user-extensible.

## Hardcoded Protection Layer

**Threat:** Modification of the agent's own identity, configuration, or security-critical files — either through direct LLM action or through a prompt injection that targets internal files.

**Defense:** A pre-Shield gate that enforces workspace boundaries and file-level protection. Runs before Tier 0 and cannot be bypassed:

| File | Protection | Effect |
|---|---|---|
| `soul.md`, `identity.md` | Read-only | Agent can read for context; writes are blocked |
| `config.yaml`, `canary.token`, `audit.jsonl`, `openparallax.db` | Full block | No read, no write |
| `.openparallax/` directory | Full block | Entire internal directory sealed |
| `security/` directory | Full block | Shield and IFC policies sealed from agent writes |
| `skills/` directory | Read-only | Agent loads skills; cannot modify them |

**Relevant threats:**
- OWASP A08: Software and Data Integrity Failures
- MITRE ATLAS T0042: Verify Attack (modifying verification mechanisms)

**Code:** `internal/engine/protection.go`

**Non-negotiable.**

## Cross-Platform Default Denylist

**Threat:** The agent accessing credential files, SSH keys, cloud provider configs, or system secrets anywhere on disk — not just within the workspace.

**Defense:** A curated denylist of restricted and protected paths applies to any path the agent touches. The denylist runs after symlink resolution — a symlink in `/tmp/safe.txt` pointing at `~/.ssh/id_rsa` is blocked.

Categories:
- **Restricted** (no read, no write): `~/.ssh`, `~/.aws`, `~/.gnupg`, `~/.docker`, `~/.kube`, `/etc/shadow`, credential files by extension (`.pem`, `.key`, `.p12`, etc.)
- **Protected** (read OK, write blocked): shell rc files (`.bashrc`, `.zshrc`), VCS configs (`.gitconfig`, `.npmrc`), system reference files (`/etc/hosts`, `/etc/passwd`)

An operations engineer managing infrastructure can trust that even if the agent is asked to "read the AWS credentials to check the region," the denylist blocks `~/.aws/credentials` before Shield even evaluates the request.

**Relevant threats:**
- OWASP LLM02: Sensitive Information Disclosure
- CWE-256: Plaintext Storage of Password
- CWE-200: Information Exposure

**Code:** `platform/denylist_{linux,darwin,windows}.go`, `internal/engine/protection.go`

**Non-negotiable.** The denylist ships in the binary and is not user-extensible.

## Information Flow Control (IFC)

**Threat:** Data at one sensitivity level (e.g., credentials, financial records, patient data) flowing to an action that would expose it to a lower-trust destination (e.g., HTTP request, email, shell command).

**Defense:** A YAML-driven policy that classifies data sources by sensitivity and controls which sink categories each level can flow to. Three presets ship with OpenParallax:

| Preset | Philosophy | Who it's for |
|---|---|---|
| `default` | Blocks credentials from external sinks; escalates restricted data | Most users |
| `permissive` | Only blocks critical data; everything else flows freely | Trusted single-user workstations |
| `strict` | Blocks confidential+ data from writes and exec; default-deny | Regulated environments (healthcare, finance, legal) |

See [IFC reference](ifc.md) for the full schema and examples.

**Relevant threats:**
- OWASP LLM02: Sensitive Information Disclosure
- CWE-200: Information Exposure
- CWE-668: Exposure of Resource to Wrong Sphere

**Code:** `ifc/policy.go`, `security/ifc/*.yaml`

**The IFC subsystem is non-negotiable** (it always runs). **The policy is fully tunable** via preset selection or custom YAML rules.

## Hash Verifier (TOCTOU Prevention)

**Threat:** An action being modified between the time it's proposed (and evaluated by Shield) and the time it's executed — a time-of-check-to-time-of-use attack.

**Defense:** Every action is hashed (SHA-256 of tool name + arguments) at proposal time. Before execution, the hash is recomputed and compared. A mismatch blocks execution.

**Relevant threats:**
- OWASP A08: Software and Data Integrity Failures
- CWE-367: Time-of-Check Time-of-Use Race Condition

**Code:** `internal/engine/verifier.go`, `crypto/hash.go`

**Non-negotiable.**
