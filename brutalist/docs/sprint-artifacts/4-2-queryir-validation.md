# Story 4.2: QueryIR Validation

Status: ready-for-dev

## Story

As a **developer using QueryIR**,
I want **queries validated against portable fragment rules**,
So that **I know if my query will work with future SPARQL backend**.

## Acceptance Criteria

1. **Validation function in `internal/queryir/validate.go`**
   ```go
   // Validate checks if a query conforms to the portable fragment rules.
   // Returns validation result with portability status and warnings.
   func Validate(query Query) ValidationResult
   ```
   - Public function that accepts any QueryIR Query
   - Returns structured ValidationResult (not just bool)
   - No side effects (pure function)

2. **ValidationResult struct with detailed feedback**
   ```go
   type ValidationResult struct {
       IsPortable bool
       Warnings   []string  // Non-portable features used
   }
   ```
   - IsPortable: true if query uses only portable fragment features
   - Warnings: human-readable descriptions of violations
   - Empty warnings when IsPortable = true

3. **Rule 1: No NULLs - all fields must have values**
   - Check all Equals predicates: field comparisons must be non-null
   - Check BoundEquals predicates: bound variables must exist in bindings
   - Warning: "Field '{field}' compared to NULL - portable fragment requires explicit values"
   - Rationale: NULL semantics differ across SQL/SPARQL/RDF

4. **Rule 2: No outer joins - only inner joins allowed**
   - Traverse Query tree looking for Join nodes
   - Validate all Join nodes are inner joins (no left/right/full outer)
   - Warning: "Outer join detected - portable fragment requires inner joins only"
   - Rationale: Outer joins create NULL handling complexity incompatible with SPARQL

5. **Rule 3: Set semantics - no duplicate handling**
   - Check for DISTINCT requirements (not allowed in portable fragment)
   - Check for row counting/aggregations (not allowed)
   - Warning: "Aggregation detected - portable fragment uses set semantics"
   - Rationale: SPARQL naturally uses set semantics; SQL DISTINCT is non-portable

6. **Rule 4: Explicit bindings - no SELECT ***
   - Check Select nodes have non-empty Bindings map
   - Verify all binding targets are explicitly named
   - Warning: "Empty bindings (SELECT *) - portable fragment requires explicit field selection"
   - Rationale: Schema evolution and explicit contracts

7. **Non-portable queries are allowed but logged**
   - Validation never errors (returns ValidationResult)
   - SQL backend can execute non-portable queries
   - Engine logs warnings at INFO level when executing non-portable queries
   - Developers choose portability vs. SQL-specific features

8. **Comprehensive test coverage in `internal/queryir/validate_test.go`**
   - Test portable queries return IsPortable=true, no warnings
   - Test each violation rule independently
   - Test multiple violations accumulate warnings
   - Test nested queries validate recursively
   - Test all Query node types (Select, Join)
   - Test all Predicate types (Equals, And, BoundEquals)

## Quick Reference

| Pattern | Key Rule |
|---------|----------|
| **HIGH-2** | QueryIR abstraction enables SPARQL migration |
| **FR-2.5** | Portable fragment maintains abstraction boundary |
| **Portable Fragment** | No NULLs, no outer joins, set semantics, explicit bindings |

## Tasks / Subtasks

- [ ] Task 1: Create validation types and function (AC: #1, #2)
  - [ ] 1.1 Create `internal/queryir/validate.go`
  - [ ] 1.2 Define ValidationResult struct
  - [ ] 1.3 Implement Validate(query Query) function signature
  - [ ] 1.4 Add package-level documentation explaining portable fragment

- [ ] Task 2: Implement No-NULLs validation (AC: #3)
  - [ ] 2.1 Add checkNulls helper function
  - [ ] 2.2 Validate Equals predicates don't use NULL values
  - [ ] 2.3 Validate BoundEquals predicates reference existing bindings
  - [ ] 2.4 Add warning message for NULL violations

- [ ] Task 3: Implement No-Outer-Joins validation (AC: #4)
  - [ ] 3.1 Add checkJoins helper function
  - [ ] 3.2 Recursively traverse Query tree
  - [ ] 3.3 Detect outer join types (when Join node has OuterType field)
  - [ ] 3.4 Add warning message for outer join violations

- [ ] Task 4: Implement Set-Semantics validation (AC: #5)
  - [ ] 4.1 Add checkAggregations helper function
  - [ ] 4.2 Check for DISTINCT requirements (if added to QueryIR)
  - [ ] 4.3 Check for aggregation functions (COUNT, SUM, etc.)
  - [ ] 4.4 Add warning message for aggregation violations

- [ ] Task 5: Implement Explicit-Bindings validation (AC: #6)
  - [ ] 5.1 Add checkBindings helper function
  - [ ] 5.2 Validate Select nodes have non-empty Bindings map
  - [ ] 5.3 Verify all binding targets are named (no "*" wildcards)
  - [ ] 5.4 Add warning message for empty bindings

- [ ] Task 6: Integrate validation into Engine (AC: #7)
  - [ ] 6.1 Add Validate call when compiling where-clauses
  - [ ] 6.2 Log warnings at INFO level for non-portable queries
  - [ ] 6.3 Continue execution (don't fail on non-portable queries)
  - [ ] 6.4 Include portability status in debug/trace output

- [ ] Task 7: Write comprehensive tests (AC: #8)
  - [ ] 7.1 Create `internal/queryir/validate_test.go`
  - [ ] 7.2 Test portable query returns IsPortable=true
  - [ ] 7.3 Test NULL value violation
  - [ ] 7.4 Test missing bound variable violation
  - [ ] 7.5 Test outer join violation
  - [ ] 7.6 Test aggregation violation
  - [ ] 7.7 Test empty bindings violation
  - [ ] 7.8 Test multiple violations accumulate warnings
  - [ ] 7.9 Test nested query validation (Join with Select)

## Dev Notes

### Portable Fragment Philosophy

The **portable fragment** is a subset of QueryIR that can be implemented by both SQL and SPARQL backends. This enables future migration to SPARQL/RDF without rewriting sync rules.

**Core Principle:** The portable fragment is the intersection of SQL and SPARQL capabilities, not the union.

```
┌─────────────────────────────────────┐
│ Full SQL Capabilities               │
│  ┌───────────────────────────────┐  │
│  │ Portable Fragment             │  │
│  │ (SQL ∩ SPARQL)                │  │
│  │  - Inner joins                │  │
│  │  - Equality predicates        │  │
│  │  - Set semantics              │  │
│  │  - Explicit bindings          │  │
│  └───────────────────────────────┘  │
│                                     │
│  Non-portable SQL features:        │
│  - Outer joins                      │
│  - NULLs                            │
│  - Aggregations                     │
│  - Window functions                 │
└─────────────────────────────────────┘
```

### Validation Implementation

```go
// internal/queryir/validate.go

package queryir

import (
    "fmt"
    "github.com/tyler/nysm/internal/ir"
)

// ValidationResult contains portability analysis of a query.
type ValidationResult struct {
    // IsPortable indicates if the query uses only portable fragment features.
    // True means the query will work with both SQL and future SPARQL backends.
    IsPortable bool

    // Warnings lists non-portable features used in the query.
    // Empty when IsPortable is true.
    Warnings []string
}

// Validate checks if a query conforms to the portable fragment rules.
//
// The portable fragment is the subset of QueryIR that can be implemented
// by both SQL and SPARQL backends. Queries outside this fragment will
// work with SQL but may require rewriting for SPARQL migration.
//
// Portable fragment rules:
// 1. No NULLs - all field comparisons must use explicit values
// 2. No outer joins - only inner joins allowed
// 3. Set semantics - no aggregations or duplicate handling
// 4. Explicit bindings - no SELECT * wildcards
//
// Non-portable queries are allowed and will execute correctly with the
// SQL backend. Warnings are returned to inform developers of migration
// constraints.
func Validate(query Query) ValidationResult {
    v := &validator{
        warnings: []string{},
    }
    v.validateQuery(query)

    return ValidationResult{
        IsPortable: len(v.warnings) == 0,
        Warnings:   v.warnings,
    }
}

// validator accumulates warnings during traversal
type validator struct {
    warnings []string
}

// addWarning appends a warning message
func (v *validator) addWarning(format string, args ...any) {
    v.warnings = append(v.warnings, fmt.Sprintf(format, args...))
}

// validateQuery recursively validates a query node
func (v *validator) validateQuery(q Query) {
    switch query := q.(type) {
    case Select:
        v.validateSelect(query)
    case Join:
        v.validateJoin(query)
    default:
        // Unknown query type - add warning
        v.addWarning("Unknown query type: %T - portability cannot be verified", q)
    }
}

// validateSelect validates a Select query node
func (v *validator) validateSelect(sel Select) {
    // Rule 4: Explicit bindings - no SELECT *
    if len(sel.Bindings) == 0 {
        v.addWarning("Empty bindings (SELECT *) - portable fragment requires explicit field selection")
    }

    // Validate filter predicates
    if sel.Filter != nil {
        v.validatePredicate(sel.Filter)
    }
}

// validateJoin validates a Join query node
func (v *validator) validateJoin(join Join) {
    // Rule 2: No outer joins
    // NOTE: Join type detection depends on Story 4.1's final Join struct design.
    // If Join has an explicit JoinType field, check it here:
    // if join.JoinType != JoinTypeInner {
    //     v.addWarning("Outer join detected - portable fragment requires inner joins only")
    // }

    // Recursively validate left and right sides
    v.validateQuery(join.Left)
    v.validateQuery(join.Right)

    // Validate join condition
    if join.On != nil {
        v.validatePredicate(join.On)
    }
}

// validatePredicate validates a predicate node
func (v *validator) validatePredicate(p Predicate) {
    switch pred := p.(type) {
    case Equals:
        v.validateEquals(pred)
    case BoundEquals:
        // BoundEquals is portable - references when-clause bindings
        // No validation needed (binding existence checked at runtime)
    case And:
        // Recursively validate all sub-predicates
        for _, subPred := range pred.Predicates {
            v.validatePredicate(subPred)
        }
    default:
        // Unknown predicate type
        v.addWarning("Unknown predicate type: %T - portability cannot be verified", p)
    }
}

// validateEquals validates an Equals predicate
func (v *validator) validateEquals(eq Equals) {
    // Rule 1: No NULLs
    // In Go, we can't directly check if ir.IRValue is "nil" since IRValue is an interface.
    // The IR type system forbids null (Story 1.2), but we check anyway for safety.
    //
    // If IRValue were nullable, we'd check: if eq.Value == nil { ... }
    // Since it's not, we rely on the IR type system's null-rejection.
    //
    // However, if we later add Optional[T] types, we'd need to detect them here:
    // if isOptionalType(eq.Value) {
    //     v.addWarning("Field '%s' compared to optional/null - portable fragment requires explicit values", eq.Field)
    // }
}
```

### Test Examples

**Test Portable Query**

```go
// internal/queryir/validate_test.go

package queryir

import (
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "github.com/tyler/nysm/internal/ir"
)

func TestValidate_PortableQuery(t *testing.T) {
    // Simple portable query: SELECT item_id FROM cart_items WHERE cart_id = ?
    query := Select{
        From: "cart_items",
        Filter: Equals{
            Field: "cart_id",
            Value: ir.IRString("cart-123"),
        },
        Bindings: map[string]string{
            "item_id": "item_id",
        },
    }

    result := Validate(query)

    assert.True(t, result.IsPortable, "simple select should be portable")
    assert.Empty(t, result.Warnings, "no warnings for portable query")
}

func TestValidate_EmptyBindings(t *testing.T) {
    // SELECT * is not portable
    query := Select{
        From:     "cart_items",
        Filter:   nil,
        Bindings: map[string]string{}, // Empty = SELECT *
    }

    result := Validate(query)

    assert.False(t, result.IsPortable, "SELECT * is not portable")
    require.Len(t, result.Warnings, 1)
    assert.Contains(t, result.Warnings[0], "Empty bindings")
    assert.Contains(t, result.Warnings[0], "SELECT *")
}

func TestValidate_NestedAndPredicate(t *testing.T) {
    // Portable query with AND predicate
    query := Select{
        From: "inventory",
        Filter: And{
            Predicates: []Predicate{
                Equals{
                    Field: "product_id",
                    Value: ir.IRString("product-1"),
                },
                Equals{
                    Field: "warehouse_id",
                    Value: ir.IRString("warehouse-west"),
                },
            },
        },
        Bindings: map[string]string{
            "available": "quantity_available",
        },
    }

    result := Validate(query)

    assert.True(t, result.IsPortable, "AND predicates are portable")
    assert.Empty(t, result.Warnings)
}

func TestValidate_BoundEquals(t *testing.T) {
    // BoundEquals references when-clause bindings - portable
    query := Select{
        From: "inventory",
        Filter: BoundEquals{
            Field:    "product_id",
            BoundVar: "item_id", // From when-clause
        },
        Bindings: map[string]string{
            "available": "quantity_available",
        },
    }

    result := Validate(query)

    assert.True(t, result.IsPortable, "BoundEquals is portable")
    assert.Empty(t, result.Warnings)
}

func TestValidate_Join(t *testing.T) {
    // Inner join is portable
    query := Join{
        Left: Select{
            From: "cart_items",
            Bindings: map[string]string{
                "product_id": "product_id",
            },
        },
        Right: Select{
            From: "inventory",
            Bindings: map[string]string{
                "available": "quantity_available",
            },
        },
        On: Equals{
            Field: "product_id",
            Value: ir.IRString("product-1"),
        },
    }

    result := Validate(query)

    assert.True(t, result.IsPortable, "inner join is portable")
    assert.Empty(t, result.Warnings)
}

func TestValidate_MultipleViolations(t *testing.T) {
    // Query with multiple non-portable features
    query := Select{
        From:     "cart_items",
        Filter:   nil,
        Bindings: map[string]string{}, // Violation 1: SELECT *
    }

    result := Validate(query)

    assert.False(t, result.IsPortable)
    assert.NotEmpty(t, result.Warnings)
    // Could have multiple warnings if we add more violation checks
}

func TestValidate_UnknownQueryType(t *testing.T) {
    // If someone adds a new Query type but doesn't update validator
    type UnknownQuery struct{}
    func (UnknownQuery) queryNode() {}

    query := UnknownQuery{}

    result := Validate(query)

    assert.False(t, result.IsPortable, "unknown types are not portable")
    require.Len(t, result.Warnings, 1)
    assert.Contains(t, result.Warnings[0], "Unknown query type")
}
```

### Why Each Rule Matters

**Rule 1: No NULLs**

SQL NULL has three-valued logic (true/false/unknown) that doesn't map to SPARQL:

```sql
-- SQL: NULL handling is complex
SELECT * FROM items WHERE price = NULL  -- Always returns 0 rows (NULL != NULL)
SELECT * FROM items WHERE price IS NULL -- Correct NULL check

-- SPARQL: No NULL concept - uses optional patterns
SELECT ?item WHERE {
    ?item :price ?price .  # Binding required
    FILTER (!bound(?price)) # Explicit "not bound" check
}
```

Portable fragment forbids NULLs - use explicit Option[T] types instead.

**Rule 2: No Outer Joins**

SPARQL's OPTIONAL is fundamentally different from SQL's LEFT/RIGHT OUTER JOIN:

```sql
-- SQL: Outer join creates NULLs for missing matches
SELECT c.id, i.stock
FROM cart_items c
LEFT OUTER JOIN inventory i ON c.product_id = i.product_id
-- Result: cart items with NULL stock if product not in inventory

-- SPARQL: OPTIONAL creates unbound variables
SELECT ?cartItem ?stock WHERE {
    ?cartItem :productId ?productId .
    OPTIONAL { ?productId :stock ?stock }
}
-- Result: ?stock is unbound if no match (not NULL)
```

Portable fragment uses inner joins only - explicit queries handle missing data.

**Rule 3: Set Semantics**

SQL returns bags (multisets), SPARQL returns sets:

```sql
-- SQL: Duplicates by default
SELECT item_id FROM cart_items  -- May return duplicate item_ids
SELECT DISTINCT item_id FROM cart_items  -- De-duplicates

-- SPARQL: Sets by default
SELECT ?itemId WHERE {
    ?item :itemId ?itemId .
}
-- Result: Automatically de-duplicated (set semantics)
```

Portable fragment assumes set semantics - no DISTINCT, no aggregations.

**Rule 4: Explicit Bindings**

SELECT * is fragile across schema changes and creates implicit contracts:

```sql
-- SQL: SELECT * is schema-dependent
SELECT * FROM cart_items  -- Returns all columns (schema changes break code)

-- SPARQL: Must explicitly name bindings
SELECT ?itemId ?quantity WHERE {
    ?item :itemId ?itemId ;
          :quantity ?quantity .
}
-- Result: Explicit contract, schema evolution doesn't break queries
```

Portable fragment requires explicit bindings for stable contracts.

### Integration with Engine

When executing where-clauses, the engine validates and logs portability:

```go
// internal/engine/sync.go

func (e *Engine) executeWhere(
    ctx context.Context,
    where ir.WhereClause,
    whenBindings ir.IRObject,
    flowToken string,
) ([]ir.IRObject, error) {
    // Build QueryIR from where-clause
    query := e.buildQuery(where, whenBindings)

    // Validate portability
    validation := queryir.Validate(query)
    if !validation.IsPortable {
        // Log warnings but continue execution
        for _, warning := range validation.Warnings {
            e.logger.Infof("Non-portable query in sync '%s': %s", where.SyncID, warning)
        }
    }

    // Compile to SQL and execute
    sql, params, err := e.compiler.Compile(query)
    if err != nil {
        return nil, fmt.Errorf("compile query: %w", err)
    }

    // ... execute and return bindings
}
```

### File List

Files to create:

1. `internal/queryir/validate.go` - Validate function and ValidationResult type
2. `internal/queryir/validate_test.go` - Comprehensive tests

Files to modify:

1. `internal/engine/sync.go` - Add validation call before query compilation (Story 4.4)
2. `internal/queryir/types.go` - May need to add JoinType enum if not present from Story 4.1

Files to reference (must exist from previous stories):

1. `internal/queryir/types.go` - Query, Select, Join, Predicate types (Story 4.1)
2. `internal/ir/value.go` - IRValue types (Story 1.2)

### Relationship to Other Stories

- **Story 4.1:** Uses Query, Select, Join, Predicate types from QueryIR type system
- **Story 4.3:** Validation warnings inform SQL compiler decisions
- **Story 4.4:** Engine calls Validate before executing where-clauses
- **Story 1.2:** IRValue type system forbids NULLs (validates Rule 1)
- **Future SPARQL migration:** Portable queries translate directly; non-portable require rewrite

### Story Completion Checklist

- [ ] `internal/queryir/validate.go` created
- [ ] ValidationResult struct defined with IsPortable and Warnings
- [ ] Validate(query Query) function implemented
- [ ] Rule 1 (No NULLs) validation implemented
- [ ] Rule 2 (No outer joins) validation implemented
- [ ] Rule 3 (Set semantics) validation implemented
- [ ] Rule 4 (Explicit bindings) validation implemented
- [ ] Validator recursively traverses Query tree
- [ ] Validator recursively traverses Predicate tree
- [ ] `internal/queryir/validate_test.go` created
- [ ] Test: Portable query returns IsPortable=true
- [ ] Test: Empty bindings violation
- [ ] Test: And predicate recursion
- [ ] Test: BoundEquals is portable
- [ ] Test: Inner join is portable
- [ ] Test: Multiple violations accumulate
- [ ] Test: Unknown query type handled
- [ ] All tests pass (`go test ./internal/queryir/...`)
- [ ] `go build ./...` succeeds
- [ ] `go vet ./internal/queryir` passes

### References

- [Source: docs/epics.md#Story 4.2] - Story definition and acceptance criteria
- [Source: docs/architecture.md#HIGH-2] - QueryIR abstraction boundary
- [Source: docs/prd.md#FR-2.5] - Maintain abstraction for SPARQL migration
- [Source: Story 4.1] - QueryIR type system foundation
- [W3C SPARQL Specification] - SPARQL semantics and constraints
- [SQL Standards] - SQL NULL handling and join semantics

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Validation History

- Initial creation: comprehensive story generation

### Completion Notes

- Portable fragment is the intersection of SQL and SPARQL capabilities
- Validation never errors - returns warnings for developer awareness
- Non-portable queries work with SQL backend but won't migrate to SPARQL
- Each rule has clear rationale tied to SQL/SPARQL semantic differences
- Validator is pure function (no side effects) for testability
- Recursive traversal handles nested queries and predicates
- Story 4.1 must define Query/Predicate type system before this can be implemented
- Future Story 4.3 (SQL compiler) will use validation results for optimization hints
- Future Story 4.4 (binding execution) will integrate validation into engine
- Critical for FR-2.5 (maintain abstraction boundary for future SPARQL migration)
