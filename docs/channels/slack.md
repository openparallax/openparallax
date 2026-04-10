# Slack

::: warning Planned — Not Yet Implemented
The Slack adapter is on the roadmap but not yet implemented. The configuration schema (`channels.slack`) ships in the binary, but enabling it does nothing — the agent does not connect to Slack. This page documents the planned design.

If you want to help build it, see [Contributing](https://github.com/openparallax/openparallax/blob/main/CONTRIBUTING.md).
:::

The Slack adapter will connect your agent to Slack workspaces using Socket Mode for real-time event delivery and the Web API for sending responses. Socket Mode uses a WebSocket connection initiated from your side, so no public URL or webhook endpoint is required. The adapter is designed to handle message formatting in Slack's `mrkdwn` syntax, reply in threads to keep channels organized, and respect channel restrictions to control where the bot operates.

## Prerequisites

1. **Slack Workspace** -- A workspace where you have admin permissions (or permission to install apps)
2. **Slack App** -- Created in the Slack API portal

## Setup

### 1. Create a Slack App

1. Go to [api.slack.com/apps](https://api.slack.com/apps)
2. Click "Create New App" > "From scratch"
3. Name your app (e.g., "Atlas Agent") and select the workspace

### 2. Configure Bot Permissions

Go to "OAuth & Permissions" in the left sidebar. Under "Bot Token Scopes", add:

| Scope | Purpose |
|---|---|
| `chat:write` | Send messages |
| `channels:history` | Read messages in public channels the bot is in |
| `channels:read` | View public channel info |
| `groups:history` | Read messages in private channels the bot is in |
| `groups:read` | View private channel info |
| `im:history` | Read direct messages |
| `im:read` | View DM info |
| `im:write` | Open DMs with users |
| `app_mentions:read` | Receive @mention events |
| `files:write` | Upload file outputs |

These are bot token scopes (the `xoxb-` token), not user token scopes. The bot acts as its own identity in the workspace, not as a user.

**Bot tokens vs. user tokens:** Bot tokens (`xoxb-`) represent the app itself. They can only access channels the bot has been explicitly invited to. User tokens (`xoxp-`) act on behalf of a user and can access everything that user can see. OpenParallax uses bot tokens exclusively -- user tokens are not supported.

### 3. Enable Socket Mode

1. Go to "Socket Mode" in the left sidebar
2. Toggle "Enable Socket Mode" on
3. When prompted, generate an **App-Level Token** with the `connections:write` scope
4. Name it (e.g., "socket-mode-token") and click "Generate"
5. Copy the token (starts with `xapp-`)

Socket Mode routes all events through a WebSocket connection instead of HTTP webhooks. The adapter opens this connection using the App-Level Token. This means:

- No public URL required
- Works behind firewalls and NAT
- No webhook URL configuration in the Slack app settings
- Connection is initiated outbound from your infrastructure

### 4. Enable Events

1. Go to "Event Subscriptions" in the left sidebar
2. Toggle "Enable Events" on
3. Under "Subscribe to bot events", add:
   - `message.channels` -- messages in public channels
   - `message.groups` -- messages in private channels
   - `message.im` -- direct messages
   - `app_mention` -- @mentions of the bot

With Socket Mode enabled, you do not need to configure a Request URL. Slack routes events through the WebSocket connection instead.

### 5. Configure the Signing Secret

The signing secret is used to verify that incoming requests (in the Events API fallback path) are genuinely from Slack. Even in Socket Mode, the signing secret is part of the app's identity.

1. Go to "Basic Information" in the left sidebar
2. Under "App Credentials", find the **Signing Secret**
3. Copy it

### 6. Install to Workspace

1. Go to "Install App" in the left sidebar
2. Click "Install to Workspace"
3. Review the permissions and click "Allow"
4. Copy the **Bot User OAuth Token** (starts with `xoxb-`)

After installation, the bot appears in the workspace's app directory. It does not automatically join any channels -- you must invite it to channels where you want it to operate.

### 7. Invite the Bot to Channels

In Slack, open the channel where you want the bot to respond and type:

```
/invite @atlas-agent
```

Or click the channel name > "Integrations" > "Add an App". The bot only receives messages from channels it has been invited to.

### 8. Set Environment Variables

```bash
export SLACK_BOT_TOKEN="xoxb-1234567890-1234567890123-abcdefghijklmnopqrstuvwx"
export SLACK_APP_TOKEN="xapp-1-A1234567890-1234567890123-abcdefghijklmnopqrstuvwxyz1234567890abcdefghijklmnopqrstuvwxyz12"
export SLACK_SIGNING_SECRET="abcdef1234567890abcdef1234567890"
```

Add these to your shell profile so they persist across sessions.

### 9. Configure OpenParallax

```yaml
channels:
  slack:
    enabled: true
    bot_token_env: SLACK_BOT_TOKEN
    app_token_env: SLACK_APP_TOKEN
    signing_secret_env: SLACK_SIGNING_SECRET
    allowed_channels:
      - "C1234567890"
    respond_to_mentions: true
```

| Field | Required | Description |
|---|---|---|
| `enabled` | Yes | Enable the Slack adapter |
| `bot_token_env` | Yes | Environment variable containing the Bot User OAuth Token (`xoxb-`) |
| `app_token_env` | Yes | Environment variable containing the App-Level Token (`xapp-`) for Socket Mode |
| `signing_secret_env` | No | Environment variable containing the Signing Secret (used for request verification in Events API mode) |
| `allowed_channels` | No | Channel IDs where the bot responds. Empty list means all channels the bot is invited to. |
| `respond_to_mentions` | No | In channels, only respond when @mentioned (default: false) |

## How It Works

### Socket Mode vs. Events API

The adapter supports two modes for receiving events from Slack.

**Socket Mode** (recommended) uses a WebSocket connection initiated by the adapter using the App-Level Token (`xapp-`). Events are delivered through this connection in real-time. No public URL is needed. This is the recommended mode for personal and internal deployments.

**Events API** uses HTTP POST requests from Slack to a public endpoint. Each request includes a signature header (`X-Slack-Signature`) that the adapter verifies against the signing secret to confirm the request is from Slack. This mode requires a publicly reachable HTTPS endpoint and is typically used for production deployments behind a load balancer.

When both `app_token_env` and `signing_secret_env` are configured, Socket Mode takes precedence. The signing secret is still validated for any direct HTTP requests.

### Request Verification

For Events API mode, every incoming request is verified:

1. Extract the `X-Slack-Request-Timestamp` header
2. Check that the timestamp is within 5 minutes of the current time (prevents replay attacks)
3. Construct the signature base string: `v0:{timestamp}:{body}`
4. Compute HMAC-SHA256 using the signing secret
5. Compare with the `X-Slack-Signature` header

Requests that fail verification are rejected with HTTP 401. This ensures that only Slack can send events to your endpoint.

### Message Flow

```
Slack user sends message or @mentions the bot
       |
       v
  Socket Mode WebSocket / Events API POST
       |
       v
  Parse event envelope
       |
       v
  Filter: ignore messages from the bot itself
       |
       v
  Filter: ignore message subtypes (join, leave, topic change)
       |
       v
  Check allowed_channels (if configured)
       |                              |
       | (allowed)                    | (not in list)
       v                              v
  Check respond_to_mentions       Silently ignored
       |                    |
       | (mentioned or DM)  | (not mentioned, channel)
       v                    v
  Strip @mention prefix     Silently ignored
       |
       v
  engine.ProcessMessageForWeb()
       |
       v
  Collect streaming events
       |
       v
  Format response in mrkdwn
       |
       v
  Post reply (in thread if in channel, direct if in DM)
```

### Session Mapping

Each Slack user gets their own session, identified by `slack:<user_id>`. In channels, even though multiple users see the same channel, each user's messages create separate sessions with independent conversation histories.

## Message Formatting (mrkdwn)

Slack uses its own markup syntax called `mrkdwn`, which differs from standard Markdown. The adapter converts agent responses to mrkdwn format:

| Format | Markdown | Slack mrkdwn |
|---|---|---|
| Bold | `**text**` | `*text*` |
| Italic | `*text*` | `_text_` |
| Strikethrough | `~~text~~` | `~text~` |
| Code | `` `code` `` | `` `code` `` |
| Code block | ` ```code``` ` | ` ```code``` ` |
| Link | `[text](url)` | `<url\|text>` |
| Blockquote | `> text` | `> text` |

The adapter performs this conversion automatically. Agent responses authored in standard Markdown are translated before sending. Code blocks are preserved as-is since the syntax is identical.

Long messages (over 4,096 characters) are split at paragraph boundaries. Each chunk is sent as a separate message.

## Thread Replies

In channels, the adapter replies in threads to keep the main channel clean. The behavior is:

1. When a user sends a message (or @mentions the bot) in a channel, the bot's response is posted as a thread reply to that message.
2. If the user continues the conversation within the thread, the bot replies within the same thread.
3. In DMs, threads are not used -- replies go directly to the DM conversation.

This keeps channel conversations organized. Each user's interaction with the bot is contained in its own thread, and other channel members can follow or ignore specific threads.

Thread replies use the `thread_ts` parameter in the `chat.postMessage` API call. The thread timestamp is derived from the original message that started the conversation.

## Channel Restriction

The `allowed_channels` field controls which channels the bot operates in. When configured:

- The bot only processes messages from listed channel IDs
- Messages in other channels are silently ignored (no rejection message)
- DMs are always allowed regardless of channel restrictions
- The bot must still be invited to the channel to receive messages

To find a channel's ID: right-click the channel name in Slack > "View channel details" > the channel ID is at the bottom of the details panel (starts with `C`).

When `allowed_channels` is empty, the bot responds in every channel it has been invited to.

## Slash Commands

| Command | Action |
|---|---|
| `/new` | Reset the session and start fresh |
| `/otr` | Switch to Off-The-Record mode |

These are handled by the adapter as text commands (the adapter intercepts messages starting with `/`). They are not registered as Slack slash commands in the app configuration. This means they do not appear in Slack's `/` command autocomplete -- users type them as regular messages.

## Access Control

### Channel Restriction

The primary access control mechanism in Slack is channel-based. Invite the bot only to channels where it should operate, and optionally configure `allowed_channels` for an additional restriction layer.

### Bot Visibility

The bot can only see messages in channels it has been explicitly invited to. Unlike user tokens, bot tokens do not have access to channels by default. This provides a natural access boundary -- if the bot is not in a channel, it cannot read or respond to messages there.

### User Filtering

The adapter does not currently support per-user filtering in Slack. Access is controlled at the channel level. To restrict which users can interact with the bot, use a private channel and control channel membership through Slack's built-in access controls.

## Troubleshooting

| Problem | Solution |
|---|---|
| Bot does not respond in a channel | Invite the bot to the channel with `/invite @botname`. Bots only see channels they are in. |
| "invalid_auth" error | The bot token is invalid or revoked. Reinstall the app to get a new token. |
| Socket Mode connection drops | Check the App-Level Token. Regenerate it if expired. The adapter reconnects automatically. |
| Bot responds to every message | Set `respond_to_mentions: true` to limit responses to @mentions only |
| Messages not arriving | Verify bot event subscriptions include `message.channels` and `message.im` |
| Bot cannot send messages | Ensure the `chat:write` scope is granted and the app is installed to the workspace |
| Thread replies not working | Ensure the bot has access to the channel. Thread replies require the same permissions as channel messages. |

## Next Steps

- [Channels Overview](/channels/) -- architecture and shared concepts
- [Configuration](/guide/configuration) -- full config.yaml reference
- [Security](/guide/security) -- how Shield protects across channels
