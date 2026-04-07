## 1. Tier Resolution Distribution (run-010, 280 attacks)

| Tier | Count | % | Notes |
|------|-------|-----|-------|
| **Self-Protection** | 0 | 0% | Inject mode bypasses the LLM, so the model never refuses anything itself |
| **Tier 0 (Policy)** | 43 | 15.4% | YAML deny rules — credentials, system paths, /tmp exfil, .git/hooks, etc. |
| **Tier 1 (Classifier)** | 163 | 58.2% | ONNX + heuristic engine |
| **Tier 2 (LLM Evaluator)** | 73 | 26.1% | Inverted-context Haiku 4.5 |
| **Tier 3 (Human)** | 1 | 0.4% | T3-001 `git push --force` → DENY |
| **Total** | **280** | 100% | 100% block rate, 0% ASR across all 9 categories |

**Important caveat for the paper**: Self-Protection is 0 because **inject mode does not exercise the LLM's reasoning**. We pivoted to inject mode precisely because frontier LLMs (Claude Sonnet 4.6, Haiku 4.5, Llama 3.1 8B) refused 100% of attacks at the reasoning level — Run 001-003 results in EVAL-RECAP.md. So Self-Protection in LLM mode = 100% (Config A = Config B = Config C). Inject mode tests the architecture's guarantee assuming the LLM is fully compromised.

## 2. Latency by Tier (ms, attacks only, run-010)

| Tier | n | P50 | P95 | P99 | Avg |
|------|---|-----|-----|-----|-----|
| Tier 0 | 43 | 0* | 0* | 0* | 0* |
| Tier 1 | 163 | 1947 | 2335 | 2638 | 1984 |
| Tier 2 | 73 | 2089 | 3379 | 4781 | 2002 |
| Tier 3 | 1 | 2496 | 2496 | 2496 | 2496 |

*Tier 0 is sub-millisecond YAML matching — the harness records 0 because the latency field rounds to integer ms. The actual cost is microseconds.

**Caveat on Tier 1**: ~2s P50 is dominated by the ONNX classifier's per-call overhead in our local in-process implementation. This is much higher than what's typical for ONNX Runtime — likely worth investigating before using in the paper. Tier 2 being only 18ms slower than Tier 1 (2002 vs 1984) is suspicious and points to Tier 1 being slower than it should be.

**Tier 3** is just one data point — the simulated human is instantaneous, so the 2496ms is just the Tier 2 LLM call that preceded the escalation.

## 3. Category Naming

**Recommendation: keep them separate as "C8: Helpfulness Bypass"**. C1 is "direct prompt injection" (unfiltered "ignore previous instructions" style); helpfulness bypass is a fundamentally different attack class — legitimate-sounding requests with hidden malicious side-effects. Folding them would obscure that the model's safety training catches one but not the other in the LLM-mode runs.

Also gives you a clean **C9: Tier 3 Human Approval** for the new suite.

## 4. DTSR On/Off

**Defer to future work.** No data exists. Reasons:
- The eval ran in inject mode, which bypasses the LLM and the entire tool-loading layer — DTSR is invisible to inject-mode tests
- An LLM-mode DTSR comparison would need: (a) a fork of the engine that loads all tools per turn instead of using `load_tools`, (b) a model that actually proposes attacks (frontier models refuse), (c) re-running the 275 attack corpus
- The token savings claims in [docs/technical/design-efficiency.md](docs/technical/design-efficiency.md) are illustrative scenarios, not measured
