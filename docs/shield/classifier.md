# ML Classifier

::: warning Deactivated
The in-process DeBERTa ONNX classifier has been removed from the main binary. Tier 1 runs heuristic-only (79 rules) by default. A retrained model and CGo sidecar binary are the [immediate next priority on the roadmap](/project/roadmap#immediate-next-steps).
:::

## Background

Shield's Tier 1 was originally designed as a DualClassifier: a DeBERTa v3 neural network running in parallel with the heuristic regex engine. The ONNX model was loaded in-process via `onnxruntime-purego`, a pure-Go ONNX runtime wrapper.

Two issues led to its deactivation:

1. **Model quality** — the training set was weighted toward injection-positive examples, causing the model to over-fire on legitimate structured payloads (`write_file`, `send_email`, `http_request`). Seven action types had to be excluded from ONNX classification entirely via `classifier_skip_types`.

2. **Runtime conflict** — `onnxruntime-purego` depends on `ebitengine/purego` which activates `fakecgo`, setting `runtime.iscgo = true` even with `CGO_ENABLED=0`. This caused `go-landlock` (via `libcap/psx` → `AllThreadsSyscall6`) to panic when the agent process applied Landlock sandboxing. Since both ONNX and Landlock were compiled into the same single binary, the conflict was unavoidable without architectural changes.

## Current State

Tier 1 runs **heuristic-only**: 79 hand-written regex rules organized by severity (critical, high, medium) with platform-specific variants for Unix and Windows. The heuristic engine catches direct injection, shell exploits, credential access, and known attack patterns.

## Sidecar Architecture (Planned)

The ML classifier will return as a separate CGo binary (`openparallax-classifier`) in its own repository:

- Wraps Microsoft's C++ ONNX Runtime directly (not the pure-Go wrapper)
- ~30ms inference latency (vs ~2s with the pure-Go runtime)
- Communicates with the engine via HTTP (`/health` + `/classify` endpoints)
- The engine auto-spawns and manages the sidecar process

The infrastructure is ready in the codebase:
- `shield.classifier_mode: sidecar` config field
- `shield.classifier_addr` for the HTTP endpoint
- `HTTPOnnxClient` in `shield/tier1_onnx.go` implements the sidecar protocol
- `DualClassifier` in `shield/tier1_classifier.go` accepts the sidecar client

When the sidecar ships, enable it:

```yaml
shield:
  classifier_enabled: true
  classifier_mode: sidecar
  classifier_addr: "localhost:8090"
```
