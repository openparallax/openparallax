# Security Policy

## Reporting Vulnerabilities

**Do not open a public issue for security vulnerabilities.**

OpenParallax uses **GitHub Private Vulnerability Reporting**. Reports are visible only to project maintainers, give you an audit trail, and integrate with CVE assignment and coordinated disclosure.

### How to Report

1. Go to [Security → Advisories → Report a vulnerability](https://github.com/openparallax/openparallax/security/advisories/new) on this repository
2. Fill in:
   - **Title** — short description
   - **Description** — what the vulnerability is and how it works
   - **Steps to reproduce** — minimal reproduction
   - **Affected versions** — release tag or commit hash
   - **Impact** — what an attacker could achieve
   - **Suggested fix** — if you have one

We will acknowledge your report within 48 hours and aim to provide a fix or mitigation within 7 days for critical issues.

### What Happens Next

- The report enters a private draft advisory only maintainers can see.
- We may invite you as a collaborator on a private fork to develop the fix together.
- Once the fix lands and a patched release is available, the advisory is published with credit to you (unless you prefer anonymity) and a CVE is requested through GitHub.

We do not maintain a security@ email alias. Private GitHub reporting is the only supported channel.

## Scope

The following are in scope for security reports:

- **Shield bypass** — any way to execute a tool call without proper Shield evaluation
- **Sandbox escape** — any way for the agent process to access resources outside its allowed scope
- **Audit tampering** — any way to modify audit log entries without breaking the hash chain
- **IFC bypass** — any way to exfiltrate data across sensitivity boundaries
- **Authentication bypass** — any way to access the web UI or gRPC services without proper credentials
- **Prompt injection** — novel techniques that bypass Shield's classifier and evaluator
- **Canary token theft** — any way for the agent to read or exfiltrate the canary token
- **File protection bypass** — any way to read or write FullBlock-protected files through the agent

The following are out of scope:

- Vulnerabilities in third-party LLM providers (Anthropic, OpenAI, Google APIs)
- Denial of service through excessive API calls (rate limiting is the user's responsibility)
- Social engineering attacks against the human operator
- Physical access attacks
- Vulnerabilities in dependencies that don't affect OpenParallax's usage of them

## Security Architecture

OpenParallax is designed with the assumption that the AI agent is untrusted:

### Defense in Depth

```
Layer 1: Kernel Sandbox    — Agent physically cannot access unauthorized resources
Layer 2: Shield Pipeline   — Every tool call evaluated before execution
Layer 3: IFC Labels        — Data flow constraints prevent exfiltration
Layer 4: File Protection   — Critical files blocked at the pipeline level
Layer 5: Chronicle         — Pre-write snapshots enable rollback
Layer 6: Audit Chain       — Tamper-evident logging for forensic analysis
Layer 7: Canary Tokens     — Verify LLM evaluator integrity
```

### Fail-Closed Design

Every security component fails closed:

- Shield error → BLOCK (action denied)
- Missing policy file → engine refuses to start
- Sandbox verification failure → agent refuses to start
- Canary verification failure → Tier 2 verdict rejected
- Audit hash chain break → detected and reported

### Process Isolation

The Agent process runs inside a kernel sandbox. On Linux (Landlock 5.13+), this means:

- No filesystem access outside the workspace (read-only)
- No network access (Landlock ABI v4+ on kernel 6.7+, best-effort on older kernels)
- No process spawning

The Engine process is privileged and unsandboxed — it is the only process that can execute actions, access the filesystem, and make network calls. The Agent can only propose actions; the Engine decides whether to execute them.

## Supported Versions

| Version | Security Updates |
|---------|:---------------:|
| Latest main | Yes |
| Previous release | 90 days after next release |

## Disclosure Policy

- We practice coordinated disclosure with a 90-day timeline
- We will credit reporters in the security advisory (unless they prefer anonymity)
- We will not pursue legal action against good-faith security researchers

## Security-Related Configuration

For production deployments:

```yaml
# Use the strict policy
shield:
  policy_file: security/shield/strict.yaml

# Use a separate provider for Shield evaluation (diversity)
shield:
  evaluator:
    provider: openai  # Different from chat provider

# Enable authentication for non-localhost web access
web:
  host: 0.0.0.0
  password_hash: "$2a$12$..."  # bcrypt hash

# Enable the ML classifier sidecar if available
# shield:
#   classifier_enabled: true
#   classifier_addr: localhost:8090
```

Run `openparallax doctor` regularly to verify your security posture.
