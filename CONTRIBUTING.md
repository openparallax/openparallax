# Contributing to OpenParallax

OpenParallax is a security-first AI agent — the architecture is designed so that a fully compromised LLM still can't cause harm. Contributions that strengthen that guarantee, expand the attack surface coverage, or make the system more accessible are all valuable.

**You don't need to be a Go developer to contribute.** Some of the highest-impact work is writing adversarial test cases (YAML), tuning Shield policies (YAML), improving documentation (Markdown), or testing IFC presets against real-world data workflows. If you work in security research, compliance, healthcare, finance, or legal and can describe what "sensitive data" looks like in your domain — that's a contribution.

For code contributions, the setup is:

```bash
# Clone the repo
git clone https://github.com/openparallax/openparallax.git
cd openparallax

# Build everything
make build-all

# Run the full test suite
make build-all && make test && make lint && cd web && npm test && cd ..

# Run the docs site locally
cd docs && npm install && npm run dev
```

**Prerequisites (code contributions only):** Go 1.25+, Node.js 20+ (for web frontend and docs), protoc (if modifying protos)

## Development Workflow

1. **Fork** the repository
2. **Create a branch** from `main` — use a descriptive name (`fix-shield-bypass`, `feat-redis-backend`)
3. **Make your changes** — follow the code standards below
4. **Run the full suite** — `make build-all && make test && make lint && cd web && npm test && cd ..`
5. **Commit** — use [conventional commits](#commit-messages)
6. **Open a PR** — describe what you changed and why

## Code Standards

These are non-negotiable. PRs that violate them will be asked to fix before merge.

### The Rules

1. **Zero TODO/FIXME.** No `// TODO`, `// HACK`, `// XXX` in committed code. If something needs follow-up, open an issue.
2. **No dead code.** No commented-out blocks, unused functions, unreachable branches.
3. **Conventional commits.** `type: description` — types: `feat`, `fix`, `refactor`, `test`, `chore`, `docs`.
4. **Clean lint.** `gofmt` + `go vet` + `golangci-lint` = zero issues. No `//nolint` without genuine justification.
5. **GoDoc on exports.** Every exported symbol gets a comment starting with the symbol name.
6. **Real tests.** Meaningful assertions with real filesystem, real SQLite. No `assert.True(true)`.
7. **Race-free.** `go test -race` with zero warnings.
8. **Fail-closed security.** Every Shield error path returns BLOCK.
9. **Zero CGo.** `CGO_ENABLED=0`. Pure Go only.
10. **Full suite passes.** `make build-all && make test && make lint && cd web && npm test` — all green before committing.

### Commit Messages

```
feat: add Redis backend for memory module
fix: prevent sandbox escape via symlink traversal
refactor: extract policy matcher into standalone package
test: add integration tests for Tier 2 canary verification
docs: update Shield standalone deployment guide
chore: upgrade golangci-lint to v2.5
```

Lowercase, present tense, no period. Keep the first line under 72 characters. Use the body for context on *why*, not *what* (the diff shows what).

## What to Contribute

### Good First Issues

Look for issues labeled `good-first-issue` — these are scoped, well-defined, and won't require deep architectural knowledge.

### Areas We Especially Welcome

**No code required:**

- **Documentation** — improvements, corrections, tutorials, translations. The docs are Markdown in `docs/`.
- **Adversarial test cases** — grow the [eval corpus](https://docs.openparallax.dev/eval/). Every new case is a permanent regression test on Shield. Cases are YAML files — no Go needed. See [Adding Test Cases](#adding-adversarial-test-cases) below.
- **Shield and IFC policies** — curated policy files for specific deployment profiles (healthcare HIPAA, financial SOX, legal privilege, air-gapped environments, multi-tenant). Policies are YAML in `security/shield/` and `security/ifc/`.
- **Domain-specific IFC source rules** — if you work with regulated data and know what "sensitive" looks like in your field (patient records, financial instruments, legal discovery), contributing source classification patterns to the IFC strict preset directly improves protection for your industry.

**Code contributions:**

- **Heuristic rules** — new patterns for `platform/shell.go` (cross-platform `XP-NNN`, Unix `UX-NNN`, Windows `WIN-NNN`) and `shield/tier1_rules.go` (cross-platform detection categories: PI, PT, DE, EE, SD, GEN, EM, SP).
- **Memory backends** — implementing the `Store` interface for new databases (MongoDB, DynamoDB, Milvus, etc.)
- **Channel adapters** — new messaging platform integrations beyond the seven already supported. Slack (Socket Mode) and Teams (Bot Framework) have config schemas defined but no implementation — code-complete adapters are an open invitation.
- **Platform support** — sandbox improvements for Windows, BSD.
- **ML classifier sidecar** — the DeBERTa ONNX classifier needs retraining (high false positive rate) and a CGo sidecar binary wrapping Microsoft's C++ ONNX Runtime. The infrastructure is ready (`classifier_mode: sidecar`, HTTP client implemented). See the [roadmap](https://docs.openparallax.dev/project/roadmap#classifier-model-retrain--cgo-sidecar).

### Before Starting Large Changes

For anything beyond bug fixes and small improvements, **open an issue first** to discuss the approach. This prevents wasted effort if the change doesn't align with the project direction.

Things that always need discussion:

- New top-level dependencies (`go get`)
- Protobuf schema changes (regenerates `internal/types/pb/`)
- Config schema changes (existing workspace `config.yaml` files must keep working)
- New module boundaries
- Changes to the Shield gateway flow
- Removing or renaming any policy rule (existing workspace policies will break)
- Changes to the eval test corpus structure
- New CGo dependencies (the answer is no — the classifier sidecar will live in its own repo)

## Adding Adversarial Test Cases

The eval corpus is intended to grow over time. Adding a case is one of the highest-leverage contributions you can make — every case becomes a permanent regression test on the security pipeline.

**Every new case must be distinguishable from existing cases on at least one dimension.** A duplicate adds no signal.

Walk through this checklist before opening a PR:

- [ ] **Category** (C1-C9, FP) — does it target a different attack class?
- [ ] **Sophistication** (basic / intermediate / advanced) — different difficulty level than every existing case in the same suite-category?
- [ ] **Action type** — does it use a different `expected_harmful_action.type`?
- [ ] **Payload field** — does the malicious data live in a different field (`path` vs `content` vs `url` vs `body` vs `command` vs `source` vs `destination`)?
- [ ] **Detection layer** — which tier should catch it (0/1/2/3)? Adding cases that exercise underused tiers is high-leverage.
- [ ] **Bypass technique** — encoding, obfuscation, social engineering, multi-step trust building, indirect injection, helpfulness framing, polyglot, Unicode normalization, etc.
- [ ] **Intent** — for FP and C9 (Tier 3) suites, malicious vs legitimate?
- [ ] **Platform** — cross-platform vs Linux/macOS/Windows specific?

If your case differs on **none** of these, it's a duplicate. See [docs/eval/contributing-tests.md](https://docs.openparallax.dev/eval/contributing-tests) for the full guide including ID conventions, the worked example, verifying the case, and what to include in the PR.

Local eval runs go in `eval-results/playground/` (gitignored). When a run is worth keeping, move it to `eval-results/runs/run-NNN/` following the [naming convention](https://github.com/openparallax/openparallax/blob/main/eval-results/runs/INDEX.md).

## Adding Heuristic and Policy Rules

### New heuristic rule

- Add the rule to `platform/shell.go` (cross-platform XP-NNN, Unix UX-NNN, Windows WIN-NNN) or `shield/tier1_rules.go` (cross-platform detection categories: PI, PT, DE, EE, SD, GEN, EM, SP)
- Use the next sequential ID
- Decide `AlwaysBlock`: only set true if the rule catches things the Tier 2 LLM evaluator demonstrably misses (provide evidence from a test case that fails without the rule)
- Bump the rule count comment at the top of the file
- Update `platform/platform_test.go` count assertion if you added a `platform/shell.go` rule
- Add at least one positive test (the rule fires on a known attack) and one negative test (the rule does not fire on a benign similar pattern)
- Run the FP suite to confirm no regression on legitimate operations

### New policy rule

- Update both `internal/templates/files/security/shield/default.yaml` and `internal/templates/files/security/shield/strict.yaml`. The strict ⊇ default invariant is enforced by `TestLoadStrictPolicy` in the shield package.
- Run all 10 eval suites and confirm no ASR or FP regression
- If you remove items from the default `shield.classifier_skip_types` list, also tighten the corresponding policy verify rules so those types still get reviewed

## Architecture Guide

Understanding the codebase:

```
cmd/agent/          → CLI entry points (start with start.go, init.go)
cmd/eval/           → Adversarial test runner (separate binary, never shipped in production)
cmd/shield/         → Standalone Shield service binary
internal/engine/    → The orchestrator (start with engine.go)
internal/agent/     → LLM reasoning loop (start with loop.go)
shield/             → Security pipeline (start with pipeline.go, gateway.go)
memory/             → Chunked vector store + FTS5 + embedding cache
sandbox/            → Kernel-level process isolation (Landlock, sandbox-exec, Job Objects)
audit/              → Append-only JSONL with SHA-256 hash chain
chronicle/          → Copy-on-write workspace snapshots
ifc/                → Information flow control with sensitivity labels
web/src/            → Svelte frontend (start with App.svelte)
eval-results/       → Test corpus, run history, narrative reports
```

Key interfaces:
- `llm.Provider` — add new LLM providers
- `EventSender` — add new client transports
- `memory.Store` (VectorStore + TextStore) — add new memory backends
- Executor interface — add new tool action types

Read the [Architecture docs](https://docs.openparallax.dev/technical/) for the full picture.

## Testing

```bash
# All Go tests with race detection
make test

# Specific package
go test -race -count=1 ./internal/shield/...

# Frontend tests
cd web && npx vitest run

# Specific frontend test
cd web && npx vitest run src/__tests__/specific.test.ts
```

Tests are integration-style — they use real filesystems, real SQLite databases, and (when API keys are set) real LLM APIs. Mock sparingly and only at system boundaries.

## Security Contributions

If your contribution touches security-critical code (Shield, sandbox, audit, IFC, file protection), please:

1. Include tests that verify the security property you're protecting
2. Test the fail-closed path (what happens when things go wrong)
3. Consider adversarial inputs — what would a prompt injection attack look like against your change?
4. Read [SECURITY.md](SECURITY.md) for the full security architecture

## Documentation

The docs live in `docs/` and use VitePress:

```bash
cd docs
npm install
npm run dev      # Dev server at localhost:5173
npm run build    # Production build
```

Each module has its own section with a distinct accent color. Follow the existing page structure when adding new pages. Update the sidebar in `docs/.vitepress/config.ts` if you add new pages.

## License

By contributing, you agree that your contributions will be licensed under the [Apache License 2.0](LICENSE).

## Questions?

- **Bug reports and feature requests:** [GitHub Issues](https://github.com/openparallax/openparallax/issues)
- **Security vulnerabilities:** [GitHub Private Vulnerability Reporting](https://github.com/openparallax/openparallax/security/advisories/new) (see [SECURITY.md](SECURITY.md))
- **General discussion:** [GitHub Discussions](https://github.com/openparallax/openparallax/discussions)
