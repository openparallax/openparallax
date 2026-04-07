# Reproducing Run-013

Run-013 is the current baseline for the published Shield numbers: **277/280 attacks blocked (98.9%), 0/50 false positives**. This page is the exact recipe to reproduce it.

## What you need

- A clean OpenParallax checkout at the same commit (or later) that produced run-013. The frozen result JSONs live at [`eval-results/runs/run-013/`](https://github.com/openparallax/openparallax/tree/main/eval-results/runs/run-013).
- A workspace initialized via `openparallax init` (named anything — examples below use "atlas")
- **Two LLM API keys** — one for the chat role and one for the Shield evaluator role. Cross-provider is recommended for diversity (an attack against one model is less likely to also work against another).
- The ONNX classifier installed (run-013 was produced with the classifier enabled)
- The default policy (`policies/default.yaml`, the one shipped with `init`)

## Step-by-step

### 1. Build everything

```bash
git clone https://github.com/openparallax/openparallax.git
cd openparallax
make build-all
go build -o dist/openparallax-eval ./cmd/eval
```

### 2. Initialize a workspace

```bash
./dist/openparallax init
```

The wizard prompts you through:

| Step | Run-013 setting |
|---|---|
| Agent name | `atlas` |
| LLM provider (chat role) | `anthropic` |
| Chat model | `claude-sonnet-4-6-20250514` |
| Shield evaluator provider | `openai` (or any OpenAI-compatible endpoint) |
| Shield evaluator model | `claude-haiku-4-5-20251001` (run-013 used Haiku via an OpenAI-compatible proxy) |
| Embedding provider | optional, skip if you only care about reproducing the eval |
| Workspace path | `~/.openparallax/atlas` (default) |
| Download Tier 1 classifier | **Yes — base model (~700MB)** |

If your provider differs, override at run time with `--model`, `--base-url`, `--api-key-env`. The exact LLM matters for Tier 2 verdicts — different models will produce slightly different ALLOW/BLOCK/ESCALATE decisions on the ambiguous edge cases.

### 3. Verify the workspace config

```bash
./dist/openparallax doctor
```

Check for:

- `Shield: policy loaded, Tier 2: 100/day budget`
- `Tier 1: classifier enabled (local mode, 7 action type(s) bypassed)`
- `Sandbox: ...` (any mode)

If `Tier 1` reports `heuristic-only`, the classifier didn't install — re-run `openparallax get-classifier` and restart.

The skip list of 7 action types (`write_file`, `delete_file`, `move_file`, `copy_file`, `send_email`, `send_message`, `http_request`) is the default. **Do not override `shield.classifier_skip_types` in your config.yaml** if you want to match run-013.

### 4. Run all 10 suites

```bash
mkdir -p eval-results/playground/repro-$(date +%Y%m%d)
OUT="eval-results/playground/repro-$(date +%Y%m%d)"

for suite in eval-results/test-suite/c1_direct_injection.yaml \
             eval-results/test-suite/c2_indirect_injection.yaml \
             eval-results/test-suite/c3_multistep_context.yaml \
             eval-results/test-suite/c4_toolchain_attacks.yaml \
             eval-results/test-suite/c5_encoding_obfuscation.yaml \
             eval-results/test-suite/c6_multiagent.yaml \
             eval-results/test-suite/c7_validator_targeted.yaml \
             eval-results/test-suite/c8_helpfulness_bypass.yaml \
             eval-results/test-suite/c9_tier3_human_approval.yaml \
             eval-results/test-suite/fp_false_positives.yaml; do
  name=$(basename "$suite" .yaml)
  echo "=== $name ==="
  ./dist/openparallax-eval \
    --suite "$suite" \
    --config C \
    --mode inject \
    --workspace ~/.openparallax/atlas \
    --output "$OUT/$name.json"
done
```

Total elapsed time: **8-12 minutes**, depending on Tier 2 LLM latency. The bottleneck is Tier 2: ~70 calls × ~2s each.

### 5. Diff against the published run

The frozen run-013 results are stored as JSON. A simple per-suite diff:

```bash
for s in c1_direct_injection c2_indirect_injection c3_multistep_context \
         c4_toolchain_attacks c5_encoding_obfuscation c6_multiagent \
         c7_validator_targeted c8_helpfulness_bypass c9_tier3_human_approval \
         fp_false_positives; do
  echo "=== $s ==="
  python3 -c "
import json
mine = json.load(open('$OUT/$s.json'))
ref  = json.load(open('eval-results/runs/run-013/$s.json'))
m_asr = mine['summary']['overall_asr']
r_asr = ref['summary']['overall_asr']
m_fp  = mine['summary']['false_positive_rate']
r_fp  = ref['summary']['false_positive_rate']
print(f'  ASR: {m_asr*100:.1f}%  (run-013: {r_asr*100:.1f}%)')
print(f'  FP:  {m_fp*100:.1f}%  (run-013: {r_fp*100:.1f}%)')
"
done
```

## What "matching" means

Exact byte equality is not the goal — Tier 2 LLM calls are non-deterministic. The same evaluator can give a slightly different verdict on edge cases between runs. What you should match:

| Metric | Run-013 | Acceptable variance |
|---|---|---|
| C1-C7 ASR | 0.0% | exact, these resolve at Tier 0 or Tier 1 deterministic paths |
| C8 ASR | 1.7% (1/60) | ±2% — Tier 2 judgment calls |
| C9 ASR | 0.0% | exact, deterministic policy resolution |
| Overall attack ASR | 1.1% | ±2% |
| FP rate | 0.0% | ±2% — also Tier 2 dependent |

If your numbers are dramatically different, the most common causes are:

1. **Wrong workspace policy** — your `policies/default.yaml` was edited or you copied a different one. The eval uses the workspace's `policies/default.yaml`. Run `diff ~/.openparallax/atlas/policies/default.yaml internal/templates/files/policies/default.yaml` to verify.
2. **Classifier missing** — `openparallax doctor` shows `heuristic-only`. Run `get-classifier` and restart.
3. **Different evaluator model** — run-013 used `claude-haiku-4-5-20251001`. A weaker or stronger model produces different judgments on the ~5-10 cases that reach Tier 2.
4. **Custom skip list** — the default is 7 action types. If you set `shield.classifier_skip_types: []`, ONNX runs on every action type and you'll see the run-010-era 36% FP rate (and possibly slightly better ASR).
5. **Modified test cases** — the suite at HEAD may differ from run-013. To match exactly, check out the commit that produced run-013: `git log --all -- eval-results/runs/run-013/`.

## What if your numbers are *better*?

Submit a PR. Include:
- Your full result JSONs in `eval-results/playground/<your-name>-<date>/`
- A short note explaining what changed (different model, tightened policy, new heuristic rule)
- Confirmation that the FP rate didn't get worse

This is exactly the kind of contribution we want.

## See also

- [Run History](/eval/runs) — all 13 historical runs and what changed between them
- [Reports](/eval/reports) — narrative writeups of every architectural decision
- [Methodology](/eval/methodology) — what configs A/B/C measure
