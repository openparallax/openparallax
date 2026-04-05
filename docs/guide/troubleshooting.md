# Troubleshooting

This guide covers diagnostics, common issues, and how to investigate problems with OpenParallax. Start with the doctor command for an automated health check, then work through the specific issue sections below.

## The Doctor Command

`openparallax doctor` is the first tool to reach for when something is not working. It runs 13 automated checks and reports pass, warning, or failure for each.

```bash
openparallax doctor
```

Example output:

```
OpenParallax System Check
--------------------------------------------

  [pass] Config           /home/user/.openparallax/atlas/config.yaml loaded
  [pass] Workspace        /home/user/.openparallax/atlas
  [pass] SQLite           openparallax.db (2.4 MB, WAL mode)
  [pass] LLM Provider     anthropic / claude-sonnet-4-20250514
  [pass] Shield           policy loaded, Tier 2: 100/day budget
  [pass] Embedding        openai / text-embedding-3-small
  [warn] Browser          no Chromium browser detected
  [warn] Email            SMTP not configured
  [warn] Calendar         not configured
  [pass] HEARTBEAT        3 scheduled tasks
  [pass] Audit            47 entries, chain valid
  [pass] Sandbox          Landlock V4 (filesystem + network)
  [pass] Web UI           port 3100

  10/13 checks passed. 3 warnings (non-critical).
```

You can point doctor at a specific config file:

```bash
openparallax doctor -c /path/to/config.yaml
```

### The 13 Checks

#### 1. Config

Loads and parses `config.yaml`. Validates YAML syntax, required fields, and cross-references (e.g., the provider name matches a known provider).

| Result | Meaning |
|---|---|
| Pass | Config file loads and parses correctly |
| Fail | File missing, YAML syntax error, or validation failure |

**If it fails:** Check for common YAML issues -- tabs instead of spaces, unclosed quotes, incorrect indentation. Run `openparallax init` to generate a fresh config.

#### 2. Workspace

Verifies the workspace directory exists and is writable. Checks for the `.openparallax/` internal directory.

| Result | Meaning |
|---|---|
| Pass | Directory exists and is writable |
| Fail | Directory missing or not writable |

**If it fails:** The workspace path in config.yaml may be wrong, or the directory was deleted. Run `openparallax init` to create a new workspace, or fix the path in config.yaml.

#### 3. SQLite

Opens the database file (`openparallax.db` in the `.openparallax/` directory) and verifies it is in WAL mode. Reports the file size.

| Result | Meaning |
|---|---|
| Pass | Database opens in WAL mode |
| Fail | Cannot open database, corrupted, or wrong mode |

**If it fails:** The database may be locked by another process, corrupted by a crash, or missing. If corrupted beyond repair, delete the database file and restart -- the engine recreates it on startup (existing session and memory data will be lost).

#### 4. LLM Provider

Verifies that a provider and model are configured. Does not make an API call -- it only checks the configuration.

| Result | Meaning |
|---|---|
| Pass | Provider and model are set in config |
| Warn | Provider not configured |

**If it warns:** Set the `llm.provider` and `llm.model` fields in config.yaml and export the corresponding API key environment variable.

#### 5. Shield

Checks that the policy file exists at the configured path and reports the Tier 2 daily evaluation budget.

| Result | Meaning |
|---|---|
| Pass | Policy file found, budget reported |
| Warn | Policy file not found |

**If it warns:** The `shield.policy_file` path in config.yaml does not point to an existing file. Policy paths are relative to the workspace directory. Run `openparallax init` in a temporary directory to regenerate default policy files, then copy the `policies/` folder to your workspace.

#### 6. Embedding

Checks whether an embedding provider is configured for vector search.

| Result | Meaning |
|---|---|
| Pass | Provider and model configured |
| Warn | Not configured (FTS5 keyword search only) |

**If it warns:** Without an embedding provider, memory search uses keyword matching only. Semantic queries will not work. Configure `memory.embedding` in config.yaml if you want vector search.

#### 7. Browser

Checks for a Chromium-based browser for the browser executor (web scraping, screenshots).

| Result | Meaning |
|---|---|
| Pass | Browser binary found |
| Warn | No browser detected |

**If it warns:** Install Chromium, Chrome, or Brave. The browser executor is optional -- the agent works without it but cannot browse web pages.

#### 8. Email

Checks whether SMTP is configured for the email executor.

| Result | Meaning |
|---|---|
| Pass | SMTP host configured |
| Warn | Not configured |

**If it warns:** The email executor is optional. Configure it in config.yaml under `email` if you want the agent to send emails.

#### 9. Calendar

Checks whether a calendar provider is configured.

| Result | Meaning |
|---|---|
| Pass | Provider configured |
| Warn | Not configured |

**If it warns:** The calendar executor is optional.

#### 10. HEARTBEAT

Parses the `HEARTBEAT.md` file and reports the number of scheduled tasks.

| Result | Meaning |
|---|---|
| Pass | File parsed, N tasks found |
| Pass | File not found (no scheduled tasks, which is fine) |

#### 11. Audit

Verifies the SHA-256 hash chain in `audit.jsonl`. Reports the total number of entries and whether the chain is valid.

| Result | Meaning |
|---|---|
| Pass | Hash chain valid, N entries |
| Fail | Hash chain broken at entry N |

**If it fails:** The audit log has been tampered with or corrupted. See the "Broken Audit Chain" section below.

#### 12. Sandbox

Checks kernel sandbox availability and reports capabilities.

| Result | Meaning |
|---|---|
| Pass | Active, capabilities listed (e.g., "Landlock V4 (filesystem + network)") |
| Warn | Not available, with reason |

**If it warns:** The agent runs without kernel sandboxing, relying on Shield as the primary security layer. See the "Sandbox Not Available" section below.

#### 13. Web UI

Reports the configured web server port.

| Result | Meaning |
|---|---|
| Pass | Port configured |
| Warn | Web UI disabled |

## Common Issues

### Agent Will Not Start

**Symptom:** `openparallax start` exits immediately or reports an error.

**Step 1: Run doctor**

```bash
openparallax doctor -c /path/to/config.yaml
```

Doctor catches most configuration issues. Address any failures it reports before proceeding.

**Step 2: Check for common causes**

| Cause | Diagnosis | Fix |
|---|---|---|
| Missing config file | `Error: config file not found` | Run `openparallax init` or use `-c path/to/config.yaml` |
| Invalid YAML syntax | `Error: yaml: line N: ...` | Fix the syntax error. Common: tabs (use spaces), unclosed quotes, wrong indentation |
| Missing API key | `Error: environment variable ANTHROPIC_API_KEY not set` | Export the key: `export ANTHROPIC_API_KEY="..."` |
| Port already in use | `Error: listen tcp :3100: bind: address already in use` | Kill the other process (`lsof -i :3100`) or change `web.port` in config.yaml |
| Already running | `Error: agent already running for workspace` | Stop the existing instance: `openparallax stop` or `kill` the process |
| Database locked | `Error: database is locked` | Another process has the database open. Stop all OpenParallax processes and retry. |

**Step 3: Start with verbose logging**

```bash
openparallax start -v
```

Verbose mode writes detailed structured logs to `<workspace>/.openparallax/engine.log`. If the agent crashes on startup, the log shows the exact failure point.

**Symptom: "engine did not start within 30 seconds"**

The process manager waits for the engine subprocess to report its gRPC port. If the engine fails to start:

1. Is the LLM API key valid? The engine tests the connection on startup. An invalid key causes the connection test to fail.
2. Is there a firewall blocking the dynamic gRPC port? The engine binds to a random port. Firewalls that block all ports except known ones can prevent the gRPC server from starting.
3. Start with `-v` and check the engine log for the actual error.

---

### Web UI Not Loading

**Symptom:** Browser shows a blank page or connection refused at `http://127.0.0.1:3100`.

**Step 1: Is the engine running?**

```bash
openparallax list
```

If nothing is listed, the engine is not running. Start it with `openparallax start`.

**Step 2: Is the web UI enabled?**

Check config.yaml:

```yaml
web:
  enabled: true
  port: 3100
```

If `web.enabled` is false, the HTTP/WebSocket server does not start.

**Step 3: Check the port**

The startup output shows the actual port. If the configured port was already in use, the engine may report `WEB_FAILED` in the logs. Check what is using the port:

```bash
lsof -i :3100
# or
ss -tlnp | grep 3100
```

**Step 4: Check firewall**

Ensure your firewall allows connections to the configured port.

::: details Linux
```bash
sudo ufw status
sudo iptables -L -n | grep 3100
```
:::

::: details macOS
```bash
sudo pfctl -sr
```
Check System Settings > Network > Firewall for application-level rules.
:::

::: details Windows (PowerShell)
```powershell
netsh advfirewall firewall show rule name=all | findstr 3100
```
:::

**Step 5: Check authentication**

If `web.auth: true`, the web UI requires a password. On first run, a one-time password is printed in the startup output. If you missed it, set a password:

```bash
openparallax auth set-password
```

**Step 6: Try a direct request**

```bash
curl -v http://127.0.0.1:3100/
```

If this returns HTML, the server is running and the issue is browser-side (cache, extension, proxy). If it returns "connection refused", the server is not listening on that port.

---

### Web UI Frozen / Not Streaming

**Symptom:** Messages appear to send but no response streams back. The UI may be unresponsive or show a spinner that never completes.

**Step 1: Check the WebSocket connection**

Look at the connection indicator in the top-right corner of the web UI:
- Green dot = connected
- Orange dot = reconnecting
- Red dot = disconnected

If disconnected, the WebSocket link is broken. The UI attempts to reconnect automatically. Wait 10 seconds for reconnection.

**Step 2: Hard refresh**

Press `Ctrl+Shift+R` (or `Cmd+Shift+R` on macOS) to reload the page and bypass the browser cache. Stale JavaScript can cause event handling to break after an engine restart.

**Step 3: Check the browser console**

Open browser developer tools (F12) and check the Console tab. Look for:
- `WebSocket connection to 'ws://...' failed` -- the WebSocket URL is wrong or the server is down
- `WebSocket is already in CLOSING or CLOSED state` -- the connection dropped and reconnection failed
- JavaScript errors -- a frontend bug may be preventing event processing

**Step 4: Check engine logs**

```bash
openparallax logs --level error
openparallax logs --event pipeline_error
```

If the engine encountered an error during pipeline processing, it may have failed to send the response. Common causes:
- LLM API timeout -- the provider took too long to respond
- Tool execution failure -- a tool call crashed
- Context window overflow -- the conversation exceeded the model's context limit

**Step 5: Restart the engine**

```bash
openparallax restart
```

Or type `/restart` in the chat input if it is still accepting text. The engine restarts, the web UI reconnects automatically, and the session history is preserved.

**Step 6: Check if the pipeline is stuck**

If the agent is executing a long-running tool (e.g., a shell command that hangs), the pipeline blocks until the tool times out. Tool execution has a 5-minute default timeout. Check the logs:

```bash
openparallax logs --event tool_call
```

Look for a tool call with no corresponding `tool_result` event.

---

### API Connection Failures

**Symptom:** "connection failed" errors, timeout errors, or "unauthorized" responses from the LLM provider.

**Step 1: Verify the API key is set**

```bash
# Check the environment variable (use the one matching your provider)
echo $ANTHROPIC_API_KEY
echo $OPENAI_API_KEY
echo $GOOGLE_AI_API_KEY
```

If empty, the variable is not set. Export it in your shell profile.

**Step 2: Verify the key is valid**

Make a direct API call to isolate whether the issue is the key or OpenParallax:

```bash
# Anthropic
curl -s -H "x-api-key: $ANTHROPIC_API_KEY" \
     -H "anthropic-version: 2023-06-01" \
     -H "content-type: application/json" \
     https://api.anthropic.com/v1/messages \
     -d '{"model":"claude-sonnet-4-20250514","max_tokens":10,"messages":[{"role":"user","content":"hi"}]}'

# OpenAI
curl -s -H "Authorization: Bearer $OPENAI_API_KEY" \
     -H "content-type: application/json" \
     https://api.openai.com/v1/chat/completions \
     -d '{"model":"gpt-4o","max_tokens":10,"messages":[{"role":"user","content":"hi"}]}'

# Google
curl -s "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent?key=$GOOGLE_AI_API_KEY" \
     -H "content-type: application/json" \
     -d '{"contents":[{"parts":[{"text":"hi"}]}]}'
```

If the direct call fails, the issue is with the API key or the provider, not OpenParallax.

**Step 3: Check the model name**

A typo in the model name causes connection test failures. Common mistakes:
- `claude-sonnet-4` instead of `claude-sonnet-4-20250514` (Anthropic requires the date suffix)
- `gpt4o` instead of `gpt-4o`
- Using a model name from a different provider

**Step 4: Check the base URL**

If using a custom `base_url` for OpenAI-compatible APIs (LM Studio, Together AI, Groq, vLLM):

```yaml
llm:
  provider: openai
  model: your-model
  base_url: "http://localhost:1234/v1"
```

Verify the service is running and reachable:

```bash
curl -s http://localhost:1234/v1/models
```

**Step 5: Check Ollama**

For Ollama, the server must be running and the model must be pulled:

```bash
# Start the server
ollama serve

# Pull the model
ollama pull llama3.2

# Verify
ollama list
```

**Step 6: Check network connectivity**

If behind a corporate proxy or VPN:

```bash
# Test connectivity to API endpoints
curl -v https://api.anthropic.com/v1/messages 2>&1 | head -20
```

Look for DNS resolution failures, connection timeouts, or TLS errors. Configure the `HTTP_PROXY` and `HTTPS_PROXY` environment variables if behind a proxy.

---

### Sandbox Not Available

**Symptom:** Doctor reports `Sandbox: not available` with a reason.

This is a warning, not a failure. The agent runs without kernel sandboxing in this case, relying on Shield policies as the primary security layer. Shield evaluates every tool call regardless of sandbox status.

**Linux (Landlock):**

::: info Linux-specific diagnostics
The following commands check Landlock LSM availability on Linux.
:::

| Check | Command | Expected |
|---|---|---|
| Kernel version | `uname -r` | 5.13+ for filesystem, 6.7+ for network |
| Landlock in LSM | `cat /sys/kernel/security/lsm` | Should include `landlock` |
| Container support | `cat /proc/1/cgroup` | If in a container, may need `--privileged` |

Landlock requires kernel 5.13 or later. Filesystem restrictions are available from 5.13, and network restrictions from 6.7 (Landlock ABI V4). If Landlock is not in the LSM list, it may be disabled in the kernel configuration. Some container runtimes (Docker without `--privileged`, some Kubernetes configurations) do not expose Landlock to containers.

**macOS (sandbox-exec):**

```bash
which sandbox-exec
# Expected: /usr/bin/sandbox-exec

# Test that sandbox-exec works
sandbox-exec -p '(version 1)(deny default)' /usr/bin/true
# Expected: exits cleanly (exit code 0)
```

`sandbox-exec` should be available on all supported macOS versions. If it is missing, the macOS installation may be non-standard.

**Windows (Job Objects):**

Job Objects are used for process isolation. They restrict child process spawning but do not restrict filesystem or network access. The sandbox warning on Windows typically means Job Objects could not be created, which can happen in some restricted environments.

To inspect a running process for Job Object assignment (PowerShell):

```powershell
Get-Process -Id $PID | Select-Object -Property Id, ProcessName, Handle, StartTime
```

---

### Shield Blocking Expected Actions

**Symptom:** The agent attempts an action that should be allowed, but Shield blocks it.

**Step 1: Identify the block**

```bash
openparallax audit --type ACTION_BLOCKED
```

Each blocked entry includes the tier that blocked it, the confidence score, and the reasoning.

**Step 2: Check the active policy**

```bash
grep policy_file config.yaml
```

The `strict.yaml` policy blocks many operations at Tier 0 or requires Tier 2 evaluation. The `permissive.yaml` policy allows most operations at Tier 0.

To see what the policy says about a specific action type:

```bash
grep -A 5 "write_file" <workspace>/policies/default.yaml
```

**Step 3: Check the daily budget**

Tier 2 evaluations call the LLM evaluator. If the daily budget (`general.daily_budget` in the policy file) is exhausted, Tier 2 evaluations fail-closed and return BLOCK. Check remaining budget in the audit log:

```bash
openparallax audit --type ACTION_EVALUATED | grep "tier.*2" | wc -l
```

Compare against the configured daily budget.

**Step 4: Check verdict caching**

Shield caches verdicts to avoid redundant evaluations. A previous BLOCK verdict for the same action hash may be served from cache. The cache TTL is controlled by `general.verdict_ttl_seconds` (default: 60). Wait for the TTL to expire, or restart the agent to clear the cache:

```bash
openparallax restart
```

**Step 5: Adjust the policy**

If the policy is too restrictive for your use case:

```yaml
shield:
  policy_file: policies/permissive.yaml
```

Or edit the policy file to add an explicit ALLOW rule for the action type that is being blocked.

---

### Missing Policy File

**Symptom:** Doctor reports "policy file not found" warning, or Shield fails with a policy load error.

**Step 1: Check the path**

Policy file paths in config.yaml are relative to the workspace directory:

```yaml
shield:
  policy_file: policies/default.yaml
```

This resolves to `<workspace>/policies/default.yaml`. Verify the file exists:

```bash
ls -la <workspace>/policies/
```

**Step 2: Regenerate defaults**

If the policy files were accidentally deleted:

```bash
# Create a temporary workspace to get fresh templates
mkdir /tmp/op-temp
cd /tmp/op-temp
openparallax init

# Copy the policies directory to your workspace
cp -r /tmp/op-temp/.openparallax/*/policies/ <your-workspace>/policies/

# Clean up
rm -rf /tmp/op-temp
```

**Step 3: Verify the evaluator prompt**

Shield Tier 2 uses an evaluator prompt file. If this file is missing, Tier 2 evaluations fail-closed (BLOCK). Check:

```bash
ls <workspace>/policies/evaluator-prompt.txt
```

---

### Broken Audit Chain

**Symptom:** Doctor reports "Audit: CHAIN BROKEN" or `openparallax audit --verify` fails.

The SHA-256 hash chain in `audit.jsonl` is an integrity mechanism. Each entry includes the hash of the previous entry, forming a chain. If any entry is modified, inserted, or deleted, the chain breaks.

**Step 1: Verify the chain and identify the break point**

```bash
openparallax audit --verify
```

The output shows the entry number where the chain breaks and the expected vs. actual hash.

**Step 2: Understand what broke it**

| Cause | What happened |
|---|---|
| Manual edit | Someone edited audit.jsonl directly |
| Truncation | Disk full during write, OS crash, or power loss mid-write |
| Concurrent write | Two processes wrote to the file simultaneously (should not happen with WAL mode) |
| File corruption | Disk hardware failure or filesystem error |

**Step 3: Investigate**

If the chain breaks at a specific entry, examine the surrounding entries:

```bash
# Show entries around the break (example: break at line 42)
sed -n '40,45p' <workspace>/.openparallax/audit.jsonl | python3 -m json.tool
```

Look for:
- Entries with unusual timestamps (out of order)
- Entries with truncated JSON (partial write)
- Entries with modified fields (manual tampering)

**Step 4: Preserve and reset**

If the chain is broken beyond repair and you need to start fresh:

```bash
# Preserve the broken file for forensic review
cp <workspace>/.openparallax/audit.jsonl <workspace>/.openparallax/audit.jsonl.broken.$(date +%Y%m%d)

# Remove the broken file
rm <workspace>/.openparallax/audit.jsonl

# The next action starts a fresh chain
```

Keep the broken file. Even with a broken chain, the individual entries are still valid JSON and contain useful forensic data. Only the integrity guarantee is lost.

---

### Memory Search Not Finding Results

**Symptom:** The agent cannot find information you know it should remember, or `openparallax memory search` returns no results.

**Step 1: Was the session OTR?**

OTR sessions do not write to memory. If the information was shared in an OTR session, it was intentionally not persisted. There is no way to recover OTR data.

**Step 2: Check the memory directly**

```bash
openparallax memory show
openparallax memory search "your search terms"
```

**Step 3: Check MEMORY.md**

The agent's working memory is stored in `<workspace>/MEMORY.md`. Open this file to see what the agent has explicitly recorded:

```bash
cat <workspace>/MEMORY.md
```

Not everything from conversations is stored in MEMORY.md. The agent selects key facts and decisions to persist. Conversational details are in the session history (SQLite) and daily logs.

**Step 4: Check embedding provider**

Without an embedding provider, memory search uses FTS5 keyword matching only. Semantic queries like "how to deploy the application" will not match entries that use words like "release process" or "pushing to production".

```bash
openparallax doctor | grep -i embedding
```

If not configured, add an embedding provider:

```yaml
memory:
  embedding:
    provider: openai
    model: text-embedding-3-small
```

**Step 5: Check daily logs**

Daily logs capture a summary of each day's conversations. Search them:

```bash
openparallax memory search "your query" --scope logs
```

---

### High Latency / Slow Responses

**Symptom:** The agent takes many seconds or minutes to respond.

**Step 1: Identify the bottleneck**

Check the engine log for timing information:

```bash
openparallax logs --event tool_call
openparallax logs --event shield_verdict
```

Look at `duration_ms` fields to identify which stage is slow.

**Step 2: Check the model**

Larger models are slower. For faster responses:

```yaml
llm:
  model: claude-haiku-4-5-20251001  # faster than sonnet
```

**Step 3: Check Shield evaluation overhead**

If many actions escalate to Tier 2, each one requires an LLM call to the evaluator. This adds 1-3 seconds per evaluation. Check how many Tier 2 evaluations are happening:

```bash
openparallax audit --type ACTION_EVALUATED | grep "tier.*2" | wc -l
```

If too many actions escalate, adjust your policy to allow common actions at Tier 0 or Tier 1.

**Step 4: Check context window size**

Long conversations accumulate context. When the context approaches the model's limit, the engine triggers compaction (summarizing earlier messages), which adds latency. Check session length:

```bash
openparallax logs --event pipeline_start | tail -5
```

If `context_tokens` is high (>80% of the model's limit), start a new session with `/new`.

**Step 5: Check embedding calls**

Each memory operation may call the embedding API. If the embedding provider is slow or rate-limited, memory operations add latency. Consider:
- Using a local embedding model via Ollama
- Disabling embedding if semantic search is not needed

**Step 6: Check network latency**

Slow API responses from the LLM provider dominate total latency. Test with a direct API call:

```bash
time curl -s -H "x-api-key: $ANTHROPIC_API_KEY" \
     -H "anthropic-version: 2023-06-01" \
     -H "content-type: application/json" \
     https://api.anthropic.com/v1/messages \
     -d '{"model":"claude-sonnet-4-20250514","max_tokens":10,"messages":[{"role":"user","content":"hi"}]}'
```

If the direct call is slow (>2 seconds for a trivial message), the issue is network latency to the provider.

---

### OTR Mode Issues

**Symptom:** Certain features do not work in OTR mode, or OTR mode does not activate.

**What OTR mode restricts:**

OTR (Off-The-Record) mode is designed to leave no persistent trace. The following is blocked:

| Feature | Normal | OTR |
|---|---|---|
| Session stored in SQLite | Yes | No (in-memory only) |
| Messages logged to daily log | Yes | No |
| Memory persistence | Yes | No |
| Filesystem write tools | Yes | Blocked |
| Memory write tools | Yes | Blocked |
| Audit logging | Yes | Yes (security audit is always on) |
| Shield evaluation | Yes | Yes |

**Why can't I write files in OTR?**

OTR mode filters tools at the definition level. Tools that create persistent outputs (file writes, memory writes) are removed from the tool list before the LLM sees them. The agent cannot call tools it does not know about. This is by design -- OTR means no persistent changes.

**Activating OTR:**

Type `/otr` in any channel (CLI, web, Telegram, etc.). The session switches to OTR mode. In the web UI, the accent color changes from cyan to amber to provide a visual indicator.

**Deactivating OTR:**

Type `/otr` again to toggle back to normal mode. Or start a new session with `/new` -- new sessions always start in normal mode.

**OTR data after restart:**

OTR session data lives in a `sync.Map` in memory. If the engine restarts, all OTR session data is lost permanently. This is intentional.

---

### Channel Adapter Not Starting

**Symptom:** A messaging channel (Telegram, Discord, Slack, Signal, Teams) is enabled in config but the bot does not respond.

**Step 1: Check the engine log**

```bash
openparallax logs --level error | grep -i "channel\|telegram\|discord\|slack\|signal\|teams"
```

The adapter logs its startup status. Look for authentication failures, missing tokens, or connection errors.

**Step 2: Verify environment variables**

Each adapter requires specific environment variables. Check that they are set:

```bash
# Telegram
echo $TELEGRAM_BOT_TOKEN

# Discord
echo $DISCORD_BOT_TOKEN

# Slack
echo $SLACK_BOT_TOKEN
echo $SLACK_APP_TOKEN

# Signal
which signal-cli

# Teams
echo $TEAMS_APP_ID
echo $TEAMS_APP_PASSWORD
```

**Step 3: Check adapter-specific issues**

Each adapter has its own troubleshooting section in its documentation:
- [Telegram troubleshooting](/channels/telegram#troubleshooting)
- [Discord troubleshooting](/channels/discord#troubleshooting)
- [Slack troubleshooting](/channels/slack#troubleshooting)
- [Signal troubleshooting](/channels/signal#troubleshooting)
- [Teams troubleshooting](/channels/teams#troubleshooting)

**Step 4: Retry behavior**

Adapters that fail on startup are retried up to 5 times with 30-second delays. Check the log for retry attempts:

```bash
openparallax logs | grep -i "retry\|reconnect"
```

If all retries fail, the adapter is disabled for the current engine session. Fix the issue and restart.

## Reading Engine Logs

When started with `-v`, the engine writes structured JSON logs to `<workspace>/.openparallax/engine.log`. Each line is a self-contained JSON object.

### Viewing Logs

```bash
# Latest entries (default: 50 lines)
openparallax logs

# More lines
openparallax logs --lines 200

# Filter by level
openparallax logs --level error      # errors only
openparallax logs --level warn       # warnings and errors
openparallax logs --level info       # info, warnings, and errors
openparallax logs --level debug      # everything

# Filter by event type
openparallax logs --event shield_verdict
openparallax logs --event tool_call
openparallax logs --event pipeline_error
openparallax logs --event pipeline_start

# Combine filters
openparallax logs --level error --lines 100
```

### Log Entry Format

Each line is a JSON object:

```json
{
  "time": "2026-04-03T10:15:30Z",
  "level": "info",
  "event": "shield_verdict",
  "session_id": "sess_abc123",
  "action_type": "write_file",
  "tier": 0,
  "decision": "ALLOW",
  "confidence": 1.0,
  "duration_ms": 0.2
}
```

Fields vary by event type, but `time`, `level`, and `event` are always present.

### Key Events

| Event | Description | Key Fields |
|---|---|---|
| `pipeline_start` | A new message entered the pipeline | `session_id`, `message_length`, `context_tokens` |
| `tool_call` | A tool was called | `session_id`, `action_type`, `action_id` |
| `shield_verdict` | Shield evaluated an action | `action_type`, `tier`, `decision`, `confidence`, `duration_ms` |
| `tool_result` | A tool returned its result | `action_id`, `success`, `duration_ms` |
| `pipeline_error` | An error occurred during processing | `session_id`, `error`, `stage` |
| `pipeline_complete` | Pipeline finished processing a message | `session_id`, `rounds`, `total_duration_ms` |
| `heartbeat_fire` | A scheduled task fired | `task_name`, `schedule` |
| `heartbeat_reload` | HEARTBEAT.md was reloaded | `task_count` |
| `session_create` | A new session was created | `session_id`, `channel` |
| `memory_write` | A memory entry was stored | `session_id`, `entry_type` |
| `channel_start` | A channel adapter started | `channel`, `status` |
| `channel_error` | A channel adapter encountered an error | `channel`, `error` |

### Common Log Patterns

**Healthy pipeline execution:**

```
pipeline_start  -> tool_call -> shield_verdict (ALLOW) -> tool_result -> pipeline_complete
```

**Blocked action:**

```
pipeline_start -> tool_call -> shield_verdict (BLOCK) -> pipeline_complete
```

The tool is never executed. The agent receives a "blocked by Shield" result.

**Tier escalation:**

```
shield_verdict tier=0 decision=ESCALATE -> shield_verdict tier=1 decision=ESCALATE -> shield_verdict tier=2 decision=ALLOW
```

The action escalated through all three tiers before being allowed.

**Pipeline error:**

```
pipeline_start -> tool_call -> shield_verdict (ALLOW) -> tool_result (error) -> pipeline_error
```

The tool was allowed but failed during execution.

## Audit Verification

The audit log (`audit.jsonl`) is an append-only file with a SHA-256 hash chain. Each entry includes the hash of the previous entry, making tampering detectable.

### Verifying the Chain

```bash
openparallax audit --verify
```

Output when valid:

```
Audit chain: VALID (47 entries)
```

Output when broken:

```
Audit chain: BROKEN at entry 23
  Expected hash: a1b2c3d4...
  Actual hash:   e5f6g7h8...
```

### What a Broken Chain Means

A broken chain means one of:

1. **An entry was modified** -- someone changed the content of an audit entry
2. **An entry was inserted** -- a new entry was added between existing ones
3. **An entry was deleted** -- an entry was removed from the middle of the file
4. **Partial write** -- a crash or disk error caused an incomplete entry

A broken chain does not necessarily mean malicious tampering. Disk errors, crashes during write, and process kills can all cause partial entries that break the chain.

### Investigating a Break

```bash
# Show entries around the break point
openparallax audit --session all | head -30

# Look at raw entries (example: break at line 23)
sed -n '21,25p' <workspace>/.openparallax/audit.jsonl
```

Check for:
- Truncated JSON (partial write from a crash)
- Out-of-order timestamps (inserted entry)
- Missing entries (gap in sequence numbers)

### Querying the Audit Log

```bash
# All entries for a session
openparallax audit --session sess_abc123

# Filter by type
openparallax audit --type ACTION_PROPOSED
openparallax audit --type ACTION_EXECUTED
openparallax audit --type ACTION_BLOCKED

# Recent entries
openparallax audit --lines 20
```

## Getting Help

If you cannot resolve an issue:

1. Run `openparallax doctor` and note all failures and warnings
2. Check the engine log for errors: `openparallax logs --level error`
3. Verify the audit chain: `openparallax audit --verify`
4. Check that environment variables are set correctly
5. Try starting with verbose logging: `openparallax start -v`
6. Open an issue on GitHub with:
   - Doctor output (full text)
   - Relevant log entries (from `openparallax logs --level error`)
   - Steps to reproduce the issue
   - OpenParallax version (`openparallax --version`)
   - OS and kernel version (`uname -a`)

## Next Steps

- [CLI Commands](/guide/cli) -- full command reference including doctor
- [Security](/guide/security) -- Shield configuration and policies
- [Configuration](/guide/configuration) -- all config.yaml options
- [Channels](/guide/channels) -- channel adapter troubleshooting
