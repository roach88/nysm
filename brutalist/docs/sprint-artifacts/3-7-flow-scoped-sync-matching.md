# Story 3.7: Flow-Scoped Sync Matching

Status: done

## Story

As a **developer defining sync rules**,
I want **syncs to only match records with the same flow token by default**,
So that **concurrent requests don't accidentally join**.

## Acceptance Criteria

1. **ScopeMode enum/type defined in `internal/engine/scope.go`**
   ```go
   type ScopeMode string

   const (
       ScopeModeFlow   ScopeMode = "flow"   // Only same flow_token (default)
       ScopeModeGlobal ScopeMode = "global" // All records regardless of flow
       ScopeModeKeyed  ScopeMode = "keyed"  // Records sharing same value for field
   )
   ```

2. **executeWhereClause adds flow token filter when scope="flow"**
   ```go
   func (e *Engine) executeWhereClause(
       ctx context.Context,
       sync ir.SyncRule,
       flowToken string,
       whenBindings ir.IRObject,
   ) ([]ir.IRObject, error) {
       // Build base query from where-clause
       query := e.compiler.Compile(sync.Where)

       // Apply flow-scoping based on sync rule scope mode
       switch sync.Scope.Mode {
       case "flow":
           // Default: only same flow_token
           query = query.WithFilter(queryir.Equals("flow_token", flowToken))
       case "global":
           // No filter - match across all flows
       case "keyed":
           // Match records with same key value
           keyValue := extractKeyValue(whenBindings, sync.Scope.Key)
           query = query.WithFilter(queryir.Equals(sync.Scope.Key, keyValue))
       }

       // Execute query with applied filters
       return e.queryBackend.Execute(ctx, query)
   }
   ```

3. **Default scope is "flow" (safe by default)**
   - When sync rule omits `scope:` field, default to `{Mode: "flow"}`
   - Requires explicit `scope: "global"` to match across flows
   - Requires explicit `scope: keyed("field")` for keyed matching

4. **"flow" mode filters by flow_token**
   - Given sync rule with `Scope.Mode = "flow"`
   - When executing where-clause query
   - Then add `WHERE flow_token = ?` filter with current flow token
   - Only records from same flow are considered

5. **"global" mode matches all records**
   - Given sync rule with `Scope.Mode = "global"`
   - When executing where-clause query
   - Then NO flow_token filter is added
   - All records are considered regardless of flow

6. **"keyed" mode filters by field value**
   - Given sync rule with `Scope.Mode = "keyed"` and `Scope.Key = "user_id"`
   - When executing where-clause query
   - Then add `WHERE user_id = ?` filter with value from when-bindings
   - Only records with same user_id are considered
   - Flow tokens can differ (allows cross-flow coordination per user)

7. **Comprehensive tests in `internal/engine/scope_test.go`**
   - Test: flow mode filters by flow_token
   - Test: global mode has no flow_token filter
   - Test: keyed mode filters by specified field
   - Test: default scope is "flow"
   - Test: missing key value in keyed mode returns error
   - Test: invalid scope mode returns error

## Quick Reference

| Pattern | Key Rule |
|---------|----------|
| **FR-3.3** | Enforce sync rules only match same flow token |
| **HIGH-1** | Flow token scoping modes (flow/global/keyed) |
| **CP-4** | Deterministic query ordering (flow scope preserves) |

## Tasks / Subtasks

- [ ] Task 1: Define scope mode types (AC: #1)
  - [ ] 1.1 Create `internal/engine/scope.go`
  - [ ] 1.2 Define ScopeMode string type
  - [ ] 1.3 Define constants: ScopeModeFlow, ScopeModeGlobal, ScopeModeKeyed
  - [ ] 1.4 Add validation function: `validateScopeMode(mode string) error`

- [ ] Task 2: Implement flow-scoped query modification (AC: #2, #4)
  - [ ] 2.1 Update `executeWhereClause` signature to accept flowToken
  - [ ] 2.2 Add switch statement for scope.Mode
  - [ ] 2.3 For "flow": add flow_token equality filter
  - [ ] 2.4 For "global": skip flow_token filter
  - [ ] 2.5 For "keyed": extract key value and add key filter

- [ ] Task 3: Implement default scope handling (AC: #3)
  - [ ] 3.1 Add `DefaultScope()` function returning `{Mode: "flow"}`
  - [ ] 3.2 Update sync rule compilation to use default when omitted
  - [ ] 3.3 Verify parser sets default scope in Story 1.7 integration

- [ ] Task 4: Implement keyed mode logic (AC: #6)
  - [ ] 4.1 Implement `extractKeyValue(bindings ir.IRObject, key string) (ir.IRValue, error)`
  - [ ] 4.2 Look up key in when-bindings
  - [ ] 4.3 Return error if key not found in bindings
  - [ ] 4.4 Return IRValue for use in query filter

- [ ] Task 5: Write comprehensive tests (AC: #7)
  - [ ] 5.1 Create `internal/engine/scope_test.go`
  - [ ] 5.2 Test flow mode adds flow_token filter
  - [ ] 5.3 Test global mode has no filter
  - [ ] 5.4 Test keyed mode adds key filter
  - [ ] 5.5 Test default scope is "flow"
  - [ ] 5.6 Test missing key in keyed mode errors
  - [ ] 5.7 Test invalid scope mode errors
  - [ ] 5.8 Test multiple concurrent flows don't cross-contaminate

- [ ] Task 6: Integration with engine loop (AC: #2)
  - [ ] 6.1 Update `processCompletion` to pass flowToken to executeWhereClause
  - [ ] 6.2 Extract flowToken from completion record
  - [ ] 6.3 Verify flow isolation in integration test
  - [ ] 6.4 Document scope semantics in engine godoc

## Dev Notes

### Critical Implementation Details

**Scope Mode Semantics**

The three scoping modes provide different isolation guarantees:

1. **"flow" (default)** - Request isolation
   - Each external request gets unique flow token
   - All invocations/completions within request share same token
   - Syncs only fire on records from same originating request
   - Prevents accidental cross-request joins
   - Use case: Normal transactional workflows (cart checkout, order processing)

2. **"global"** - Cross-request coordination
   - No flow token filtering
   - Syncs can match ANY record in the database
   - Enables system-wide coordination
   - Use case: Global inventory limits, rate limiting, deduplication

3. **"keyed(field)"** - Grouped coordination
   - Flows grouped by field value (e.g., user_id, session_id)
   - Syncs match records with same key value regardless of flow token
   - Enables per-entity coordination across multiple requests
   - Use case: Per-user rate limits, session tracking, user quotas

**Why Default to "flow"?**

Safety and correctness:
- **Least privilege:** Most restrictive mode by default
- **Prevents bugs:** Accidental cross-flow joins are common bug source
- **Explicit opt-in:** Developers must consciously choose global/keyed
- **Matches mental model:** Each request is isolated transaction-like unit

**Implementation Strategy**

The scope filter is applied BEFORE the where-clause query execution:

```
1. Parse sync rule when-clause → extract bindings
2. Match completion against when-clause → success
3. Build where-clause query from sync rule
4. Apply scope filter based on sync.Scope.Mode:
   - flow: Add flow_token = current_flow filter
   - global: No additional filter
   - keyed: Add key_field = bound_key_value filter
5. Execute modified query → get result rows
6. Extract bindings from each result row
7. Fire sync rule for each binding set
```

### Function Signatures

**Scope Mode Type**

```go
// internal/engine/scope.go
package engine

import (
    "fmt"

    "github.com/tyler/nysm/internal/ir"
)

// ScopeMode defines how sync rules match records across flows
type ScopeMode string

const (
    // ScopeModeFlow matches only records with same flow_token (default)
    ScopeModeFlow ScopeMode = "flow"

    // ScopeModeGlobal matches all records regardless of flow_token
    ScopeModeGlobal ScopeMode = "global"

    // ScopeModeKeyed matches records sharing same value for specified field
    ScopeModeKeyed ScopeMode = "keyed"
)

// ValidateScopeMode checks if mode is valid
func ValidateScopeMode(mode string) error {
    switch ScopeMode(mode) {
    case ScopeModeFlow, ScopeModeGlobal, ScopeModeKeyed:
        return nil
    default:
        return fmt.Errorf("invalid scope mode %q: must be flow, global, or keyed", mode)
    }
}

// DefaultScope returns the default scope spec (flow mode)
func DefaultScope() ir.ScopeSpec {
    return ir.ScopeSpec{
        Mode: string(ScopeModeFlow),
    }
}
```

**executeWhereClause with Scope Handling**

```go
// internal/engine/engine.go
package engine

import (
    "context"
    "fmt"

    "github.com/tyler/nysm/internal/ir"
    "github.com/tyler/nysm/internal/queryir"
)

// executeWhereClause executes a where-clause query with scope filtering.
// Returns a slice of binding sets (one per matching record).
func (e *Engine) executeWhereClause(
    ctx context.Context,
    sync ir.SyncRule,
    flowToken string,
    whenBindings ir.IRObject,
) ([]ir.IRObject, error) {
    // If no where-clause, return single binding set (when-bindings only)
    if sync.Where == nil {
        return []ir.IRObject{whenBindings}, nil
    }

    // Build base query from where-clause
    query, err := e.queryCompiler.Compile(*sync.Where)
    if err != nil {
        return nil, fmt.Errorf("compile where-clause: %w", err)
    }

    // Apply flow-scoping based on sync rule scope mode
    switch ScopeMode(sync.Scope.Mode) {
    case ScopeModeFlow:
        // Default: only same flow_token
        query = query.WithFilter(queryir.Equals("flow_token", ir.IRString(flowToken)))

    case ScopeModeGlobal:
        // No filter - match across all flows
        // Intentionally empty - global scope has no restrictions

    case ScopeModeKeyed:
        // Match records with same key value
        if sync.Scope.Key == "" {
            return nil, fmt.Errorf("keyed scope requires non-empty key field")
        }

        keyValue, err := extractKeyValue(whenBindings, sync.Scope.Key)
        if err != nil {
            return nil, fmt.Errorf("extract key value for keyed scope: %w", err)
        }

        query = query.WithFilter(queryir.Equals(sync.Scope.Key, keyValue))

    default:
        return nil, fmt.Errorf("invalid scope mode %q", sync.Scope.Mode)
    }

    // Execute query with applied filters
    results, err := e.queryBackend.Execute(ctx, query)
    if err != nil {
        return nil, fmt.Errorf("execute query: %w", err)
    }

    // Merge when-bindings with each result row
    bindings := make([]ir.IRObject, len(results))
    for i, result := range results {
        bindings[i] = mergeBindings(whenBindings, result)
    }

    return bindings, nil
}

// extractKeyValue extracts the key field value from bindings.
// Returns error if key not found (required for keyed scope).
func extractKeyValue(bindings ir.IRObject, key string) (ir.IRValue, error) {
    value, exists := bindings[key]
    if !exists {
        return nil, fmt.Errorf("key field %q not found in when-bindings (required for keyed scope)", key)
    }

    return value, nil
}

// mergeBindings combines when-bindings and where-bindings.
// Where-bindings take precedence in case of conflicts.
func mergeBindings(whenBindings, whereBindings ir.IRObject) ir.IRObject {
    merged := make(ir.IRObject, len(whenBindings)+len(whereBindings))

    // Copy when-bindings first
    for k, v := range whenBindings {
        merged[k] = v
    }

    // Where-bindings override (if any conflicts)
    for k, v := range whereBindings {
        merged[k] = v
    }

    return merged
}
```

### Query Modification Examples

**Example 1: Flow Mode (Default)**

```go
// Sync rule
sync := ir.SyncRule{
    ID: "cart-to-inventory",
    Scope: ir.ScopeSpec{
        Mode: "flow", // Default
    },
    Where: &ir.WhereClause{
        Source: "Cart.items",
        Filter: "item_id == bound.item_id",
    },
}

// Before scope filter
query = SELECT * FROM cart_items WHERE item_id = ?

// After scope filter (flow mode)
query = SELECT * FROM cart_items
        WHERE item_id = ?
        AND flow_token = 'flow-abc-123'

// Result: Only cart items from SAME flow as triggering completion
```

**Example 2: Global Mode**

```go
// Sync rule for global inventory check
sync := ir.SyncRule{
    ID: "global-inventory-check",
    Scope: ir.ScopeSpec{
        Mode: "global", // Explicit opt-in
    },
    Where: &ir.WhereClause{
        Source: "Inventory.items",
        Filter: "item_id == bound.item_id",
    },
}

// Before scope filter
query = SELECT * FROM inventory_items WHERE item_id = ?

// After scope filter (global mode)
query = SELECT * FROM inventory_items WHERE item_id = ?
// No flow_token filter added!

// Result: All inventory items across ALL flows
```

**Example 3: Keyed Mode (Per-User)**

```go
// Sync rule for per-user rate limiting
sync := ir.SyncRule{
    ID: "user-rate-limit",
    Scope: ir.ScopeSpec{
        Mode: "keyed",
        Key:  "user_id", // Group by user_id
    },
    Where: &ir.WhereClause{
        Source: "API.requests",
        Filter: "timestamp > bound.window_start",
    },
}

// When-bindings contain: {"user_id": "user-456", "window_start": 1700000000}

// Before scope filter
query = SELECT * FROM api_requests WHERE timestamp > ?

// After scope filter (keyed mode)
query = SELECT * FROM api_requests
        WHERE timestamp > ?
        AND user_id = 'user-456'

// Result: All API requests from SAME user across ALL flows
```

### Test Examples

**Test: Flow Mode Filters by Flow Token**

```go
func TestExecuteWhereClause_FlowMode(t *testing.T) {
    engine := setupTestEngine(t)

    // Create sync rule with flow scope (default)
    sync := ir.SyncRule{
        ID: "test-sync",
        Scope: ir.ScopeSpec{
            Mode: "flow",
        },
        Where: &ir.WhereClause{
            Source: "Cart.items",
            Filter: "status == 'pending'",
        },
    }

    // Set up test data with two different flows
    flow1 := "flow-aaa"
    flow2 := "flow-bbb"

    // Insert records for flow1
    engine.store.WriteState(ctx, "Cart.items", ir.IRObject{
        "item_id":    ir.IRString("item-1"),
        "status":     ir.IRString("pending"),
        "flow_token": ir.IRString(flow1),
    })

    // Insert records for flow2
    engine.store.WriteState(ctx, "Cart.items", ir.IRObject{
        "item_id":    ir.IRString("item-2"),
        "status":     ir.IRString("pending"),
        "flow_token": ir.IRString(flow2),
    })

    // Execute where-clause for flow1
    results, err := engine.executeWhereClause(
        ctx,
        sync,
        flow1, // Current flow token
        ir.IRObject{}, // Empty when-bindings
    )

    require.NoError(t, err)
    assert.Len(t, results, 1, "should only match flow1 records")

    // Verify only item-1 matched (from flow1)
    itemID := results[0]["item_id"].(ir.IRString)
    assert.Equal(t, ir.IRString("item-1"), itemID)
}
```

**Test: Global Mode Matches All Flows**

```go
func TestExecuteWhereClause_GlobalMode(t *testing.T) {
    engine := setupTestEngine(t)

    // Create sync rule with global scope
    sync := ir.SyncRule{
        ID: "global-sync",
        Scope: ir.ScopeSpec{
            Mode: "global",
        },
        Where: &ir.WhereClause{
            Source: "Inventory.items",
            Filter: "quantity < 10",
        },
    }

    // Insert records for different flows
    engine.store.WriteState(ctx, "Inventory.items", ir.IRObject{
        "item_id":    ir.IRString("item-1"),
        "quantity":   ir.IRInt(5),
        "flow_token": ir.IRString("flow-aaa"),
    })

    engine.store.WriteState(ctx, "Inventory.items", ir.IRObject{
        "item_id":    ir.IRString("item-2"),
        "quantity":   ir.IRInt(3),
        "flow_token": ir.IRString("flow-bbb"),
    })

    // Execute where-clause (flow token should be ignored)
    results, err := engine.executeWhereClause(
        ctx,
        sync,
        "flow-aaa", // Current flow token
        ir.IRObject{},
    )

    require.NoError(t, err)
    assert.Len(t, results, 2, "should match records from ALL flows")

    // Verify both items matched
    itemIDs := []string{
        string(results[0]["item_id"].(ir.IRString)),
        string(results[1]["item_id"].(ir.IRString)),
    }
    assert.ElementsMatch(t, []string{"item-1", "item-2"}, itemIDs)
}
```

**Test: Keyed Mode Filters by Field**

```go
func TestExecuteWhereClause_KeyedMode(t *testing.T) {
    engine := setupTestEngine(t)

    // Create sync rule with keyed scope
    sync := ir.SyncRule{
        ID: "user-sync",
        Scope: ir.ScopeSpec{
            Mode: "keyed",
            Key:  "user_id",
        },
        Where: &ir.WhereClause{
            Source: "API.requests",
            Filter: "method == 'POST'",
        },
    }

    // Insert records for user-123 (different flows)
    engine.store.WriteState(ctx, "API.requests", ir.IRObject{
        "request_id": ir.IRString("req-1"),
        "user_id":    ir.IRString("user-123"),
        "method":     ir.IRString("POST"),
        "flow_token": ir.IRString("flow-aaa"),
    })

    engine.store.WriteState(ctx, "API.requests", ir.IRObject{
        "request_id": ir.IRString("req-2"),
        "user_id":    ir.IRString("user-123"),
        "method":     ir.IRString("POST"),
        "flow_token": ir.IRString("flow-bbb"),
    })

    // Insert record for user-456 (different user)
    engine.store.WriteState(ctx, "API.requests", ir.IRObject{
        "request_id": ir.IRString("req-3"),
        "user_id":    ir.IRString("user-456"),
        "method":     ir.IRString("POST"),
        "flow_token": ir.IRString("flow-ccc"),
    })

    // When-bindings contain user_id
    whenBindings := ir.IRObject{
        "user_id": ir.IRString("user-123"),
    }

    // Execute where-clause (should match user-123 from all flows)
    results, err := engine.executeWhereClause(
        ctx,
        sync,
        "flow-aaa", // Current flow token (ignored in keyed mode)
        whenBindings,
    )

    require.NoError(t, err)
    assert.Len(t, results, 2, "should match both user-123 requests across flows")

    // Verify only user-123 requests matched
    for _, result := range results {
        userID := result["user_id"].(ir.IRString)
        assert.Equal(t, ir.IRString("user-123"), userID)
    }
}
```

**Test: Default Scope is Flow**

```go
func TestDefaultScope_IsFlow(t *testing.T) {
    defaultScope := DefaultScope()

    assert.Equal(t, "flow", defaultScope.Mode)
    assert.Empty(t, defaultScope.Key)
}
```

**Test: Missing Key in Keyed Mode Errors**

```go
func TestExecuteWhereClause_KeyedMode_MissingKey(t *testing.T) {
    engine := setupTestEngine(t)

    // Create sync rule with keyed scope
    sync := ir.SyncRule{
        ID: "user-sync",
        Scope: ir.ScopeSpec{
            Mode: "keyed",
            Key:  "user_id", // Required key
        },
        Where: &ir.WhereClause{
            Source: "API.requests",
            Filter: "method == 'POST'",
        },
    }

    // When-bindings MISSING user_id
    whenBindings := ir.IRObject{
        "request_id": ir.IRString("req-1"),
        // user_id is missing!
    }

    // Execute where-clause
    results, err := engine.executeWhereClause(
        ctx,
        sync,
        "flow-aaa",
        whenBindings,
    )

    require.Error(t, err)
    assert.Nil(t, results)
    assert.Contains(t, err.Error(), "user_id")
    assert.Contains(t, err.Error(), "not found")
}
```

**Test: Invalid Scope Mode Errors**

```go
func TestValidateScopeMode_Invalid(t *testing.T) {
    testCases := []string{
        "invalid",
        "FLOW", // Case-sensitive
        "keyed(user_id)", // Not just "keyed"
        "",
    }

    for _, mode := range testCases {
        t.Run(mode, func(t *testing.T) {
            err := ValidateScopeMode(mode)
            require.Error(t, err)
            assert.Contains(t, err.Error(), "invalid scope mode")
        })
    }
}
```

**Test: Multiple Concurrent Flows Don't Cross-Contaminate**

```go
func TestExecuteWhereClause_FlowIsolation(t *testing.T) {
    engine := setupTestEngine(t)

    sync := ir.SyncRule{
        ID: "test-sync",
        Scope: ir.ScopeSpec{
            Mode: "flow",
        },
        Where: &ir.WhereClause{
            Source: "Orders",
            Filter: "status == 'pending'",
        },
    }

    // Simulate two concurrent flows
    flow1 := "flow-user-alice"
    flow2 := "flow-user-bob"

    // Alice's order
    engine.store.WriteState(ctx, "Orders", ir.IRObject{
        "order_id":   ir.IRString("order-alice"),
        "status":     ir.IRString("pending"),
        "flow_token": ir.IRString(flow1),
    })

    // Bob's order
    engine.store.WriteState(ctx, "Orders", ir.IRObject{
        "order_id":   ir.IRString("order-bob"),
        "status":     ir.IRString("pending"),
        "flow_token": ir.IRString(flow2),
    })

    // Execute where-clause for Alice's flow
    aliceResults, err := engine.executeWhereClause(ctx, sync, flow1, ir.IRObject{})
    require.NoError(t, err)
    assert.Len(t, aliceResults, 1)
    assert.Equal(t, ir.IRString("order-alice"), aliceResults[0]["order_id"])

    // Execute where-clause for Bob's flow
    bobResults, err := engine.executeWhereClause(ctx, sync, flow2, ir.IRObject{})
    require.NoError(t, err)
    assert.Len(t, bobResults, 1)
    assert.Equal(t, ir.IRString("order-bob"), bobResults[0]["order_id"])
}
```

### Integration with Engine Loop

**processCompletion with Flow Token Propagation**

```go
// internal/engine/engine.go

func (e *Engine) processCompletion(ctx context.Context, comp ir.Completion) error {
    // Write completion to event log
    if err := e.store.WriteCompletion(ctx, comp); err != nil {
        return fmt.Errorf("write completion: %w", err)
    }

    // Lookup invocation to get action URI
    inv, err := e.store.GetInvocation(ctx, comp.InvocationID)
    if err != nil {
        return fmt.Errorf("get invocation %s: %w", comp.InvocationID, err)
    }

    // Check all sync rules
    for _, sync := range e.syncs {
        // Check when-clause match
        if !e.matcher.Matches(sync.When, inv, comp) {
            continue
        }

        // Extract when-bindings
        whenBindings, err := e.matcher.ExtractBindings(sync.When, comp)
        if err != nil {
            return fmt.Errorf("extract when-bindings for sync %s: %w", sync.ID, err)
        }

        // Execute where-clause WITH FLOW TOKEN
        // This is where flow-scoping is applied!
        whereBindings, err := e.executeWhereClause(
            ctx,
            sync,
            comp.FlowToken, // Propagate flow token from completion
            whenBindings,
        )
        if err != nil {
            return fmt.Errorf("execute where-clause for sync %s: %w", sync.ID, err)
        }

        // Fire sync for each binding set
        for _, bindings := range whereBindings {
            if err := e.fireSyncRule(ctx, sync, comp, bindings); err != nil {
                return fmt.Errorf("fire sync %s: %w", sync.ID, err)
            }
        }
    }

    return nil
}
```

### CUE Syntax Examples

**Flow Scope (Default)**

```cue
sync "cart-to-inventory" {
    // scope: "flow" is default, can be omitted
    when: Cart.checkout.completed {
        case: "Success"
        bind: { order_id: result.order_id }
    }
    where: Cart.items[order_id == bound.order_id]
    then: Inventory.reserve(
        item_id: bound.item_id,
        quantity: bound.quantity
    )
}
```

**Global Scope (Explicit Opt-In)**

```cue
sync "global-inventory-check" {
    scope: "global"  // Explicit opt-in for cross-flow matching
    when: Inventory.reserve.invoked {
        bind: { item_id: args.item_id, quantity: args.quantity }
    }
    where: Inventory.items[item_id == bound.item_id]
    then: Inventory.checkStock(
        item_id: bound.item_id,
        requested: bound.quantity,
        available: bound.stock
    )
}
```

**Keyed Scope (Per-User Coordination)**

```cue
sync "user-rate-limit" {
    scope: keyed("user_id")  // Group by user_id across flows
    when: API.request.completed {
        bind: { user_id: args.user_id, timestamp: result.timestamp }
    }
    where: API.requests[
        user_id == bound.user_id &&
        timestamp > bound.window_start
    ]
    then: RateLimit.check(
        user_id: bound.user_id,
        request_count: count(bound.requests)
    )
}
```

### Scope Mode Decision Table

| Use Case | Scope Mode | Rationale |
|----------|------------|-----------|
| Normal transactional workflow | `flow` | Each request isolated, no cross-contamination |
| Cart checkout → inventory | `flow` | Only reserve inventory for THIS cart |
| Order processing pipeline | `flow` | All steps belong to same request |
| Global inventory limits | `global` | Check total inventory across all orders |
| System-wide rate limiting | `global` | Count ALL requests system-wide |
| Deduplication across users | `global` | Find duplicate records regardless of flow |
| Per-user rate limiting | `keyed("user_id")` | Count requests per user across sessions |
| Session-based tracking | `keyed("session_id")` | Track state per session |
| Tenant-based isolation | `keyed("tenant_id")` | Isolate data per tenant |

### File List

Files to create:

1. `internal/engine/scope.go` - Scope mode types and validation
2. `internal/engine/scope_test.go` - Comprehensive tests

Files to modify:

1. `internal/engine/engine.go` - Update executeWhereClause signature and implementation
2. `internal/ir/sync.go` - Ensure ScopeSpec struct exists (should from Story 1.7)

Files to reference (must exist from previous stories):

1. `internal/ir/sync.go` - SyncRule, ScopeSpec types (Story 1.7)
2. `internal/ir/value.go` - IRValue types (Story 1.2)
3. `internal/queryir/query.go` - QueryIR types (Story 4.1)
4. `internal/engine/matcher.go` - Matcher implementation (Story 3.3)

### Story Completion Checklist

- [ ] ScopeMode type defined (flow/global/keyed)
- [ ] ValidateScopeMode function implemented
- [ ] DefaultScope function returns "flow" mode
- [ ] executeWhereClause accepts flowToken parameter
- [ ] Flow mode adds flow_token filter
- [ ] Global mode has no filter
- [ ] Keyed mode adds key field filter
- [ ] extractKeyValue function implemented
- [ ] mergeBindings function implemented
- [ ] Test: flow mode filters by flow_token passes
- [ ] Test: global mode matches all flows passes
- [ ] Test: keyed mode filters by field passes
- [ ] Test: default scope is "flow" passes
- [ ] Test: missing key in keyed mode errors passes
- [ ] Test: invalid scope mode errors passes
- [ ] Test: flow isolation with concurrent flows passes
- [ ] processCompletion passes flowToken to executeWhereClause
- [ ] `go vet ./internal/engine/...` passes
- [ ] `go test ./internal/engine/...` passes

### References

- [Source: docs/epics.md#Story 3.7] - Story definition
- [Source: docs/prd.md#FR-3.3] - Enforce sync rules only match same flow token
- [Source: docs/architecture.md#HIGH-1] - Flow token scoping modes
- [Source: Story 1.7] - CUE sync rule parser with ScopeSpec
- [Source: Story 2.2] - Event log schema with flow_token field
- [Source: Story 3.3] - When-clause matching foundation
- [Source: Story 3.6] - Invoked event matching (prerequisite)

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Validation History

- Initial creation: comprehensive story generation from epics.md and architecture.md

### Completion Notes

- **Flow scope is default:** Safe by default - prevents accidental cross-flow joins
- **Global scope requires opt-in:** Developers must explicitly choose `scope: "global"`
- **Keyed scope enables per-entity coordination:** Allows cross-flow matching within entity boundaries
- **Scope filter applied BEFORE where-clause:** Flow scoping happens at query construction time
- **extractKeyValue validates key existence:** Missing key in keyed mode is hard error
- **mergeBindings preserves when-bindings:** Where-bindings supplement, don't replace
- **Flow token propagates from completion:** Engine passes comp.FlowToken to executeWhereClause
- **Deterministic query ordering preserved:** Flow scope doesn't interfere with CP-4
- **Security context orthogonal:** Flow scoping and security context are independent concerns
- **Integration with Story 3.6:** Assumes invoked event matching is working (prerequisite)
- **Integration with Story 4.x:** Uses QueryIR abstraction from Epic 4 (query layer)
- **No transactions needed:** Flow isolation provides consistency without distributed transactions
