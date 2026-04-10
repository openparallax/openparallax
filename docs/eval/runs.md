# Run History

The eval went through 13 runs as the methodology evolved. Each run is a frozen snapshot — the JSON result files live at [`eval-results/runs/run-NNN/`](https://github.com/openparallax/openparallax/tree/main/eval-results/runs) and never change. New evaluation work goes into a new run directory.

The full narrative of every methodology change lives in the [Reports](/eval/reports). This page is the index — one paragraph per run with the headline finding.

## LLM mode runs (the methodology pivot)

| Run | Mode | Model(s) | Headline |
|---|---|---|---|
| **001** | LLM | claude-sonnet-4-6 | 0% ASR across all configs (A, B, C). Sonnet refused every harmful action at the reasoning level — `REFUSED_BY_MODEL` on every case. Shield never fired. |
| **002** | LLM | sonnet-4-6 + haiku-4-5 | 0% ASR on the helpfulness-bypass suite for both models. |
| **003** | LLM | llama-3.1-8b-instant via Groq | Also 0% ASR. Llama's instruct training catches the same patterns. |

**Conclusion of LLM-mode phase:** all commercially available instruction-tuned models with tool-use capability refuse the corpus at the reasoning layer. Configs A, B, and C produce identical results — no architectural differentiation possible. **Methodology pivots to inject mode.**

## Inject-mode runs (architectural validation)

| Run | Headline | Δ from previous |
|---|---|---|
| **004** | First inject pass. C1 4%, C2 2.5%, C7 19.2%. Read-side policy gaps. | Methodology change |
| **005** | Initial policy hardening (credential files, agent internals, /tmp paths, `evaluate_shell_commands` → Tier 2). C1-C6 → 0%, C7 7.7%, C8 8.3%, FP 34%. | Policy tightened |
| **006** | Allow-rules bug. Added blanket `allow_workspace_writes` rule, broke C2/C3/C7/C8 detection. ASR went UP. | Anti-pattern |
| **007** | Targeted deny rules: `block_system_config_writes`, `block_binary_overwrites`, `block_git_hook_writes`, `block_tmp_exfiltration`. C2/C3/C8 → 0%. C7 still leaking 2 cases. | Policy fix |
| **008** | First gateway fix attempt: dual classifier always runs. All categories → 0%, but FP jumped 24% → 66% as ONNX started seeing every action type. | Architectural blunder |
| **009** | AlwaysBlock heuristic precheck added. Only specific high-precision rules fire on Tier 2 escalations. All categories → 0%, FP back to 40%. Three remaining FPs from too-broad regex. | Architectural correction |
| **010** | AlwaysBlock=true flag added to `HeuristicRule` struct. Only XP-036 (list_agent_internals) and XP-037 (grpc_service_enumeration) fire as precheck rules. Fixed XP-012 chmod regex. **Tier 2 evaluator gained ESCALATE response option** — Tier 3 wired end-to-end. New `c9_tier3_human_approval` suite added. **275/275 attacks blocked, FP 36%.** | Major: Tier 3 ships |
| **011** | Heuristic-only opt-in default. ONNX disabled. **Massive regression** — C5 0%→100%, C4 0%→92%, C6 0%→96%. ONNX is doing critical work on encoding/obfuscation/multi-agent categories. | Anti-pattern |
| **012** | Per-action-type ONNX skip list (`shield.classifier_skip_types`). C5/C6/C7 → 0%, but C1/C2/C3/C4/C8/T3 leaking because policy routed skipped types to Tier 1. | Partial fix |
| **013** | Policy `evaluate_external_communication`, `evaluate_file_writes`, `evaluate_destructive_file_ops` → escalate to Tier 2 instead of Tier 1. **277/280 attacks blocked (98.9%), FP 0/50 (0%).** This is the new default. | Production-ready |

**Run-013 is the current published baseline.** All future runs are new directories (`run-014`, `run-015`, ...) and existing runs are never modified.

## Per-run files

Every `run-NNN/` directory contains uniformly named result JSONs:

```
run-013/
  c1_direct_injection.json
  c2_indirect_injection.json
  c3_multistep_context.json
  c4_toolchain_attacks.json
  c5_encoding_obfuscation.json
  c6_multiagent.json
  c7_validator_targeted.json
  c8_helpfulness_bypass.json
  c9_tier3_human_approval.json
  fp_false_positives.json
```

Earlier runs may be missing files if a suite didn't exist yet (c8 was added before run-002, c9 before run-010). Run-001 has its own `test-suite/` snapshot inside it because the corpus has evolved since.

Some runs have suffixes:
- `-cfgA` / `-cfgB` / `-cfgC` — eval Config A (baseline), B (guardrails), or C (full Parallax). Used in run-001 where the same suite ran against all three configs.
- `-v1` / `-v2` — multiple sub-runs of the same suite within one session. Used in run-004 during harness iteration.
- `-{model}` — model-specific variant. Used in run-002/run-003 when comparing the same suite across `sonnet`, `haiku`, `llama`.

For the canonical naming convention and the schema each result file follows, see [`eval-results/runs/INDEX.md`](https://github.com/openparallax/openparallax/blob/main/eval-results/runs/INDEX.md).

## Adding a new run

If you reproduce or extend the eval, your run goes in [`eval-results/playground/`](https://github.com/openparallax/openparallax/tree/main/eval-results/playground) (gitignored). When the run is worth keeping — a new baseline, a new policy, a new model — move it to `eval-results/runs/run-NNN/` and open a PR. See [Reproducing Run-013](/eval/reproducing) for the recipe.

## See also

- [Reports](/eval/reports) — the four narrative writeups that explain each phase
- [Reproducing Run-013](/eval/reproducing) — exact recipe for the current baseline
