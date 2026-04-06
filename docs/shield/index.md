# Shield

Shield is a 3-tier AI security pipeline that evaluates every tool call an LLM agent proposes before it executes. It catches prompt injection, blocks access to sensitive resources, and provides cryptographic proof of every security decision. Shield implements the security pipeline described in [*Parallax: Why AI Agents That Think Must Never Act*](https://github.com/openparallax/openparallax) (forthcoming on arXiv).

Shield runs inside OpenParallax as the security core, but it is also a standalone product. You can drop it into any AI agent, any MCP server, any tool-calling pipeline — in Go, Python, Node.js, or as a standalone binary.

## Why Shield Exists

LLMs can be manipulated. Prompt injection, jailbreaking, indirect injection through retrieved documents — these are not theoretical risks. They are documented, reproducible attack vectors that work against every major model.

MCP servers are powerful. They give LLMs the ability to read files, execute shell commands, send emails, make HTTP requests, and modify databases. Every tool call is a potential attack vector. A single manipulated tool call can exfiltrate secrets, destroy data, or compromise infrastructure.

The standard response is to trust the model. Shield takes the opposite approach: **trust nothing, verify everything**.

## Design Philosophy

### Fail-Closed

Every error path returns BLOCK. If the policy engine cannot parse a rule, the action is blocked. If the classifier throws an exception, the action is blocked. If the LLM evaluator is unreachable, the action is blocked. If the canary token is missing from the evaluator response, the action is blocked.

There is no mode where a failure silently allows an action through. An operator can explicitly set `fail_closed: false` in configuration, but the default is always closed.

### Defense in Depth

No single security mechanism is sufficient. Shield layers three independent evaluation strategies:

- **Tier 0** uses deterministic rules that execute in microseconds. No ML, no LLM, no network calls. Fast and predictable, but only catches what you explicitly write rules for.
- **Tier 1** uses machine learning (DeBERTa prompt injection classifier) and pattern matching (heuristic rules). Catches novel attacks the policy author did not anticipate, but can produce false positives.
- **Tier 2** uses a separate LLM to reason about whether an action is safe in context. The most capable tier, but the slowest and most expensive.

Each tier catches attacks the others miss. Together, they provide comprehensive coverage.

### Progressive Escalation

Not every action needs the same level of scrutiny. Reading a file in the workspace is low-risk — Tier 0 can approve it in microseconds. Executing a shell command is higher-risk — it should pass through the classifier. Modifying the agent's identity files is critical — it needs full LLM evaluation.

Shield's escalation model sends each action to the minimum tier required, escalating only when a lower tier cannot make a confident decision. This keeps latency low for common operations while providing deep evaluation for dangerous ones.

## The 3-Tier Pipeline

```
                    ┌─────────────────────────┐
                    │      Action Request      │
                    └────────────┬────────────┘
                                 │
                                 ▼
                    ┌─────────────────────────┐
                    │   Tier 0: YAML Policy   │
                    │   (microseconds)        │
                    └────────────┬────────────┘
                                 │
                    ┌────────────┼────────────┐
                    │            │            │
                    ▼            ▼            ▼
                  DENY        ALLOW       ESCALATE
                   │            │            │
                   ▼            │            ▼
                 BLOCK     (if minTier   ┌─────────────────────────┐
                            ≤ 0)         │  Tier 1: DualClassifier │
                            │            │  ONNX DeBERTa +         │
                            ▼            │  Heuristic Patterns     │
                          ALLOW          │  (milliseconds)         │
                                         └────────────┬────────────┘
                                                      │
                                         ┌────────────┼────────────┐
                                         │            │            │
                                         ▼            ▼            ▼
                                       BLOCK       ALLOW       ESCALATE
                                         │            │            │
                                         ▼       (if minTier      ▼
                                       BLOCK       ≤ 1)    ┌──────────────────┐
                                                   │       │  Tier 2: LLM     │
                                                   ▼       │  Evaluator       │
                                                 ALLOW     │  + Canary Token  │
                                                           │  (seconds)       │
                                                           └────────┬─────────┘
                                                                    │
                                                              ┌─────┴─────┐
                                                              │           │
                                                              ▼           ▼
                                                            BLOCK       ALLOW
                                                              │
                                                         (uncertain?)
                                                              │
                                                              ▼
                                                    ┌──────────────────┐
                                                    │  Tier 3: Human   │
                                                    │  Approval        │
                                                    │  (user decision) │
                                                    └────────┬─────────┘
                                                              │
                                                        ┌─────┴─────┐
                                                        │           │
                                                        ▼           ▼
                                                      BLOCK       ALLOW
```

### Tier 0 -- YAML Policy

Deterministic pattern matching against a YAML policy file. Rules match on action type (e.g., `write_file`, `execute_command`), file path patterns (glob syntax), and content patterns. Each rule specifies a decision: `deny` (BLOCK immediately), `allow` (ALLOW if no higher tier is required), or `verify` (ESCALATE to a specified tier).

Tier 0 executes in microseconds with zero external dependencies. It is always available.

Read the full [Tier 0 documentation](/shield/tier0) and [Policy Syntax reference](/shield/policies).

### Tier 1 -- DualClassifier

Two classifiers run in parallel and the most severe result wins:

- **ONNX DeBERTa**: A fine-tuned DeBERTa v3 model that detects prompt injection in action payloads. Runs in-process via `onnxruntime-purego` (pure Go, no CGo). Confidence >= 0.85 for INJECTION label triggers BLOCK; below threshold triggers ESCALATE to Tier 2.
- **Heuristic Engine**: Regex-based pattern matching for known attack signatures — prompt injection phrases, path traversal sequences, data exfiltration patterns, sensitive data exposure, encoding evasion, and more.

The DualClassifier combines results using severity ranking: BLOCK > ESCALATE > ALLOW. If either classifier says BLOCK, the action is blocked.

Read the full [Tier 1 documentation](/shield/tier1) and [ONNX Classifier deep dive](/shield/classifier).

### Tier 2 -- LLM Evaluator

An independent LLM (can be a different provider and model than the chat LLM) evaluates the action in context. The evaluator prompt instructs the LLM to analyze the action for security risks and return a structured JSON verdict.

A **canary token** is embedded in the evaluator prompt and must appear in the response. If the canary is missing, it means the evaluator LLM itself was manipulated (the action payload contained an injection that overrode the evaluator's instructions). Missing canary = automatic BLOCK with confidence 1.0.

Tier 2 is rate-limited with a daily budget to control costs.

Read the full [Tier 2 documentation](/shield/tier2).

### Tier 3 -- Human Approval

Tier 3 escalates uncertain verdicts to the user for human approval. When a lower tier cannot make a confident decision and the action does not clearly warrant a BLOCK, Shield can present the action to the user via an inline approval prompt (CLI, web UI, or channel adapter). The user sees the tool name, arguments, and Shield's reasoning, then approves or denies. If the user does not respond within the timeout (default 300 seconds), the action is denied (fail-closed).

## Verdict System

Every Shield evaluation produces a `Verdict` with these fields:

| Field | Type | Description |
|-------|------|-------------|
| `Decision` | `ALLOW` / `BLOCK` | The final security decision |
| `Tier` | `0`, `1`, or `2` | Which tier made the decision |
| `Confidence` | `0.0` - `1.0` | How confident the tier is in its decision |
| `Reasoning` | string | Human-readable explanation |
| `ActionHash` | string | SHA-256 hash of the evaluated action |
| `EvaluatedAt` | timestamp | When the evaluation occurred |
| `ExpiresAt` | timestamp | When the verdict expires (TTL-based caching) |

### Severity Ranking

When combining results from multiple classifiers or deciding between tiers, Shield uses strict severity ranking:

```
BLOCK > ESCALATE > ALLOW
```

If any component says BLOCK, the final decision is BLOCK. ESCALATE means "I cannot decide — send it to a higher tier." ALLOW means the action is safe as far as this tier can determine.

## Standalone Value

Shield is not just an OpenParallax component. It is independently useful for:

- **MCP security gateway**: Drop Shield between your MCP client and MCP servers. Every tool call passes through 3-tier evaluation before reaching the server. Works with Claude Desktop, Cursor, and any MCP-compatible client.
- **Agent framework security**: Building your own agent with LangChain, CrewAI, AutoGen, or raw API calls? Wrap every tool execution with `shield.Evaluate()`.
- **API security layer**: Validate LLM-generated API calls before they hit your backend.
- **Audit and compliance**: Every Shield decision is logged with cryptographic hash chains for tamper-evident audit trails.

## Available In

| Platform | Package | Install |
|----------|---------|---------|
| Go | `github.com/openparallax/openparallax/shield` | `go get` |
| Python | `openparallax-shield` | `pip install openparallax-shield` |
| Node.js | `@openparallax/shield` | `npm install @openparallax/shield` |
| Standalone | `openparallax-shield` binary | [curl, brew, scoop](/shield/standalone) |

## Next Steps

- [Quick Start](/shield/quickstart) -- get Shield running in 5 minutes
- [Tier 0 -- Policy](/shield/tier0) -- YAML policy syntax and examples
- [Tier 1 -- Classifier](/shield/tier1) -- ONNX + heuristic classifier
- [Tier 2 -- LLM Evaluator](/shield/tier2) -- LLM-based evaluation with canary verification
- [Go Library](/shield/go) -- use Shield in your Go application
- [MCP Gateway](/shield/mcp-proxy) -- Shield as an MCP security proxy
- [Policy Syntax](/shield/policies) -- full policy reference
- [Configuration](/shield/configuration) -- all configuration options
