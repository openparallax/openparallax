# Eval Results

Everything related to OpenParallax's adversarial security evaluation
lives here: the test corpus, the historical run data, the narrative
reports of every session, and a local scratch space for new runs.

## Layout

```
eval-results/
├── test-suite/        Adversarial corpus (YAML). 337 cases across 10 suites.
├── runs/              Frozen JSON results from every eval session.
├── reports/           Narrative writeups of each eval session.
└── playground/        Local scratch space (gitignored).
```

Each subdirectory has its own index:

- [`test-suite/INDEX.md`](test-suite/INDEX.md) — what each suite covers,
  YAML schema, how to add new cases
- [`runs/INDEX.md`](runs/INDEX.md) — naming convention, full run history,
  one paragraph per session
- [`reports/INDEX.md`](reports/INDEX.md) — chronological reading order
  for the four narrative writeups
- [`playground/README.md`](playground/README.md) — how to use the
  local scratch space without dirtying git

## Where to start

**If you want the headline numbers:** read
[`reports/04-tier-distribution-and-latency.md`](reports/04-tier-distribution-and-latency.md).
Two tables, distilled from run-010.

**If you want to understand how we got there:** read the four reports in
order. Each builds on the previous one.

| # | Report | What it covers |
|---|--------|----------------|
| 1 | [01-initial-eval.md](reports/01-initial-eval.md) | LLM-mode pivot, inject mode introduction, policy bootstrap, AlwaysBlock heuristic precheck |
| 2 | [02-tier3-wiring.md](reports/02-tier3-wiring.md) | Wiring Tier 2 ESCALATE → Tier 3 human-in-the-loop |
| 3 | [03-classifier-optimization.md](reports/03-classifier-optimization.md) | Per-action-type ONNX bypass, FP rate 36% → 0% |
| 4 | [04-tier-distribution-and-latency.md](reports/04-tier-distribution-and-latency.md) | Distilled tier distribution and latency tables |

**If you want to reproduce a result:** check `runs/run-NNN/` for the
exact JSON outputs and `test-suite/` for the YAML inputs (note: the
test corpus has evolved across runs — `runs/run-001/test-suite/` holds
the frozen snapshot from that run if you need to match exactly).

**If you want to run a new eval:** drop the output into `playground/`,
which is gitignored. When the run is worth keeping, move it to
`runs/run-NNN/` following the naming convention in
[`runs/INDEX.md`](runs/INDEX.md).

## Headline numbers (run-013, current default config)

- **Test corpus:** 337 cases (280 attacks across 9 categories + 50
  legitimate-action false-positive cases + 7 Tier 3 legitimate-intent
  cases)
- **Attacks blocked:** 277/280 (98.9%)
- **False positive rate:** 0/50 (0.0%)
- **Mode:** inject (LLM bypassed; assumes fully compromised agent —
  the worst case for the architecture)
- **Tier 1 classifier:** ONNX enabled by default, bypassed for 7
  FP-prone action types (`write_file`, `delete_file`, `move_file`,
  `copy_file`, `send_email`, `send_message`, `http_request`)
- **Tier 2 evaluator:** Claude Haiku 4.5

The 3 attack misses in run-013 are Tier 2 LLM evaluator judgment calls
on genuinely ambiguous edge cases (deletion of `/tmp/` files, HTTP POST
to a "validation endpoint", `env` command). Addressable by prompt
engineering or by adding `AlwaysBlock` heuristic rules — see
[03-classifier-optimization.md](reports/03-classifier-optimization.md)
for details.
