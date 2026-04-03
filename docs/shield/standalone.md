# Standalone Binary

Shield runs as a standalone binary -- no OpenParallax installation required. It exposes a gRPC and REST API for action evaluation and can serve as an MCP security proxy.

## Installation

### Linux / macOS (curl)

```bash
curl -sSL https://get.openparallax.dev/shield | sh
```

This installs the `openparallax-shield` binary to `/usr/local/bin/`.

### macOS (Homebrew)

```bash
brew install openparallax/tap/shield
```

### Windows (Scoop)

```powershell
scoop bucket add openparallax https://github.com/openparallax/scoop-bucket
scoop install openparallax-shield
```

### From Source

```bash
git clone https://github.com/openparallax/openparallax.git
cd openparallax
make build-shield
# Binary at: dist/openparallax-shield
```

## Configuration

Create a `shield.yaml` file:

```yaml
# Shield standalone configuration

# Server settings
listen: localhost:9090
grpc_listen: localhost:9091

# Policy
policy:
  file: policies/default.yaml

# Classifier
classifier:
  model_dir: ~/.openparallax/models/prompt-injection/
  threshold: 0.85

# Heuristic engine
heuristic:
  enabled: true

# Tier 2 LLM evaluator
evaluator:
  provider: anthropic
  model: claude-sonnet-4-20250514
  api_key_env: ANTHROPIC_API_KEY

# Security
canary_token: SHIELD-CANARY-a8f3e9b2c4d5e6f7
fail_closed: true

# Rate limiting
rate_limit: 60        # evaluations per minute
daily_budget: 100     # Tier 2 evaluations per day
verdict_ttl: 300      # verdict cache TTL in seconds

# Audit logging
audit:
  enabled: true
  file: shield-audit.jsonl

# Logging
log_level: info
log_file: shield.log
```

### Configuration Reference

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `listen` | string | `localhost:9090` | REST API listen address |
| `grpc_listen` | string | `localhost:9091` | gRPC API listen address |
| `policy.file` | string | required | Path to YAML policy file |
| `classifier.model_dir` | string | `~/.openparallax/models/prompt-injection/` | ONNX model directory |
| `classifier.threshold` | float | `0.85` | INJECTION confidence threshold |
| `heuristic.enabled` | bool | `true` | Enable heuristic pattern matching |
| `evaluator.provider` | string | -- | LLM provider for Tier 2 |
| `evaluator.model` | string | -- | LLM model for Tier 2 |
| `evaluator.api_key_env` | string | -- | Env var for the API key |
| `evaluator.base_url` | string | -- | Custom base URL |
| `canary_token` | string | auto-generated | Token for evaluator injection detection |
| `fail_closed` | bool | `true` | Block on errors |
| `rate_limit` | int | `60` | Evaluations per minute |
| `daily_budget` | int | `100` | Tier 2 evaluations per day |
| `verdict_ttl` | int | `300` | Verdict cache TTL (seconds) |
| `audit.enabled` | bool | `false` | Enable audit logging |
| `audit.file` | string | `shield-audit.jsonl` | Audit log file path |
| `log_level` | string | `info` | Log level: debug, info, warn, error |
| `log_file` | string | -- | Log file path (stdout if omitted) |

## Commands

### `serve`

Start the Shield server:

```bash
openparallax-shield serve
openparallax-shield serve --config shield.yaml
openparallax-shield serve --config shield.yaml --port 8080
```

### `get-classifier`

Download the ONNX classifier model:

```bash
openparallax-shield get-classifier
openparallax-shield get-classifier --variant small
```

### `evaluate`

Evaluate a single action from the command line:

```bash
openparallax-shield evaluate \
  --action-type read_file \
  --payload '{"path": "/home/user/.ssh/id_rsa"}' \
  --config shield.yaml
```

### `status`

Show Shield status:

```bash
openparallax-shield status --config shield.yaml
```

### `mcp-proxy`

Start Shield as an MCP security proxy:

```bash
openparallax-shield mcp-proxy --config shield.yaml
```

See [MCP Gateway](/shield/mcp-proxy) for full documentation.

## REST API

### `POST /evaluate`

Evaluate an action.

**Request:**

```json
{
  "action_type": "execute_command",
  "payload": {
    "command": "rm -rf /"
  },
  "min_tier": 0
}
```

**Response:**

```json
{
  "decision": "BLOCK",
  "tier": 1,
  "confidence": 0.95,
  "reasoning": "heuristic: rm -rf detected (critical)",
  "action_hash": "sha256:a1b2c3d4e5f6...",
  "evaluated_at": "2026-04-03T10:30:00Z",
  "expires_at": "2026-04-03T10:35:00Z"
}
```

### `GET /health`

Health check.

**Response:**

```json
{
  "status": "ok",
  "tier0": true,
  "tier1_onnx": true,
  "tier1_heuristic": true,
  "tier2": true
}
```

### `GET /status`

Shield status including budget usage.

**Response:**

```json
{
  "active": true,
  "tier2_enabled": true,
  "tier2_used": 42,
  "tier2_budget": 100
}
```

## Running as a Service

### systemd (Linux)

Create `/etc/systemd/system/openparallax-shield.service`:

```ini
[Unit]
Description=OpenParallax Shield - AI Security Pipeline
After=network.target

[Service]
Type=simple
User=shield
Group=shield
WorkingDirectory=/opt/shield
ExecStart=/usr/local/bin/openparallax-shield serve --config /opt/shield/shield.yaml
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal

# Security hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=read-only
ReadWritePaths=/opt/shield/logs /opt/shield/audit
PrivateTmp=true
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true

# Resource limits
LimitNOFILE=65536
MemoryMax=2G

# Environment
Environment=ANTHROPIC_API_KEY=
EnvironmentFile=-/opt/shield/.env

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable openparallax-shield
sudo systemctl start openparallax-shield
sudo systemctl status openparallax-shield
```

### launchd (macOS)

Create `~/Library/LaunchAgents/dev.openparallax.shield.plist`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>dev.openparallax.shield</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/openparallax-shield</string>
        <string>serve</string>
        <string>--config</string>
        <string>/opt/shield/shield.yaml</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/opt/shield/logs/shield.log</string>
    <key>StandardErrorPath</key>
    <string>/opt/shield/logs/shield-error.log</string>
    <key>EnvironmentVariables</key>
    <dict>
        <key>ANTHROPIC_API_KEY</key>
        <string></string>
    </dict>
</dict>
</plist>
```

Load and start:

```bash
launchctl load ~/Library/LaunchAgents/dev.openparallax.shield.plist
```

### Windows Service

Use `sc.exe` to register Shield as a Windows service, or use [NSSM (Non-Sucking Service Manager)](https://nssm.cc/) for more control over logging and restart behavior.

**Using sc.exe:**

```powershell
sc.exe create OpenParallaxShield `
  binPath= "C:\Program Files\OpenParallax\openparallax-shield.exe serve --config C:\shield\shield.yaml" `
  start= auto `
  DisplayName= "OpenParallax Shield"

sc.exe start OpenParallaxShield
```

**Using NSSM** (recommended for production):

```powershell
# Install NSSM (via Scoop or download from nssm.cc)
scoop install nssm

# Register the service
nssm install OpenParallaxShield "C:\Program Files\OpenParallax\openparallax-shield.exe"
nssm set OpenParallaxShield AppParameters "serve --config C:\shield\shield.yaml"
nssm set OpenParallaxShield AppDirectory "C:\shield"
nssm set OpenParallaxShield AppStdout "C:\shield\logs\shield.log"
nssm set OpenParallaxShield AppStderr "C:\shield\logs\shield-error.log"
nssm set OpenParallaxShield AppEnvironmentExtra "ANTHROPIC_API_KEY=sk-ant-..."

# Start the service
nssm start OpenParallaxShield
```

NSSM provides automatic restart on crash, log rotation, and a GUI for editing service parameters (`nssm edit OpenParallaxShield`).

## Docker Deployment

### Dockerfile

```dockerfile
FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 make build-shield

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /src/dist/openparallax-shield /usr/local/bin/
COPY policies/ /opt/shield/policies/
COPY prompts/ /opt/shield/prompts/

WORKDIR /opt/shield
EXPOSE 9090 9091

ENTRYPOINT ["openparallax-shield"]
CMD ["serve", "--config", "/opt/shield/shield.yaml"]
```

### Docker Compose

```yaml
version: '3.8'

services:
  shield:
    build: .
    ports:
      - "9090:9090"
      - "9091:9091"
    volumes:
      - ./shield.yaml:/opt/shield/shield.yaml:ro
      - ./policies:/opt/shield/policies:ro
      - ./prompts:/opt/shield/prompts:ro
      - shield-models:/root/.openparallax/models
      - shield-audit:/opt/shield/audit
    environment:
      - ANTHROPIC_API_KEY
    restart: unless-stopped
    deploy:
      resources:
        limits:
          memory: 2G
          cpus: '2'

volumes:
  shield-models:
  shield-audit:
```

### Downloading the Model in Docker

```bash
# Run the get-classifier command inside the container.
docker compose run --rm shield get-classifier

# Or mount a pre-downloaded model directory.
docker run -v ~/.openparallax/models:/root/.openparallax/models ...
```

## Monitoring

### Prometheus Metrics

Shield exposes metrics at `/metrics` when running as a standalone server:

```
# Evaluation counts by tier and decision
shield_evaluations_total{tier="0", decision="BLOCK"} 42
shield_evaluations_total{tier="0", decision="ALLOW"} 1337
shield_evaluations_total{tier="1", decision="BLOCK"} 5
shield_evaluations_total{tier="2", decision="ALLOW"} 28

# Evaluation latency
shield_evaluation_duration_seconds{tier="0"} 0.0001
shield_evaluation_duration_seconds{tier="1"} 0.052
shield_evaluation_duration_seconds{tier="2"} 1.2

# Budget usage
shield_tier2_budget_used 42
shield_tier2_budget_total 100

# Classifier status
shield_classifier_available{type="onnx"} 1
shield_classifier_available{type="heuristic"} 1
```

### Health Checks

The `/health` endpoint returns HTTP 200 when Shield is operational. Use it for load balancer health checks, Kubernetes liveness probes, and Docker health checks:

```yaml
# Docker Compose
healthcheck:
  test: ["CMD", "wget", "-q", "--spider", "http://localhost:9090/health"]
  interval: 10s
  timeout: 5s
  retries: 3
```

```yaml
# Kubernetes
livenessProbe:
  httpGet:
    path: /health
    port: 9090
  initialDelaySeconds: 10
  periodSeconds: 10
```

## Next Steps

- [MCP Gateway](/shield/mcp-proxy) -- use Shield as an MCP security proxy
- [Configuration](/shield/configuration) -- full configuration reference
- [Policy Syntax](/shield/policies) -- write custom policies
