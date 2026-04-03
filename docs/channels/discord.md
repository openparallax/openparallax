# Discord

The Discord adapter connects your agent to Discord using the Gateway WebSocket API for real-time message delivery and the REST API for sending responses. The bot can operate in guild channels (when @mentioned or in designated channels), in threads, and in direct messages. Responses use Discord's native Markdown, and file artifacts are delivered as attachments.

## Prerequisites

1. **Discord Account** -- Any Discord account
2. **Discord Server** -- A server where you have admin permissions (or permission to invite bots)
3. **Bot Application** -- Created in the Discord Developer Portal

## Setup

### 1. Create a Bot Application

1. Go to the [Discord Developer Portal](https://discord.com/developers/applications)
2. Click "New Application" and give it a name (e.g., "Atlas Agent")
3. Go to the "Bot" section in the left sidebar
4. Click "Reset Token" to generate a bot token. Copy it immediately -- Discord only shows it once.
5. Under "Privileged Gateway Intents", enable:
   - **Message Content Intent** -- required to read the text of messages. Without this, the bot receives message events but the `content` field is empty.
   - **Server Members Intent** -- optional, enables user filtering by guild membership.

The Message Content Intent is classified as privileged. For bots in fewer than 100 servers, you can enable it in the Developer Portal. For bots in 100+ servers, you must apply for verification from Discord. For personal use, this is not a concern.

### 2. Invite the Bot to Your Server

1. Go to the "OAuth2" section in the left sidebar
2. Under "URL Generator", select the `bot` scope
3. Under "Bot Permissions", select:
   - `Send Messages` -- send responses
   - `Read Message History` -- read conversation context
   - `Attach Files` -- send file artifacts
   - `Use Slash Commands` -- register and respond to slash commands
   - `Create Public Threads` -- create threads for long conversations (optional)
   - `Send Messages in Threads` -- respond in threads (optional)
4. Copy the generated URL and open it in your browser
5. Select the server you want to add the bot to and click "Authorize"

The bot appears in the server's member list immediately after authorization.

### 3. Get Channel and Guild IDs

Discord uses numeric IDs for servers (guilds), channels, and users. To copy them:

1. In Discord, go to Settings > Advanced > enable "Developer Mode"
2. Right-click a server name and select "Copy Server ID" -- this is the guild ID
3. Right-click a channel and select "Copy Channel ID"
4. Right-click a user and select "Copy User ID"

### 4. Set Environment Variable

```bash
export DISCORD_BOT_TOKEN="ODk..."
```

Add this to your shell profile so it persists across sessions.

### 5. Configure OpenParallax

```yaml
channels:
  discord:
    enabled: true
    token_env: DISCORD_BOT_TOKEN
    allowed_channels:
      - "1234567890123456789"
    allowed_users:
      - "9876543210987654321"
    respond_to_mentions: true
```

| Field | Required | Description |
|---|---|---|
| `enabled` | Yes | Enable the Discord adapter |
| `token_env` | Yes | Environment variable containing the bot token |
| `allowed_channels` | No | Channel IDs where the bot responds. Empty list means all channels in all guilds. |
| `allowed_users` | No | User IDs allowed to interact. Empty list means all users. |
| `respond_to_mentions` | No | In guild channels, only respond when @mentioned (default: false) |

## How It Works

### WebSocket Gateway

The adapter connects to Discord's WebSocket Gateway and maintains a persistent connection. The Gateway delivers real-time events including message creation, message updates, and interaction callbacks. Unlike polling-based transports, the Gateway provides sub-second message delivery latency.

The adapter handles Gateway lifecycle automatically: initial handshake (IDENTIFY), heartbeat keepalives, session resumption after disconnects, and reconnection with exponential backoff. If the Gateway connection drops, the adapter resumes the session to avoid missing events during the disconnect window.

### Message Flow

```
Discord user sends message or slash command
       |
       v
  Gateway delivers MESSAGE_CREATE or INTERACTION_CREATE
       |
       v
  Filter: ignore messages from the bot itself
       |
       v
  Check allowed_channels (if configured)
       |                              |
       | (allowed)                    | (not in list)
       v                              v
  Check allowed_users               Silently ignored
       |                    |
       | (allowed)          | (not in list)
       v                    v
  Check respond_to_mentions     Silently ignored
       |                    |
       | (mentioned or DM)  | (not mentioned, guild channel)
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
  Send response (text, embeds, or files)
```

### Session Mapping

Each Discord user gets their own session, identified by `discord:<user_id>`. In guild channels, even though multiple users see the same channel, each user's messages create separate sessions. This means two users talking in the same channel have independent conversation histories with the agent.

In DMs, the session maps directly to the DM channel, which is inherently per-user.

### Mention Mode

When `respond_to_mentions` is `true`, the bot only responds in guild channels when explicitly @mentioned. In DMs, it always responds regardless of this setting.

```
User: @Atlas what time zone are we deploying in?
Atlas: The deployment configuration specifies UTC...
```

The mention prefix (`<@botid>` or `<@!botid>`) is automatically stripped from the message text before processing, so the agent never sees the raw mention syntax.

When `respond_to_mentions` is `false`, the bot responds to every message in allowed channels. This is useful for dedicated bot channels where every message is intended for the agent.

## Thread Support

The adapter supports Discord threads as a natural extension of channel conversations:

- When a user creates a thread and mentions the bot, the bot responds within the thread.
- Thread messages use the same session as the parent channel for that user, maintaining conversation continuity.
- The bot can create threads for long responses by sending the initial reply as a thread starter. This keeps the main channel clean.

Threads inherit the `allowed_channels` filter from their parent channel. If a parent channel is in the allow list, threads within it are also allowed.

To enable thread creation, the bot needs the `Create Public Threads` and `Send Messages in Threads` permissions (configured during the invite step).

## Supported Message Types

### Incoming

| Type | Supported | Notes |
|---|---|---|
| Text | Yes | Plain text messages |
| Markdown | Yes | Discord Markdown is passed through as-is |
| Attachments | No | File attachments are not downloaded or processed |
| Embeds | No | Embed content is not extracted |
| Reactions | No | — |
| Voice | No | — |

### Outgoing

| Type | Supported | Notes |
|---|---|---|
| Text (Markdown) | Yes | Discord natively supports Markdown -- no escaping needed |
| File attachments | Yes | Sent via `ChannelFileSend` for artifacts |
| Embeds | Yes | Used for structured responses (see below) |
| Reactions | No | — |
| Voice | No | — |

### Embed Formatting

For structured responses -- such as Shield verdicts, status reports, or action results -- the adapter formats output as Discord embeds. Embeds provide a visually distinct card with a colored sidebar, title, description, and fields.

Shield verdicts use color coding:
- Green (`0x00FF00`) -- ALLOW
- Red (`0xFF0000`) -- BLOCK
- Amber (`0xFFAA00`) -- CONFIRM (waiting for user approval)

Regular agent responses are sent as plain text with Discord Markdown formatting. Embeds are reserved for structured, machine-generated content.

### Message Length

Discord limits messages to 2,000 characters. The adapter splits long responses at paragraph boundaries (double newlines, then single newlines, then spaces). Each chunk is sent as a separate message with a short delay between sends to respect Discord's rate limits.

## Slash Command Registration

The adapter registers slash commands with Discord on startup. These appear in Discord's command palette when a user types `/` in the chat input.

| Command | Description |
|---|---|
| `/new` | Reset the session and start fresh |
| `/otr` | Switch to Off-The-Record mode |

Slash commands are registered per-guild when `guild_ids` is configured in the channels overview config, or globally when no guild restriction is set. Guild-specific registration is faster (commands appear immediately) while global registration can take up to an hour to propagate.

When the bot shuts down cleanly, it deregisters its slash commands to avoid stale entries.

## Access Control

### Channel Filtering

When `allowed_channels` is configured, the bot only processes messages from listed channel IDs. Messages in other channels are silently ignored -- the bot does not send a rejection message, it simply does not respond. This is the recommended approach for guild deployments: create a dedicated channel for the bot and list only that channel.

### User Filtering

When `allowed_users` is configured, only listed Discord user IDs can interact with the bot. Messages from other users are silently ignored.

### Combined Filtering

Both filters apply simultaneously. A message must pass the channel filter AND the user filter to be processed. For example, if `allowed_channels` contains channel A and `allowed_users` contains user X, then only messages from user X in channel A are processed.

## Required Gateway Intents

The adapter requests these Gateway Intents when connecting:

| Intent | Purpose | Privileged |
|---|---|---|
| `GuildMessages` | Receive message events in guild channels | No |
| `DirectMessages` | Receive message events in DMs | No |
| `MessageContent` | Read the actual message content | Yes |

The `MessageContent` intent must be enabled in the Developer Portal under "Privileged Gateway Intents". Without it, the bot receives message events but the content field is empty, making the adapter non-functional.

## Troubleshooting

| Problem | Solution |
|---|---|
| Bot is online but does not respond | Check that Message Content Intent is enabled in the Developer Portal |
| "Invalid token" error | The bot token is invalid. Reset it in the Developer Portal and update the environment variable. |
| Slash commands not appearing | Guild-specific commands appear instantly. Global commands can take up to an hour. Try restarting the bot. |
| Bot responds to everyone | Set `respond_to_mentions: true` or configure `allowed_users` to restrict access |
| "Missing Permissions" error | The bot lacks required permissions. Re-invite with the correct permission set. |
| Messages in threads ignored | Ensure the bot has `Send Messages in Threads` permission |

## Next Steps

- [Channels Overview](/channels/) -- architecture and shared concepts
- [Configuration](/guide/configuration) -- full config.yaml reference
- [Security](/guide/security) -- how Shield protects across channels
