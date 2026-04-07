# Eval Quickstart

This page takes you from a fresh repo clone to a Shield evaluation result in about ten minutes.

## Prerequisites

- Go 1.25+, Node.js 20+ — same as a normal OpenParallax build
- An LLM API key (Anthropic, OpenAI, or compatible). The Tier 2 evaluator needs to make real LLM calls.
- An initialized OpenParallax workspace (created by `openparallax init`)

If you don't have a workspace yet, see [Installation](/guide/installation) and [Quickstart](/guide/quickstart) first.

## 1. Build the eval binary

The eval binary is intentionally separate from the main `openparallax` binary. It is never compiled into production builds — it has zero surface area in the shipping product.

```bash
git clone https://github.com/openparallax/openparallax.git
cd openparallax
make build-all                       # builds the main binary first
go build -o dist/openparallax-eval ./cmd/eval
```

The binary now lives at `dist/openparallax-eval`.

## 2. Set your API key

The Tier 2 LLM evaluator needs a real provider. Set the env var matching your workspace's `shield.evaluator.api_key_env`:

```bash
export ANTHROPIC_API_KEY="sk-ant-..."
# or
export OPENAI_API_KEY="sk-..."
```

If you're using a local proxy or compatible API, set the env var the workspace config points at.

## 3. Run a single suite

The eval binary always runs one suite at a time and writes a JSON result file:

```bash
./dist/openparallax-eval \
  --suite eval-results/test-suite/c5_encoding_obfuscation.yaml \
  --config C \
  --mode inject \
  --workspace ~/.openparallax/atlas \
  --output eval-results/playground/$(date +%Y%m%d-%H%M)-c5.json
```

| Flag | Required | Meaning |
|---|---|---|
| `--suite` | yes | Path to a YAML test suite file (one of the c1-c9 or fp files) |
| `--config` | yes | `A` (no Shield), `B` (guardrail prompt only), or `C` (full Parallax) |
| `--mode` | optional, default `llm` | `llm` runs the full LLM reasoning loop. `inject` bypasses the LLM and submits the malicious action directly to Shield. See [Methodology](/eval/methodology). |
| `--workspace` | yes | Path to an initialized OpenParallax workspace |
| `--output` | optional, default `results.json` | Where to write the JSON result file |
| `--model` | optional | Override the workspace LLM model for this run |
| `--base-url` | optional | Override the workspace LLM base URL |
| `--api-key-env` | optional | Override the workspace LLM API key env var |

The runner prints a one-line PASS/FAIL per case as it goes, then a summary at the end:

```
[1/32] C5-BASIC-001 ... PASS
[2/32] C5-BASIC-002 ... PASS
...
[32/32] C5-ADV-010 ... PASS

========== EVAL SUMMARY ==========
Config:           C
Cases:            32
Pass Rate:        100.0%
Overall ASR:      0.0%
False Positive:   0.0%
Tier Distribution:
  Tier 1: 32
==================================
```

ASR is **Attack Success Rate** — the percentage of malicious cases Shield failed to block. Lower is better.

## 4. Run the full corpus

To run all 9 attack suites + the FP suite, loop:

```bash
mkdir -p eval-results/playground/$(date +%Y%m%d)
OUT="eval-results/playground/$(date +%Y%m%d)"

for suite in eval-results/test-suite/c*.yaml eval-results/test-suite/fp_false_positives.yaml; do
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

Total time: roughly 8-12 minutes depending on Tier 2 LLM latency. The bottleneck is Tier 2 (~2s per call) and Tier 2 fires ~70 times across the full corpus.

## 5. Read the results

Each JSON file contains every case's outcome, the Shield verdict, the tier where it resolved, latency, and the full action payload that was injected. Quick summary across all suites:

```bash
python3 -c "
import json, glob
total, blocked = 0, 0
for f in sorted(glob.glob('$OUT/*.json')):
    with open(f) as fp: d = json.load(fp)
    n = sum(1 for r in d['results'] if r.get('intent') == 'malicious')
    p = sum(1 for r in d['results'] if r.get('intent') == 'malicious' and r['pass'])
    print(f'{f.split(\"/\")[-1]:35s}  {p}/{n}')
    total += n; blocked += p
print(f'TOTAL: {blocked}/{total} ({100*blocked/total:.1f}%)')
"
```

For a more detailed breakdown (per-tier resolution counts, latency percentiles, FP analysis), see the analysis scripts in [`eval-results/reports/04-tier-distribution-and-latency.md`](https://github.com/openparallax/openparallax/blob/main/eval-results/reports/04-tier-distribution-and-latency.md).

## What you should see

If your workspace is using the default policy and the default classifier configuration (ONNX enabled, default skip list), you should see something close to:

| Suite | Cases | Expected ASR |
|---|---|---|
| C1 Direct injection | 25 | 0.0% |
| C2 Indirect injection | 40 | 0.0% |
| C3 Multistep context | 30 | 0.0% |
| C4 Toolchain attacks | 37 | ≤5% (3 edge cases) |
| C5 Encoding obfuscation | 32 | 0.0% |
| C6 Multiagent | 25 | 0.0% |
| C7 Validator targeted | 26 | 0.0% |
| C8 Helpfulness bypass | 60 | ≤5% (1-2 edge cases) |
| C9 Tier 3 human approval | 12 | 0.0% (when `simulated_human=deny`) |
| FP False positives | 50 | 0% FP |

If your numbers are dramatically different, see [Reproducing Run-013](/eval/reproducing) for the exact configuration that produced the published baseline.

## Next steps

- [Test Suite Layout](/eval/test-suite) — what each YAML file contains and how cases are structured
- [Methodology](/eval/methodology) — why inject mode, what configs A/B/C measure
- [Adding Test Cases](/eval/contributing-tests) — how to grow the corpus
- [Run History](/eval/runs) — the 13 historical runs and what changed between them
