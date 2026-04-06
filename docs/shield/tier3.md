# Tier 3 — Human Approval

Tier 3 is the final tier of the Shield pipeline. When the classifier (Tier 1) and LLM evaluator (Tier 2) cannot confidently determine whether an action is safe, Shield escalates to the user for a human decision.

## When Tier 3 Triggers

Shield returns `VerdictEscalate` when:

- Tier 1 classifies an action as suspicious but below the block threshold
- Tier 2 evaluates an action as ambiguous (not clearly ALLOW or BLOCK)
- A policy verify rule escalates beyond the available automated tiers

The engine routes the escalation through the Tier3Manager, which broadcasts the approval request to all connected clients.

## Approval Flow

```
Shield returns VerdictEscalate
  → Engine checks Tier 3 rate limit (max per hour)
  → Broadcasts tier3_approval_required event to:
      - Web UI (via EventBroadcaster → WebSocket)
      - CLI TUI (via gRPC stream)
      - Messaging channels (via ApprovalNotifier → Telegram, Discord, etc.)
  → Blocks the tool call, waiting for response
  → First response wins (approve or deny)
  → Timeout defaults to deny (configurable, default 300 seconds)
```

## Event Structure

The `tier3_approval_required` event payload:

```json
{
  "action_id": "unique-action-id",
  "tool_name": "execute_command",
  "reasoning": "Shell command modifies system configuration",
  "timeout_secs": 300
}
```

## Responding to Approvals

### Web UI

The `Tier3Approval.svelte` component displays a card with the action details, reasoning, and a countdown timer. The user clicks Approve or Deny. The decision is sent via WebSocket:

```json
{
  "type": "tier3_decision",
  "action_id": "unique-action-id",
  "decision": "approve"
}
```

### gRPC (CLI)

The `ResolveApproval` RPC on ClientService delivers the decision:

```protobuf
rpc ResolveApproval(ApprovalResponse) returns (ApprovalAck);
```

### Messaging Channels

Channel adapters that implement the `ApprovalHandler` interface receive approval requests and format them for their platform (inline buttons for Telegram, reactions for Discord). Responses route back through `HandleApprovalResponse` on the channel manager.

## Session Binding

Approval decisions are bound to the session that created them. A WebSocket client can only approve or deny actions from its own session. This prevents cross-session approval spoofing.

## Rate Limiting

Tier 3 has a separate hourly rate limit (default: 10 per hour, configurable via `shield.tier3.max_per_hour`). When the limit is exhausted, escalation requests are denied immediately without prompting the user. This prevents approval fatigue attacks.

## Configuration

```yaml
shield:
  tier3:
    max_per_hour: 10
    timeout_seconds: 300
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `max_per_hour` | int | 10 | Maximum Tier 3 prompts per hour |
| `timeout_seconds` | int | 300 | Seconds before unanswered approval auto-denies |

## Timeout Behavior

When the timeout expires without a response, the action is **denied** (fail-closed). The tool call returns an error to the LLM explaining that approval timed out. The LLM can inform the user or attempt an alternative approach.
