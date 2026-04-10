# Node.js Wrapper

The `@openparallax/shield` package provides a TypeScript-native API for Shield. Like the Python wrapper, it communicates with a pre-built Go binary over JSON-RPC via stdin/stdout.

## Install

```bash
npm install @openparallax/shield
```

The package downloads a pre-built Go binary for your platform on `postinstall`. No Go toolchain required.

### Requirements

- Node.js 18+
- No native dependencies

## API Reference

### `Shield`

```typescript
class Shield {
  constructor(options?: ShieldOptions)
  evaluate(request: EvaluateRequest): Promise<Verdict>
  status(): Promise<ShieldStatus>
  close(): Promise<void>
}
```

### `ShieldOptions`

```typescript
interface ShieldOptions {
  /** Path to the YAML policy file. Default: "security/shield/default.yaml" */
  policyFile?: string

  /** Enable heuristic pattern matching. Default: true */
  heuristicEnabled?: boolean

  /** Block on errors vs. allow with reduced confidence. Default: true */
  failClosed?: boolean

  /** ONNX INJECTION confidence threshold for BLOCK. Default: 0.85 */
  onnxThreshold?: number

  /** Max evaluations per minute. Default: 60 */
  rateLimit?: number

  /** Max Tier 2 evaluations per day. Default: 100 */
  dailyBudget?: number

  /** Verdict cache TTL in seconds. Default: 300 */
  verdictTtl?: number

  /** Tier 2 LLM evaluator configuration. Omit to disable Tier 2. */
  evaluator?: EvaluatorConfig

  /** Canary token for evaluator injection detection. */
  canaryToken?: string

  // The evaluator prompt is compiled into the binary and does not need to be specified.
}
```

### `EvaluatorConfig`

```typescript
interface EvaluatorConfig {
  /** LLM provider: "anthropic", "openai", "google", "ollama" */
  provider: string

  /** Model name, e.g., "claude-sonnet-4-6", "gpt-5.4" */
  model: string

  /** Environment variable name containing the API key */
  apiKeyEnv: string

  /** Custom base URL (for Ollama or proxies) */
  baseUrl?: string
}
```

### `EvaluateRequest`

```typescript
interface EvaluateRequest {
  /** Action type, e.g., "execute_command", "write_file" */
  actionType: string

  /** Action-specific parameters */
  payload?: Record<string, unknown>

  /** Minimum evaluation tier (0, 1, or 2). Default: 0 */
  minTier?: number
}
```

### `Verdict`

```typescript
interface Verdict {
  /** "ALLOW" or "BLOCK" */
  decision: 'ALLOW' | 'BLOCK'

  /** Which tier made the decision: 0, 1, or 2 */
  tier: number

  /** Confidence level from 0.0 to 1.0 */
  confidence: number

  /** Human-readable explanation */
  reasoning: string

  /** SHA-256 hash of the evaluated action */
  actionHash: string

  /** ISO 8601 timestamp of when the evaluation occurred */
  evaluatedAt: string

  /** ISO 8601 timestamp of when the verdict expires */
  expiresAt: string
}
```

### `ShieldStatus`

```typescript
interface ShieldStatus {
  /** Whether Shield is running */
  active: boolean

  /** Whether Tier 2 evaluator is configured */
  tier2Enabled: boolean

  /** Tier 2 evaluations used today */
  tier2Used: number

  /** Tier 2 daily budget */
  tier2Budget: number
}
```

## Examples

### Basic Usage

```typescript
import { Shield } from '@openparallax/shield'

const shield = new Shield({ policyFile: 'security/shield/default.yaml' })

const verdict = await shield.evaluate({
  actionType: 'read_file',
  payload: { path: '/home/user/.ssh/id_rsa' },
})

console.log(verdict.decision)   // "BLOCK"
console.log(verdict.tier)       // 0
console.log(verdict.reasoning)  // "policy deny [block_sensitive_system_paths]: read_file on \"/home/user/.ssh/id_rsa\" matched a policy pattern"

await shield.close()
```

### With Full Configuration

```typescript
const shield = new Shield({
  policyFile: 'security/shield/strict.yaml',
  heuristicEnabled: true,
  failClosed: true,
  onnxThreshold: 0.85,
  rateLimit: 60,
  dailyBudget: 100,
  evaluator: {
    provider: 'anthropic',
    model: 'claude-sonnet-4-6',
    apiKeyEnv: 'ANTHROPIC_API_KEY',
  },
  canaryToken: 'MY-CANARY-TOKEN-12345',
})
```

### Guard Function Pattern

```typescript
import { Shield, type Verdict } from '@openparallax/shield'

const shield = new Shield({ policyFile: 'security/shield/default.yaml' })

async function guardedExecute(
  actionType: string,
  payload: Record<string, unknown>,
  execute: () => Promise<unknown>,
): Promise<{ result?: unknown; blocked?: Verdict }> {
  const verdict = await shield.evaluate({ actionType, payload })

  if (verdict.decision === 'BLOCK') {
    return { blocked: verdict }
  }

  const result = await execute()
  return { result }
}

// Usage
const { result, blocked } = await guardedExecute(
  'execute_command',
  { command: 'ls -la' },
  () => execAsync('ls -la'),
)

if (blocked) {
  console.error(`Blocked: ${blocked.reasoning}`)
}
```

### Batch Evaluation

```typescript
const actions = [
  { actionType: 'read_file', payload: { path: '/home/user/workspace/README.md' } },
  { actionType: 'execute_command', payload: { command: 'npm test' } },
  { actionType: 'write_file', payload: { path: '/tmp/output.txt', content: 'data' } },
  { actionType: 'read_file', payload: { path: '/etc/shadow' } },
]

const verdicts = await Promise.all(
  actions.map(action => shield.evaluate(action)),
)

for (let i = 0; i < actions.length; i++) {
  const v = verdicts[i]
  const status = v.decision === 'ALLOW' ? 'PASS' : 'FAIL'
  console.log(`[${status}] ${actions[i].actionType}: ${v.reasoning}`)
}
```

### Express Middleware

```typescript
import express from 'express'
import { Shield } from '@openparallax/shield'

const app = express()
const shield = new Shield({ policyFile: 'security/shield/default.yaml' })

app.use(express.json())

// Shield middleware for tool execution endpoints.
app.post('/api/tools/:toolName', async (req, res) => {
  const verdict = await shield.evaluate({
    actionType: req.params.toolName,
    payload: req.body,
  })

  if (verdict.decision === 'BLOCK') {
    res.status(403).json({
      error: 'Action blocked by security policy',
      tier: verdict.tier,
      reasoning: verdict.reasoning,
      confidence: verdict.confidence,
    })
    return
  }

  // Execute the tool...
  const result = await executeTooll(req.params.toolName, req.body)
  res.json(result)
})

// Shield status endpoint.
app.get('/api/shield/status', async (_req, res) => {
  const status = await shield.status()
  res.json(status)
})

app.listen(3000)
```

### MCP Server Integration

```typescript
import { Server } from '@modelcontextprotocol/sdk/server/index.js'
import { Shield } from '@openparallax/shield'

const shield = new Shield({ policyFile: 'security/shield/default.yaml' })

// Wrap MCP tool execution with Shield.
server.setRequestHandler('tools/call', async (request) => {
  const { name, arguments: args } = request.params

  const verdict = await shield.evaluate({
    actionType: name,
    payload: args,
  })

  if (verdict.decision === 'BLOCK') {
    return {
      content: [{
        type: 'text',
        text: `Security policy blocked this action: ${verdict.reasoning}`,
      }],
      isError: true,
    }
  }

  // Execute the tool normally.
  return await executeTool(name, args)
})
```

### Status Monitoring

```typescript
const shield = new Shield({
  policyFile: 'security/shield/default.yaml',
  dailyBudget: 100,
})

// Check budget periodically.
setInterval(async () => {
  const status = await shield.status()
  if (status.tier2Used > status.tier2Budget * 0.8) {
    console.warn(
      `Shield Tier 2 budget warning: ${status.tier2Used}/${status.tier2Budget} used`,
    )
  }
}, 60_000)
```

### Error Handling

```typescript
import { Shield, ShieldError, PolicyLoadError } from '@openparallax/shield'

try {
  const shield = new Shield({ policyFile: 'nonexistent.yaml' })
} catch (err) {
  if (err instanceof PolicyLoadError) {
    console.error(`Policy file error: ${err.message}`)
  }
}

try {
  const verdict = await shield.evaluate({
    actionType: 'read_file',
    payload: { path: '/tmp/test.txt' },
  })
} catch (err) {
  if (err instanceof ShieldError) {
    console.error(`Shield evaluation error: ${err.message}`)
  }
}
```

### Cleanup

The Go process is cleaned up when `close()` is called. Use `try/finally` or process event handlers:

```typescript
const shield = new Shield({ policyFile: 'security/shield/default.yaml' })

// Clean up on process exit.
process.on('SIGINT', async () => {
  await shield.close()
  process.exit(0)
})

process.on('SIGTERM', async () => {
  await shield.close()
  process.exit(0)
})
```

## TypeScript Types

All types are exported from the package root:

```typescript
import type {
  Shield,
  ShieldOptions,
  EvaluateRequest,
  Verdict,
  ShieldStatus,
  EvaluatorConfig,
} from '@openparallax/shield'
```

## Next Steps

- [Python wrapper](/shield/python) -- use Shield from Python
- [Go library](/shield/go) -- use Shield directly in Go
- [Standalone binary](/shield/standalone) -- run Shield as a service
- [MCP Gateway](/shield/mcp-proxy) -- dedicated MCP security proxy
