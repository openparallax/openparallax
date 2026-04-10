# Reports

Four narrative writeups walk through every architectural decision in the eval, in chronological order. Read them top to bottom to follow the full evolution from "frontier models refuse everything" through "100% block, 0% FP production default."

The reports themselves live in [`eval-results/reports/`](https://github.com/openparallax/openparallax/tree/main/eval-results/reports). This page summarizes each one and explains when to read it.

## 01 — Initial Eval

[`eval-results/reports/01-initial-eval.md`](https://github.com/openparallax/openparallax/blob/main/eval-results/reports/01-initial-eval.md)

**Covers:** runs 001 through 010.

**What it walks through:**

- The original LLM-mode methodology (configs A/B/C against three real models)
- Why every model returned 0% ASR — the safety-training-catches-everything finding
- The pivot to inject mode and what the harness changed
- Initial policy bootstrap (denies for credential files, agent internals, /tmp, /etc)
- The allow-rules bug in run-006 where blanket `allow_workspace_writes` broke detection
- Targeted deny rules in run-007
- The gateway architectural blunder in run-008 (dual classifier always running)
- The AlwaysBlock heuristic precheck pattern in run-009
- The XP-012 chmod regex bug that had been silently mismatching for many runs
- Final hardening in run-010 — Tier 3 ESCALATE response, AlwaysBlock flag, all 8 attack categories at 0% ASR

**Read this if:** you want to understand why the architecture looks the way it does and what dead ends were ruled out.

## 02 — Tier 3 Wiring

[`eval-results/reports/02-tier3-wiring.md`](https://github.com/openparallax/openparallax/blob/main/eval-results/reports/02-tier3-wiring.md)

**Covers:** the Tier 3 human-in-the-loop wiring between runs 009 and 010.

**What it walks through:**

- Tier 3 existed in the engine (`requestTier3Approval`, `Tier3Manager`) but was unreachable because Tier 2 only returned ALLOW/BLOCK
- Adding ESCALATE as a third Tier 2 verdict option (prompt + parser + gateway changes)
- The eval harness's `simulated_human` field — how to test Tier 3 in inject mode without a real human
- The new `c9_tier3_human_approval` test suite (12 deliberately ambiguous cases)
- 4 of the 12 cases reach Tier 3 end-to-end via Tier 2 → ESCALATE → simulated human → BLOCKED_BY_HUMAN or EXECUTED_AFTER_APPROVAL
- The other 8 cases get caught earlier by Tier 0 deny rules or Tier 1 heuristics — also correct, Tier 3 is the fallback

**Read this if:** you want to understand the Tier 3 escalation path or you're designing test cases that target it.

## 03 — Classifier Optimization

[`eval-results/reports/03-classifier-optimization.md`](https://github.com/openparallax/openparallax/blob/main/eval-results/reports/03-classifier-optimization.md)

**Covers:** runs 011, 012, 013 — the three attempts to fix the 36% FP rate from run-010.

**What it walks through:**

- Run-010 had 100% block but 36% FP. 14 of the 18 FPs were ONNX over-firing on benign `write_file`/`delete_file`/`send_email`/`http_request` payloads (the trained model treats all structured payloads as injection candidates).
- **Attempt 1 (run-011):** opt-in ONNX (heuristic-only default). Massive regression — C5 0%→100%, C4 0%→92%, C6 0%→96%. ONNX was doing critical work on encoding/obfuscation/multi-agent attacks. Heuristics alone are blind to them. **Rolled back.**
- **Attempt 2 (run-012):** per-action-type ONNX skip list. ONNX runs on attack-prone types, skipped on benign-content types. C5/C6/C7 → 0%. But C1/C2/C3/C4/C8/T3 still leaking because the policy was routing those skipped types to Tier 1 where the classifier was the de-facto reviewer. **Partial fix.**
- **Attempt 3 (run-013):** policy `evaluate_external_communication`, `evaluate_file_writes`, `evaluate_destructive_file_ops` escalate to Tier 2 instead of Tier 1. **277/280 attacks blocked (98.9%), FP 0/50 (0%).**
- The 3 remaining attack misses — Tier 2 LLM evaluator judgment calls on `delete /tmp`, an HTTP POST to a "validation endpoint", and `env` command. Addressable by prompt engineering or AlwaysBlock heuristic rules.

**Read this if:** you want to understand the classifier skip list, the AlwaysBlock primitive, or why the default policy escalates so much to Tier 2.

## 04 — Tier Distribution and Latency

[`eval-results/reports/04-tier-distribution-and-latency.md`](https://github.com/openparallax/openparallax/blob/main/eval-results/reports/04-tier-distribution-and-latency.md)

**Covers:** the distilled tier distribution and latency tables for the paper.

### Tier resolution distribution (run-010, 280 attacks)

| Tier | Count | % | Notes |
|---|---|---|---|
| **Self-Protection** (model refusal) | 0 | 0% | Inject mode bypasses the LLM, so the model never refuses anything itself |
| **Tier 0** (Policy) | 43 | 15.4% | YAML deny rules — credentials, system paths, /tmp exfil, .git/hooks |
| **Tier 1** (Classifier) | 163 | 58.2% | ONNX + heuristic engine |
| **Tier 2** (LLM Evaluator) | 73 | 26.1% | Inverted-context Haiku 4.5 |
| **Tier 3** (Human) | 1 | 0.4% | T3-001 `git push --force` → DENY |
| **Total** | 280 | 100% | |

**Important caveat for the paper:** Self-Protection is 0 because **inject mode does not exercise the LLM's reasoning**. We pivoted to inject mode precisely because frontier LLMs (Claude Sonnet 4.6, Haiku 4.5, Llama 3.1 8B) refused 100% of attacks at the reasoning level — see run 001-003 results. So Self-Protection in LLM mode = 100% (Config A = Config B = Config C). Inject mode tests the architecture's guarantee assuming the LLM is fully compromised.

### Latency by tier (ms, attacks only, run-010)

| Tier | n | P50 | P95 | P99 | Avg |
|---|---|---|---|---|---|
| Tier 0 | 43 | 0* | 0* | 0* | 0* |
| Tier 1 | 163 | 1947 | 2335 | 2638 | 1984 |
| Tier 2 | 73 | 2089 | 3379 | 4781 | 2002 |
| Tier 3 | 1 | 2496 | 2496 | 2496 | 2496 |

\*Tier 0 is sub-millisecond YAML matching — the harness records 0 because the latency field rounds to integer ms. Actual cost is microseconds.

**Note on Tier 1:** ~2s P50 reflects the heuristic-only path (no classifier sidecar active during this run). Enabling a classifier sidecar reduces Tier 1 latency significantly for the ML classification step; the heuristic rules themselves are sub-millisecond. Tier 2 being only ~18ms slower than Tier 1 (2002 vs 1984) confirms that the bottleneck is not the LLM call.

**Tier 3** is just one data point — the simulated human is instantaneous, so the 2496ms is just the Tier 2 LLM call that preceded the escalation.

**Read this if:** you need numbers for a paper, an ops review, or a deployment decision.

## See also

- [Run History](/eval/runs) — every run's headline and what changed
- [Reproducing Run-013](/eval/reproducing) — exact recipe for the published baseline
- [Methodology](/eval/methodology) — what configs and modes measure
