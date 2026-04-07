# Tier 1 -- Classifier

Tier 1 is the machine learning and pattern matching tier. It runs when Tier 0 either escalates an action or produces no match. Two classifiers operate in parallel -- the ONNX DeBERTa model for prompt injection detection and a heuristic engine for known attack signatures -- and the most severe result wins.

## DualClassifier

The `DualClassifier` is the orchestrator for Tier 1. It launches both classifiers concurrently using goroutines and combines their results using severity ranking.

```
         Action arrives at Tier 1
                    │
         ┌──────────┴──────────┐
         │                     │
         ▼                     ▼
  ┌──────────────┐    ┌──────────────────┐
  │ ONNX DeBERTa │    │ Heuristic Engine │
  │   Classifier │    │  (regex rules)   │
  └──────┬───────┘    └────────┬─────────┘
         │                     │
         └──────────┬──────────┘
                    │
                    ▼
           Severity Ranking
         BLOCK > ESCALATE > ALLOW
                    │
                    ▼
              Final Result
```

### Combining Results

The severity ranking is strict: `BLOCK > ESCALATE > ALLOW`. The classifier with the more severe decision wins. If both produce the same severity, the one with higher confidence wins.

```go
// Severity ranking used by the DualClassifier.
func decisionSeverity(d VerdictDecision) int {
    switch d {
    case VerdictBlock:    return 2
    case VerdictEscalate: return 1
    default:              return 0  // VerdictAllow
    }
}
```

This means:
- If ONNX says BLOCK and heuristic says ALLOW, the result is BLOCK.
- If ONNX says ALLOW and heuristic says BLOCK, the result is BLOCK.
- If both say ALLOW, the result is ALLOW (with the higher confidence value).
- If ONNX says ESCALATE and heuristic says ALLOW, the result is ESCALATE.

### Graceful Degradation

Either classifier can be absent:

- If the ONNX model is not installed, heuristic-only mode is used.
- If heuristic is disabled (`heuristic_enabled: false`), only ONNX runs.
- If both are unavailable, Tier 1 returns ALLOW with confidence 0.5 and source `"none"`.

## ONNX DeBERTa Classifier

The ONNX classifier uses a fine-tuned DeBERTa v3 model trained specifically for prompt injection detection. It runs entirely in-process -- no HTTP calls, no sidecar processes.

### How It Works

1. The action is formatted as a text string: `"{action_type}: {payload}"`
2. The text is tokenized using the model's HuggingFace tokenizer
3. Tokens are padded or truncated to 512 tokens (the model's maximum sequence length)
4. Three input tensors are created: `input_ids`, `attention_mask`, `token_type_ids`
5. The ONNX session runs inference and produces logits
6. Softmax converts logits to probabilities
7. The label with the highest probability is selected: `SAFE` or `INJECTION`

### Threshold Logic

The classifier produces a confidence score (0.0 - 1.0) for the predicted label. The threshold determines what action to take:

| Label | Confidence | Decision | Rationale |
|-------|-----------|----------|-----------|
| `INJECTION` | >= 0.85 | **BLOCK** | High-confidence injection detection. Block immediately. |
| `INJECTION` | < 0.85 | **ESCALATE** | Model suspects injection but is not confident enough. Send to Tier 2 for LLM evaluation. |
| `SAFE` | any | **ALLOW** | Model believes the action is safe. |

The 0.85 threshold is the default. It can be configured:

```go
shield.NewPipeline(shield.Config{
    OnnxThreshold: 0.90, // Stricter: fewer blocks, more escalations
})
```

```yaml
# In shield.yaml (standalone)
classifier:
  threshold: 0.90
```

::: tip
Lower thresholds catch more attacks but produce more false positives (and more Tier 2 evaluations, which cost money). Higher thresholds are more precise but may miss borderline attacks. The default of 0.85 balances precision and recall.
:::

### Model Variants

| Variant | Size | Accuracy | Speed | Use Case |
|---------|------|----------|-------|----------|
| DeBERTa v3 Base | ~700MB | 98.8% | ~50ms/eval | Production systems with strong security requirements |
| DeBERTa v3 Small | ~250MB | 94.3% | ~15ms/eval | Resource-constrained environments, edge deployment |

Download either variant:

```bash
# Base model (default, recommended)
openparallax get-classifier

# Small model (faster, smaller)
openparallax get-classifier --variant small
```

See [ONNX Classifier](/shield/classifier) for the full deep dive on model internals, the download process, and in-process inference.

## Heuristic Engine

The heuristic engine evaluates actions against a library of compiled regex patterns. Each pattern detects a specific attack category. The engine scans security-relevant payload fields -- `command`, `path`, `source`, `destination`, `url`, and `pattern` -- to avoid false positives on file content being written.

### Rule Categories

The heuristic engine ships with rules across these categories:

#### Prompt Injection (5 rules)

Detects attempts to override the LLM's instructions:

| Rule ID | Name | Severity | Pattern |
|---------|------|----------|---------|
| PI-001 | `ignore_instructions` | critical | `ignore (all )?(previous\|prior\|above) instructions` |
| PI-002 | `system_message_spoof` | critical | `(system\|admin\|root): (you are\|your new\|override)` |
| PI-003 | `role_switch` | critical | `(you are now\|act as\|pretend to be) (hacker\|admin\|root\|unrestricted)` |
| PI-004 | `jailbreak_markers` | high | `(DAN\|do anything now\|developer mode\|god mode\|jailbreak)` |
| PI-005 | `instruction_override` | critical | `(forget\|disregard\|override\|bypass) (your\|all) (rules\|instructions)` |

#### Path Traversal (3 rules)

Detects directory traversal and null byte injection:

| Rule ID | Name | Severity | Pattern |
|---------|------|----------|---------|
| PT-001 | `dot_dot_traversal` | high | `../../` |
| PT-002 | `null_byte` | critical | `%00`, `\x00`, `\0` |
| PT-003 | `url_encoded_traversal` | high | `%2e%2e/%2e%2e/` |

#### Data Exfiltration (3 rules)

Detects attempts to send data to external services:

| Rule ID | Name | Severity | Pattern |
|---------|------|----------|---------|
| DE-001 | `base64_in_url` | high | Base64 payload (40+ chars) in URL parameter |
| DE-002 | `dns_exfil` | medium | Long subdomain (30+ chars) suggesting DNS exfiltration |
| DE-003 | `webhook_exfil` | high | Known webhook endpoints (Slack, Discord) |

#### Sensitive Data (3 rules)

Detects credentials and secrets in action payloads:

| Rule ID | Name | Severity | Pattern |
|---------|------|----------|---------|
| SD-001 | `private_key_content` | critical | `-----BEGIN (RSA\|EC\|OPENSSH) PRIVATE KEY-----` |
| SD-002 | `aws_key` | critical | `AKIA[0-9A-Z]{16}` |
| SD-003 | `jwt_token` | high | JWT format (`eyJ...eyJ...signature`) |

#### Encoding Evasion (1 rule)

Detects zero-width characters used to hide payloads:

| Rule ID | Name | Severity | Pattern |
|---------|------|----------|---------|
| EE-001 | `zero_width_chars` | high | Zero-width spaces, joiners, non-breaking hyphens |

#### Self-Protection (1 rule)

Detects shell commands that attempt to modify protected identity files:

| Rule ID | Name | Severity | Pattern |
|---------|------|----------|---------|
| SP-001 | `shell_writes_protected_file` | critical | Shell redirect/copy/move/delete targeting SOUL.md, IDENTITY.md |

#### Generation Safety (3 rules)

Detects unsafe content generation requests:

| Rule ID | Name | Severity | Pattern |
|---------|------|----------|---------|
| GEN-001 | `gen_real_person_explicit` | critical | Explicit content of real persons |
| GEN-002 | `gen_csam_adjacent` | critical | CSAM-adjacent content |
| GEN-003 | `gen_weapons_visual` | critical | Weapons manufacturing instructions |

#### Email Safety (2 rules)

Detects destructive email operations:

| Rule ID | Name | Severity | Pattern |
|---------|------|----------|---------|
| EM-001 | `email_move_to_trash` | high | Moving emails to trash |
| EM-002 | `email_bulk_mark` | medium | Bulk email flag modification |

#### Platform-Specific Shell Injection

Additional rules are loaded based on the host operating system. These detect platform-specific shell injection patterns (e.g., PowerShell-specific attacks on Windows, bash-specific attacks on Linux/macOS).

### Severity to Confidence Mapping

Heuristic results map severity levels to confidence scores:

| Severity | Confidence | Decision |
|----------|-----------|----------|
| critical | 0.95 | BLOCK |
| high | 0.85 | BLOCK |
| medium | 0.70 | BLOCK |
| low | 0.50 | BLOCK |
| no match | 0.70 | ALLOW |

All matched heuristic rules produce BLOCK decisions. The severity determines the confidence level, which affects how the result is combined with the ONNX result and how it appears in audit logs.

### Scanning Strategy

The heuristic engine only scans security-relevant fields to minimize false positives:

```go
securityFields := []string{"command", "path", "source", "destination", "url", "pattern"}
```

This means a `write_file` action with injection text in the `content` field will **not** trigger a heuristic match. The content is what the user asked the LLM to write -- scanning it would produce constant false positives on legitimate operations like writing security documentation. Only the operational fields (the command to run, the path to write to, the URL to call) are scanned.

## Tier 1 Outcomes

After the DualClassifier produces a result, the Gateway handles it:

| Tier 1 Decision | Gateway Action |
|-----------------|---------------|
| BLOCK | Return BLOCK verdict immediately |
| ALLOW (and `minTier < 2`) | Return ALLOW verdict |
| ESCALATE | Continue to Tier 2 |
| Error (fail-closed mode) | Return BLOCK verdict |

## Next Steps

- [ONNX Classifier](/shield/classifier) -- deep dive on model loading, inference, and the `get-classifier` command
- [Tier 2 -- LLM Evaluator](/shield/tier2) -- what happens when Tier 1 escalates
- [Tier 0 -- Policy](/shield/tier0) -- the tier that runs before the classifier
