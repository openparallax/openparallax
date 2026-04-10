---
description: Multi-platform messaging adapters for AI agents — WhatsApp, Telegram, Discord, Slack, Signal, Teams, and iMessage with a unified interface.
---

# Channels

Channels is a multi-platform messaging adapter system that connects AI agents to WhatsApp, Telegram, Discord, Slack, Signal, Microsoft Teams, and iMessage. Each adapter handles platform-specific webhook processing, message normalization, and response delivery, so you write your agent logic once and deploy it to any messaging platform.

## Why Channels Exists

Every messaging platform has its own API, authentication mechanism, webhook format, message length limits, and rate limiting rules. Building a Telegram bot is completely different from building a WhatsApp integration. Channels abstracts these differences behind a single interface.

Your agent receives normalized messages and sends plain text or Markdown responses. The adapter handles everything else: webhook verification, long-polling, WebSocket connections, message splitting, format conversion, and platform-specific quirks.

## Supported Platforms

| Platform | Transport | Auth | Status |
|---|---|---|---|
| WhatsApp | Webhook (Cloud API) | OAuth access token | Available |
| Telegram | Long-polling (Bot API) | Bot token | Available |
| Discord | WebSocket (Gateway) | Bot token | Available |
| Slack | Socket Mode | Bot + App tokens | Available |
| Signal | subprocess (signal-cli) | Account number | Available |
| Microsoft Teams | Webhook (Graph API) | App ID + password | Available |
| iMessage | AppleScript (Messages.app) | Apple ID | Available (macOS only) |

## Architecture

```
                Messaging Platform
                       │
                       ▼
              ┌─────────────────┐
              │  Channel Adapter │  (platform-specific)
              │  - Webhook/WS   │
              │  - Auth          │
              │  - Normalize     │
              └────────┬────────┘
                       │
                       ▼
              ┌─────────────────┐
              │  Channel Manager │  (platform-agnostic)
              │  - Session map   │
              │  - Message routing│
              │  - Retry logic   │
              └────────┬────────┘
                       │
                       ▼
              ┌─────────────────┐
              │     Engine       │
              │  ProcessMessage  │
              │  ForWeb()        │
              └─────────────────┘
```

### ChannelAdapter Interface

Every adapter implements:

```go
type ChannelAdapter interface {
    Name() string
    Start(ctx context.Context) error
    Stop() error
    SendMessage(chatID string, message *ChannelMessage) error
    IsConfigured() bool
}
```

### Channel Manager

The Manager handles adapter lifecycle and message routing:

- **Session mapping**: Maps `adapter:chatID` pairs to session IDs. Each chat gets a persistent session.
- **Message routing**: Calls `engine.ProcessMessageForWeb()` with a `responseCollector` that aggregates streaming events into a final response string.
- **Retry logic**: Adapters that fail on startup are retried up to 5 times with 30-second delays.
- **Session reset**: The `/new` command in any platform creates a fresh session for that chat.

### Message Normalization

Incoming messages from all platforms are reduced to a string of text content. The Manager routes this to the Engine pipeline. The Engine's response (collected from streaming events) is sent back through the adapter's `SendMessage` method.

### Message Splitting

Platform message length limits vary:

| Platform | Max Length |
|---|---|
| Telegram | 4,096 characters |
| WhatsApp | 4,096 characters |
| Discord | 2,000 characters |
| Slack | 4,096 characters |

`SplitMessage()` splits long responses at paragraph boundaries (newlines, then spaces) to stay within limits. Each chunk is sent as a separate message.

## Standalone Value

The channels module can be used independently of the rest of OpenParallax. Import it, configure an adapter, and connect it to any message handler:

```go
import "github.com/openparallax/openparallax/internal/channels"
import "github.com/openparallax/openparallax/internal/channels/telegram"
```

You provide the engine integration (or any message handler that implements the routing interface). The adapter handles all platform-specific concerns.

## Configuration

All channel adapters are configured in `config.yaml` under the `channels` section:

```yaml
channels:
  telegram:
    enabled: true
    token_env: TELEGRAM_BOT_TOKEN
    allowed_users: [123456789]

  whatsapp:
    enabled: true
    phone_number_id: "1234567890"
    access_token_env: WHATSAPP_ACCESS_TOKEN
    verify_token: "my-verify-token"
    webhook_port: 9443
    allowed_numbers: ["+1234567890"]

  discord:
    enabled: true
    token_env: DISCORD_BOT_TOKEN
    allowed_channels: ["channel-id-1"]
    respond_to_mentions: true

  slack:
    enabled: true
    bot_token_env: SLACK_BOT_TOKEN
    app_token_env: SLACK_APP_TOKEN

  signal:
    enabled: true
    cli_path: /usr/local/bin/signal-cli
    account: "+1234567890"
    allowed_numbers: ["+0987654321"]

  teams:
    enabled: true
    app_id_env: TEAMS_APP_ID
    password_env: TEAMS_APP_PASSWORD

  imessage:
    enabled: true             # macOS only
    apple_id: "you@icloud.com"
```

API keys and tokens are read from environment variables, never stored in the config file.

## Slash Commands

All adapters support platform-specific versions of the slash commands:

| Command | Action |
|---|---|
| `/new` | Reset the session for this chat |
| `/otr` | Start an Off-The-Record session |
| `/help` | Show available commands (Telegram) |
| `/status` | Show agent status (Telegram) |

The adapter intercepts these commands before routing to the engine.

## Security

### Access Control

Every adapter supports an allow-list to restrict who can interact with the agent:

- **WhatsApp**: `allowed_numbers` (phone numbers)
- **Telegram**: `allowed_users` (user IDs)
- **Discord**: `allowed_channels` and `allowed_users` (Discord IDs)
- **Signal**: `allowed_numbers` (phone numbers)

Messages from unauthorized users are logged and silently dropped (or receive a "This agent is private" response on Telegram).

### Rate Limiting

Telegram includes built-in rate limiting (30 messages per minute per user). Other adapters rely on platform-level rate limiting.

### OTR Mode

All adapters support OTR mode via the `/otr` command. OTR sessions use in-memory storage only and do not persist to SQLite. Tool access is restricted (no filesystem writes, no memory persistence).
