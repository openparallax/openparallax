# Contributing to OpenParallax

We welcome contributions to OpenParallax. Whether you're fixing a bug, adding a feature, improving documentation, or writing tests, your work helps make AI agents safer and more capable.

## Getting Started

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

**Prerequisites:** Go 1.25+, Node.js 20+ (for web frontend and docs), protoc (if modifying protos)

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

- **Memory backends** — implementing the `Store` interface for new databases (MongoDB, DynamoDB, Milvus, etc.)
- **Channel adapters** — new messaging platform integrations
- **Shield policies** — curated policy files for specific use cases
- **Documentation** — improvements, corrections, tutorials
- **Tests** — especially integration tests for security-critical paths
- **Platform support** — sandbox improvements for Windows, BSD

### Before Starting Large Changes

For anything beyond bug fixes and small improvements, **open an issue first** to discuss the approach. This prevents wasted effort if the change doesn't align with the project direction.

Things that always need discussion:
- New dependencies (`go get`)
- Protobuf schema changes
- Config schema changes
- New module boundaries
- Changes to the security pipeline

## Architecture Guide

Understanding the codebase:

```
cmd/agent/          → CLI entry points (start with start.go, init.go)
internal/engine/    → The orchestrator (start with engine.go)
internal/agent/     → LLM reasoning loop (start with loop.go)
internal/shield/    → Security pipeline (start with pipeline.go, gateway.go)
web/src/            → Svelte frontend (start with App.svelte)
```

Key interfaces:
- `llm.Provider` — add new LLM providers
- `EventSender` — add new client transports
- `memory.Store` (VectorStore + TextStore) — add new memory backends
- Executor interface — add new tool action types

Read the [Architecture docs](https://openparallax.dev/technical/) for the full picture.

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
