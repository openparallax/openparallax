# Microsoft Teams

::: warning Planned — Not Yet Implemented
The Microsoft Teams adapter is on the roadmap but not yet implemented. The configuration schema (`channels.teams`) ships in the binary, but enabling it does nothing — the agent does not connect to Teams. This page documents the planned design.

If you want to help build it, see [Adding a Channel Adapter](/channels/extending) and [Contributing](https://github.com/openparallax/openparallax/blob/main/CONTRIBUTING.md).
:::

The Microsoft Teams adapter will connect your agent to Teams using the Microsoft Bot Framework for message exchange and Azure Active Directory for authentication. The bot is designed to operate in 1:1 chats, group chats, and team channels, with rich responses rendered using Adaptive Cards for structured action approvals, status reports, and formatted output.

## Prerequisites

1. **Azure Account** -- With access to Azure Active Directory (Entra ID)
2. **Microsoft 365 Organization** -- A tenant with Teams enabled
3. **Admin Permissions** -- Depending on your organization's policies, deploying a custom bot to Teams may require admin approval
4. **Public HTTPS Endpoint** -- Teams requires a publicly reachable HTTPS URL for the messaging endpoint

## Setup

### 1. Register an Application in Azure AD

1. Go to the [Azure Portal](https://portal.azure.com)
2. Navigate to "Azure Active Directory" > "App registrations" > "New registration"
3. Name the application (e.g., "OpenParallax Agent")
4. Under "Supported account types", select:
   - **Single tenant** if the bot will only be used in your organization
   - **Multi-tenant** if you want to deploy across organizations
5. Leave the Redirect URI empty (not needed for bots)
6. Click "Register"

After registration, note the following values from the overview page:
- **Application (client) ID** -- this is your App ID
- **Directory (tenant) ID** -- this is your Tenant ID

### 2. Create a Client Secret

1. In the app registration, go to "Certificates & secrets"
2. Under "Client secrets", click "New client secret"
3. Add a description (e.g., "OpenParallax bot secret") and select an expiration period
4. Click "Add"
5. Copy the **Value** immediately -- it is only shown once

The client secret is used to authenticate the bot when sending messages through the Bot Framework REST API. Store it securely and rotate it before it expires.

### 3. Create an Azure Bot Resource

1. In the Azure Portal, search for "Azure Bot" and click "Create"
2. Fill in the required fields:
   - **Bot handle** -- a unique identifier (e.g., `openparallax-agent`)
   - **Subscription** -- your Azure subscription
   - **Resource group** -- create a new one or use an existing one
   - **Pricing tier** -- F0 (free) is sufficient for personal use
3. Under "Microsoft App ID", select "Use existing app registration" and enter your App ID from step 1
4. Click "Create"

### 4. Configure the Messaging Endpoint

1. In the Azure Bot resource, go to "Configuration"
2. Set the **Messaging endpoint** to: `https://your-domain.com/api/teams/messages`
3. Click "Apply"

This URL is where Teams sends incoming messages (activities) to your bot. It must be:
- **HTTPS** -- Teams refuses to send messages to HTTP endpoints
- **Publicly reachable** -- Teams servers must be able to POST to this URL
- **Responsive** -- Must respond within 15 seconds or Teams considers the request failed

For development, use ngrok or Cloudflare Tunnel to expose a local port:

```bash
ngrok http 3100
```

Then set the messaging endpoint to `https://xxxx.ngrok-free.app/api/teams/messages`.

### 5. Add the Teams Channel

1. In the Azure Bot resource, go to "Channels"
2. Click "Microsoft Teams" under "Available channels"
3. Accept the Terms of Service
4. Click "Apply"

This connects the Azure Bot to Teams so it can send and receive messages in the Teams client.

### 6. Install the Bot in Teams

There are three ways to make the bot available in Teams:

**Option A: Open in Teams (development)**

1. In the Azure Bot resource, go to "Channels" > "Microsoft Teams"
2. Click "Open in Teams"
3. Teams opens with the bot ready for 1:1 chat

This is the quickest way to test but only adds the bot for your account.

**Option B: Create a Teams App Package (recommended for teams)**

Create a `manifest.json` file for a Teams app:

```json
{
  "$schema": "https://developer.microsoft.com/en-us/json-schemas/teams/v1.17/MicrosoftTeams.schema.json",
  "manifestVersion": "1.17",
  "version": "1.0.0",
  "id": "YOUR-APP-ID",
  "developer": {
    "name": "Your Name",
    "websiteUrl": "https://your-domain.com",
    "privacyUrl": "https://your-domain.com/privacy",
    "termsOfUseUrl": "https://your-domain.com/terms"
  },
  "name": {
    "short": "Atlas Agent",
    "full": "OpenParallax Atlas Agent"
  },
  "description": {
    "short": "AI assistant powered by OpenParallax",
    "full": "A personal AI agent with a 4-tier security pipeline."
  },
  "icons": {
    "outline": "outline.png",
    "color": "color.png"
  },
  "accentColor": "#00DCFF",
  "bots": [
    {
      "botId": "YOUR-APP-ID",
      "scopes": ["personal", "team", "groupChat"],
      "commandLists": [
        {
          "scopes": ["personal"],
          "commands": [
            {"title": "new", "description": "Start a new session"},
            {"title": "otr", "description": "Toggle Off-The-Record mode"}
          ]
        }
      ]
    }
  ]
}
```

Package the manifest with two icon images (32x32 outline, 192x192 color) into a ZIP file and upload it through Teams Admin Center or sideload it in Teams (Settings > Manage Apps > Upload a custom app).

**Option C: Organization-wide deployment (admin)**

A Teams admin can deploy the app to all users through the Teams Admin Center > Manage Apps > Upload new app.

### 7. Set Environment Variables

```bash
export TEAMS_APP_ID="12345678-1234-1234-1234-123456789012"
export TEAMS_APP_PASSWORD="your-client-secret-value"
```

Add these to your shell profile so they persist across sessions.

### 8. Configure OpenParallax

```yaml
channels:
  teams:
    enabled: true
    app_id_env: TEAMS_APP_ID
    password_env: TEAMS_APP_PASSWORD
    tenant_id: "12345678-1234-1234-1234-123456789012"
```

| Field | Required | Description |
|---|---|---|
| `enabled` | Yes | Enable the Teams adapter |
| `app_id_env` | Yes | Environment variable containing the Azure Application (client) ID |
| `password_env` | Yes | Environment variable containing the Azure Client Secret |
| `tenant_id` | No | Azure AD tenant ID. Required for single-tenant apps. Multi-tenant apps can omit this. |

## How It Works

### Bot Framework Protocol

The Teams adapter uses the Microsoft Bot Framework protocol (Bot Framework v4). Teams communicates with the bot by sending HTTP POST requests containing "activities" to the messaging endpoint. Each activity represents a message, conversation update, or other event.

The bot responds by calling the Bot Framework REST API to send reply activities back to Teams. The adapter handles the activity lifecycle: receiving, parsing, routing to the engine pipeline, and sending the response.

### Authentication Flow

Teams authentication is more involved than other channel adapters because it uses OAuth 2.0 with JWT validation.

**Incoming request validation:**

1. Teams sends an HTTP POST with an `Authorization: Bearer <token>` header
2. The adapter fetches Microsoft's OpenID Connect metadata from `https://login.botframework.com/v1/.well-known/openidconfiguration`
3. The JWT token is validated:
   - Signature verification against Microsoft's public keys (JWKS)
   - Issuer must match `https://api.botframework.com`
   - Audience must match the configured App ID
   - Token must not be expired
4. If validation fails, the request is rejected with HTTP 401

**Outgoing request authentication:**

1. The adapter requests an OAuth 2.0 access token from `https://login.microsoftonline.com/{tenantId}/oauth2/v2.0/token`
2. The token request uses client credentials flow (App ID + Client Secret)
3. The access token is cached and refreshed before expiration
4. Outgoing API calls include the token in the `Authorization` header

Token caching means the adapter makes an OAuth call only when the cached token is about to expire (typically every 60 minutes). This keeps outgoing message latency low.

### Message Flow

```
Teams user sends message (1:1, group chat, or channel @mention)
       |
       v
  Teams servers POST activity to messaging endpoint
       |
       v
  Validate JWT token (signature, issuer, audience, expiry)
       |                              |
       | (valid)                      | (invalid)
       v                              v
  Parse activity                   HTTP 401 Unauthorized
       |
       v
  Extract: message text, user ID, conversation ID, tenant ID
       |
       v
  Check for slash command prefix
       |                     |
       | (not a command)     | (/new, /otr)
       v                     v
  engine.ProcessMessage   Handle command locally
  ForWeb()
       |
       v
  Collect streaming events
       |
       v
  Format response (text or Adaptive Card)
       |
       v
  POST replyToActivity via Bot Framework REST API
```

### Session Mapping

Sessions are identified by `teams:<conversation_id>`. In 1:1 chats, the conversation ID is unique per user-bot pair. In group chats, all participants share one conversation ID (and therefore one session). In team channels, the conversation ID maps to the channel.

### Proactive Messaging

The Bot Framework allows proactive messaging -- sending messages to users without them messaging first. The adapter stores conversation references (including the service URL, conversation ID, and tenant ID) when it first receives a message from a user. These references can be used to send notifications or scheduled responses later.

Proactive messaging is used by the heartbeat system to deliver scheduled task results through Teams.

## Adaptive Cards

For structured responses, the adapter renders output using [Adaptive Cards](https://adaptivecards.io/). Adaptive Cards are a JSON-based UI framework supported by Teams that render as rich, interactive cards within the chat.

### When Adaptive Cards Are Used

| Content | Format |
|---|---|
| Regular text responses | Plain text with Markdown |
| Shield verdicts | Adaptive Card with colored banner |
| Action approvals | Adaptive Card with Approve/Deny buttons |
| Status reports | Adaptive Card with structured fields |
| Error messages | Adaptive Card with error styling |
| File outputs | Adaptive Card with download link |

### Shield Verdict Card

When Shield evaluates a tool call, the result is displayed as an Adaptive Card:

```json
{
  "type": "AdaptiveCard",
  "$schema": "http://adaptivecards.io/schemas/adaptive-card.json",
  "version": "1.5",
  "body": [
    {
      "type": "Container",
      "style": "attention",
      "items": [
        {
          "type": "TextBlock",
          "text": "Shield Evaluation",
          "weight": "Bolder",
          "size": "Medium"
        }
      ]
    },
    {
      "type": "FactSet",
      "facts": [
        {"title": "Action", "value": "write_file"},
        {"title": "Path", "value": "/home/user/project/config.json"},
        {"title": "Tier", "value": "0 (Policy)"},
        {"title": "Decision", "value": "ALLOW"},
        {"title": "Confidence", "value": "1.00"}
      ]
    }
  ]
}
```

The card uses color-coded container styles:
- `good` (green) for ALLOW verdicts
- `attention` (amber) for CONFIRM verdicts requiring user approval
- `warning` (red) for BLOCK verdicts

### Action Approval Card

When a tool call requires user confirmation, the Adaptive Card includes action buttons:

```json
{
  "type": "AdaptiveCard",
  "version": "1.5",
  "body": [
    {
      "type": "TextBlock",
      "text": "Action requires approval",
      "weight": "Bolder"
    },
    {
      "type": "FactSet",
      "facts": [
        {"title": "Action", "value": "shell_exec"},
        {"title": "Command", "value": "rm -rf /tmp/build/*"}
      ]
    }
  ],
  "actions": [
    {
      "type": "Action.Submit",
      "title": "Approve",
      "data": {"action": "approve", "action_id": "act_abc123"}
    },
    {
      "type": "Action.Submit",
      "title": "Deny",
      "data": {"action": "deny", "action_id": "act_abc123"}
    }
  ]
}
```

When the user clicks Approve or Deny, Teams sends a submit activity to the messaging endpoint. The adapter extracts the action data and routes the approval decision to the engine. The card is updated to show the outcome (approved or denied) and the buttons are removed.

Action approvals expire after 120 seconds. If neither button is pressed, the action is denied by default (fail-closed).

## Supported Message Types

### Incoming

| Type | Supported | Notes |
|---|---|---|
| Text | Yes | Plain text messages and @mentions |
| Markdown | Yes | Teams Markdown is passed through |
| File attachments | No | Attachments are not downloaded or processed |
| Adaptive Cards (input) | Yes | Submit actions from cards are processed |
| Reactions | No | — |
| Mentions of other users | No | — |

### Outgoing

| Type | Supported | Notes |
|---|---|---|
| Text (Markdown) | Yes | Used for regular conversational responses |
| Adaptive Cards | Yes | Used for structured content (verdicts, approvals, status) |
| File attachments | No | Files cannot be sent directly; use OneDrive links instead |
| Images | No | — |

### Message Length

Teams supports messages up to 28 KB (approximately 28,000 characters). The adapter splits messages at paragraph boundaries when they exceed this limit. In practice, most agent responses are well within the limit.

## 1:1 Chat vs. Group Chat vs. Channel

### 1:1 Chat (Personal)

In personal chat, every message is processed. No mention or prefix is needed. The session is unique to the user-bot conversation. This is the most common mode for personal agents.

### Group Chat

When the bot is added to a group chat, it receives all messages by default. All participants share one session. The bot responds in the group, visible to all members.

To add the bot to a group chat: open the chat, click the participants icon, and search for the bot by name.

### Team Channel

In team channels, the bot only responds when @mentioned:

```
@Atlas Agent what is the deployment status?
```

The @mention tag is automatically stripped from the message text before processing. Channel messages create per-channel sessions, shared among all users who interact with the bot in that channel.

## Slash Commands

| Command | Action |
|---|---|
| `/new` | Reset the session and start fresh |
| `/otr` | Switch to Off-The-Record mode |

Commands are typed as regular messages in the chat. They are intercepted by the adapter before reaching the engine pipeline. The commands also appear in Teams' bot command menu if you defined them in the manifest (see setup step 6, Option B).

## Tenant Isolation

For single-tenant deployments, the `tenant_id` configuration ensures the bot only accepts requests from your Azure AD tenant. Activities from other tenants are rejected.

For multi-tenant deployments (where the bot serves multiple organizations), omit `tenant_id`. The adapter validates the JWT token against any Microsoft tenant but still verifies the App ID claim.

## Development Setup

### Local Development with ngrok

For local development, expose your OpenParallax instance using ngrok:

```bash
# Start ngrok
ngrok http 3100

# Note the HTTPS URL (e.g., https://abc123.ngrok-free.app)
```

Update the messaging endpoint in the Azure Bot configuration:

```
https://abc123.ngrok-free.app/api/teams/messages
```

Remember to update this URL each time ngrok generates a new address (unless using a paid plan with a stable subdomain).

### Testing the Bot

After setup, test the bot in Teams:

1. Open Teams and find the bot in your chat list (or search for it)
2. Send a simple message like "hello"
3. The bot should respond within a few seconds
4. If no response, check the engine logs: `openparallax logs --level error`

### Bot Framework Emulator

For testing without Teams, use the [Bot Framework Emulator](https://github.com/microsoft/BotFramework-Emulator). Point it at `http://localhost:3100/api/teams/messages` with your App ID and password. The emulator simulates Teams activities locally.

## Troubleshooting

| Problem | Solution |
|---|---|
| "Unauthorized" error (401) | The App ID or Client Secret is wrong. Verify in Azure Portal and update environment variables. |
| Bot does not respond in Teams | Check the messaging endpoint URL in Azure Bot Configuration. Ensure it is HTTPS and publicly reachable. |
| "Bot is not found" in Teams | The Teams channel is not added to the Azure Bot. Go to Channels and add Microsoft Teams. |
| Client secret expired | Create a new secret in Azure AD > App registrations > Certificates & secrets. Update the environment variable. |
| Activities arrive but responses fail | Check that the bot has the correct App ID and password. The adapter needs valid credentials to call the Bot Framework REST API. |
| Admin won't approve the app | For testing, sideload the app in Teams (Settings > Manage Apps > Upload a custom app). This does not require admin approval in most configurations. |
| Adaptive Card not rendering | Ensure the card schema version is `1.5` or lower. Teams does not support all Adaptive Card features. |
| Proactive messages not working | The adapter needs a stored conversation reference. Send at least one message to the bot first. |

## Required Azure Permissions

The Azure AD app registration needs minimal permissions:

| Permission | Type | Purpose |
|---|---|---|
| None (Bot Framework) | — | Bot Framework uses its own authentication; no Graph API permissions are needed for basic chat |

The bot communicates through the Bot Framework REST API, not the Microsoft Graph API. No Graph API permissions are required for message exchange. If you add features that access Graph resources (e.g., reading calendar, files), additional permissions would be needed, but the base Teams adapter does not require them.

## Next Steps

- [Channels Overview](/channels/) -- architecture and shared concepts
- [Configuration](/guide/configuration) -- full config.yaml reference
- [Security](/guide/security) -- how Shield protects across channels
