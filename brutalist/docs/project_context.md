---
project_name: 'NYSM'
user_name: 'Tyler'
date: '2025-12-12'
status: 'complete'
sections_completed: ['technology_stack', 'determinism_rules', 'naming_conventions', 'package_architecture', 'testing_rules']
---

# Project Context for AI Agents

_This file contains critical rules and patterns that AI agents must follow when implementing code in this project. Focus on unobvious details that agents might otherwise miss._

---

## Technology Stack & Versions

| Component | Package | Version | Notes |
|-----------|---------|---------|-------|
| **Language** | Go | 1.25 | Use `slices`, `maps` stdlib packages |
| **CUE SDK** | `cuelang.org/go` | v0.15.1 | Native Go API, not CLI subprocess |
| **SQLite** | `github.com/mattn/go-sqlite3` | v1.14.32 | Requires CGO_ENABLED=1 |
| **CLI** | `github.com/spf13/cobra` | v1.10.2 | - |
| **UUID** | `github.com/google/uuid` | v1.6.0 | UUIDv7 for flow tokens |
| **Testing** | `github.com/stretchr/testify` | v1.11.1 | - |
| **Diffs** | `github.com/google/go-cmp` | v0.7.0 | For struct comparison |
| **Golden** | `github.com/sebdah/goldie/v2` | v2.8.0 | Snapshot testing |

**Version Constraints:**
- CUE SDK and Go versions must match (CUE is Go-native)
- SQLite requires CGO; use `modernc.org/sqlite` v1.40.1 for pure-Go cross-compile

---

## Critical Determinism Rules

**NYSM's core guarantee is deterministic replay. These rules are MANDATORY for correctness.**

### Never Use (Expanded)

| Forbidden | Use Instead | Why |
|-----------|-------------|-----|
| `time.Now()`, `time.Since()`, `time.After()` | Injected `Clock` interface | Wall/monotonic time breaks replay |
| `time.Local`, `time.Format()` | UTC only, canonical format | Locale/trimming variance |
| `rand.*`, `crypto/rand` | Injected `RandomSource` | Non-deterministic |
| `uuid.New()`, `uuid.NewV7()` | Injected `FlowTokenGenerator` | Random component |
| `for k := range map` | `slices.Sorted(maps.Keys(m))` | Iteration order varies |
| `reflect.Value.MapKeys()` | Sorted wrapper | Reflect has same problem |
| `float64` in IR | `int64` (micros, banker's rounding) | Formatting drift |
| `map[string]any` | Constrained `IRValue` types | Admits forbidden types |
| `encoding/json` for hashes | `ir.MarshalCanonical()` | UTF-8 order ≠ RFC 8785 |
| `CURRENT_TIMESTAMP`, `random()` | `seq INTEGER`, recorded values | SQLite non-determinism |
| Auto-increment IDs | Content-addressed hash | Identity must be stable |
| `SELECT *` | Explicit column list | Schema evolution safety |
| Goroutines in engine path | Single-writer loop | Race conditions |
| Timers, tickers, `time.Sleep` | Event-driven scheduling | Timing non-determinism |
| Context cancellation affecting output | Advisory context only | Cancel must not change results |

### SQLite Query Determinism

```sql
-- WRONG: Non-deterministic ordering
SELECT * FROM completions WHERE flow_token = ?

-- CORRECT: Deterministic collation + tiebreaker
SELECT id, flow_token, action_uri, result
FROM completions
WHERE flow_token = ?
ORDER BY seq ASC, id ASC COLLATE BINARY
```

**Rules:**
- ALL queries MUST have `ORDER BY` with unique tiebreaker
- Use `COLLATE BINARY` for text ordering
- Avoid `GROUP BY`/`DISTINCT` without explicit output ordering
- Never use `random()`, `strftime('now')`, `julianday('now')`

### Canonical JSON (RFC 8785)

```go
// ONLY use the project canonicalizer - NEVER encoding/json for hashes
canonical, err := ir.MarshalCanonical(record)
id := sha256([]byte("nysm/invocation/v1\x00"), canonical) // Bytes, not string concat
```

**Canonicalizer Requirements:**
- UTF-16 code unit key ordering (not UTF-8 bytes)
- No `-0`, no exponent notation, no trailing `.0`
- No `NaN`/`Inf` (floats forbidden anyway)
- Canonicalizer version pinned in IR schema

### Determinism Checklist (Every PR)

- [ ] No `time.*` in engine code path (except injected Clock)
- [ ] No `rand.*` or `crypto/rand` in engine path
- [ ] No `uuid.New*()` - use FlowTokenGenerator
- [ ] No unsorted map iteration (including nested maps)
- [ ] No `reflect.Value.MapKeys()` without sorting
- [ ] All SQL queries have `ORDER BY` + tiebreaker + `COLLATE BINARY`
- [ ] No `SELECT *` - explicit columns only
- [ ] All IDs are content-addressed via `ir.MarshalCanonical()`
- [ ] No goroutines/timers/tickers in sync evaluation
- [ ] Context used for cancellation doesn't affect outputs
- [ ] No map-printed strings in persisted events/errors

---

## Naming Conventions

### Code Naming

| Element | Convention | Example |
|---------|------------|---------|
| Packages | `lowercase`, single word | `engine`, `store`, `ir` |
| Exported types | `PascalCase` | `ConceptSpec`, `SyncRule` |
| Exported functions | Package-qualified verb | `compiler.Compile()`, `store.Write()` |
| Internal functions | `camelCase` | `parseActionSig`, `bindWhereClause` |
| Error variables | `Err` prefix | `ErrInvalidSpec`, `ErrCyclicSync` |
| Error types | `Error` suffix | `ValidationError` |
| Interfaces | `-er` suffix or descriptive | `FlowTokenGenerator`, `QueryCompiler` |

### SQLite Naming

| Element | Convention | Example |
|---------|------------|---------|
| Tables | `snake_case`, plural | `invocations`, `sync_firings` |
| Columns | `snake_case` | `flow_token`, `binding_hash` |
| Foreign keys | `{table_singular}_id` | `completion_id` |
| Indexes | `idx_{table}_{columns}` | `idx_completions_flow_token` |
| Logical clock | `seq` | Never `created_at` or `timestamp` |

### IR/JSON Naming

| Element | Convention | Example |
|---------|------------|---------|
| JSON fields | `snake_case` | `flow_token`, `output_case` |
| Go struct tags | Match JSON exactly | `json:"flow_token"` |
| Never | `camelCase` in JSON | ❌ `flowToken` |

### URI Scheme

```
nysm://{namespace}/{type}/{name}@{semver}

Examples:
nysm://myapp/concept/Cart@1.0.0
nysm://myapp/action/Cart/addItem@1.0.0
nysm://myapp/sync/cart-inventory@1.0.0
```

### CLI Flags

| Type | Convention | Example |
|------|------------|---------|
| Long flags | `kebab-case` | `--flow-token`, `--output-format` |
| Short flags | Single letter | `-v`, `-o`, `-q` |
| Boolean | Positive form | `--verbose` not `--no-quiet` |

---

## Package Architecture

### Dependency DAG (Strictly Enforced)

```
ir → compiler/store/queryir → engine → cli
```

**Rules:**
1. `ir/` imports ONLY stdlib - it's the leaf dependency
2. `store/` is the ONLY package importing `database/sql`
3. `compiler/` imports `ir/` and `cuelang.org/go/*` only
4. `engine/` imports `ir/`, `store/`, `queryir/`, `querysql/`
5. `cli/` imports all internal packages
6. NO import cycles - use interfaces at boundaries

### Interface Boundaries

```go
// store/store.go - Storage abstraction
type Store interface {
    WriteInvocation(ctx context.Context, inv ir.Invocation) error
    WriteCompletion(ctx context.Context, comp ir.Completion) error
    WriteSyncFiring(ctx context.Context, firing ir.SyncFiring) error
}

// queryir/types.go - Query compilation abstraction
type Compiler interface {
    Compile(q Query) (sql string, args []any, err error)
}

// engine/flow.go - Flow token injection
type FlowTokenGenerator interface {
    Generate() string
}

// engine/engine.go - Clock injection
type Clock interface {
    Now() int64  // Returns logical sequence, NOT time
}
```

### Package Responsibilities

| Package | Does | Does NOT |
|---------|------|----------|
| `ir/` | Define types, canonical JSON, hashing | Import other internal packages |
| `compiler/` | Parse CUE, validate, emit IR | Execute anything |
| `engine/` | Run sync loop, match rules, invoke | Touch database directly |
| `store/` | All SQLite operations | Business logic |
| `queryir/` | Define query AST | Compile to SQL |
| `querysql/` | Compile QueryIR to SQL | Define query types |
| `harness/` | Run conformance tests | Production code paths |

---

## Testing Rules

### Test Organization

| Type | Location | Command |
|------|----------|---------|
| Unit tests | `{pkg}/*_test.go` | `go test ./internal/{pkg}` |
| Golden tests | `testdata/{pkg}/*.golden` | Auto-discovered by goldie |
| Fuzz tests | `{pkg}/fuzz_test.go` | `go test -fuzz=FuzzX` |
| Integration | `{pkg}/integration_test.go` | `go test -tags=integration` |
| E2E | `testdata/e2e/*.txtar` | `testscript` |

### Table-Driven Tests (Required)

```go
func TestCompile(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    *ir.ConceptSpec
        wantErr string  // Empty = expect success
    }{
        {name: "valid", input: `...`, want: &ir.ConceptSpec{...}},
        {name: "missing purpose", input: `...`, wantErr: "missing required field"},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := compiler.Compile(tt.input)
            if tt.wantErr != "" {
                require.ErrorContains(t, err, tt.wantErr)
                return
            }
            require.NoError(t, err)
            assert.Equal(t, tt.want, got)
        })
    }
}
```

### Golden File Testing

```go
// Use goldie for snapshot comparison
func TestCanonicalJSON(t *testing.T) {
    g := goldie.New(t)
    result, _ := ir.MarshalCanonical(input)
    g.Assert(t, "canonical_output", result)
}
```

**Update golden files:** `go test ./... -update`

### Determinism Invariant Tests

```go
// REQUIRED: Verify replay produces identical results
func TestReplayDeterminism(t *testing.T) {
    // Run scenario twice with same inputs
    result1 := runScenario(inputs)
    result2 := runScenario(inputs)

    // Must be byte-identical
    assert.Equal(t, result1, result2, "replay must be deterministic")
}
```

### Test Helpers Location

All shared test helpers go in `internal/testutil/`:
- `clock.go` - Deterministic clock
- `flow.go` - Fixed flow token generator
- `db.go` - Test database setup/teardown

---

