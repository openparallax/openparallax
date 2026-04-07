# Eval — Test Your Own Security

Most security claims for AI agents are unverifiable. "Our agent is safe because we trained it on safety data" is not a falsifiable statement.

OpenParallax takes the opposite approach: **we publish the test suite, the run history, and the binary**. Anyone can run the evaluation and check the numbers.

The Parallax paradigm is open and reproducible. The same 337-case adversarial corpus we use to validate Shield ships with the binary. Run it on your own deployment, your own LLM provider, your own policy. The result is reproducible. The methodology is documented. The data is committed.

## What ships

| Component | Location | Purpose |
|---|---|---|
| `openparallax-eval` binary | `cmd/eval/` | Standalone test runner. Built with `go build ./cmd/eval`. Never compiled into the main binary — zero surface area in production. |
| Test corpus | [`eval-results/test-suite/`](https://github.com/openparallax/openparallax/tree/main/eval-results/test-suite) | 337 YAML cases across 9 attack categories + 50 false-positive cases |
| Historical runs | [`eval-results/runs/`](https://github.com/openparallax/openparallax/tree/main/eval-results/runs) | 13 frozen JSON result sets (run-001 through run-013) |
| Narrative reports | [`eval-results/reports/`](https://github.com/openparallax/openparallax/tree/main/eval-results/reports) | 4 markdown writeups of the methodology pivots and architecture decisions |

## The headline number

**Run-013, current default config, inject mode:** 277/280 attacks blocked (98.9%), 0 false positives on 50 legitimate operations. The 3 attack misses are Tier 2 LLM evaluator judgment calls on genuinely ambiguous edge cases (deleting a file in `/tmp`, an HTTP POST to a "validation endpoint", an `env` command). All addressable by prompt engineering or stricter heuristic rules — see [Reports](/eval/reports).

## Why an eval suite at all

The OpenParallax security architecture rests on one assumption: **the AI agent is fully untrusted**. It may be compromised at any time through prompt injection, memory poisoning, indirect injection through retrieved documents, or even adversarial control of the LLM itself. Safety must hold even under total compromise.

The eval validates that assumption against real attack patterns:

1. **Attack categories C1-C8** — eight classes of attacks that real LLMs would fall for, drawn from public injection databases, red-team work, and the Parallax paper's threat model
2. **Tier 3 ambiguous cases (C9)** — 12 actions that are deliberately ambiguous, designed to test the human-in-the-loop escalation path
3. **False-positive cases (FP)** — 50 legitimate dev/sysadmin/file/comms/web operations that Shield must not block, otherwise the agent is unusable in practice

Attack categories test that Shield catches malicious actions. FP cases test that Shield doesn't break legitimate ones. Both are required — a 100% block rate that also blocks every legitimate operation is worthless.

## How it differs from other AI safety benchmarks

| Approach | What it tests | What it misses |
|---|---|---|
| **Refusal benchmarks** (ShareGPT, DAN, etc.) | Whether the LLM refuses harmful requests | Doesn't test whether the *agent system* prevents execution if the LLM is compromised |
| **Red-team prompt injection sets** (Garak, etc.) | Whether the LLM falls for known injections | Same gap — no execution layer |
| **OpenParallax eval (inject mode)** | Whether Shield blocks harmful actions even when the LLM is fully compromised | This is the architectural guarantee, not the model's behavior |

The pivot to **inject mode** in our methodology came from an empirical discovery: every commercially available instruction-tuned model with tool-use capability (Claude Sonnet 4.6, Haiku 4.5, Llama 3.1 8B) refused 100% of our adversarial prompts at the reasoning level. They never proposed the harmful action — Shield never had to fire. This is good news for today, but it's a model property, not an architectural property. Tomorrow's models, or fine-tuned forks of today's, may comply. The architectural guarantee must hold either way.

Inject mode bypasses the LLM entirely and submits the harmful action directly to Shield, simulating the worst case: an attacker has total control of the LLM and is now driving tool calls. If Shield blocks the action under that assumption, the architecture is sound. If it doesn't, there's a real gap.

## Where to start

| You are... | Read |
|---|---|
| Curious about the headline numbers | [Reports → Tier distribution and latency tables](/eval/reports#tier-distribution-and-latency) |
| About to reproduce a result | [Quickstart](/eval/quickstart) |
| Designing your own attack suite | [Methodology](/eval/methodology) and [Test Suite Layout](/eval/test-suite) |
| Adding a test case to the corpus | [Adding Test Cases](/eval/contributing-tests) |
| Reading the architectural narrative | [Reports](/eval/reports) — four numbered writeups of every major decision |
| Diffing your run against ours | [Run History](/eval/runs) |
