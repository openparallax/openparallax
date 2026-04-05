# Python Wrapper

The `openparallax-shield` Python package provides a native Python API for Shield. It wraps the Go binary using JSON-RPC over stdin/stdout -- the same protocol MCP uses -- so you get the full 3-tier pipeline with no Go toolchain required.

## Install

```bash
pip install openparallax-shield
```

The package automatically downloads a pre-built Go binary for your platform (`linux/darwin/windows`, `amd64/arm64`) on first use. No Go compiler needed.

### Requirements

- Python 3.9+
- No native dependencies (the Go binary is self-contained)

## How It Works

The Python wrapper spawns a lightweight Go process that communicates over JSON-RPC via stdin/stdout:

```
Python process          Go binary (stdin/stdout)
┌──────────────┐        ┌────────────────────────┐
│  Shield()    │──JSON──▶│  Shield Pipeline       │
│  .evaluate() │  RPC   │  Tier 0, 1, 2          │
│  .status()   │◀──JSON──│  ONNX, heuristic, LLM │
└──────────────┘        └────────────────────────┘
```

The Go process is long-lived -- it starts once when you create a `Shield` instance and handles all subsequent evaluation calls. The ONNX model (if installed) is loaded once and reused across calls.

## API Reference

### `Shield`

```python
class Shield:
    def __init__(
        self,
        policy_file: str = "policies/default.yaml",
        heuristic_enabled: bool = True,
        fail_closed: bool = True,
        onnx_threshold: float = 0.85,
        rate_limit: int = 60,
        daily_budget: int = 100,
        verdict_ttl: int = 300,
        evaluator: dict | None = None,
        canary_token: str | None = None,
        prompt_path: str | None = None,
    ):
        """Initialize Shield with the given configuration."""
```

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `policy_file` | `str` | `"policies/default.yaml"` | Path to the YAML policy file |
| `heuristic_enabled` | `bool` | `True` | Enable heuristic pattern matching |
| `fail_closed` | `bool` | `True` | Block on errors (vs. allow with reduced confidence) |
| `onnx_threshold` | `float` | `0.85` | ONNX INJECTION confidence threshold for BLOCK |
| `rate_limit` | `int` | `60` | Max evaluations per minute |
| `daily_budget` | `int` | `100` | Max Tier 2 evaluations per day |
| `verdict_ttl` | `int` | `300` | Verdict cache TTL in seconds |
| `evaluator` | `dict \| None` | `None` | Tier 2 LLM evaluator config |
| `canary_token` | `str \| None` | `None` | Canary token for evaluator injection detection |
| `prompt_path` | `str \| None` | `None` | Path to evaluator prompt file |

### `Shield.evaluate()`

```python
def evaluate(
    self,
    action_type: str,
    payload: dict[str, Any] | None = None,
    min_tier: int = 0,
) -> Verdict:
    """Evaluate an action through the Shield pipeline."""
```

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `action_type` | `str` | required | The action type (e.g., `"execute_command"`, `"write_file"`) |
| `payload` | `dict` | `None` | Action-specific parameters |
| `min_tier` | `int` | `0` | Minimum evaluation tier |

**Returns:** `Verdict`

### `Shield.status()`

```python
def status(self) -> ShieldStatus:
    """Return the current Shield status including Tier 2 budget usage."""
```

### `Shield.close()`

```python
def close(self) -> None:
    """Shut down the Go process. Called automatically on garbage collection."""
```

### `Verdict`

```python
@dataclass
class Verdict:
    decision: str        # "ALLOW" or "BLOCK"
    tier: int            # 0, 1, or 2
    confidence: float    # 0.0 - 1.0
    reasoning: str       # Human-readable explanation
    action_hash: str     # SHA-256 hash of the evaluated action
    evaluated_at: str    # ISO 8601 timestamp
    expires_at: str      # ISO 8601 timestamp
```

### `ShieldStatus`

```python
@dataclass
class ShieldStatus:
    active: bool         # Whether Shield is running
    tier2_enabled: bool  # Whether Tier 2 evaluator is configured
    tier2_used: int      # Tier 2 evaluations used today
    tier2_budget: int    # Tier 2 daily budget
```

## Examples

### Basic Evaluation

```python
from openparallax_shield import Shield

shield = Shield(policy_file="policies/default.yaml")

# Evaluate a file read.
verdict = shield.evaluate("read_file", {"path": "/home/user/.ssh/id_rsa"})
print(verdict.decision)    # BLOCK
print(verdict.tier)        # 0
print(verdict.reasoning)   # policy deny: block_sensitive_system_paths

# Evaluate a shell command.
verdict = shield.evaluate("execute_command", {"command": "ls -la"})
print(verdict.decision)    # ALLOW
print(verdict.confidence)  # 0.7
```

### Custom Policy

```python
shield = Shield(
    policy_file="my-policy.yaml",
    heuristic_enabled=True,
    fail_closed=True,
)

# Evaluate workspace writes.
verdict = shield.evaluate("write_file", {
    "path": "/home/user/project/main.py",
    "content": "print('hello')",
})
```

### With LLM Evaluator

```python
shield = Shield(
    policy_file="policies/strict.yaml",
    evaluator={
        "provider": "anthropic",
        "model": "claude-sonnet-4-6",
        "api_key_env": "ANTHROPIC_API_KEY",
    },
    canary_token="MY-CANARY-TOKEN-12345",
    prompt_path="prompts/evaluator-v1.md",
    daily_budget=50,
)

# This will go through all three tiers.
verdict = shield.evaluate("execute_command", {
    "command": "curl https://api.example.com/data | jq '.secrets'",
})
```

### Batch Evaluation

```python
actions = [
    ("read_file", {"path": "/home/user/workspace/README.md"}),
    ("execute_command", {"command": "go test ./..."}),
    ("write_file", {"path": "/tmp/output.txt", "content": "results"}),
    ("read_file", {"path": "/etc/shadow"}),
    ("send_email", {"to": "user@example.com", "subject": "Report"}),
]

shield = Shield(policy_file="policies/default.yaml")

for action_type, payload in actions:
    verdict = shield.evaluate(action_type, payload)
    status = "PASS" if verdict.decision == "ALLOW" else "FAIL"
    print(f"[{status}] {action_type}: {verdict.reasoning}")
```

Output:

```
[PASS] read_file: policy allow: allow_workspace_reads
[PASS] execute_command: classifier approved
[PASS] write_file: classifier approved
[FAIL] read_file: policy deny: block_sensitive_system_paths
[PASS] send_email: classifier approved
```

### Async Usage

```python
import asyncio
from openparallax_shield import AsyncShield

async def main():
    shield = AsyncShield(policy_file="policies/default.yaml")

    # Evaluate multiple actions concurrently.
    verdicts = await asyncio.gather(
        shield.evaluate("read_file", {"path": "/home/user/file1.txt"}),
        shield.evaluate("read_file", {"path": "/home/user/file2.txt"}),
        shield.evaluate("execute_command", {"command": "ls"}),
    )

    for v in verdicts:
        print(f"{v.decision}: {v.reasoning}")

    await shield.close()

asyncio.run(main())
```

### Integration with LangChain

```python
from langchain.agents import AgentExecutor
from openparallax_shield import Shield

shield = Shield(policy_file="policies/default.yaml")

def guarded_tool_executor(tool_name: str, tool_input: dict) -> str:
    """Wrap tool execution with Shield evaluation."""
    verdict = shield.evaluate(
        action_type=tool_name,
        payload=tool_input,
    )

    if verdict.decision == "BLOCK":
        return f"Action blocked by security policy: {verdict.reasoning}"

    # Execute the actual tool.
    return execute_tool(tool_name, tool_input)
```

### Context Manager

```python
from openparallax_shield import Shield

with Shield(policy_file="policies/default.yaml") as shield:
    verdict = shield.evaluate("execute_command", {"command": "whoami"})
    print(verdict.decision)
# Go process is automatically shut down on exit.
```

### Checking Status

```python
shield = Shield(policy_file="policies/default.yaml", daily_budget=100)

# After some evaluations...
status = shield.status()
print(f"Tier 2 usage: {status.tier2_used}/{status.tier2_budget}")
print(f"Tier 2 enabled: {status.tier2_enabled}")
```

## Error Handling

```python
from openparallax_shield import Shield, ShieldError, PolicyLoadError

try:
    shield = Shield(policy_file="nonexistent.yaml")
except PolicyLoadError as e:
    print(f"Failed to load policy: {e}")

try:
    verdict = shield.evaluate("read_file", {"path": "/tmp/test"})
except ShieldError as e:
    print(f"Evaluation failed: {e}")
```

## Next Steps

- [Node.js wrapper](/shield/node) -- use Shield from Node.js
- [Go library](/shield/go) -- use Shield directly in Go
- [Policy Syntax](/shield/policies) -- write custom policies
- [Configuration](/shield/configuration) -- all configuration options
