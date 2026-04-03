# Heartbeat

Heartbeat is OpenParallax's cron-based scheduler. It reads task definitions from `HEARTBEAT.md` in the workspace and fires them automatically at specified times. This enables proactive agent behavior — the agent can perform recurring tasks without being prompted.

## How It Works

1. On startup, the engine reads and parses `HEARTBEAT.md` for cron entries
2. A background loop ticks every 60 seconds
3. Each tick, all cron entries are checked against the current time
4. Matching entries fire their task through the standard pipeline (including Shield evaluation)
5. Each entry is tracked to prevent double-firing within the same minute

## HEARTBEAT.md Format

The file uses a simple list format where each entry is a markdown list item containing a cron expression in backticks followed by a task description:

```markdown
# Heartbeat

Proactive scheduled tasks. Add entries below to have the agent run tasks automatically.

---
schedules: []
---

- `0 9 * * *` — Check email and summarize unread messages
- `30 17 * * 1,2,3,4,5` — Write a daily standup summary
- `0 */4 * * *` — Check for new GitHub notifications
- `0 8 * * 1` — Review weekly calendar and flag conflicts
```

### Cron Expression Format

Each cron expression has 5 fields:

```
┌───────── minute (0-59)
│ ┌─────── hour (0-23)
│ │ ┌───── day of month (1-31)
│ │ │ ┌─── month (1-12)
│ │ │ │ ┌─ day of week (0-6, 0=Sunday)
│ │ │ │ │
* * * * *
```

**Supported syntax:**

| Syntax | Meaning | Example |
|--------|---------|---------|
| `*` | Every value | `* * * * *` (every minute) |
| `N` | Exact value | `30 9 * * *` (9:30 AM) |
| `*/N` | Every N values | `*/15 * * * *` (every 15 minutes) |
| `N,M` | Multiple values | `0,30 * * * *` (on the hour and half hour) |

### Task Description

The text after the em dash (`—`), en dash (`–`), or hyphen (`-`) is the task description. This is sent to the agent as a message, which then processes it through the normal pipeline — loading tools, calling Shield, executing actions.

## Examples

### Daily Email Summary

```markdown
- `0 9 * * *` — Check my email inbox and give me a summary of anything important
```

Fires at 9:00 AM every day. The agent loads email tools, reads the inbox, and produces a summary.

### Weekday Standup

```markdown
- `30 8 * * 1,2,3,4,5` — Review what I worked on yesterday based on git logs and session history, then write a standup summary
```

Fires at 8:30 AM Monday through Friday. The agent checks git logs and recent sessions to compose a standup.

### Periodic Health Check

```markdown
- `0 */6 * * *` — Run a workspace health check: verify disk space, check for stale branches, and report any issues
```

Fires every 6 hours. The agent runs diagnostics and reports findings.

### Weekly Calendar Review

```markdown
- `0 8 * * 1` — Read my calendar for this week, identify any conflicts or back-to-back meetings, and suggest adjustments
```

Fires at 8:00 AM on Mondays.

### Monthly Backup Reminder

```markdown
- `0 10 1 * *` — Remind me to review and rotate API keys, and check that backups are current
```

Fires at 10:00 AM on the first day of each month.

### Git Repository Maintenance

```markdown
- `0 2 * * 0` — List all local git branches that have been merged into main and suggest which ones to delete
```

Fires at 2:00 AM on Sundays.

## Managing Scheduled Tasks

### Via the Agent

Ask the agent to create or modify scheduled tasks:

```
Schedule a daily email check at 9am
```

The agent uses the `create_schedule` tool to add an entry to HEARTBEAT.md. Schedule modifications are evaluated at Tier 2 in the default Shield policy because they affect the agent's autonomous behavior.

### Via Tools

Three schedule tools are available in the `schedule` tool group:

| Tool | Description |
|------|-------------|
| `create_schedule` | Add a new cron entry to HEARTBEAT.md |
| `delete_schedule` | Remove a cron entry |
| `list_schedules` | List all scheduled tasks |

### Manual Editing

You can edit HEARTBEAT.md directly with any text editor. The engine reloads the file periodically to pick up changes. For immediate effect, restart the agent.

### Checking Scheduled Tasks

The `openparallax doctor` command reports the number of scheduled tasks:

```
  HEARTBEAT        3 scheduled tasks
```

## Security

Scheduled task modifications go through Shield evaluation:

- **Default policy:** HEARTBEAT.md modifications are evaluated at Tier 2 (LLM evaluator). This prevents an injection from silently adding malicious scheduled tasks.
- **Strict policy:** Same Tier 2 evaluation.
- **Permissive policy:** Schedule creation and deletion are allowed without higher-tier evaluation.

When a scheduled task fires, the resulting tool calls go through the normal Shield pipeline. A scheduled task cannot bypass security — it is processed identically to a user-initiated request.

## Reload Behavior

The heartbeat loop reloads HEARTBEAT.md when:

- The engine starts
- The file is modified via the `create_schedule` or `delete_schedule` tools
- The engine restarts

The reload parses all cron entries and resets the firing state. Entries that already fired in the current minute will not fire again.

## Limitations

- **Minimum granularity:** 1 minute (the tick loop runs every 60 seconds)
- **No seconds field:** Standard 5-field cron only
- **No timezone specification:** Uses the system timezone
- **No range syntax:** Use comma-separated values instead of `1-5` (use `1,2,3,4,5`)
- **Agent must be running:** Tasks only fire when the engine is active. Missed tasks are not retroactively executed.

## Next Steps

- [Tools](/guide/tools) — the schedule tool group
- [Security](/guide/security) — how Shield evaluates schedule changes
- [Configuration](/guide/configuration) — workspace path where HEARTBEAT.md lives
