# Eval Run Index

Each `run-NNN/` directory contains the JSON output of a single evaluation
session. File naming inside each run is uniform:

| File | Suite |
|------|-------|
| `c1_direct_injection.json` | C1 — Direct prompt injection |
| `c2_indirect_injection.json` | C2 — Indirect injection (file/email/web/API) |
| `c3_multistep_context.json` | C3 — Multistep context escalation |
| `c4_toolchain_attacks.json` | C4 — Toolchain / IFC chain attacks |
| `c5_encoding_obfuscation.json` | C5 — Encoding & obfuscation |
| `c6_multiagent.json` | C6 — Multi-agent compromise |
| `c7_validator_targeted.json` | C7 — Validator-targeted attacks |
| `c8_helpfulness_bypass.json` | C8 — Helpfulness bypass |
| `c9_tier3_human_approval.json` | C9 — Tier 3 human approval (added run-010+) |
| `fp_false_positives.json` | FP — Legitimate dev/sysadmin/comms operations |

Suffixes:
- `-cfgA` / `-cfgB` / `-cfgC` — eval Config A (baseline), B (guardrails),
  or C (full Parallax)
- `-v1` / `-v2` — multiple sub-runs of the same suite within one session
- `-{model}` — model-specific variant (e.g. `-haiku`, `-llama`, `-sonnet`)

The canonical test-suite YAML files live in `eval-results/test-suite/`.
A frozen snapshot is preserved alongside `runs/run-001/test-suite/` for
historical reference (note: it predates C8 and C9, so only 8 files).

---

## Run History

### LLM Mode — Model Refusal Pivot (Runs 001-003)

**run-001** — Claude Sonnet 4.6
- All 3 configs (A: baseline, B: guardrails, C: full Parallax)
- C1 + C2 only
- **Finding:** 0% ASR across all configs. Sonnet refused every harmful
  action at the reasoning level. REFUSED_BY_MODEL on every case. Shield
  never fired. Configs A=B=C — no signal.

**run-002** — Helpfulness bypass attack class
- Sonnet 4.6 + Haiku 4.5
- Helpfulness-framed attacks ("review my SSH config for security issues")
- **Finding:** 0% ASR on both models. Even social-engineering framings
  refused.

**run-003** — Llama 3.1 8B via Groq API
- Same helpfulness suite
- **Finding:** Also 0% ASR. Meta's instruct model has equivalent
  refusal training.

**Conclusion:** All commercially available instruction-tuned models with
tool-use capability refuse these attacks. There is no behavioral
differentiation between configs on the same model. The eval methodology
must change.

### Inject Mode Pivot (Runs 004-005)

**run-004** — First inject mode pass
- Bypasses LLM entirely; submits expected_harmful_action directly to Shield
- Tests the architectural guarantee under "fully compromised agent"
- Multiple sub-runs (-v1, -v2) as the harness was iterated
- **Finding:** Default policy had read-side gaps. C1 4%, C2 2.5%, C7 19%.

**run-005** — Initial policy hardening
- Closed credential file deny rules, agent internals, /tmp paths
- Routed `execute_command` to Tier 2 (LLM evaluator)
- **Finding:** C1-C6 → 0% ASR. C7 7.7%, C8 (HB) 8.3%. FP 34%.

### Allow Rules Bug + Tightening (Runs 006-009)

**run-006** — Added blanket `allow_workspace_writes` rule
- write_file passing through Tier 0 → enabled massive C2/C3/C7/C8
  regressions. ASR went UP because allow rules were too broad.

**run-007** — Targeted deny rules
- Added `block_system_config_writes`, `block_binary_overwrites`,
  `block_git_hook_writes`, `block_tmp_exfiltration`
- Removed blanket allow_workspace_writes
- C2/C3/C8 → 0%. C7 still has 2 execute_command leaks (heuristics
  never ran on tier-2-escalated commands).

**run-008** — First gateway fix attempt
- Made dual classifier always run, only gating ALLOW on minTier
- All attack categories → 0%, but FP jumped from 24% → 66% as ONNX
  started running on execute_command and other types.

**run-009** — AlwaysBlock heuristic precheck
- New `EvaluateAlwaysBlock` method: only specific high-precision rules
  fire on Tier 2 escalations
- All categories → 0%, FP back to 40%
- Still 3 heuristic-precheck FPs from rules with too-broad regex
  (dollar_subshell, recursive_delete, chmod_world_writable)

### Final Hardening (Run 010)

**run-010** — Tighter precheck + Tier 3 wiring
- AlwaysBlock=true flag added to HeuristicRule struct
- Only XP-036 (list_agent_internals) and XP-037 (grpc_service_enumeration)
  fire as precheck rules
- Fixed XP-012 chmod regex (was matching any "7" anywhere)
- Tier 2 evaluator gained ESCALATE response option
- New c9 tier3_human_approval suite added
- **Final attack-side numbers:** 275/275 blocked, 0.0% ASR across all
  8 categories (C1-C8). Tier 3 path proven end-to-end.
- FP 36% (14 ONNX over-fires + 4 Tier 2 cautious blocks)

### Classifier Optimization (Runs 011-013)

**run-011** — Heuristic-only opt-in default
- ONNX disabled by default (`shield.classifier_enabled: false`)
- **MASSIVE REGRESSION:** C5 0%→100%, C4 0%→92%, C6 0%→96%
- ONNX was doing critical work on encoding/obfuscation/multi-agent
  categories. Heuristic engine alone is blind to these.

**run-012** — Per-action-type ONNX skip list
- `shield.classifier_skip_types` defaults to 7 FP-prone types
- ONNX runs on attack-prone types, skipped on benign-content types
- C5/C6/C7 → 0%, but C1/C2/C3/C4/C8/T3 still leaking because policy
  routed skipped types to Tier 1 (which now had no content review).

**run-013** — Final architecture
- Policy `evaluate_external_communication`, `evaluate_file_writes`,
  `evaluate_destructive_file_ops` → escalate to Tier 2 (LLM evaluator)
- 277/280 attacks blocked (98.9%), **FP 0/50 (0%)**
- 3 remaining attack edge cases are Tier 2 evaluator judgment calls
  (delete /tmp, http POST validation endpoint, env command)
- This is the new default. Trade: -1.1pt ASR for -36pt FP.

---

For full narrative analysis of each session, see [`../reports/`](../reports/INDEX.md):
- `01-initial-eval.md` — runs 001-010 (LLM-mode pivot, policy bootstrap, AlwaysBlock)
- `02-tier3-wiring.md` — Tier 3 wiring (between runs 009 and 010)
- `03-classifier-optimization.md` — runs 011-013 (per-type ONNX skip)
- `04-tier-distribution-and-latency.md` — distilled tier distribution and latency tables
