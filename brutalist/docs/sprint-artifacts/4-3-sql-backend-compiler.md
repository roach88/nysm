# Story 4.3: SQL Backend Compiler

Status: done

## Story

As a **developer executing queries**,
I want **QueryIR compiled to parameterized SQL**,
So that **where-clauses execute against SQLite**.

## Acceptance Criteria

1. **SQLCompiler defined in `internal/querysql/compile.go`**
   ```go
   type SQLCompiler struct{}

   func (c *SQLCompiler) Compile(q queryir.Query) (string, []any, error)
   ```

2. **ALL queries MUST have ORDER BY with deterministic tiebreaker per CP-4**
   ```sql
   -- CORRECT: Every query gets stable ordering
   SELECT id, name, quantity FROM inventory
   WHERE category = ?
   ORDER BY id ASC COLLATE BINARY

   -- WRONG: Missing ORDER BY (non-deterministic)
   SELECT id, name, quantity FROM inventory
   WHERE category = ?
   ```

3. **String values NEVER interpolated - always use ? parameters**
   ```go
   // CORRECT: Parameterized query
   sql := "SELECT * FROM inventory WHERE category = ?"
   params := []any{category}

   // WRONG: String interpolation (SQL injection risk!)
   sql := fmt.Sprintf("SELECT * FROM inventory WHERE category = '%s'", category)
   ```

4. **COLLATE BINARY used for text ordering**
   - All ORDER BY clauses on text fields use COLLATE BINARY
   - Ensures stable lexicographic ordering across SQLite versions
   - Prevents locale-dependent collation issues

5. **Compile handles all QueryIR node types**
   - `queryir.Select` → SELECT statement with WHERE and ORDER BY
   - `queryir.Join` → INNER JOIN statement
   - `queryir.Equals` → `field = ?` with parameter
   - `queryir.And` → Conjunction of predicates with `AND`
   - `queryir.BoundEquals` → `field = ?` with bound variable value

6. **stableOrderKey() ensures ORDER BY on every query**
   - Function: `func (c *SQLCompiler) stableOrderKey(q queryir.Query) string`
   - For Select: Returns primary key or unique identifier column
   - Always includes COLLATE BINARY for text columns
   - Multiple columns allowed for composite ordering (e.g., "seq ASC, id ASC COLLATE BINARY")

7. **Comprehensive tests in `internal/querysql/compile_test.go`**
   - Test: Simple select compiles to SQL with ORDER BY
   - Test: Filter compiles to WHERE clause with parameters
   - Test: Join compiles to INNER JOIN
   - Test: And predicate compiles to multiple WHERE conditions
   - Test: BoundEquals uses bound variable value as parameter
   - Test: All SQL output includes ORDER BY (no exceptions)
   - Test: String values never interpolated (golden test verification)

## Quick Reference

| Pattern | Key Rule |
|---------|----------|
| **FR-2.3** | Compile where-clause to SQL |
| **CP-4** | ALL queries MUST have ORDER BY with deterministic tiebreaker |
| **HIGH-3** | Parameterized queries only, no string interpolation |
| **COLLATE BINARY** | Text ordering must be deterministic |

## Tasks / Subtasks

- [ ] Task 1: Define SQLCompiler struct (AC: #1)
  - [ ] 1.1 Create `internal/querysql/compile.go`
  - [ ] 1.2 Define SQLCompiler struct
  - [ ] 1.3 Implement Compile(queryir.Query) signature

- [ ] Task 2: Implement Select compilation (AC: #2, #5)
  - [ ] 2.1 Compile SELECT clause from bindings map
  - [ ] 2.2 Compile FROM clause from table name
  - [ ] 2.3 Compile WHERE clause from filter predicate
  - [ ] 2.4 Collect parameters from predicates
  - [ ] 2.5 ALWAYS append ORDER BY from stableOrderKey()

- [ ] Task 3: Implement predicate compilation (AC: #5)
  - [ ] 3.1 Compile Equals to `field = ?` with parameter
  - [ ] 3.2 Compile And to conjunction with `AND`
  - [ ] 3.3 Compile BoundEquals to `field = ?` with bound value
  - [ ] 3.4 Recursive compilation for nested predicates

- [ ] Task 4: Implement Join compilation (AC: #5)
  - [ ] 4.1 Compile INNER JOIN between left and right queries
  - [ ] 4.2 Compile ON predicate
  - [ ] 4.3 Ensure ORDER BY on final result

- [ ] Task 5: Implement stableOrderKey (AC: #6)
  - [ ] 5.1 For Select: return primary key column
  - [ ] 5.2 Add COLLATE BINARY for text columns
  - [ ] 5.3 Support composite keys (e.g., "seq, id")
  - [ ] 5.4 Default to "id ASC COLLATE BINARY" if no explicit key

- [ ] Task 6: Ensure parameterized queries (AC: #3, #4)
  - [ ] 6.1 All values passed via []any parameters
  - [ ] 6.2 NO fmt.Sprintf or string interpolation
  - [ ] 6.3 COLLATE BINARY on all text ORDER BY clauses

- [ ] Task 7: Write comprehensive tests
  - [ ] 7.1 Test simple select with filter
  - [ ] 7.2 Test select with multiple predicates (And)
  - [ ] 7.3 Test join compilation
  - [ ] 7.4 Test ORDER BY always present
  - [ ] 7.5 Test parameters never interpolated (golden test)
  - [ ] 7.6 Test COLLATE BINARY on text ordering
  - [ ] 7.7 Test empty filter (no WHERE, but still ORDER BY)

## Dev Notes

### Critical Implementation Details

**SQLCompiler Structure**

```go
// internal/querysql/compile.go

package querysql

import (
    "fmt"
    "strings"

    "github.com/tyler/nysm/internal/ir"
    "github.com/tyler/nysm/internal/queryir"
)

// SQLCompiler compiles QueryIR to parameterized SQL for SQLite.
// CRITICAL: ALL queries include ORDER BY per CP-4.
type SQLCompiler struct{}

// Compile converts a QueryIR query to parameterized SQL.
// Returns (sql, params, error) tuple.
// MANDATORY: Every query includes ORDER BY with deterministic tiebreaker.
func (c *SQLCompiler) Compile(q queryir.Query) (string, []any, error) {
    switch query := q.(type) {
    case queryir.Select:
        return c.compileSelect(query)
    case queryir.Join:
        return c.compileJoin(query)
    default:
        return "", nil, fmt.Errorf("unsupported query type: %T", q)
    }
}
```

### CP-4: Deterministic Query Ordering

**CRITICAL:** ALL queries MUST include `ORDER BY` with a deterministic tiebreaker. This is NON-NEGOTIABLE.

```go
// stableOrderKey returns the ORDER BY clause for a query.
// MANDATORY: Every query MUST call this function.
// For Select queries, defaults to "id ASC COLLATE BINARY" if no explicit ordering.
func (c *SQLCompiler) stableOrderKey(q queryir.Query) string {
    switch query := q.(type) {
    case queryir.Select:
        // Default to id as primary key
        // COLLATE BINARY ensures deterministic text ordering
        return "id ASC COLLATE BINARY"
    default:
        return "id ASC COLLATE BINARY"
    }
}
```

**Why ORDER BY is mandatory:**
- SQLite does NOT guarantee result ordering without ORDER BY
- Different SQLite versions may return rows in different orders
- Query plan changes can change result order
- `id ASC COLLATE BINARY` provides stable lexicographic ordering
- Breaks deterministic replay if omitted
- Makes tests flaky and non-reproducible

### Select Compilation

```go
// compileSelect compiles a queryir.Select to SQL.
// MANDATORY: Includes ORDER BY per CP-4.
func (c *SQLCompiler) compileSelect(q queryir.Select) (string, []any, error) {
    // Build SELECT clause from bindings
    selectClause := c.compileBindings(q.Bindings)

    // Build FROM clause
    fromClause := q.From

    // Build WHERE clause and collect parameters
    var whereClause string
    var params []any
    if q.Filter != nil {
        filterSQL, filterParams, err := c.compilePredicate(q.Filter)
        if err != nil {
            return "", nil, fmt.Errorf("compile filter: %w", err)
        }
        whereClause = " WHERE " + filterSQL
        params = filterParams
    }

    // MANDATORY: Always add ORDER BY per CP-4
    orderByClause := " ORDER BY " + c.stableOrderKey(q)

    // Assemble SQL
    sql := fmt.Sprintf("SELECT %s FROM %s%s%s",
        selectClause,
        fromClause,
        whereClause,
        orderByClause)

    return sql, params, nil
}

// compileBindings converts bindings map to SELECT column list.
// Example: {"item": "item_name", "qty": "quantity"} → "item_name AS item, quantity AS qty"
func (c *SQLCompiler) compileBindings(bindings map[string]string) string {
    if len(bindings) == 0 {
        return "*"
    }

    // Sort keys for deterministic output (testing)
    keys := make([]string, 0, len(bindings))
    for k := range bindings {
        keys = append(keys, k)
    }
    sort.Strings(keys) // Simple sort OK here (for output determinism, not RFC 8785)

    var parts []string
    for _, boundVar := range keys {
        fieldName := bindings[boundVar]
        if boundVar == fieldName {
            // No alias needed
            parts = append(parts, fieldName)
        } else {
            // Alias: field AS bound_var
            parts = append(parts, fmt.Sprintf("%s AS %s", fieldName, boundVar))
        }
    }

    return strings.Join(parts, ", ")
}
```

### Predicate Compilation

```go
// compilePredicate compiles a queryir.Predicate to SQL WHERE clause.
// Returns (sql, params, error).
// CRITICAL: Values NEVER interpolated - always use ? placeholders.
func (c *SQLCompiler) compilePredicate(p queryir.Predicate) (string, []any, error) {
    switch pred := p.(type) {
    case queryir.Equals:
        return c.compileEquals(pred)
    case queryir.And:
        return c.compileAnd(pred)
    case queryir.BoundEquals:
        return c.compileBoundEquals(pred)
    default:
        return "", nil, fmt.Errorf("unsupported predicate type: %T", p)
    }
}

// compileEquals compiles an Equals predicate to "field = ?".
// CRITICAL: Value is NEVER interpolated - always parameterized.
func (c *SQLCompiler) compileEquals(eq queryir.Equals) (string, []any, error) {
    // Convert IRValue to Go native type for SQL parameter
    param, err := irValueToParam(eq.Value)
    if err != nil {
        return "", nil, fmt.Errorf("convert value: %w", err)
    }

    sql := fmt.Sprintf("%s = ?", eq.Field)
    params := []any{param}

    return sql, params, nil
}

// compileAnd compiles an And predicate to conjunction with AND.
func (c *SQLCompiler) compileAnd(and queryir.And) (string, []any, error) {
    if len(and.Predicates) == 0 {
        return "1 = 1", nil, nil // Always true
    }

    var sqlParts []string
    var allParams []any

    for _, pred := range and.Predicates {
        sql, params, err := c.compilePredicate(pred)
        if err != nil {
            return "", nil, err
        }
        sqlParts = append(sqlParts, sql)
        allParams = append(allParams, params...)
    }

    // Join with AND
    sql := strings.Join(sqlParts, " AND ")

    return sql, allParams, nil
}

// compileBoundEquals compiles a BoundEquals predicate.
// BoundEquals references a variable from when-clause bindings.
// The bound value is passed as a parameter (NEVER interpolated).
func (c *SQLCompiler) compileBoundEquals(beq queryir.BoundEquals) (string, []any, error) {
    // NOTE: The actual bound value will be provided by the engine at execution time.
    // For now, we compile to "field = ?" and the engine will supply the parameter.
    // This is a placeholder - Story 4.4 will integrate with engine execution.

    sql := fmt.Sprintf("%s = ?", beq.Field)
    params := []any{} // Engine will provide bound value

    return sql, params, nil
}

// irValueToParam converts an ir.IRValue to a Go native type for SQL parameter.
func irValueToParam(v ir.IRValue) (any, error) {
    switch val := v.(type) {
    case ir.IRString:
        return string(val), nil
    case ir.IRInt:
        return int64(val), nil
    case ir.IRBool:
        return bool(val), nil
    default:
        return nil, fmt.Errorf("unsupported IRValue type for SQL parameter: %T", v)
    }
}
```

### Join Compilation

```go
// compileJoin compiles a queryir.Join to SQL INNER JOIN.
func (c *SQLCompiler) compileJoin(j queryir.Join) (string, []any, error) {
    // Compile left and right queries
    // NOTE: For MVP, we only support simple table references, not nested queries
    // This is a simplified implementation - full subquery support deferred

    leftTable, ok := j.Left.(queryir.Select)
    if !ok {
        return "", nil, fmt.Errorf("join left must be Select for MVP")
    }

    rightTable, ok := j.Right.(queryir.Select)
    if !ok {
        return "", nil, fmt.Errorf("join right must be Select for MVP")
    }

    // Compile ON predicate
    onSQL, onParams, err := c.compilePredicate(j.On)
    if err != nil {
        return "", nil, fmt.Errorf("compile join ON: %w", err)
    }

    // Build JOIN SQL
    // Simplified: assumes left and right are simple table selects
    sql := fmt.Sprintf("%s INNER JOIN %s ON %s",
        leftTable.From,
        rightTable.From,
        onSQL)

    // MANDATORY: Add ORDER BY
    // For joins, order by first table's primary key
    sql += " ORDER BY " + leftTable.From + ".id ASC COLLATE BINARY"

    return sql, onParams, nil
}
```

### HIGH-3: Parameterized Queries

**CRITICAL:** NEVER use string interpolation or `fmt.Sprintf` to inject values into SQL.

```go
// CORRECT: Parameterized query
func (c *SQLCompiler) compileEquals(eq queryir.Equals) (string, []any, error) {
    param, _ := irValueToParam(eq.Value)
    sql := fmt.Sprintf("%s = ?", eq.Field) // Only interpolate FIELD NAME, not value
    params := []any{param}                 // Value goes in parameters
    return sql, params, nil
}

// WRONG: String interpolation (SQL injection risk!)
func (c *SQLCompiler) compileEquals(eq queryir.Equals) (string, []any, error) {
    sql := fmt.Sprintf("%s = '%s'", eq.Field, eq.Value) // ❌ NEVER DO THIS
    return sql, nil, nil
}
```

**Why parameterized queries:**
- Prevents SQL injection attacks
- SQLite can compile prepared statements once and reuse them
- Type safety - SQLite handles escaping and quoting
- Enforced by HIGH-3 security requirement

### COLLATE BINARY for Text Ordering

```go
// stableOrderKey always uses COLLATE BINARY for text columns
func (c *SQLCompiler) stableOrderKey(q queryir.Query) string {
    // CRITICAL: COLLATE BINARY ensures deterministic text ordering
    // Without it, SQLite may use locale-dependent collation
    return "id ASC COLLATE BINARY"
}

// For composite keys:
func (c *SQLCompiler) stableOrderKeyComposite(columns []string) string {
    var parts []string
    for _, col := range columns {
        parts = append(parts, col + " ASC COLLATE BINARY")
    }
    return strings.Join(parts, ", ")
}
```

**Why COLLATE BINARY:**
- Default collation is BINARY, but explicit is better
- Prevents locale-dependent sorting (e.g., case-insensitive)
- Ensures same results across SQLite versions
- Matches content-addressed hash ordering

### Test Examples

**Test: Simple Select with Filter**

```go
// internal/querysql/compile_test.go

package querysql

import (
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "github.com/tyler/nysm/internal/ir"
    "github.com/tyler/nysm/internal/queryir"
)

func TestCompile_SimpleSelect(t *testing.T) {
    compiler := &SQLCompiler{}

    query := queryir.Select{
        From: "inventory",
        Bindings: map[string]string{
            "item": "item_name",
            "qty":  "quantity",
        },
        Filter: queryir.Equals{
            Field: "category",
            Value: ir.IRString("widgets"),
        },
    }

    sql, params, err := compiler.Compile(query)
    require.NoError(t, err)

    // Verify SQL structure
    assert.Contains(t, sql, "SELECT")
    assert.Contains(t, sql, "FROM inventory")
    assert.Contains(t, sql, "WHERE category = ?")
    assert.Contains(t, sql, "ORDER BY") // MANDATORY

    // Verify parameterized query (no interpolation)
    assert.NotContains(t, sql, "widgets") // Value NOT in SQL
    assert.Equal(t, []any{"widgets"}, params) // Value in parameters

    // Verify COLLATE BINARY
    assert.Contains(t, sql, "COLLATE BINARY")
}
```

**Test: ORDER BY Always Present**

```go
func TestCompile_OrderByMandatory(t *testing.T) {
    compiler := &SQLCompiler{}

    testCases := []struct {
        name  string
        query queryir.Query
    }{
        {
            name: "select with filter",
            query: queryir.Select{
                From:   "inventory",
                Filter: queryir.Equals{Field: "category", Value: ir.IRString("widgets")},
            },
        },
        {
            name: "select without filter",
            query: queryir.Select{
                From: "inventory",
            },
        },
        {
            name: "select with And predicate",
            query: queryir.Select{
                From: "inventory",
                Filter: queryir.And{
                    Predicates: []queryir.Predicate{
                        queryir.Equals{Field: "category", Value: ir.IRString("widgets")},
                        queryir.Equals{Field: "in_stock", Value: ir.IRBool(true)},
                    },
                },
            },
        },
    }

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            sql, _, err := compiler.Compile(tc.query)
            require.NoError(t, err)

            // CRITICAL: Every query MUST have ORDER BY
            assert.Contains(t, sql, "ORDER BY",
                "Query MUST include ORDER BY per CP-4: %s", sql)
            assert.Contains(t, sql, "COLLATE BINARY",
                "ORDER BY MUST use COLLATE BINARY: %s", sql)
        })
    }
}
```

**Test: String Values Never Interpolated**

```go
func TestCompile_NoStringInterpolation(t *testing.T) {
    compiler := &SQLCompiler{}

    // Use a value that would be dangerous if interpolated
    dangerousValue := "'; DROP TABLE inventory; --"

    query := queryir.Select{
        From: "inventory",
        Filter: queryir.Equals{
            Field: "category",
            Value: ir.IRString(dangerousValue),
        },
    }

    sql, params, err := compiler.Compile(query)
    require.NoError(t, err)

    // Verify SQL does NOT contain the dangerous value
    assert.NotContains(t, sql, dangerousValue,
        "Value MUST NOT be interpolated into SQL (SQL injection risk)")

    // Verify value is in parameters
    assert.Contains(t, params, dangerousValue,
        "Value MUST be in parameters array")

    // Verify SQL uses ? placeholder
    assert.Contains(t, sql, "category = ?",
        "SQL MUST use ? placeholder, not interpolated value")
}
```

**Test: And Predicate Compilation**

```go
func TestCompile_AndPredicate(t *testing.T) {
    compiler := &SQLCompiler{}

    query := queryir.Select{
        From: "inventory",
        Filter: queryir.And{
            Predicates: []queryir.Predicate{
                queryir.Equals{Field: "category", Value: ir.IRString("widgets")},
                queryir.Equals{Field: "in_stock", Value: ir.IRBool(true)},
                queryir.Equals{Field: "quantity", Value: ir.IRInt(10)},
            },
        },
    }

    sql, params, err := compiler.Compile(query)
    require.NoError(t, err)

    // Verify WHERE clause with AND
    assert.Contains(t, sql, "WHERE")
    assert.Contains(t, sql, "category = ?")
    assert.Contains(t, sql, "AND")
    assert.Contains(t, sql, "in_stock = ?")
    assert.Contains(t, sql, "quantity = ?")

    // Verify parameters in order
    assert.Equal(t, []any{"widgets", true, int64(10)}, params)

    // Verify ORDER BY present
    assert.Contains(t, sql, "ORDER BY")
}
```

**Test: Join Compilation**

```go
func TestCompile_InnerJoin(t *testing.T) {
    compiler := &SQLCompiler{}

    query := queryir.Join{
        Left: queryir.Select{From: "orders"},
        Right: queryir.Select{From: "customers"},
        On: queryir.Equals{
            Field: "orders.customer_id",
            Value: ir.IRString("customers.id"),
        },
    }

    sql, params, err := compiler.Compile(query)
    require.NoError(t, err)

    // Verify INNER JOIN syntax
    assert.Contains(t, sql, "INNER JOIN")
    assert.Contains(t, sql, "orders")
    assert.Contains(t, sql, "customers")
    assert.Contains(t, sql, "ON")

    // Verify ORDER BY present (even on joins)
    assert.Contains(t, sql, "ORDER BY")
    assert.Contains(t, sql, "COLLATE BINARY")
}
```

**Test: Empty Filter (No WHERE, But Still ORDER BY)**

```go
func TestCompile_EmptyFilter(t *testing.T) {
    compiler := &SQLCompiler{}

    query := queryir.Select{
        From:   "inventory",
        Filter: nil, // No filter
    }

    sql, _, err := compiler.Compile(query)
    require.NoError(t, err)

    // Verify no WHERE clause
    assert.NotContains(t, sql, "WHERE")

    // Verify ORDER BY STILL present (mandatory)
    assert.Contains(t, sql, "ORDER BY",
        "ORDER BY MUST be present even without WHERE clause")
}
```

**Golden Test: SQL Output Verification**

```go
func TestCompile_GoldenSQL(t *testing.T) {
    compiler := &SQLCompiler{}

    testCases := []struct {
        name     string
        query    queryir.Query
        wantSQL  string
        wantParams []any
    }{
        {
            name: "simple select",
            query: queryir.Select{
                From: "inventory",
                Bindings: map[string]string{"item": "name"},
                Filter: queryir.Equals{Field: "category", Value: ir.IRString("widgets")},
            },
            wantSQL: "SELECT name AS item FROM inventory WHERE category = ? ORDER BY id ASC COLLATE BINARY",
            wantParams: []any{"widgets"},
        },
        {
            name: "select with And",
            query: queryir.Select{
                From: "inventory",
                Filter: queryir.And{
                    Predicates: []queryir.Predicate{
                        queryir.Equals{Field: "category", Value: ir.IRString("widgets")},
                        queryir.Equals{Field: "in_stock", Value: ir.IRBool(true)},
                    },
                },
            },
            wantSQL: "SELECT * FROM inventory WHERE category = ? AND in_stock = ? ORDER BY id ASC COLLATE BINARY",
            wantParams: []any{"widgets", true},
        },
    }

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            sql, params, err := compiler.Compile(tc.query)
            require.NoError(t, err)

            assert.Equal(t, tc.wantSQL, sql, "SQL mismatch")
            assert.Equal(t, tc.wantParams, params, "Parameters mismatch")
        })
    }
}
```

### Package Structure

```
internal/querysql/
├── compile.go          # SQLCompiler implementation
└── compile_test.go     # Comprehensive tests
```

**Exported vs Unexported:**
- `SQLCompiler` - exported (public API)
- `Compile(queryir.Query)` - exported method
- `compileSelect()`, `compilePredicate()`, etc. - unexported (internal helpers)
- `stableOrderKey()` - unexported (internal)

**Dependencies:**
- `internal/queryir` - Query and Predicate types
- `internal/ir` - IRValue types

### File List

Files to create:

1. `internal/querysql/compile.go` - SQLCompiler implementation
2. `internal/querysql/compile_test.go` - Comprehensive tests

Files to reference (must exist from previous stories):

1. `internal/queryir/types.go` - Query and Predicate definitions (Story 4.1)
2. `internal/queryir/validate.go` - QueryIR validation (Story 4.2)
3. `internal/ir/value.go` - IRValue types (Story 1.2)

### Story Completion Checklist

- [ ] SQLCompiler struct defined in `internal/querysql/compile.go`
- [ ] Compile(queryir.Query) method implemented
- [ ] Select compilation works (SELECT, FROM, WHERE, ORDER BY)
- [ ] Predicate compilation works (Equals, And, BoundEquals)
- [ ] Join compilation works (INNER JOIN with ON)
- [ ] stableOrderKey() ensures ORDER BY on every query
- [ ] COLLATE BINARY used on all text ORDER BY clauses
- [ ] String values NEVER interpolated - always parameterized
- [ ] irValueToParam() converts IRValue to SQL parameter
- [ ] Tests verify ORDER BY on all queries
- [ ] Tests verify no string interpolation (golden tests)
- [ ] Tests verify COLLATE BINARY on text columns
- [ ] Tests verify And predicate with multiple conditions
- [ ] Tests verify Join compilation
- [ ] Tests verify empty filter still has ORDER BY
- [ ] `go vet ./internal/querysql/...` passes
- [ ] `go test ./internal/querysql/...` passes

### References

- [Source: docs/epics.md#Story 4.3] - Story definition
- [Source: docs/prd.md#FR-2.3] - Compile where-clause to SQL
- [Source: docs/architecture.md#CP-4] - Deterministic query ordering
- [Source: docs/architecture.md#HIGH-3] - Parameterized queries only
- [Source: Story 4.1] - QueryIR type system
- [Source: Story 4.2] - QueryIR validation
- [Source: Story 1.2] - IRValue type system

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Validation History

- Initial creation: 2025-12-12 - Story file created based on Epic 4.3 requirements

### Completion Notes

- **CP-4 Enforcement:** stableOrderKey() MUST be called on every query - no exceptions
- **Security:** String interpolation forbidden - all values via ? parameters
- **Determinism:** COLLATE BINARY ensures stable text ordering across SQLite versions
- **Simplicity:** MVP implementation handles basic Select, Join, Equals, And predicates
- **Extensibility:** QueryIR abstraction allows future SPARQL backend without changing syncs
- **Testing:** Golden tests verify SQL output; injection tests verify parameterization
- **Integration:** Story 4.4 will use this compiler to execute where-clauses
- **Deferred:** Full subquery support, aggregations, and advanced SQL features deferred to future stories
- **Foundation:** This compiler is the bridge between QueryIR (portable) and SQLite (implementation)
