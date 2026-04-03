# Security Policy

## Reporting Vulnerabilities

**Do not open a public issue for security vulnerabilities.**

If you discover a security vulnerability in OpenParallax, please report it responsibly:

1. **Email:** security@openparallax.dev
2. **Subject:** `[SECURITY] Brief description`
3. **Include:**
   - Description of the vulnerability
   - Steps to reproduce
   - Affected versions (or commit hash)
   - Impact assessment (what an attacker could achieve)
   - Suggested fix, if you have one

We will acknowledge your report within 48 hours and aim to provide a fix or mitigation within 7 days for critical issues.

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

The Agent process runs inside a kernel sandbox. On Linux (Landlock), this means:

- No filesystem access outside the workspace (read-only)
- No network access except the configured LLM API host
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
  policy_file: policies/strict.yaml

# Use a separate provider for Shield evaluation (diversity)
shield:
  evaluator:
    provider: openai  # Different from chat provider

# Enable authentication for non-localhost web access
web:
  host: 0.0.0.0
  password_hash: "$2a$12$..."  # bcrypt hash

# Keep the default ONNX classifier
# Run: openparallax get-classifier
```

Run `openparallax doctor` regularly to verify your security posture.
