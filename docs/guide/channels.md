# Channels

OpenParallax supports multi-channel messaging, allowing you to interact with your agent through various platforms beyond the CLI and web UI. Each channel connects to the same engine and goes through the same pipeline — Shield evaluation, audit logging, and memory persistence apply identically regardless of the channel.

## Architecture

Every channel adapter implements the same pattern:

1. Receive a message from the external platform
2. Normalize it into a standard format (sender, session ID, message content)
3. Call `engine.ProcessMessageForWeb(ctx, sender, sessionID, messageID, content, mode)`
4. Receive pipeline events via the `EventSender` interface
5. Format events back into the platform's message format and send to the user

Each channel maintains independent sessions. A Telegram conversation and a Discord conversation are separate sessions with separate histories.

## Supported Channels

### WhatsApp (Cloud API)

Connect your agent to WhatsApp via the Meta Cloud API.

**Configuration:**

```yaml
channels:
  whatsapp:
    enabled: true
    phone_number_id: "1234567890"
    access_token_env: WHATSAPP_ACCESS_TOKEN
    verify_token: "your-verify-token"
    webhook_path: /webhooks/whatsapp
```

| Field | Description |
|-------|-------------|
| `phone_number_id` | WhatsApp Business phone number ID from Meta Developer Portal |
| `access_token_env` | Environment variable containing the permanent access token |
| `verify_token` | Token for webhook verification (you choose this value) |
| `webhook_path` | URL path where WhatsApp sends webhook events |

**Setup steps:**

1. Create a Meta Developer account and a WhatsApp Business app
2. In the WhatsApp product settings, get your Phone Number ID and generate a permanent access token
3. Set the access token as an environment variable: `export WHATSAPP_ACCESS_TOKEN="..."`
4. Configure the webhook URL in Meta Developer Portal: `https://your-domain:3100/webhooks/whatsapp`
5. Enter the verify token you chose in step 3
6. Subscribe to the `messages` webhook field
7. Enable the channel in config.yaml and restart

The agent needs to be reachable from the internet for WhatsApp webhooks. Use a reverse proxy (nginx, Caddy) or a tunnel (ngrok, Cloudflare Tunnel) to expose the webhook endpoint.

---

### Telegram (Bot API)

Connect via a Telegram bot.

**Configuration:**

```yaml
channels:
  telegram:
    enabled: true
    bot_token_env: TELEGRAM_BOT_TOKEN
    webhook_url: "https://your-domain.com/webhooks/telegram"
    allowed_users: ["123456789"]
```

| Field | Description |
|-------|-------------|
| `bot_token_env` | Environment variable containing the bot token from @BotFather |
| `webhook_url` | Public URL for receiving Telegram updates. Leave empty for long polling. |
| `allowed_users` | List of Telegram user IDs that can interact with the bot. Empty list allows all users. |

**Setup steps:**

1. Create a bot via [@BotFather](https://t.me/BotFather) on Telegram
2. Copy the bot token and set it: `export TELEGRAM_BOT_TOKEN="..."`
3. Find your Telegram user ID (send a message to @userinfobot)
4. Add your user ID to `allowed_users` to restrict access
5. Optionally set a webhook URL for push-based delivery, or leave empty for long polling
6. Enable the channel and restart

Long polling mode does not require a public URL, making it easier to set up for personal use.

---

### Discord (Bot)

Connect via a Discord bot.

**Configuration:**

```yaml
channels:
  discord:
    enabled: true
    bot_token_env: DISCORD_BOT_TOKEN
    application_id: "1234567890"
    guild_ids: ["9876543210"]
```

| Field | Description |
|-------|-------------|
| `bot_token_env` | Environment variable containing the Discord bot token |
| `application_id` | Discord application ID from the Developer Portal |
| `guild_ids` | List of Discord server (guild) IDs where the bot operates. Empty list registers commands globally. |

**Setup steps:**

1. Create an application in the [Discord Developer Portal](https://discord.com/developers/applications)
2. Create a bot user and copy the token: `export DISCORD_BOT_TOKEN="..."`
3. Enable the `MESSAGE CONTENT` privileged gateway intent
4. Generate an invite URL with `bot` and `applications.commands` scopes
5. Invite the bot to your server
6. Add the guild ID to `guild_ids` for server-specific slash commands
7. Enable the channel and restart

The bot responds to direct messages and mentions in channels. Each Discord user gets their own session.

---

### Slack (App)

Connect via a Slack app using Socket Mode or webhooks.

**Configuration:**

```yaml
channels:
  slack:
    enabled: true
    bot_token_env: SLACK_BOT_TOKEN
    app_token_env: SLACK_APP_TOKEN
    signing_secret_env: SLACK_SIGNING_SECRET
```

| Field | Description |
|-------|-------------|
| `bot_token_env` | Bot User OAuth Token (`xoxb-...`) |
| `app_token_env` | App-Level Token (`xapp-...`) for Socket Mode |
| `signing_secret_env` | Signing secret for verifying webhook requests |

**Setup steps:**

1. Create a new app at [api.slack.com](https://api.slack.com/apps)
2. Enable Socket Mode in the app settings
3. Generate an App-Level Token with `connections:write` scope
4. Under OAuth & Permissions, add bot token scopes: `chat:write`, `app_mentions:read`, `im:history`, `im:read`, `im:write`
5. Install the app to your workspace
6. Set environment variables: `SLACK_BOT_TOKEN`, `SLACK_APP_TOKEN`, `SLACK_SIGNING_SECRET`
7. Enable the channel and restart

The bot responds to direct messages and @mentions. Socket Mode means no public URL is required.

---

### Signal (signal-cli)

Connect via Signal using [signal-cli](https://github.com/AsamK/signal-cli) as the transport layer.

**Configuration:**

```yaml
channels:
  signal:
    enabled: true
    signal_cli_path: "/usr/local/bin/signal-cli"
    phone_number: "+1234567890"
    allowed_numbers: ["+1987654321"]
```

| Field | Description |
|-------|-------------|
| `signal_cli_path` | Path to the signal-cli binary |
| `phone_number` | The phone number registered with Signal for the bot |
| `allowed_numbers` | List of phone numbers allowed to message the bot. Empty list allows all. |

**Setup steps:**

1. Install [signal-cli](https://github.com/AsamK/signal-cli#installation)
2. Register or link a phone number: `signal-cli -u +1234567890 register` and verify
3. Set the path and phone number in config.yaml
4. Add trusted phone numbers to `allowed_numbers`
5. Enable the channel and restart

Signal provides end-to-end encrypted messaging. All messages between you and the agent are encrypted in transit.

---

### Microsoft Teams (Graph API)

Connect via Microsoft Teams using the Microsoft Graph API.

**Configuration:**

```yaml
channels:
  teams:
    enabled: true
    app_id: "your-app-id"
    app_password_env: TEAMS_APP_PASSWORD
    tenant_id: "your-tenant-id"
```

| Field | Description |
|-------|-------------|
| `app_id` | Azure AD application (client) ID |
| `app_password_env` | Environment variable containing the client secret |
| `tenant_id` | Azure AD tenant ID |

**Setup steps:**

1. Register an application in [Azure AD](https://portal.azure.com/#blade/Microsoft_AAD_RegisteredApps/ApplicationsListBlade)
2. Add Microsoft Graph API permissions: `ChatMessage.Read`, `ChatMessage.Send`
3. Create a client secret and set it: `export TEAMS_APP_PASSWORD="..."`
4. Create a Bot Channel Registration in Azure
5. Configure the messaging endpoint URL
6. Add the bot to your Teams workspace
7. Enable the channel and restart

---

## Message Normalization

Regardless of the source channel, messages are normalized into a common format before entering the pipeline:

- **Sender** — channel-specific user identifier
- **Session ID** — unique per user per channel
- **Message content** — text content extracted from platform-specific formatting
- **Mode** — normal or OTR

Platform-specific features (reactions, threads, attachments) are handled by each channel adapter as appropriate. Text content is always extracted and passed through.

## Security Across Channels

All channels go through the identical Shield pipeline. There is no bypass for any channel:

- Every tool call is evaluated by Shield
- Every action is logged in the audit trail
- Every channel respects the active policy file
- OTR mode works in all channels (the agent checks session mode, not channel type)

The `allowed_users` / `allowed_numbers` fields on Telegram, Signal, and other channels provide an additional access control layer that restricts who can interact with the bot.

## Multiple Channels Simultaneously

You can enable multiple channels at the same time. The engine handles all channel adapters concurrently:

```yaml
channels:
  telegram:
    enabled: true
    bot_token_env: TELEGRAM_BOT_TOKEN
    allowed_users: ["123456789"]
  discord:
    enabled: true
    bot_token_env: DISCORD_BOT_TOKEN
    application_id: "1234567890"
  slack:
    enabled: true
    bot_token_env: SLACK_BOT_TOKEN
    app_token_env: SLACK_APP_TOKEN
    signing_secret_env: SLACK_SIGNING_SECRET
```

Each channel maintains its own sessions. The web UI and CLI are always available alongside any configured messaging channels.

## Next Steps

- [Configuration](/guide/configuration) — full channel configuration reference
- [Security](/guide/security) — how Shield protects across channels
- [Sessions](/guide/sessions) — how cross-channel sessions work
