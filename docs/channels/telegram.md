# Telegram

The Telegram adapter connects your agent to Telegram using the Bot API. It supports both long-polling and webhook delivery, text and document messaging, group chats, inline keyboards for action approvals, and MarkdownV2 formatted responses.

## Prerequisites

1. **Telegram Account** -- Any Telegram account
2. **Bot Token** -- Created via @BotFather
3. **Public URL** -- Required only if using webhook mode (not needed for long-polling)

## Setup

### 1. Create a Bot via BotFather

Open Telegram and search for [@BotFather](https://t.me/BotFather). BotFather is Telegram's official bot for creating and managing bots. Send it the `/newbot` command and follow the interactive prompts:

1. Send `/newbot`
2. Choose a display name for your bot (e.g., "My Atlas Agent")
3. Choose a username that ends in `bot` (e.g., `my_atlas_bot`)
4. BotFather replies with a token like `1234567890:ABCDefGHIJKlmnOPQRSTuvwxyz`

Keep this token secret. Anyone with the token can control your bot.

**Optional BotFather configuration:**

| Command | Purpose |
|---|---|
| `/setdescription` | Short description shown on the bot's profile |
| `/setabouttext` | Bio text shown when users open the bot |
| `/setuserpic` | Profile photo for the bot |
| `/setcommands` | Register command hints (see the Commands section below) |

To register command hints so Telegram shows autocomplete suggestions in the chat input field:

```
/setcommands
```

Then send a list in the format `command - description`:

```
new - Start a new session
otr - Switch to Off-The-Record mode
help - Show available commands
status - Show agent status
```

### 2. Get Your User ID

You need your numeric Telegram user ID to restrict access. Your user ID is not your username -- it is an integer that uniquely identifies your account.

1. Search for [@userinfobot](https://t.me/userinfobot) on Telegram
2. Send it any message
3. It replies with your user ID (a number like `123456789`)

If you want other people to use the bot, have them do the same and send you their user IDs.

### 3. Set Environment Variable

```bash
export TELEGRAM_BOT_TOKEN="1234567890:ABCDefGHIJKlmnOPQRSTuvwxyz"
```

Add this to your shell profile (`~/.bashrc`, `~/.zshrc`) so it persists across sessions.

### 4. Configure OpenParallax

**Long-polling mode (recommended for personal use):**

```yaml
channels:
  telegram:
    enabled: true
    token_env: TELEGRAM_BOT_TOKEN
    allowed_users:
      - 123456789
    polling_interval: 1
```

**Webhook mode (for always-on deployments):**

```yaml
channels:
  telegram:
    enabled: true
    token_env: TELEGRAM_BOT_TOKEN
    webhook_url: "https://your-domain.com/webhooks/telegram"
    allowed_users:
      - 123456789
```

| Field | Required | Description |
|---|---|---|
| `enabled` | Yes | Enable the Telegram adapter |
| `token_env` | Yes | Environment variable containing the bot token from @BotFather |
| `allowed_users` | No | Telegram user IDs allowed to interact. Empty list allows all users. |
| `polling_interval` | No | Seconds between poll requests in long-polling mode (default: 1) |
| `webhook_url` | No | Public URL for receiving Telegram updates. Leave empty for long-polling. |

## How It Works

### Long-Polling vs. Webhook

The adapter supports two delivery modes for receiving messages from Telegram.

**Long-polling** calls the Telegram `getUpdates` endpoint in a loop with a 30-second timeout. When no messages are pending, the request blocks until a message arrives or the timeout expires. This approach requires no public URL, works behind firewalls and NAT, and is the simplest way to run the bot for personal use. The `polling_interval` field controls the delay between successive poll cycles (default: 1 second). The actual latency for message delivery is typically under 1 second because the long-poll returns immediately when a message arrives.

**Webhook** mode registers a URL with Telegram. When a message arrives, Telegram sends an HTTPS POST to the configured `webhook_url`. This requires a publicly reachable HTTPS endpoint. Use a reverse proxy (nginx, Caddy) or a tunnel (ngrok, Cloudflare Tunnel) to expose the webhook endpoint. Webhook mode is more efficient for high-traffic bots because it eliminates polling overhead.

When `webhook_url` is set, the adapter registers the webhook with Telegram on startup and deregisters it on shutdown. When `webhook_url` is empty, the adapter uses long-polling.

### Message Flow

```
Telegram user sends message
       |
       v
  getUpdates (polling) / webhook POST
       |
       v
  Parse Update object
       |
       v
  Extract: message text, user ID, chat ID, message type
       |
       v
  Access control: check user ID against allowed_users
       |                                    |
       | (allowed)                          | (denied)
       v                                    v
  Rate limit check (30/min/user)        "This agent is private."
       |                                    |
       | (within limit)                     | (exceeded)
       v                                    v
  Check for slash command prefix        "Rate limit exceeded."
       |                     |
       | (not a command)     | (/new, /otr, /help, /status)
       v                     v
  engine.ProcessMessage   Handle command locally
  ForWeb()                (reset session, toggle OTR, etc.)
       |
       v
  Stream events via EventSender
       |
       v
  Collect response text
       |
       v
  sendMessage (MarkdownV2) or sendDocument
```

### Message Normalization

The adapter normalizes every incoming Telegram update into the common message format before it enters the engine pipeline:

- **Sender** -- `telegram:<user_id>` (e.g., `telegram:123456789`)
- **Session ID** -- Derived from the chat ID. Each Telegram chat (private or group) maps to one session. The session persists across restarts because the chat ID is stable.
- **Message content** -- The text body of the message. For documents and images with captions, the caption text is used. For messages with no text (e.g., a photo with no caption), the adapter sends a description like `[Photo]`.
- **Mode** -- Normal or OTR, depending on the current session state.

### Rate Limiting

Built-in rate limiting: 30 messages per minute per user. The rate limiter uses a sliding window. Users who exceed the limit receive a "Rate limit exceeded" message. The limit applies per user, not per chat, so a user in multiple group chats shares one rate limit budget.

## Supported Message Types

### Incoming

| Type | Supported | Notes |
|---|---|---|
| Text | Yes | Plain text messages are passed directly to the pipeline |
| Photos | Partial | Caption is extracted as text. The image itself is not processed. |
| Documents | Partial | Caption is extracted as text. The file is not downloaded. |
| Voice/Video | No | Ignored |
| Stickers | No | Ignored |
| Location | No | Ignored |
| Polls | No | Ignored |

### Outgoing

| Type | Supported | Notes |
|---|---|---|
| Text (MarkdownV2) | Yes | All responses are formatted with MarkdownV2 |
| Documents | Yes | File outputs from the engine are sent via `sendDocument` |
| Inline keyboards | Yes | Used for Shield action approvals (see below) |
| Images | No | — |
| Audio | No | — |

### MarkdownV2 Escaping

Telegram's MarkdownV2 format requires escaping 18 special characters. The adapter handles this automatically when sending messages. The escaped characters are: `_ * [ ] ( ) ~ ` > # + - = | { } . !`

Long messages (over 4,096 characters) are automatically split at paragraph boundaries. Each chunk is sent as a separate message to stay within Telegram's length limit.

## Private Chat vs. Group Chat

### Private Chat (DM)

In a private chat with the bot, every message is processed. No mention or prefix is needed. The session is tied to the user's chat ID, which means the conversation history persists naturally.

### Group Chat

When the bot is added to a group chat, its behavior depends on the privacy mode setting:

- **Privacy mode ON (default):** The bot only receives messages that start with `/` (commands) or that mention the bot by @username. Regular messages between group members are invisible to the bot.
- **Privacy mode OFF:** The bot receives all messages in the group. Disable privacy mode via BotFather: `/setprivacy` then select "Disable".

In group chats, the session ID is derived from the group chat ID, not the individual user ID. All group members share one session. Access control still applies per-user -- if `allowed_users` is set, only listed users' messages are processed, even within a group.

To change privacy mode:

1. Open BotFather
2. Send `/setprivacy`
3. Select your bot
4. Choose "Disable" to receive all group messages

## Commands

The adapter intercepts messages that start with `/` and handles them before they reach the engine pipeline. The bot username suffix (e.g., `/new@my_atlas_bot`) is automatically stripped.

| Command | Action |
|---|---|
| `/new` | Reset the session for this chat and start fresh |
| `/otr` | Toggle Off-The-Record mode for this session |
| `/help` | Show available commands |
| `/status` | Ask the agent for its current status |

Unknown commands (any message starting with `/` that does not match the table above) are silently ignored. This prevents the bot from trying to interpret Telegram commands meant for other bots in group chats.

## Inline Keyboards for Action Approvals

When Shield evaluates a tool call and the verdict requires user confirmation, the adapter sends an inline keyboard with Approve and Deny buttons. This happens for actions where the policy specifies `confirm: true` or when a Tier 2 evaluation returns `CONFIRM`.

```
Shield wants to execute: write_file
Path: /home/user/project/config.json

[Approve]  [Deny]
```

The inline keyboard is tied to the specific action by a callback query ID. Only the user who triggered the action can press the buttons (the adapter verifies the callback query's user ID against the original sender). The buttons expire after 120 seconds -- if neither button is pressed, the action is denied by default (fail-closed).

When the user presses Approve, the adapter sends the approval to the engine and the tool call proceeds. When the user presses Deny, the adapter notifies the engine and the agent receives a "user denied this action" result.

The inline keyboard message is edited after the user's choice to show the outcome:

```
Shield: write_file -- Approved by user
```

## Access Control

When `allowed_users` is configured, only listed Telegram user IDs can interact with the agent. Unauthorized users receive a single "This agent is private." message. Their messages are logged (user ID and timestamp) but not processed.

When the list is empty, all users are accepted. For personal use, always configure `allowed_users` to prevent unauthorized access.

## Development with ngrok

For webhook mode during development, use ngrok to expose a local port:

```bash
ngrok http 3100
```

Use the ngrok HTTPS URL as your `webhook_url`:

```yaml
channels:
  telegram:
    enabled: true
    token_env: TELEGRAM_BOT_TOKEN
    webhook_url: "https://xxxx-xxxx-xxxx.ngrok-free.app/webhooks/telegram"
```

Remember to update the `webhook_url` each time ngrok generates a new URL (unless you have a paid ngrok plan with a stable subdomain).

## Troubleshooting

| Problem | Solution |
|---|---|
| Bot does not respond | Check `allowed_users` -- your user ID must be listed (or the list must be empty) |
| "Unauthorized" error in logs | The bot token is invalid or expired. Generate a new one via BotFather `/token` |
| Messages not arriving in groups | Privacy mode is ON by default. Disable it via BotFather or mention the bot |
| Webhook not receiving updates | Verify the URL is HTTPS and publicly reachable. Check with `curl -I https://your-url/webhooks/telegram` |
| Duplicate responses | The bot may be running in both polling and webhook mode. Ensure only one delivery mode is configured. |
| MarkdownV2 parse errors | The adapter escapes special characters automatically. If you see parse errors, check for raw HTML in agent responses. |

## Next Steps

- [Channels Overview](/channels/) -- architecture and shared concepts
- [Configuration](/guide/configuration) -- full config.yaml reference
- [Security](/guide/security) -- how Shield protects across channels
