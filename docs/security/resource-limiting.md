# Resource & Rate Limiting

These mechanisms bound the blast radius of runaway operations, whether caused by LLM loops, prompt injection, or misconfiguration. They prevent denial-of-service conditions and limit the cost of security incidents.

## Global Rate Limit

**Threat:** A prompt injection or LLM loop that floods the system with rapid-fire tool calls, overwhelming Shield evaluation, exhausting API quotas, or creating audit log spam.

**Defense:** `general.rate_limit` (default 30) caps actions per minute. When hit, Shield returns BLOCK at Tier 0 with "rate limit exceeded" — fast, deterministic, no downstream evaluation cost.

**Code:** `shield/gateway.go` (`RateLimiter`)

**Tunable** via `config.yaml`. The rate limit cannot be set to zero (that would disable it).

## Tier 2 Daily Budget

**Threat:** Unbounded LLM evaluator spend. Each Tier 2 evaluation costs tokens (1–3 seconds, real money). A busy workspace with aggressive escalation rules could burn through hundreds of evaluations per day.

**Defense:** `general.daily_budget` (default 100) caps Tier 2 evaluations per calendar day. When exhausted, the gateway skips Tier 2 and falls through to the next handling (ALLOW with reduced confidence if `fail_closed: false`, BLOCK if `fail_closed: true`). The budget persists across engine restarts via `metrics_daily`.

**Code:** `shield/gateway.go` (`checkBudget`), `internal/storage/metrics.go` (`GetDailyMetric`)

**Tunable** via `config.yaml`.

## Verdict TTL

**Threat:** Stale Shield verdicts being reused for changed conditions — an action that was safe a minute ago may not be safe now if the workspace state changed.

**Defense:** `general.verdict_ttl_seconds` (default 60) expires cached verdicts. After expiration, the same action is re-evaluated from scratch.

**Code:** `shield/gateway.go`

**Tunable** via `config.yaml`.

## Tier 3 Hourly Cap

**Threat:** Alert fatigue. If the agent triggers dozens of Tier 3 human approval requests per hour, the human operator stops reading them carefully — defeating the purpose of human-in-the-loop security.

**Defense:** `shield.tier3.max_per_hour` caps approval requests per hour. When the cap is hit, the action is denied without asking the human.

**Code:** `internal/engine/tier3.go`

**Tunable** via `config.yaml`.

## Crash Restart Budget

**Threat:** A crashing engine or agent process being restarted in an infinite loop, consuming system resources and potentially causing data corruption on rapid restart/crash cycles.

**Defense:** `agents.crash_restart_budget` (default 5) and `agents.crash_window_seconds` (default 60) cap how many times the process manager will restart a crashing process. After the budget is exhausted, the process manager gives up and logs the failure.

**Code:** `cmd/agent/start.go` (engine), `cmd/agent/internal_engine.go` (agent)

**Tunable** via `config.yaml`.

## Sub-Agent Concurrency and Timeout

**Threat:** Unbounded sub-agent spawning consuming all system memory and CPU, or a single sub-agent running indefinitely and blocking the parent session.

**Defense:**
- `agents.max_concurrent_sub_agents` (default 10) — caps simultaneously running sub-agents
- `agents.sub_agent_timeout_seconds` (default 900 / 15 minutes) — kills sub-agents that exceed their time budget

A project manager delegating research to sub-agents can fan out work without worrying about resource exhaustion. The concurrency cap and timeout together bound both the breadth and depth of delegation.

**Code:** `internal/engine/subagent.go`

**Tunable** via `config.yaml`.

## Reasoning Loop Max Rounds

**Threat:** An LLM tool-call loop that never converges — the agent keeps calling tools but never produces a final answer, consuming tokens indefinitely.

**Defense:** `agents.max_tool_rounds` (default 25) caps the number of tool-call round-trips per message. When the cap is hit, the loop exits and the agent produces whatever response it has.

**Code:** `internal/agent/loop.go`

**Tunable** via `config.yaml`.
