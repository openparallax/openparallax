# Eval Reports

Narrative writeups of each eval session, in chronological order. Read
these top-to-bottom to follow the full evolution from "frontier models
refuse everything" through "100% block, 0% FP production default."

| File | Covers | Runs |
|------|--------|------|
| [01-initial-eval.md](01-initial-eval.md) | Methodology pivot from LLM mode → inject mode, initial policy bootstrap, AlwaysBlock heuristic precheck, all 8 attack categories to 0% ASR | run-001 → run-010 |
| [02-tier3-wiring.md](02-tier3-wiring.md) | Wiring Tier 2 ESCALATE → Tier 3 human-in-the-loop, new c9 test suite | between run-009 and run-010 |
| [03-classifier-optimization.md](03-classifier-optimization.md) | Per-action-type ONNX bypass, escalating sensitive ops to Tier 2, FP rate 36% → 0% | run-011 → run-013 |
| [04-tier-distribution-and-latency.md](04-tier-distribution-and-latency.md) | Distilled tier distribution + latency tables for the paper | run-010 |

For the raw JSON results referenced by these reports, see
[`../runs/`](../runs/INDEX.md). For the test corpus YAML files, see
[`../test-suite/`](../test-suite/).
