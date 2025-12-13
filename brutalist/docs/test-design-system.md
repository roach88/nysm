---
stepsCompleted: [0, 1]
inputDocuments:
  - docs/prd.md
  - docs/architecture.md
  - docs/epics.md
workflowType: 'test-design'
mode: 'system-level'
lastStep: 1
project_name: 'NYSM'
user_name: 'Tyler'
date: '2025-12-12'
---

# System-Level Test Design - NYSM

**Author:** Tyler
**Date:** 2025-12-12
**Status:** Draft
**Phase:** Solutioning (Pre-Implementation Readiness)

---

## Executive Summary

This document establishes the system-level test strategy for NYSM (Now You See Me), a compiler + runtime framework implementing deterministic event-sourced systems. The architecture has been reviewed for testability against the framework's unique correctness requirements.

**Key Finding:** The architecture is **highly testable** due to its determinism-first design with injectable dependencies and content-addressed identity. The primary testing challenges are RFC 8785 JSON canonicalization cross-language verification and deterministic replay validation.

---

## Testability Assessment

### Controllability: PASS

**Can we control system state for testing?**

| Aspect | Assessment | Evidence |
|--------|------------|----------|
| **State Initialization** | ✅ Excellent | SQLite in-memory mode + embedded schema.sql enables fresh databases per test |
| **Dependency Injection** | ✅ Excellent | Architecture specifies interfaces: `FlowTokenGenerator`, `Clock`, `Store`, `QueryCompiler` |
| **External Dependencies** | ✅ Excellent | Only external dep is SQLite (embeddable); CUE SDK is pure Go |
| **Error Injection** | ✅ Good | Action implementations are pluggable; can inject error-returning actions |
| **Time Control** | ✅ Excellent | No wall-clock (`time.Now()`) in engine path per CP-2; injectable clock |
| **Random Control** | ✅ Excellent | Flow tokens via injectable `FlowTokenGenerator`; can use `FixedGenerator` |

**Controllability Notes:**
- Architecture explicitly eliminates non-determinism sources (timestamps, auto-increment IDs, map iteration order)
- All queries parameterized per HIGH-3; can assert on exact SQL + args
- Projections (state tables) can be queried directly for assertion

### Observability: PASS

**Can we inspect system state?**

| Aspect | Assessment | Evidence |
|--------|------------|----------|
| **Structured Logging** | ✅ Excellent | slog with correlation keys (flow_token, sync_id, binding_hash, seq) |
| **Event Visibility** | ✅ Excellent | Append-only event log is the source of truth; all events queryable |
| **Provenance Queries** | ✅ Excellent | `provenance_edges` table enables "why did this happen?" queries |
| **Test Determinism** | ✅ Excellent | Content-addressed IDs + logical clocks enable byte-identical comparison |
| **Error Visibility** | ✅ Good | Typed error codes (E001-E399) with structured details |
| **Golden Comparison** | ✅ Excellent | Architecture specifies goldie for snapshot testing |

**Observability Notes:**
- Log timestamps removable via slog ReplaceAttr for deterministic snapshots
- Every sync firing records completion_id, sync_id, binding_hash - full audit trail
- QueryIR compilation produces (sql, []args) tuple - can assert parameterized output

### Reliability: PASS

**Are tests isolated and deterministic?**

| Aspect | Assessment | Evidence |
|--------|------------|----------|
| **Test Isolation** | ✅ Excellent | In-memory SQLite per test; no shared state |
| **Parallel Safety** | ✅ Excellent | Single-writer event loop per engine instance; tests can run in parallel |
| **Deterministic Results** | ✅ Excellent | RFC 8785 JSON + logical clocks + sorted iteration per CP-3, CP-4 |
| **Reproducible Failures** | ✅ Excellent | Content-addressed IDs mean same inputs = same outputs |
| **Cleanup Discipline** | ✅ Good | Fresh DB per test; no cleanup required |
| **Loose Coupling** | ✅ Excellent | Package DAG prevents cycles; interface boundaries enable mocking |

**Reliability Notes:**
- Fuzz tests specified for RFC 8785 canonicalization (`FuzzCanonicalJSON`)
- Race detector (`-race`) integration specified in CI
- Map iteration sorted via `slices.Sorted(maps.Keys())` per anti-pattern table

---

## Architecturally Significant Requirements (ASRs)

### High-Risk ASRs (Score ≥6)

| ASR ID | Requirement | Source | Probability | Impact | Score | Test Strategy |
|--------|-------------|--------|-------------|--------|-------|---------------|
| **ASR-1** | Deterministic replay produces identical results | NFR-2.1 | 3 (Likely) | 3 (Critical) | **9** | Replay invariant tests: run scenario twice, compare byte-identical traces |
| **ASR-2** | Sync rules fire exactly once per binding | NFR-2.2, CRITICAL-1 | 2 (Possible) | 3 (Critical) | **6** | Multi-binding scenario tests with firing count assertions |
| **ASR-3** | Sync engine terminates for all inputs | CRITICAL-3 | 2 (Possible) | 3 (Critical) | **6** | Cycle detection + quota tests with intentionally recursive rules |
| **ASR-4** | RFC 8785 UTF-16 key ordering correct | Architecture | 2 (Possible) | 3 (Critical) | **6** | Cross-language fixtures: Go, Python, JS produce identical canonical JSON |

### Medium-Risk ASRs (Score 3-4)

| ASR ID | Requirement | Source | Probability | Impact | Score | Test Strategy |
|--------|-------------|--------|-------------|--------|-------|---------------|
| **ASR-5** | Flow token scoping prevents cross-request pollution | NFR-2.3 | 2 (Possible) | 2 (Degraded) | **4** | Concurrent flow tests with assertions on isolation |
| **ASR-6** | Query IR compiles to parameterized SQL only | HIGH-3 | 1 (Unlikely) | 3 (Critical) | **3** | SQL compilation tests asserting no string interpolation |
| **ASR-7** | Provenance edges enable "why" queries | NFR-1.3 | 1 (Unlikely) | 3 (Critical) | **3** | Provenance traversal tests from effect to cause |
| **ASR-8** | CUE specs validate against IR schema | FR-1.2 | 2 (Possible) | 2 (Degraded) | **4** | Invalid spec fixtures with expected validation errors |

### Low-Risk ASRs (Score 1-2)

| ASR ID | Requirement | Source | Probability | Impact | Score | Test Strategy |
|--------|-------------|--------|-------------|--------|-------|---------------|
| **ASR-9** | Error codes are actionable | NFR-4.1 | 1 (Unlikely) | 2 (Degraded) | **2** | Error message assertions in compilation tests |
| **ASR-10** | CLI commands produce correct output | CLI | 1 (Unlikely) | 1 (Minor) | **1** | testscript E2E tests for all commands |

---

## Test Levels Strategy

### Recommended Test Distribution

| Level | Percentage | Rationale |
|-------|------------|-----------|
| **Unit** | 60% | Core logic (IR canonicalization, hashing, cycle detection) is highly testable in isolation |
| **Integration** | 30% | Store ↔ Engine ↔ QuerySQL integration critical for correctness |
| **E2E** | 10% | CLI commands + demo scenarios; expensive but essential for user-facing validation |

### Test Level Assignments

| Component | Primary Level | Secondary Level | Notes |
|-----------|---------------|-----------------|-------|
| `internal/ir/` | **Unit** | Fuzz | Table-driven for types; fuzz for canonicalization |
| `internal/compiler/` | **Unit** | Integration | Table-driven with CUE fixtures |
| `internal/store/` | **Integration** | Unit | Requires real SQLite; unit tests for SQL generation |
| `internal/engine/` | **Integration** | Unit | Determinism tests require store; unit for matching logic |
| `internal/queryir/` | **Unit** | - | Pure data structures and validation |
| `internal/querysql/` | **Unit** | - | Golden tests for SQL output |
| `internal/harness/` | **Integration** | Unit | Scenario execution requires full stack |
| `internal/cli/` | **E2E** | Unit | testscript for commands; unit for output formatting |

### Test Types per Level

**Unit Tests:**
- Table-driven tests with `testing` + `testify`
- Fuzz tests for canonicalization (`go test -fuzz`)
- Golden tests for SQL compilation (`goldie`)

**Integration Tests:**
- Replay invariant tests (run twice, compare)
- Multi-binding sync firing tests
- Cycle detection + quota enforcement tests
- Provenance traversal tests

**E2E Tests:**
- CLI command tests (`testscript`)
- Demo scenario validation (Cart checkout → Inventory reserve)
- Golden trace snapshots for canonical demo

---

## NFR Testing Approach

### NFR-1: Legibility

| NFR | Test Approach | Tools |
|-----|--------------|-------|
| NFR-1.1: Self-documenting specs | Validate CUE specs contain purpose, state, actions, operational principles | Compiler validation tests |
| NFR-1.2: No implicit behavior | Assert sync rules declare scope explicitly; no default behaviors | Compiler tests for scope validation |
| NFR-1.3: Queryable provenance | Provenance traversal tests from any completion to root invocation | Integration tests with `store.ReadProvenance()` |

### NFR-2: Correctness (Critical)

| NFR | Test Approach | Tools |
|-----|--------------|-------|
| NFR-2.1: Deterministic replay | **Replay Invariant Tests**: Execute scenario → capture trace → replay → compare byte-identical | `go-cmp`, goldie |
| NFR-2.2: Idempotent sync firing | Multi-binding tests with firing count assertions; duplicate completion tests | Integration tests with firing count queries |
| NFR-2.3: Flow isolation | Concurrent flow tests with cross-flow pollution assertions | Parallel integration tests |

**Deterministic Replay Test Pattern:**
```go
func TestReplayInvariant(t *testing.T) {
    // Run 1: Fresh database
    db1 := testutil.NewDB(t)
    engine1 := engine.New(db1, fixedFlowGen, fixedClock)
    trace1 := engine1.Run(scenario)

    // Run 2: Replay from empty
    db2 := testutil.NewDB(t)
    engine2 := engine.New(db2, fixedFlowGen, fixedClock)
    trace2 := engine2.Run(scenario)

    // Byte-identical comparison
    require.Equal(t, trace1, trace2)
}
```

### NFR-3: Extensibility

| NFR | Test Approach | Tools |
|-----|--------------|-------|
| NFR-3.1: Versioned IR | Version marker tests on all records | Integration tests checking `engine_version`, `ir_version`, `spec_hash` |
| NFR-3.2: Query abstraction | QueryIR portable fragment validation tests; SQL compilation tests | Unit tests for QueryIR validation |
| NFR-3.3: Surface format abstraction | Compiler produces stable IR regardless of CUE formatting | Golden tests for IR output |

### NFR-4: Developer Experience

| NFR | Test Approach | Tools |
|-----|--------------|-------|
| NFR-4.1: Actionable errors | Error message assertions with specific code and context | Compiler tests with invalid fixtures |
| NFR-4.2: Traceable evaluation | Log output tests with correlation key assertions | Integration tests capturing slog output |
| NFR-4.3: Trace diff output | Golden comparison tests with `-update` flag workflow | goldie + testscript |

---

## Test Environment Requirements

### Local Development

| Requirement | Solution |
|-------------|----------|
| **Go Version** | 1.25 (per architecture) |
| **CGO** | Required for SQLite (mattn/go-sqlite3); optional with modernc.org/sqlite |
| **Database** | SQLite in-memory (`:memory:`) for unit/integration tests |
| **Fixtures** | `testdata/fixtures/` - CUE specs, RFC 8785 JSON, IR validation |
| **Golden Files** | `testdata/golden/` - updated with `-update` flag |

### CI Environment

| Requirement | Solution |
|-------------|----------|
| **OS Matrix** | Linux (primary), macOS (secondary) |
| **Go Version** | 1.25 |
| **CGO** | Enabled for SQLite |
| **Race Detector** | Required (`go test -race`) |
| **Fuzz Testing** | 30s minimum (`go test -fuzz -fuzztime=30s`) |
| **Coverage** | Target 80% for `internal/` packages |

### Test Data Requirements

| Data Type | Location | Purpose |
|-----------|----------|---------|
| **Valid CUE Specs** | `testdata/fixtures/concepts/valid_*.cue` | Compilation success tests |
| **Invalid CUE Specs** | `testdata/fixtures/concepts/invalid_*.cue` | Validation error tests |
| **RFC 8785 Fixtures** | `testdata/fixtures/rfc8785/*.json` | Cross-language canonicalization |
| **IR Fixtures** | `testdata/fixtures/ir/*.json` | IR validation tests |
| **Scenarios** | `testdata/scenarios/*.yaml` | Conformance harness tests |
| **Golden Traces** | `testdata/golden/engine/*.golden` | Replay comparison |
| **E2E Scripts** | `testdata/e2e/*.txtar` | CLI command tests |

---

## Testability Concerns

### No Blockers Identified

The architecture has been designed with testability as a first-class concern. Key enablers:

1. **Injectable Dependencies**: All non-determinism sources (clock, random, flow tokens) are injectable
2. **Single-Writer Loop**: Deterministic scheduling eliminates race conditions in core path
3. **Content-Addressed IDs**: Same inputs = same outputs, enabling byte-identical comparison
4. **Append-Only Log**: Full audit trail available for assertion
5. **Interface Boundaries**: `Store`, `QueryCompiler`, `FlowTokenGenerator`, `Clock` enable mocking

### Concerns to Monitor

| Concern | Risk | Mitigation |
|---------|------|------------|
| **RFC 8785 UTF-16 Ordering** | Medium | Cross-language fixture tests; document edge cases |
| **SQLite CGO Dependency** | Low | Provide pure-Go alternative (modernc.org/sqlite) for restricted environments |
| **Fuzz Test Coverage** | Low | Minimum 30s fuzz time in CI; extend for critical paths |
| **Golden File Drift** | Low | Clear `-update` workflow; diff review in PRs |

---

## Recommendations for Sprint 0

### Test Infrastructure Setup

| Task | Priority | Rationale |
|------|----------|-----------|
| **Create `internal/testutil/` package** | P0 | Shared helpers: `NewDB()`, `FixedFlowGen`, `FixedClock` |
| **Set up goldie configuration** | P0 | Golden testing for IR compilation and SQL generation |
| **Create RFC 8785 cross-language fixtures** | P0 | Validate canonicalization before any hashing |
| **Configure CI pipeline** | P0 | Race detector, fuzz testing, coverage reporting |
| **Create scenario YAML schema** | P1 | Standardize conformance harness input format |

### Test Implementation Order

1. **`internal/ir/` tests first** - Foundation for everything; no dependencies
2. **RFC 8785 canonicalization fuzz tests** - Critical for correctness
3. **Content-addressed hash tests** - Validate domain separation
4. **`internal/store/` integration tests** - Verify schema and queries
5. **Replay invariant tests** - Core correctness property

### CI Configuration

```yaml
# .github/workflows/ci.yml
name: CI
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'

      - name: Vet
        run: go vet ./...

      - name: Lint
        run: golangci-lint run

      - name: Unit Tests
        run: go test ./...

      - name: Race Tests
        run: go test -race ./...

      - name: Fuzz Tests
        run: |
          go test -fuzz=FuzzCanonicalJSON -fuzztime=30s ./internal/ir
          go test -fuzz=FuzzContentHash -fuzztime=30s ./internal/ir

      - name: Coverage
        run: go test -coverprofile=coverage.out ./...

      - name: Coverage Check
        run: |
          coverage=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//')
          if (( $(echo "$coverage < 80" | bc -l) )); then
            echo "Coverage $coverage% is below 80% threshold"
            exit 1
          fi
```

---

## Quality Gate Criteria

### Pre-Implementation Gate (Sprint 0)

- [ ] Test infrastructure (`testutil/`) created
- [ ] Goldie configured for golden testing
- [ ] RFC 8785 fixtures created and passing
- [ ] CI pipeline running with race detector and fuzz

### Per-Epic Gate

- [ ] All unit tests pass (100%)
- [ ] Integration tests pass (100%)
- [ ] No high-risk ASRs (score ≥6) unmitigated
- [ ] Coverage ≥80% for new code
- [ ] Golden files reviewed and committed

### Pre-Release Gate

- [ ] All P0 tests pass (100%)
- [ ] Replay invariant tests pass
- [ ] Demo scenario (cart checkout) passes end-to-end
- [ ] No known determinism issues
- [ ] All golden traces committed

---

## Summary

**Testability Assessment:** PASS (Excellent)

| Dimension | Rating | Key Enabler |
|-----------|--------|-------------|
| Controllability | ✅ Excellent | Injectable dependencies, embeddable SQLite |
| Observability | ✅ Excellent | Append-only log, provenance edges, structured logging |
| Reliability | ✅ Excellent | Deterministic design, content-addressed IDs, sorted iteration |

**High-Priority Test Areas:**

1. **ASR-1: Deterministic Replay** (Score 9) - Replay invariant tests
2. **ASR-2: Idempotent Sync Firing** (Score 6) - Multi-binding tests
3. **ASR-3: Sync Termination** (Score 6) - Cycle detection tests
4. **ASR-4: RFC 8785 Correctness** (Score 6) - Cross-language fixtures

**Test Distribution:** 60% Unit / 30% Integration / 10% E2E

**Next Steps:**
1. Create test infrastructure in Sprint 0
2. Implement RFC 8785 fixtures before any IR code
3. Establish replay invariant test pattern early
4. Run `implementation-readiness` workflow to validate alignment

---

**Generated by:** BMad TEA Agent - Test Architect Module
**Workflow:** `.bmad/bmm/workflows/testarch/test-design`
**Mode:** System-Level (Phase 3)
