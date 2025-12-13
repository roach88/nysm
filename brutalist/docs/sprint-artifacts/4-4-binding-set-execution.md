# Story 4.4: Binding Set Execution

Status: ready-for-dev

## Story

As a **developer running sync rules**,
I want **where-clauses to return a SET of bindings**,
So that **then-clauses can fire multiple invocations**.

## Acceptance Criteria

1. **executeWhere function signature in `internal/engine/engine.go`**
   ```go
   func (e *Engine) executeWhere(
       ctx context.Context,
       where ir.WhereClause,
       whenBindings ir.IRObject,
       flowToken string,
   ) ([]ir.IRObject, error)
   ```

2. **Function returns []ir.IRObject (zero or more binding sets)**
   - Returns empty slice `[]` when no records match (valid, not error)
   - Returns single-element slice when one record matches
   - Returns N-element slice when N records match
   - Each element is an ir.IRObject containing bound variables

3. **Bindings are ORDERED per query ORDER BY (not a set)**
   - Order is deterministic per CP-4 (Deterministic Query Ordering)
   - Same query always returns bindings in same order
   - Order determined by SQL `ORDER BY` clause with tiebreaker
   - Replay produces identical binding sequence

4. **Empty result is valid (sync doesn't fire)**
   ```go
   bindings, err := e.executeWhere(ctx, where, whenBindings, flowToken)
   // err == nil AND len(bindings) == 0 is VALID
   // Means: No records matched, sync rule skips firing
   ```

5. **Multiple bindings generate multiple invocations**
   ```go
   // Given where-clause returns 3 bindings
   bindings, _ := e.executeWhere(ctx, where, whenBindings, flowToken)
   // len(bindings) == 3

   // Then-clause fires 3 times:
   for _, binding := range bindings {
       inv := e.buildInvocation(then, binding)
       e.Invoke(ctx, inv)
   }
   ```

6. **Implementation compiles QueryIR to SQL**
   ```go
   func (e *Engine) executeWhere(
       ctx context.Context,
       where ir.WhereClause,
       whenBindings ir.IRObject,
       flowToken string,
   ) ([]ir.IRObject, error) {
       // Build QueryIR query from where-clause
       query := e.buildQuery(where, whenBindings)

       // Compile to SQL
       sql, params, err := e.compiler.Compile(query)
       if err != nil {
           return nil, fmt.Errorf("compile query: %w", err)
       }

       // Execute query
       rows, err := e.store.Query(ctx, sql, params...)
       if err != nil {
           return nil, fmt.Errorf("execute query: %w", err)
       }
       defer rows.Close()

       // Scan results into binding sets
       var bindings []ir.IRObject
       for rows.Next() {
           binding, err := e.scanBinding(rows, where.Bindings)
           if err != nil {
               return nil, fmt.Errorf("scan binding: %w", err)
           }
           bindings = append(bindings, binding)
       }

       return bindings, nil
   }
   ```

7. **Comprehensive tests in `internal/engine/execute_where_test.go`**
   - Test: Zero bindings (empty result)
   - Test: One binding (single record match)
   - Test: N bindings (multiple record matches)
   - Test: Binding order is deterministic across multiple executions
   - Test: Empty result doesn't error
   - Test: scanBinding extracts correct fields

## Quick Reference

| Pattern | Key Rule |
|---------|----------|
| **FR-2.4** | Execute then-clause to generate invocations from bindings |
| **CP-4** | Deterministic query ordering (ORDER BY with tiebreaker) |
| **CRITICAL-2** | Identical replay requires deterministic binding order |

## Tasks / Subtasks

- [ ] Task 1: Implement executeWhere function (AC: #1, #2, #6)
  - [ ] 1.1 Create `internal/engine/execute_where.go`
  - [ ] 1.2 Define executeWhere function signature
  - [ ] 1.3 Call buildQuery to construct QueryIR from where-clause
  - [ ] 1.4 Call compiler.Compile to get SQL and params
  - [ ] 1.5 Execute query via store.Query
  - [ ] 1.6 Scan rows into []ir.IRObject
  - [ ] 1.7 Return empty slice (not error) when no matches

- [ ] Task 2: Implement buildQuery helper (AC: #6)
  - [ ] 2.1 Create buildQuery(where, whenBindings) function
  - [ ] 2.2 Extract filter from where.Filter
  - [ ] 2.3 Extract binding spec from where.Bindings
  - [ ] 2.4 Construct queryir.Select with FROM, WHERE, ORDER BY
  - [ ] 2.5 Add flow_token filter if scope mode is "flow"

- [ ] Task 3: Implement scanBinding helper (AC: #6)
  - [ ] 3.1 Create scanBinding(rows, bindingSpec) function
  - [ ] 3.2 Scan row columns into temporary variables
  - [ ] 3.3 Build ir.IRObject from scanned values
  - [ ] 3.4 Map columns to binding variable names per bindingSpec
  - [ ] 3.5 Return populated ir.IRObject

- [ ] Task 4: Test zero bindings (AC: #4, #7)
  - [ ] 4.1 Create test with where-clause that matches nothing
  - [ ] 4.2 Verify executeWhere returns empty slice
  - [ ] 4.3 Verify no error returned
  - [ ] 4.4 Verify sync rule skips firing

- [ ] Task 5: Test one binding (AC: #7)
  - [ ] 5.1 Create test with where-clause that matches one record
  - [ ] 5.2 Verify executeWhere returns single-element slice
  - [ ] 5.3 Verify binding contains correct field values
  - [ ] 5.4 Verify sync rule fires once

- [ ] Task 6: Test N bindings (AC: #5, #7)
  - [ ] 6.1 Create test with where-clause that matches 3 records
  - [ ] 6.2 Verify executeWhere returns 3-element slice
  - [ ] 6.3 Verify each binding has distinct values
  - [ ] 6.4 Verify sync rule fires 3 times

- [ ] Task 7: Test deterministic ordering (AC: #3, #7)
  - [ ] 7.1 Execute same query multiple times
  - [ ] 7.2 Verify binding order is identical every time
  - [ ] 7.3 Verify ORDER BY is applied to SQL
  - [ ] 7.4 Test with records having same sort key (tiebreaker)

- [ ] Task 8: Integration with engine loop (AC: #5)
  - [ ] 8.1 Update processCompletion to call executeWhere
  - [ ] 8.2 Loop over returned bindings
  - [ ] 8.3 Fire then-clause for each binding
  - [ ] 8.4 Track each firing separately in sync_firings table

## Dev Notes

### Critical Implementation Details

**Binding Set Semantics**

The where-clause produces a **binding set** - zero or more variable assignments that satisfy the query. This is fundamental to NYSM's sync semantics:

```
when → where → then
 ↓      ↓       ↓
 1    0..N     0..N
```

- **when-clause**: Produces exactly 1 binding (the triggering completion)
- **where-clause**: Produces 0..N bindings (records matching query)
- **then-clause**: Fires 0..N times (once per binding)

**Why Zero Bindings is Valid**

Consider this sync rule:
```cue
sync "out-of-stock-notification" {
  when: Inventory.reserve.completed {
    case: "InsufficientStock"
    bind: { item_id: result.item_id }
  }
  where: Users[wants_notifications == true && watching == bound.item_id]
  then: Notification.send(user: bound.user_id, message: "Back in stock!")
}
```

**Scenario:** Item goes out of stock, but NO users are watching it.

**Expected Behavior:**
1. where-clause executes: `SELECT * FROM Users WHERE wants_notifications AND watching = ?`
2. Query returns 0 rows (no users watching)
3. executeWhere returns `[]` (empty slice)
4. then-clause doesn't fire (no users to notify)
5. **Result:** Sync completes successfully. No error. No notifications sent.

**Why This is Not An Error:**

- Zero bindings is semantically valid (similar to empty `for` loop)
- Distinguishes "no matches" from "query failed"
- Prevents spurious errors in logs
- Matches paper's formal semantics

**Why Multiple Bindings Matter**

Consider this sync rule:
```cue
sync "reserve-cart-items" {
  when: Cart.checkout.completed {
    case: "Success"
    bind: { cart_id: result.cart_id }
  }
  where: CartItems[cart_id == bound.cart_id]
  then: Inventory.reserve(item: bound.item_id, qty: bound.quantity)
}
```

**Scenario:** Cart contains 3 items: [apple, banana, carrot]

**Execution:**
1. where-clause executes: `SELECT * FROM CartItems WHERE cart_id = ?`
2. Query returns 3 rows (3 items in cart)
3. executeWhere returns 3 bindings:
   ```go
   [
     {"item_id": "apple",  "quantity": 2},
     {"item_id": "banana", "quantity": 1},
     {"item_id": "carrot", "quantity": 3},
   ]
   ```
4. then-clause fires 3 times:
   - `Inventory.reserve(item: "apple",  qty: 2)`
   - `Inventory.reserve(item: "banana", qty: 1)`
   - `Inventory.reserve(item: "carrot", qty: 3)`
5. **Result:** All cart items reserved separately

**What Happens Without Binding Sets:**

If executeWhere returned only a single binding (first match), only the apple would be reserved. Cart checkout would succeed, but banana and carrot wouldn't be reserved. **Data corruption.**

**Order Matters for Determinism**

Binding sets are **ordered sequences**, not mathematical sets. Order must be deterministic per CP-4.

**Why Order Matters:**

1. **Replay must be identical** - After crash, re-executing where-clause must produce same binding order
2. **Idempotency checking** - Binding hash depends on values, but firing order affects system state
3. **Dependent invocations** - If invocations have side effects, order determines final state

**Example of Order Dependency:**

```cue
sync "update-inventory-sequence" {
  when: Batch.import.completed {
    bind: { batch_id: result.batch_id }
  }
  where: InventoryDeltas[batch_id == bound.batch_id]
    ORDER BY timestamp ASC  // ORDER BY is MANDATORY
  then: Inventory.adjust(item: bound.item_id, delta: bound.delta)
}
```

**Scenario:** Batch contains 3 deltas for same item:
- Delta 1: item=X, delta=+10 (timestamp: 100)
- Delta 2: item=X, delta=-3  (timestamp: 200)
- Delta 3: item=X, delta=+5  (timestamp: 300)

**With Deterministic Order (timestamp ASC):**
- Execution 1: 0 → +10 → 7 → 12 ✓
- Replay:     0 → +10 → 7 → 12 ✓
- **Result:** Final inventory = 12 (consistent)

**Without Deterministic Order:**
- Execution 1: 0 → -3 (ERROR: negative inventory)
- Replay:     0 → +10 → 7 → 12
- **Result:** Non-deterministic behavior, replay divergence

**CP-4 Enforcement:**

ALL queries MUST include `ORDER BY` with tiebreaker:
```sql
SELECT item_id, quantity FROM cart_items
WHERE cart_id = ?
ORDER BY item_id ASC COLLATE BINARY  -- Deterministic tiebreaker
```

Without tiebreaker, SQLite can return rows in arbitrary order. This breaks replay.

### Function Signatures

**executeWhere Implementation**

```go
// internal/engine/execute_where.go

package engine

import (
    "context"
    "database/sql"
    "fmt"

    "github.com/tyler/nysm/internal/ir"
    "github.com/tyler/nysm/internal/queryir"
)

// executeWhere executes a where-clause query and returns binding sets.
// Returns zero or more bindings (empty slice is valid, not an error).
// Bindings are ordered deterministically per CP-4.
func (e *Engine) executeWhere(
    ctx context.Context,
    where ir.WhereClause,
    whenBindings ir.IRObject,
    flowToken string,
) ([]ir.IRObject, error) {
    // Build QueryIR query from where-clause
    query, err := e.buildQuery(where, whenBindings)
    if err != nil {
        return nil, fmt.Errorf("build query: %w", err)
    }

    // Compile QueryIR to SQL
    sql, params, err := e.compiler.Compile(query)
    if err != nil {
        return nil, fmt.Errorf("compile query: %w", err)
    }

    // Execute query against store
    rows, err := e.store.Query(ctx, sql, params...)
    if err != nil {
        return nil, fmt.Errorf("execute query: %w", err)
    }
    defer rows.Close()

    // Scan rows into binding sets
    var bindings []ir.IRObject
    for rows.Next() {
        binding, err := e.scanBinding(rows, where.Bindings)
        if err != nil {
            return nil, fmt.Errorf("scan binding: %w", err)
        }

        // Merge when-bindings with where-bindings
        mergedBinding := e.mergeBindings(whenBindings, binding)
        bindings = append(bindings, mergedBinding)
    }

    // Check for row iteration errors
    if err := rows.Err(); err != nil {
        return nil, fmt.Errorf("rows iteration: %w", err)
    }

    // Empty slice is valid (zero matches)
    return bindings, nil
}

// buildQuery constructs a QueryIR query from a where-clause and when-bindings.
// Applies flow-scoping based on sync rule scope mode.
func (e *Engine) buildQuery(
    where ir.WhereClause,
    whenBindings ir.IRObject,
) (queryir.Query, error) {
    // Parse filter expression from where.Filter
    filterExpr, err := e.parseFilter(where.Filter, whenBindings)
    if err != nil {
        return nil, fmt.Errorf("parse filter: %w", err)
    }

    // Build SELECT query
    query := queryir.Select{
        From:     where.Source,
        Filter:   filterExpr,
        Bindings: where.Bindings,
    }

    // Apply flow-scoping (from Story 3.7)
    // This is where flow_token filter is added if scope mode is "flow"

    return query, nil
}

// scanBinding scans a SQL row into an ir.IRObject.
// Maps SQL columns to binding variable names per bindingSpec.
func (e *Engine) scanBinding(
    rows *sql.Rows,
    bindingSpec map[string]string,
) (ir.IRObject, error) {
    // Get column names from result set
    columns, err := rows.Columns()
    if err != nil {
        return nil, fmt.Errorf("get columns: %w", err)
    }

    // Create scan targets (interface{} for each column)
    values := make([]interface{}, len(columns))
    valuePtrs := make([]interface{}, len(columns))
    for i := range values {
        valuePtrs[i] = &values[i]
    }

    // Scan row into values
    if err := rows.Scan(valuePtrs...); err != nil {
        return nil, fmt.Errorf("scan row: %w", err)
    }

    // Build binding object
    binding := make(ir.IRObject)
    for i, colName := range columns {
        // Look up variable name for this column
        varName, exists := bindingSpec[colName]
        if !exists {
            // Column not in binding spec - skip
            continue
        }

        // Convert SQL value to IRValue
        irValue, err := e.sqlToIRValue(values[i])
        if err != nil {
            return nil, fmt.Errorf("convert column %s: %w", colName, err)
        }

        binding[varName] = irValue
    }

    return binding, nil
}

// mergeBindings combines when-bindings and where-bindings.
// Where-bindings take precedence if there are conflicts.
func (e *Engine) mergeBindings(
    whenBindings ir.IRObject,
    whereBindings ir.IRObject,
) ir.IRObject {
    merged := make(ir.IRObject, len(whenBindings)+len(whereBindings))

    // Copy when-bindings first
    for k, v := range whenBindings {
        merged[k] = v
    }

    // Where-bindings override (if conflicts)
    for k, v := range whereBindings {
        merged[k] = v
    }

    return merged
}

// sqlToIRValue converts a SQL value (from database/sql) to an ir.IRValue.
func (e *Engine) sqlToIRValue(v interface{}) (ir.IRValue, error) {
    if v == nil {
        return ir.IRNull{}, nil
    }

    switch val := v.(type) {
    case int64:
        return ir.IRInt(val), nil
    case float64:
        // CP-5: Floats are FORBIDDEN in IR - they break determinism.
        // SQL REAL/FLOAT columns must be avoided in schema design.
        // If you hit this, either:
        // 1. Change the column type to INTEGER (store cents not dollars)
        // 2. Store as TEXT with explicit precision
        return nil, fmt.Errorf("float64 values are forbidden in IR (CP-5): %v - use INTEGER or TEXT instead", val)
    case string:
        return ir.IRString(val), nil
    case []byte:
        return ir.IRString(string(val)), nil
    case bool:
        return ir.IRBool(val), nil
    default:
        return nil, fmt.Errorf("unsupported SQL type: %T", v)
    }
}
```

**Integration with Engine Loop**

```go
// internal/engine/engine.go

// processCompletion handles a completion event and fires matching sync rules.
func (e *Engine) processCompletion(ctx context.Context, comp ir.Completion) error {
    // Write completion to event log
    if err := e.store.WriteCompletion(ctx, comp); err != nil {
        return fmt.Errorf("write completion: %w", err)
    }

    // Check all sync rules
    for _, sync := range e.syncs {
        // Match when-clause
        if !e.matcher.Matches(sync.When, comp) {
            continue
        }

        // Extract when-bindings
        whenBindings, err := e.matcher.ExtractBindings(sync.When, comp)
        if err != nil {
            return fmt.Errorf("extract when-bindings: %w", err)
        }

        // Execute where-clause → get binding sets
        bindings, err := e.executeWhere(
            ctx,
            sync.Where,
            whenBindings,
            comp.FlowToken,
        )
        if err != nil {
            return fmt.Errorf("execute where-clause for sync %s: %w", sync.ID, err)
        }

        // Fire then-clause for each binding
        for _, binding := range bindings {
            // Check if already fired (idempotency)
            bindingHash := ir.BindingHash(binding)
            hasFired, err := e.store.HasFiring(ctx, comp.ID, sync.ID, bindingHash)
            if err != nil {
                return fmt.Errorf("check firing: %w", err)
            }
            if hasFired {
                // Skip - already fired (replay or duplicate)
                continue
            }

            // Generate invocation from then-clause
            inv, err := e.buildInvocation(sync.Then, binding, comp.FlowToken)
            if err != nil {
                return fmt.Errorf("build invocation: %w", err)
            }

            // Write invocation
            if err := e.store.WriteInvocation(ctx, inv); err != nil {
                return fmt.Errorf("write invocation: %w", err)
            }

            // Record firing
            firing := ir.SyncFiring{
                CompletionID: comp.ID,
                SyncID:       sync.ID,
                BindingHash:  bindingHash,
                Seq:          e.nextSeq(),
            }
            if _, err := e.store.WriteSyncFiring(ctx, firing); err != nil {
                return fmt.Errorf("write firing: %w", err)
            }

            // Record provenance edge
            edge := ir.ProvenanceEdge{
                FromCompletion: comp.ID,
                ToInvocation:   inv.ID,
                SyncID:         sync.ID,
            }
            if err := e.store.WriteProvenanceEdge(ctx, edge); err != nil {
                return fmt.Errorf("write provenance: %w", err)
            }
        }
    }

    return nil
}
```

### Test Examples

**Test: Zero Bindings (Empty Result)**

```go
// internal/engine/execute_where_test.go

package engine

import (
    "context"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "github.com/tyler/nysm/internal/ir"
)

func TestExecuteWhere_ZeroBindings(t *testing.T) {
    ctx := context.Background()
    engine := setupTestEngine(t)

    // Create where-clause that matches nothing
    where := ir.WhereClause{
        Source: "CartItems",
        Filter: "cart_id = ?",
        Bindings: map[string]string{
            "item_id":  "item_id",
            "quantity": "quantity",
        },
    }

    // When-bindings with non-existent cart
    whenBindings := ir.IRObject{
        "cart_id": ir.IRString("cart-does-not-exist"),
    }

    // Execute where-clause
    bindings, err := engine.executeWhere(ctx, where, whenBindings, "flow-123")

    // Verify zero bindings returned (NOT an error)
    require.NoError(t, err, "Zero bindings should not be an error")
    assert.Empty(t, bindings, "Should return empty slice for zero matches")
    assert.NotNil(t, bindings, "Should return empty slice, not nil")
}
```

**Test: One Binding**

```go
func TestExecuteWhere_OneBinding(t *testing.T) {
    ctx := context.Background()
    engine := setupTestEngine(t)

    // Insert test data
    engine.store.Exec(`
        INSERT INTO cart_items (cart_id, item_id, quantity)
        VALUES ('cart-1', 'apple', 5)
    `)

    where := ir.WhereClause{
        Source: "CartItems",
        Filter: "cart_id = ?",
        Bindings: map[string]string{
            "item_id":  "item_id",
            "quantity": "quantity",
        },
    }

    whenBindings := ir.IRObject{
        "cart_id": ir.IRString("cart-1"),
    }

    // Execute where-clause
    bindings, err := engine.executeWhere(ctx, where, whenBindings, "flow-123")

    // Verify single binding returned
    require.NoError(t, err)
    assert.Len(t, bindings, 1, "Should return one binding")

    // Verify binding contents
    binding := bindings[0]
    assert.Equal(t, ir.IRString("apple"), binding["item_id"])
    assert.Equal(t, ir.IRInt(5), binding["quantity"])

    // Verify when-bindings are merged
    assert.Equal(t, ir.IRString("cart-1"), binding["cart_id"])
}
```

**Test: N Bindings (Multiple Records)**

```go
func TestExecuteWhere_MultipleBindings(t *testing.T) {
    ctx := context.Background()
    engine := setupTestEngine(t)

    // Insert test data - 3 items in cart
    engine.store.Exec(`
        INSERT INTO cart_items (cart_id, item_id, quantity) VALUES
        ('cart-1', 'apple',  2),
        ('cart-1', 'banana', 1),
        ('cart-1', 'carrot', 3)
    `)

    where := ir.WhereClause{
        Source: "CartItems",
        Filter: "cart_id = ?",
        Bindings: map[string]string{
            "item_id":  "item_id",
            "quantity": "quantity",
        },
    }

    whenBindings := ir.IRObject{
        "cart_id": ir.IRString("cart-1"),
    }

    // Execute where-clause
    bindings, err := engine.executeWhere(ctx, where, whenBindings, "flow-123")

    // Verify 3 bindings returned
    require.NoError(t, err)
    assert.Len(t, bindings, 3, "Should return three bindings")

    // Verify each binding has distinct values
    itemIDs := []string{
        string(bindings[0]["item_id"].(ir.IRString)),
        string(bindings[1]["item_id"].(ir.IRString)),
        string(bindings[2]["item_id"].(ir.IRString)),
    }
    assert.ElementsMatch(t, []string{"apple", "banana", "carrot"}, itemIDs)

    // Verify all bindings include when-bindings
    for _, binding := range bindings {
        assert.Equal(t, ir.IRString("cart-1"), binding["cart_id"])
    }
}
```

**Test: Deterministic Ordering**

```go
func TestExecuteWhere_DeterministicOrder(t *testing.T) {
    ctx := context.Background()
    engine := setupTestEngine(t)

    // Insert test data with SAME timestamp (tests tiebreaker)
    engine.store.Exec(`
        INSERT INTO cart_items (cart_id, item_id, quantity, created_at) VALUES
        ('cart-1', 'zebra',  1, 100),
        ('cart-1', 'apple',  2, 100),
        ('cart-1', 'banana', 3, 100)
    `)

    where := ir.WhereClause{
        Source: "CartItems",
        Filter: "cart_id = ?",
        Bindings: map[string]string{
            "item_id": "item_id",
        },
    }

    whenBindings := ir.IRObject{
        "cart_id": ir.IRString("cart-1"),
    }

    // Execute query 5 times
    var firstOrder []string
    for i := 0; i < 5; i++ {
        bindings, err := engine.executeWhere(ctx, where, whenBindings, "flow-123")
        require.NoError(t, err)

        // Extract item IDs in order
        order := make([]string, len(bindings))
        for j, binding := range bindings {
            order[j] = string(binding["item_id"].(ir.IRString))
        }

        if i == 0 {
            // First execution - record order
            firstOrder = order
        } else {
            // Subsequent executions - verify same order
            assert.Equal(t, firstOrder, order,
                "Iteration %d: binding order must be deterministic", i)
        }
    }

    // Verify order is lexicographic (tiebreaker applied)
    assert.Equal(t, []string{"apple", "banana", "zebra"}, firstOrder,
        "Should be ordered by item_id ASC (tiebreaker)")
}
```

**Test: Empty Result Doesn't Error**

```go
func TestExecuteWhere_EmptyResultValid(t *testing.T) {
    ctx := context.Background()
    engine := setupTestEngine(t)

    // No data in database

    where := ir.WhereClause{
        Source: "CartItems",
        Filter: "cart_id = ?",
        Bindings: map[string]string{
            "item_id": "item_id",
        },
    }

    whenBindings := ir.IRObject{
        "cart_id": ir.IRString("cart-empty"),
    }

    // Execute where-clause
    bindings, err := engine.executeWhere(ctx, where, whenBindings, "flow-123")

    // Empty result is VALID (not an error)
    assert.NoError(t, err, "Empty result should not error")
    assert.Empty(t, bindings, "Should return empty slice")

    // Verify sync rule handling
    // When bindings is empty, sync rule should skip firing
    for _, binding := range bindings {
        t.Errorf("Unexpected binding: %v (should be zero iterations)", binding)
    }
}
```

**Test: scanBinding Extracts Correct Fields**

```go
func TestScanBinding_ExtractsFields(t *testing.T) {
    ctx := context.Background()
    engine := setupTestEngine(t)

    // Insert test data
    engine.store.Exec(`
        INSERT INTO cart_items (item_id, quantity, price)
        VALUES ('apple', 5, 1.99)
    `)

    // Query with specific columns
    rows, err := engine.store.Query(ctx, `
        SELECT item_id, quantity, price FROM cart_items WHERE item_id = 'apple'
    `)
    require.NoError(t, err)
    defer rows.Close()

    // Scan binding
    require.True(t, rows.Next(), "Should have at least one row")

    bindingSpec := map[string]string{
        "item_id":  "item",
        "quantity": "qty",
        "price_cents": "price",  // CP-5: Store prices as INTEGER cents, not REAL
    }

    binding, err := engine.scanBinding(rows, bindingSpec)
    require.NoError(t, err)

    // Verify extracted values
    assert.Equal(t, ir.IRString("apple"), binding["item"])
    assert.Equal(t, ir.IRInt(5), binding["qty"])
    assert.Equal(t, ir.IRInt(199), binding["price"])  // CP-5: 199 cents = $1.99
}
```

**Integration Test: Multiple Bindings Generate Multiple Invocations**

```go
func TestEngine_MultipleFirings(t *testing.T) {
    ctx := context.Background()
    engine := setupTestEngine(t)

    // Load sync rule
    sync := ir.SyncRule{
        ID: "reserve-cart-items",
        When: ir.WhenClause{
            Action:     "Cart.checkout",
            Event:      "completed",
            OutputCase: ptr("Success"),
            Bindings: map[string]string{
                "cart_id": "cart_id",
            },
        },
        Where: ir.WhereClause{
            Source: "CartItems",
            Filter: "cart_id = ?",
            Bindings: map[string]string{
                "item_id":  "item_id",
                "quantity": "quantity",
            },
        },
        Then: ir.ThenClause{
            Action: "Inventory.reserve",
            Args: map[string]string{
                "item": "item_id",
                "qty":  "quantity",
            },
        },
    }
    engine.RegisterSync(sync)

    // Insert cart with 3 items
    engine.store.Exec(`
        INSERT INTO cart_items (cart_id, item_id, quantity) VALUES
        ('cart-1', 'apple',  2),
        ('cart-1', 'banana', 1),
        ('cart-1', 'carrot', 3)
    `)

    // Trigger completion
    comp := ir.Completion{
        ID:           "comp-1",
        InvocationID: "inv-checkout",
        ActionURI:    "Cart.checkout",
        OutputCase:   "Success",
        Result: ir.IRObject{
            "cart_id": ir.IRString("cart-1"),
        },
        FlowToken: "flow-123",
        Seq:       1,
    }

    // Process completion (fires sync rule)
    err := engine.processCompletion(ctx, comp)
    require.NoError(t, err)

    // Verify 3 invocations generated
    invocations, err := engine.store.ReadInvocationsByFlow(ctx, "flow-123")
    require.NoError(t, err)
    assert.Len(t, invocations, 3, "Should generate 3 Inventory.reserve invocations")

    // Verify invocations have correct args
    invArgs := []map[string]interface{}{
        {"item": "apple",  "qty": 2},
        {"item": "banana", "qty": 1},
        {"item": "carrot", "qty": 3},
    }

    for i, inv := range invocations {
        assert.Equal(t, ir.ActionRef("Inventory.reserve"), inv.ActionURI)
        assert.Equal(t, ir.IRString(invArgs[i]["item"].(string)), inv.Args["item"])
        assert.Equal(t, ir.IRInt(invArgs[i]["qty"].(int)), inv.Args["qty"])
    }

    // Verify 3 sync firings recorded
    firings, err := engine.store.ReadFiringsByCompletion(ctx, comp.ID)
    require.NoError(t, err)
    assert.Len(t, firings, 3, "Should record 3 separate firings")

    // Verify each firing has different binding_hash
    hashes := make(map[string]bool)
    for _, firing := range firings {
        assert.False(t, hashes[firing.BindingHash], "Binding hashes should be unique")
        hashes[firing.BindingHash] = true
    }
}
```

### File List

Files to create:

1. `internal/engine/execute_where.go` - executeWhere, buildQuery, scanBinding, mergeBindings functions
2. `internal/engine/execute_where_test.go` - Comprehensive tests

Files to modify:

1. `internal/engine/engine.go` - Update processCompletion to call executeWhere and loop over bindings
2. `internal/ir/types.go` - Ensure WhereClause struct exists (should from Story 4.1)

Files to reference (must exist from previous stories):

1. `internal/ir/value.go` - IRValue types (Story 1.2)
2. `internal/queryir/query.go` - QueryIR types (Story 4.1)
3. `internal/querysql/compile.go` - SQL compiler (Story 4.3)
4. `internal/store/store.go` - Query, WriteInvocation, WriteSyncFiring functions
5. `internal/engine/matcher.go` - Matcher.Matches, ExtractBindings (Story 3.3)

### Story Completion Checklist

- [ ] executeWhere function implemented and returns []ir.IRObject
- [ ] buildQuery function constructs QueryIR from where-clause
- [ ] scanBinding function extracts SQL row into ir.IRObject
- [ ] mergeBindings function combines when-bindings and where-bindings
- [ ] sqlToIRValue function converts SQL types to IRValue types
- [ ] Test: zero bindings (empty result) passes
- [ ] Test: one binding (single record) passes
- [ ] Test: N bindings (multiple records) passes
- [ ] Test: deterministic ordering across multiple executions passes
- [ ] Test: empty result doesn't error passes
- [ ] Test: scanBinding extracts correct fields passes
- [ ] Integration test: multiple bindings generate multiple invocations passes
- [ ] processCompletion loops over bindings and fires each one
- [ ] Each firing tracked separately in sync_firings table
- [ ] `go vet ./internal/engine/...` passes
- [ ] `go test ./internal/engine/...` passes

### References

- [Source: docs/epics.md#Story 4.4] - Story definition
- [Source: docs/prd.md#FR-2.4] - Execute then-clause to generate invocations from bindings
- [Source: docs/architecture.md#CRITICAL-2] - Deterministic replay requirement
- [Source: docs/architecture.md#CP-4] - Deterministic query ordering
- [Source: Story 4.1] - QueryIR abstraction layer (prerequisite)
- [Source: Story 4.2] - QueryIR builder interface (prerequisite)
- [Source: Story 4.3] - SQL backend compiler (prerequisite)
- [Source: Story 2.5] - Sync firings table with binding_hash (idempotency)
- [Source: Story 3.3] - When-clause matching (prerequisite)
- [Source: Story 3.7] - Flow-scoped sync matching (flow_token filtering)

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Validation History

- Initial creation: 2025-12-12 - Comprehensive story file created from epics.md and architecture.md

### Completion Notes

- **Implements FR-2.4:** Execute then-clause to generate invocations from bindings
- **Critical for sync semantics:** Where-clause produces 0..N bindings, not just 0..1
- **Deterministic ordering per CP-4:** Bindings are ordered sequences, not sets
- **Empty result is valid:** Zero bindings means sync doesn't fire (not an error)
- **Multiple bindings = multiple invocations:** One-to-many relationship
- **Idempotency via binding_hash:** Each binding tracked separately in sync_firings table
- **Replay-safe:** Deterministic order ensures identical replay
- **Integration point:** executeWhere bridges query layer (Epic 4) and sync engine (Epic 3)
- **Type safety:** Returns []ir.IRObject, not []map[string]any
- **Merges when-bindings:** Each where-binding includes variables from when-clause
- **SQL abstraction:** Uses QueryIR compiler from Story 4.3, not raw SQL
