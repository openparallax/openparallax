# ncruces/go-sqlite3 + sqlite-vec Spike Report

Date: 2026-03-30

## Summary

**Recommendation: Don't migrate yet.** ncruces/go-sqlite3 v0.33 is excellent — faster builds, smaller binary, all ops work. But sqlite-vec bindings are incompatible with the latest ncruces (v0.33 removed `sqlite3.Binary` for wasm2go). Until the bindings catch up, we can't get native vec0 tables. Without vec0, there's no reason to switch from modernc.

## Test Results

| Test | ncruces v0.33 | modernc (current) |
|------|--------------|-------------------|
| Schema creation (CREATE TABLE) | PASS | PASS |
| FTS5 virtual table | PASS | PASS |
| FTS5 MATCH queries with ranking | PASS | PASS |
| WAL mode | PASS | PASS |
| Concurrent writes (20 goroutines) | PASS | PASS |
| sqlite-vec (vec0 + KNN) | **FAIL** — bindings incompatible | N/A (no vec support) |
| CGO_ENABLED=0 | PASS | PASS |

## Performance

| Metric | ncruces v0.33 | modernc v1.48 | Delta |
|--------|--------------|---------------|-------|
| 1000 FTS5 inserts | 107ms | 154ms | ncruces 30% faster |
| 100 FTS5 searches (avg) | 6.3ms | 6.0ms | ~equal |

Performance is within 5% for searches, ncruces is actually faster for inserts.

## Binary Size

| Driver | Minimal binary | Delta |
|--------|---------------|-------|
| modernc.org/sqlite | 8.5 MB | baseline |
| ncruces/go-sqlite3 | 3.0 MB | **-65%** |
| Full openparallax binary (modernc) | 57 MB | current |

ncruces produces a dramatically smaller SQLite binary. The full openparallax binary would shrink by ~5MB.

## sqlite-vec Compatibility Issue

The `github.com/asg017/sqlite-vec-go-bindings` package (latest v0.1.7-alpha.2) expects `sqlite3.Binary` — a variable that held the WASM binary bytes. ncruces v0.33 switched to wasm2go (compiled Go code, no runtime WASM interpreter), removing `sqlite3.Binary` entirely.

Versions that still have `sqlite3.Binary` (ncruces v0.22 and below) use wazero, which has its own incompatibility with the sqlite-vec WASM binary's atomic operations on Go 1.26.

**Status:** Blocked on upstream. The sqlite-vec Go bindings need to be updated for ncruces wasm2go. This is a known gap in the ecosystem.

## Decision

**Keep modernc.org/sqlite for now.** Re-evaluate when:
1. sqlite-vec Go bindings support ncruces v0.33+ (wasm2go), OR
2. ncruces adds a built-in vec extension

**Alternative for vector search:** Continue using our current approach — compute embeddings in Go, store as BLOBs, do KNN in application code. This works, doesn't depend on sqlite-vec, and is already tested.

## What Would Change If We Migrate Later

| File | Change |
|------|--------|
| go.mod | Replace `modernc.org/sqlite` with `ncruces/go-sqlite3` |
| internal/storage/db.go | Change driver name `"sqlite"` to `"sqlite3"` |
| internal/storage/db.go | Update `sql.Open` call |
| All `*_test.go` in storage | Same driver name change |

Estimated effort: 30 minutes. The `database/sql` interface is identical.
