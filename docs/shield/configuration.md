# Configuration Reference

Shield can be configured in three contexts: as part of OpenParallax (`config.yaml`), as a standalone binary (`shield.yaml`), or through environment variables. This page covers all configuration options, startup validation, provider setup, and operational guidance.

## OpenParallax Integration (`config.yaml`)

When Shield runs inside OpenParallax, it is configured in the `shield` section of the workspace `config.yaml`:

```yaml
shield:
  # Path to the YAML policy file (relative to workspace root)
  policy_file: policies/default.yaml

  # Tier 2 LLM evaluator configuration
  evaluator:
    provider: anthropic
    model: claude-sonnet-4-6
    api_key_env: ANTHROPIC_API_KEY

  # ONNX classifier threshold (0.0 - 1.0)
  # Higher = fewer blocks, more escalations
  # Lower = more blocks, fewer escalations
  onnx_threshold: 0.85

  # Enable heuristic pattern matching
  heuristic_enabled: true

  # Block on errors (true) vs. allow with reduced confidence (false)
  fail_closed: true

  # Maximum Tier 2 evaluations per minute
  rate_limit: 60

  # Maximum Tier 2 evaluations per day
  daily_budget: 100

  # Verdict cache TTL in seconds
  verdict_ttl: 300
```

The canary token and evaluator prompt path are managed automatically by OpenParallax. The canary is generated at workspace initialization and the prompt is loaded from the embedded `prompts/` directory.

## Standalone Configuration (`shield.yaml`)

When Shield runs as a standalone binary (`openparallax-shield serve`), all configuration lives in `shield.yaml`:

```yaml
# ── Server ──
listen: localhost:9090           # REST API listen address
grpc_listen: localhost:9091      # gRPC API listen address

# ── Policy ──
policy:
  file: policies/default.yaml   # Path to YAML policy file

# ── Classifier ──
classifier:
  model_dir: ~/.openparallax/models/prompt-injection/  # ONNX model directory
  threshold: 0.85                                       # INJECTION threshold

# ── Heuristic ──
heuristic:
  enabled: true                  # Enable heuristic pattern matching

# ── Tier 2 Evaluator ──
evaluator:
  provider: anthropic            # LLM provider
  model: claude-sonnet-4-6 # LLM model
  api_key_env: ANTHROPIC_API_KEY # Env var for API key
  base_url:                      # Custom base URL (Ollama, proxies)

# ── Security ──
canary_token:                    # Canary token (auto-generated if blank)
prompt_path: prompts/evaluator-v1.md  # Evaluator prompt file
fail_closed: true                # Block on errors

# ── Rate Limiting ──
rate_limit: 60                   # Evaluations per minute
daily_budget: 100                # Tier 2 evaluations per day
verdict_ttl: 300                 # Verdict cache TTL (seconds)

# ── MCP Proxy (optional) ──
mcp:
  servers:
    - name: filesystem
      transport: stdio
      command: npx
      args: ["@modelcontextprotocol/server-filesystem", "/home/user"]
    - name: remote
      transport: streamable-http
      url: https://mcp-server.example.com

  tool_mapping:                  # MCP tool name → Shield action type
    custom_tool: execute_command

# ── Audit ──
audit:
  enabled: true                  # Enable audit logging
  file: shield-audit.jsonl       # Audit log file path

# ── Logging ──
log_level: info                  # debug, info, warn, error
log_file: shield.log             # Log file (stdout if omitted)
```

## Environment Variable Overrides

Environment variables override configuration file values. They use the `OP_SHIELD_` prefix:

| Variable | Description | Overrides |
|----------|-------------|-----------|
| `OP_SHIELD_POLICY` | Path to the YAML policy file | `policy.file` / `shield.policy_file` |
| `OP_SHIELD_CLASSIFIER_DIR` | ONNX model directory | `classifier.model_dir` |
| `OP_SHIELD_THRESHOLD` | ONNX INJECTION confidence threshold | `classifier.threshold` / `shield.onnx_threshold` |
| `OP_SHIELD_FAIL_CLOSED` | `true` or `false` | `fail_closed` / `shield.fail_closed` |
| `OP_SHIELD_DAILY_BUDGET` | Daily Tier 2 evaluation budget | `daily_budget` / `shield.daily_budget` |
| `OP_SHIELD_RATE_LIMIT` | Evaluations per minute | `rate_limit` / `shield.rate_limit` |
| `OP_SHIELD_LOG_LEVEL` | Log level | `log_level` |

The API key environment variable names (e.g., `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`) are configured in the `evaluator.api_key_env` field -- they are not Shield-specific variables.

### Example

```bash
# Override the policy file and threshold via environment
export OP_SHIELD_POLICY=/opt/shield/policies/strict.yaml
export OP_SHIELD_THRESHOLD=0.90
export OP_SHIELD_DAILY_BUDGET=200
export ANTHROPIC_API_KEY=sk-ant-...

openparallax-shield serve --config shield.yaml
```

## Configuration Precedence

Configuration values are resolved in this order (highest priority first):

1. **Environment variables** (`OP_SHIELD_*`)
2. **CLI flags** (`--port`, `--config`)
3. **Configuration file** (`shield.yaml` or `config.yaml`)
4. **Defaults**

## Separate Providers for Chat and Shield

It is recommended to use different LLM providers or models for the chat conversation and the Shield Tier 2 evaluator. This provides **security diversity** -- if an attack is crafted to exploit a specific model's weaknesses, a different model in the evaluator may catch it.

```yaml
models:
  - name: chat
    provider: anthropic
    model: claude-sonnet-4-6
    api_key_env: ANTHROPIC_API_KEY
  - name: shield
    provider: openai
    model: gpt-5.4
    api_key_env: OPENAI_API_KEY

roles:
  chat: chat
  shield: shield
```

Both providers must have valid API keys set. The Shield evaluator only runs for actions that escalate to Tier 2, so the cost is proportional to the number of escalations, not the number of total actions.

You can also use the same provider with a different model:

```yaml
models:
  - name: chat
    provider: anthropic
    model: claude-sonnet-4-6
    api_key_env: ANTHROPIC_API_KEY
  - name: shield
    provider: anthropic
    model: claude-opus-4-20250514  # More capable model for security decisions
    api_key_env: ANTHROPIC_API_KEY

roles:
  chat: chat
  shield: shield
```

## Classifier Setup

The ONNX DeBERTa classifier runs locally for Tier 1 evaluation. It detects prompt injection patterns in action payloads.

### Installing the Classifier Model

```bash
openparallax get-classifier
```

This downloads:
- `model.onnx` -- the DeBERTa v3 model fine-tuned for prompt injection detection
- `tokenizer.json` -- the tokenizer configuration
- The ONNX Runtime library for your platform

Files are placed in `~/.openparallax/models/prompt-injection/`.

### Verifying the Classifier

After installation, verify the model loads correctly:

```bash
openparallax doctor
```

The doctor check reports:
- Whether the model files exist at the expected path
- Whether the ONNX Runtime library loads correctly
- Whether inference produces valid output on a test input

### Classifier Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `onnx_threshold` / `classifier.threshold` | float64 | `0.85` | Confidence threshold for the INJECTION label. Scores >= threshold trigger BLOCK; scores below trigger ESCALATE to Tier 2. |
| `classifier.model_dir` | string | `~/.openparallax/models/prompt-injection/` | Directory containing `model.onnx`, `tokenizer.json`, and the ONNX Runtime library. |

### Heuristic-Only Mode

If the ONNX model is not installed, Tier 1 operates in **heuristic-only mode**. The heuristic engine (regex pattern matching for known attack signatures) runs alone, without the DeBERTa classifier. A one-time warning is logged at startup:

```
WARN: ONNX classifier not found, running heuristic-only mode
```

This is a valid configuration for environments where:
- The ONNX model cannot be downloaded (air-gapped networks)
- The ONNX Runtime is not available for the platform
- You want to minimize memory usage (the DeBERTa model uses ~300MB RAM)

Heuristic-only mode provides less coverage than the full DualClassifier but still catches known attack patterns.

## Evaluator Setup

The Tier 2 LLM evaluator uses a separate LLM to reason about whether an action is safe in context.

### Configuration Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `evaluator.provider` | string | -- | LLM provider: `anthropic`, `openai`, `google`, `ollama`. Omit to disable Tier 2. |
| `evaluator.model` | string | -- | Model name (e.g., `claude-sonnet-4-6`, `gpt-5.4`, `gemini-3.1-pro`). |
| `evaluator.api_key_env` | string | -- | Name of the environment variable containing the API key. |
| `evaluator.base_url` | string | -- | Custom base URL for the provider (e.g., `http://localhost:11434` for Ollama). |

### Canary Token

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `canary_token` | string | auto-generated | Random token embedded in the evaluator prompt to detect injection. Auto-generated at workspace init (OpenParallax) or first run (standalone). Must be random and unpredictable. |

### Evaluator Prompt

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `prompt_path` | string | `prompts/evaluator-v1.md` | Path to the evaluator prompt markdown file. The prompt instructs the LLM how to evaluate actions and includes the canary token. |

### Daily Budget and Rate Limiting

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `rate_limit` | int | `60` | Maximum Shield evaluations per minute. Uses token bucket algorithm. Applies to all tiers. |
| `daily_budget` | int | `100` | Maximum Tier 2 (LLM evaluator) evaluations per day. Resets at midnight server local time. When exhausted, actions that would escalate to Tier 2 are blocked (fail-closed) or allowed with reduced confidence (fail-open). |
| `verdict_ttl` | int | `300` | How long a verdict is valid in seconds. The same action hash returns the cached verdict within the TTL, avoiding duplicate evaluations for identical actions. |

### Disabling Tier 2

To run without the LLM evaluator (Tier 0 + Tier 1 only), omit the `evaluator` section entirely:

```yaml
shield:
  policy_file: policies/default.yaml
  heuristic_enabled: true
  onnx_threshold: 0.85
  fail_closed: true
```

Actions that would escalate to Tier 2 will be blocked (fail-closed) or allowed with reduced confidence (fail-open), depending on the `fail_closed` setting.

## Security Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `fail_closed` | bool | `true` | When `true`, any error in the pipeline (classifier failure, evaluator timeout, parse error, missing canary) results in BLOCK. When `false`, errors result in ALLOW with reduced confidence (0.5). |

::: danger
Setting `fail_closed: false` weakens security. In fail-open mode, a crashed classifier or unreachable evaluator silently allows actions through. Only use fail-open in development environments where blocking would disrupt testing workflows.
:::

## Startup Validation

Shield validates its configuration at startup. Missing or invalid configuration produces specific errors:

| Condition | Behavior |
|-----------|----------|
| Missing policy file | Fatal error. Shield cannot start without a policy. |
| Invalid YAML in policy file | Fatal error. The policy cannot be parsed. |
| Invalid glob pattern in a policy rule | Warning log. The pattern is skipped, other patterns still work. |
| Missing evaluator prompt (when evaluator is configured) | Fatal error. Tier 2 cannot function without its prompt. |
| Missing evaluator API key (when evaluator is configured) | Fatal error. The evaluator cannot authenticate. |
| Missing ONNX model | Warning. Tier 1 runs in heuristic-only mode. |
| Missing canary token (when evaluator is configured) | Auto-generated. A new canary token is created and saved. |
| Invalid `onnx_threshold` (outside 0.0-1.0) | Fatal error. |

## Complete Configuration Fields Reference

### Policy Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `policy_file` / `policy.file` | string | required | Path to the YAML policy file. Relative paths are resolved from the working directory (OpenParallax: workspace root). |

### Classifier Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `onnx_threshold` / `classifier.threshold` | float64 | `0.85` | Confidence threshold for the INJECTION label. |
| `classifier.model_dir` | string | `~/.openparallax/models/prompt-injection/` | Directory containing the ONNX model and tokenizer. |
| `heuristic_enabled` | bool | `true` | Enable the heuristic pattern matching engine in Tier 1. |

### Evaluator Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `evaluator.provider` | string | -- | LLM provider for Tier 2. Omit to disable Tier 2. |
| `evaluator.model` | string | -- | Model name. |
| `evaluator.api_key_env` | string | -- | Environment variable containing the API key. |
| `evaluator.base_url` | string | -- | Custom base URL (Ollama, proxies, Azure). |
| `canary_token` | string | auto-generated | Token for evaluator response verification. |
| `prompt_path` | string | `prompts/evaluator-v1.md` | Evaluator prompt file path. |

### Security Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `fail_closed` | bool | `true` | Block on any pipeline error. |

### Rate Limiting Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `rate_limit` | int | `60` | Maximum evaluations per minute. |
| `daily_budget` | int | `100` | Maximum Tier 2 evaluations per day. |
| `verdict_ttl` | int | `300` | Verdict cache TTL in seconds. |

### Server Configuration (Standalone Only)

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `listen` | string | `localhost:9090` | REST API listen address. |
| `grpc_listen` | string | `localhost:9091` | gRPC API listen address. |
| `log_level` | string | `info` | Logging level: `debug`, `info`, `warn`, `error`. |
| `log_file` | string | -- | Log file path. Stdout if omitted. |

### MCP Proxy Configuration (Standalone Only)

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `mcp.servers` | list | -- | Upstream MCP servers. |
| `mcp.servers[].name` | string | required | Unique server name. |
| `mcp.servers[].transport` | string | required | `stdio` or `streamable-http`. |
| `mcp.servers[].command` | string | -- | Command to run (stdio). |
| `mcp.servers[].args` | list | -- | Command arguments (stdio). |
| `mcp.servers[].env` | map | -- | Environment variables (stdio). Supports `${ENV_VAR}` substitution. |
| `mcp.servers[].url` | string | -- | Server URL (streamable-http). |
| `mcp.servers[].headers` | map | -- | HTTP headers (streamable-http). Supports `${ENV_VAR}` substitution. |
| `mcp.tool_mapping` | map | -- | Custom MCP tool name to Shield action type mapping. |

### Audit Configuration (Standalone Only)

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `audit.enabled` | bool | `false` | Enable audit logging. |
| `audit.file` | string | `shield-audit.jsonl` | Audit log file path. |

## Minimal Configurations

### Tier 0 Only (Policy Matching)

The smallest useful configuration. Policy matching only, no ML, no LLM:

```yaml
shield:
  policy_file: policies/default.yaml
  heuristic_enabled: false
  fail_closed: true
```

### Tier 0 + Tier 1 (Policy + Classifier)

Adds heuristic pattern matching. Optionally install the ONNX model for the DeBERTa classifier:

```yaml
shield:
  policy_file: policies/default.yaml
  heuristic_enabled: true
  onnx_threshold: 0.85
  fail_closed: true
```

### Full Pipeline (All Three Tiers)

Complete security pipeline with policy, classifier, and LLM evaluator:

```yaml
shield:
  policy_file: policies/default.yaml
  heuristic_enabled: true
  onnx_threshold: 0.85
  fail_closed: true
  rate_limit: 60
  daily_budget: 100
  evaluator:
    provider: anthropic
    model: claude-sonnet-4-6
    api_key_env: ANTHROPIC_API_KEY
```

## Tuning Guide

### False Positives Too High

If legitimate actions are being blocked or escalated too often:

- Increase `onnx_threshold` from 0.85 to 0.90 or 0.95 (fewer classifier blocks)
- Switch to `permissive.yaml` policy (fewer deny rules)
- Add explicit `allow` rules for the action patterns being falsely blocked
- Review the audit log to identify which rules or classifier patterns cause false positives

### Security Too Loose

If too many actions are passing without evaluation:

- Switch to `strict.yaml` policy (more deny rules, higher tier overrides)
- Lower `onnx_threshold` to 0.80 (more aggressive classifier)
- Set `fail_closed: true` (block on errors instead of allowing)
- Add `verify` rules with `tier_override: 2` for sensitive operations
- Add `deny` rules for paths and action types that should never be allowed

### Tier 2 Costs Too High

If the LLM evaluator costs are a concern:

- Increase `onnx_threshold` (fewer escalations from Tier 1 to Tier 2)
- Lower `daily_budget` to cap daily costs
- Use a cheaper model for the evaluator (e.g., `claude-haiku-4-5-20251001` instead of `claude-sonnet-4-6`)
- Add more `allow` and `deny` rules to Tier 0 so fewer actions reach Tier 2
- Increase `verdict_ttl` to cache verdicts longer (reduces repeat evaluations)

### Latency Too High

If Shield evaluation is slowing down the agent:

- Add `allow` rules for common safe operations to keep them at Tier 0 (microseconds)
- Increase `verdict_ttl` to cache more verdicts
- Disable Tier 2 for non-critical deployments (Tier 0 + Tier 1 only)
- Ensure the ONNX model is loaded locally (network-based classifiers add latency)

## Next Steps

- [Policy Syntax](/shield/policies) -- full policy file reference
- [Tier 0 -- Policy](/shield/tier0) -- how policy matching works
- [Tier 1 -- Classifier](/shield/tier1) -- classifier model details
- [Tier 2 -- Evaluator](/shield/tier2) -- LLM evaluator details
- [ONNX Classifier](/shield/classifier) -- classifier model deep dive
- [Standalone binary](/shield/standalone) -- running Shield as a service
