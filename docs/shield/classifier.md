# ONNX Classifier Deep Dive

The ONNX classifier runs a DeBERTa v3 model in-process for prompt injection detection. No HTTP calls, no sidecar processes, no CGo -- pure Go inference via `onnxruntime-purego`.

## The `get-classifier` Command

The classifier model is not bundled with the Shield binary (it is 250-700MB depending on variant). Download it with:

```bash
# Default: DeBERTa v3 Base (~700MB)
openparallax get-classifier

# Small variant (~250MB, faster, slightly less accurate)
openparallax get-classifier --variant small

# Standalone binary
openparallax-shield get-classifier
openparallax-shield get-classifier --variant small
```

### What Gets Downloaded

The command downloads three files to `~/.openparallax/models/prompt-injection/`:

| File | Size (Base) | Size (Small) | Purpose |
|------|-------------|-------------|---------|
| `model.onnx` | ~680MB | ~240MB | The ONNX-exported DeBERTa v3 model |
| `tokenizer.json` | ~2MB | ~2MB | HuggingFace tokenizer configuration |
| `libonnxruntime.so` | ~30MB | ~30MB | ONNX Runtime shared library |

The library filename varies by platform:

| Platform | Filename |
|----------|----------|
| Linux | `libonnxruntime.so` |
| macOS | `libonnxruntime.dylib` |
| Windows | `onnxruntime.dll` |

### Storage Location

```
~/.openparallax/
  models/
    prompt-injection/
      model.onnx            # DeBERTa ONNX model
      tokenizer.json         # HuggingFace tokenizer config
      libonnxruntime.so      # ONNX Runtime (platform-specific)
```

This path is hardcoded. Shield looks here automatically on startup -- no configuration needed.

## Model Variants

### DeBERTa v3 Base

- **Size**: ~700MB
- **Accuracy**: 98.8% on the prompt injection benchmark
- **Inference time**: ~50ms per evaluation (CPU, single thread)
- **Architecture**: 12 transformer layers, 768 hidden dimensions, 12 attention heads
- **Recommended for**: Production deployments where accuracy is critical

### DeBERTa v3 Small

- **Size**: ~250MB
- **Accuracy**: 97.2% on the prompt injection benchmark
- **Inference time**: ~15ms per evaluation (CPU, single thread)
- **Architecture**: 6 transformer layers, 768 hidden dimensions, 12 attention heads
- **Recommended for**: Resource-constrained environments, edge deployment, development/testing

Both variants are fine-tuned on the same prompt injection dataset and share the same tokenizer. The only difference is model depth.

## In-Process Inference

Shield runs inference entirely in-process using three libraries:

| Library | Role | CGo? |
|---------|------|------|
| `onnxruntime-purego` | Loads the ONNX Runtime shared library and calls it via `purego` (Go's FFI) | No |
| `sugarme/tokenizer` | Tokenizes input text using the HuggingFace tokenizer format | No |
| `onnxruntime` (C library) | The actual ONNX Runtime that executes the model | Loaded at runtime via `dlopen` |

The key insight: the ONNX Runtime is a pre-built shared library (`.so`/`.dylib`/`.dll`) that is loaded at runtime via `purego`, Go's pure-Go FFI mechanism. This means the Go binary itself is compiled with `CGO_ENABLED=0` -- no C compiler needed at build time. The shared library is only needed at runtime, and only on machines where the classifier is used.

## Inference Flow

Here is the complete inference pipeline for a single action:

### Step 1: Text Formatting

The action is converted to a single text string:

```go
text := fmt.Sprintf("%s: %v", action.Type, action.Payload)
// Example: "execute_command: map[command:rm -rf /]"
```

### Step 2: Tokenization

The text is tokenized using the model's HuggingFace tokenizer:

```go
encoding, err := tokenizer.EncodeSingle(text)
ids := encoding.GetIds()           // [101, 3452, 1035, ...]
attMask := encoding.GetAttentionMask() // [1, 1, 1, ...]
```

### Step 3: Pad/Truncate to 512

The model accepts exactly 512 tokens. Shorter inputs are padded with zeros, longer inputs are truncated:

```go
const maxSeqLength = 512

if len(ids) > maxSeqLength {
    ids = ids[:maxSeqLength]
    attMask = attMask[:maxSeqLength]
}
for len(ids) < maxSeqLength {
    ids = append(ids, 0)       // pad token
    attMask = append(attMask, 0)
}
```

### Step 4: Create Tensors

Three input tensors are created for the ONNX session:

```go
shape := []int64{1, 512}  // batch size 1, sequence length 512

inputIDsTensor, _ := ort.NewTensorValue(runtime, inputIDs, shape)
attMaskTensor, _ := ort.NewTensorValue(runtime, attention, shape)
ttidsTensor, _ := ort.NewTensorValue(runtime, tokenTypeIDs, shape) // all zeros for single-sentence
```

### Step 5: Run Session

The ONNX session runs inference:

```go
inputs := map[string]*ort.Value{
    "input_ids":      inputIDsTensor,
    "attention_mask": attMaskTensor,
    "token_type_ids": ttidsTensor,
}
outputs, _ := session.Run(ctx, inputs)
```

### Step 6: Softmax

The model outputs raw logits. Softmax converts them to probabilities:

```go
logits, _, _ := ort.GetTensorData[float32](outputs["logits"])
// logits[0] = SAFE score (raw), logits[1] = INJECTION score (raw)

probs := softmax(logits)
// probs[0] = probability of SAFE
// probs[1] = probability of INJECTION
```

The softmax implementation:

```go
func softmax(logits []float32) []float32 {
    maxVal := logits[0]
    for _, v := range logits[1:] {
        if v > maxVal {
            maxVal = v
        }
    }
    sum := float32(0)
    result := make([]float32, len(logits))
    for i, v := range logits {
        result[i] = float32(math.Exp(float64(v - maxVal)))
        sum += result[i]
    }
    for i := range result {
        result[i] /= sum
    }
    return result
}
```

### Step 7: Map to Label

The highest probability determines the label. The confidence is the probability value:

```go
label := "SAFE"
confidence := float64(probs[0])
if len(probs) > 1 && probs[1] > probs[0] {
    label = "INJECTION"
    confidence = float64(probs[1])
}
```

### Step 8: Decision

The label and confidence map to a verdict decision:

| Condition | Decision |
|-----------|----------|
| `label == "INJECTION" && confidence >= threshold` | BLOCK |
| `label == "INJECTION" && confidence < threshold` | ESCALATE |
| `label == "SAFE"` | ALLOW |

## Heuristic-Only Fallback

When the ONNX model is not installed, Shield operates in heuristic-only mode. The DualClassifier runs only the heuristic engine, which provides pattern-based detection for known attack signatures.

Heuristic-only mode:
- Detects known injection patterns (e.g., "ignore previous instructions")
- Detects path traversal, data exfiltration, credential exposure
- Does **not** detect novel or obfuscated attacks that ML models can catch
- Has zero startup latency (no model loading)
- Uses zero additional memory

Shield logs a warning at startup when running in heuristic-only mode:

```
WARN onnx_classifier_unavailable: Shield running in heuristic-only mode.
     Run 'openparallax get-classifier' for enhanced protection.
```

## HTTP Sidecar Fallback

If the local ONNX model is not available but a classifier HTTP endpoint is configured (`classifier_addr` in config), Shield falls back to making HTTP calls to the classifier sidecar. This is a legacy deployment mode -- in-process inference is preferred for both latency and reliability.

```go
shield.NewPipeline(shield.Config{
    ClassifierAddr: "localhost:8081",  // HTTP sidecar fallback
})
```

The sidecar exposes two endpoints:
- `GET /health` -- returns 200 if ready
- `POST /classify` -- accepts `{"text": "..."}`, returns `{"label": "SAFE|INJECTION", "confidence": 0.95}`

## Resource Usage

| Resource | Base Model | Small Model |
|----------|-----------|-------------|
| Disk space | ~710MB | ~270MB |
| Memory (loaded) | ~800MB | ~300MB |
| CPU per inference | ~50ms (1 thread) | ~15ms (1 thread) |
| Startup time | ~2s | ~1s |

The ONNX session is configured with `IntraOpNumThreads: 1` to avoid contention with the rest of the system. For higher throughput, increase this value:

```go
sess, _ := ortRT.NewSession(ortEnv, modelPath, &ort.SessionOptions{
    IntraOpNumThreads: 4,
})
```

## Cleanup

The `LocalOnnxClient` holds ONNX Runtime resources that should be released when Shield is shut down:

```go
client := tier1.NewLocalOnnxClient(0.85)
defer client.Close()
```

`Close()` releases the ONNX session, environment, and runtime in the correct order.

## Next Steps

- [Tier 1 -- Classifier](/shield/tier1) -- how the DualClassifier uses ONNX + heuristic results
- [Tier 2 -- LLM Evaluator](/shield/tier2) -- the final evaluation tier
- [Configuration](/shield/configuration) -- classifier configuration options
