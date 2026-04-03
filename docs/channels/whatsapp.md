# WhatsApp

The WhatsApp adapter connects your agent to WhatsApp using the WhatsApp Business Cloud API (Meta). Messages are received via webhooks and sent through the Graph API.

## Prerequisites

1. **Meta Business Account** -- Sign up at [business.facebook.com](https://business.facebook.com)
2. **WhatsApp Business App** -- Create an app in the [Meta Developer Portal](https://developers.facebook.com)
3. **Phone Number** -- Register a phone number in the WhatsApp Business settings
4. **Public Webhook URL** -- The webhook must be reachable from Meta's servers (use ngrok for development)

## Setup

### 1. Create a Meta App

1. Go to [developers.facebook.com](https://developers.facebook.com)
2. Create a new app, select "Business" type
3. Add the "WhatsApp" product to your app
4. In WhatsApp > API Setup, note your **Phone Number ID** and generate a **Permanent Access Token**

### 2. Configure the Webhook

1. In your Meta App dashboard, go to WhatsApp > Configuration
2. Set the Webhook URL to `https://your-domain.com:9443/webhook`
3. Set a Verify Token (any string you choose)
4. Subscribe to the `messages` webhook field

### 3. Set Environment Variable

```bash
export WHATSAPP_ACCESS_TOKEN="EAAxxxxxxxxxxxxxxxxxxxxxxxxx"
```

### 4. Configure OpenParallax

```yaml
channels:
  whatsapp:
    enabled: true
    phone_number_id: "1234567890123456"
    access_token_env: WHATSAPP_ACCESS_TOKEN
    verify_token: "my-secret-verify-token"
    webhook_port: 9443
    allowed_numbers:
      - "+1234567890"
      - "+0987654321"
```

| Field | Required | Description |
|---|---|---|
| `enabled` | Yes | Enable the WhatsApp adapter |
| `phone_number_id` | Yes | Your WhatsApp Business phone number ID (from Meta Dashboard) |
| `access_token_env` | Yes | Environment variable containing the permanent access token |
| `verify_token` | Yes | Token used to verify webhook registration (you choose this) |
| `webhook_port` | No | Port for the webhook HTTP server (default: 9443) |
| `allowed_numbers` | No | Phone numbers allowed to interact. Empty means all numbers. |

## How It Works

### Incoming Messages

1. A user sends a WhatsApp message to your business number
2. Meta sends a webhook POST to `https://your-domain.com:9443/webhook`
3. The adapter parses the webhook payload and extracts the message text and sender number
4. If `allowed_numbers` is set, unauthorized senders are silently dropped
5. The message is routed through the engine pipeline
6. The response is sent back via the Graph API

### Outgoing Messages

Messages are sent via the WhatsApp Cloud API:

```
POST https://graph.facebook.com/v21.0/{phone_number_id}/messages
Authorization: Bearer {access_token}
Content-Type: application/json

{
  "messaging_product": "whatsapp",
  "to": "+1234567890",
  "type": "text",
  "text": {"body": "Response from the agent"}
}
```

Long messages (over 4,096 characters) are automatically split at paragraph boundaries.

### Webhook Verification

Meta verifies your webhook by sending a GET request with a challenge. The adapter handles this automatically:

```
GET /webhook?hub.mode=subscribe&hub.verify_token=my-secret-verify-token&hub.challenge=XXXX
```

If the `hub.verify_token` matches your configured token, the adapter responds with the challenge string.

## Supported Message Types

| Type | Incoming | Outgoing |
|---|---|---|
| Text | Yes | Yes |
| Images | No | No |
| Documents | No | No |
| Audio | No | No |
| Location | No | No |

The current implementation handles text messages only.

## Slash Commands

| Command | Action |
|---|---|
| `/new` | Reset the session and start fresh |
| `/otr` | Switch to Off-The-Record mode |

## Access Control

When `allowed_numbers` is configured, only listed phone numbers can interact with the agent. Messages from unauthorized numbers are logged and silently dropped. When the list is empty, all numbers are accepted.

## Development with ngrok

For local development, use ngrok to expose your webhook port:

```bash
ngrok http 9443
```

Use the ngrok URL as your webhook URL in the Meta App dashboard:

```
https://xxxx-xxxx-xxxx.ngrok-free.app/webhook
```
