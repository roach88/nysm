# Story 7.9: Demo Sync Rules

Status: ready-for-dev

## Story

As a **developer learning NYSM**,
I want **canonical sync rules demonstrating the 3-clause pattern**,
So that **I understand how concepts coordinate**.

## Acceptance Criteria

1. **specs/cart-inventory.sync.cue demonstrates 3-clause coordination**
   - Contains at least 2 sync rules showing different patterns
   - Uses when → where → then structure
   - Demonstrates multi-binding pattern (where returns multiple results)
   - Shows error handling with output case matching

2. **cart-inventory-reserve sync rule (happy path)**
   ```cue
   sync "cart-inventory-reserve" {
       scope: "flow"

       when: {
           action: "Cart.checkout"
           event: "completed"
           case: "Success"
           bind: { cart_id: "result.cart_id" }
       }

       where: {
           from: "CartItem"
           filter: "flow_token == bound.flow_token"
           bind: { item_id: "item_id", quantity: "quantity" }
       }

       then: {
           action: "Inventory.reserve"
           args: {
               item_id: "bound.item_id"
               quantity: "bound.quantity"
           }
       }
   }
   ```

3. **handle-insufficient-stock sync rule (error handling)**
   ```cue
   sync "handle-insufficient-stock" {
       scope: "flow"

       when: {
           action: "Inventory.reserve"
           event: "completed"
           case: "InsufficientStock"
           bind: {
               item: "result.item"
               available: "result.available"
               requested: "result.requested"
           }
       }

       where: {}  // No query needed

       then: {
           action: "Cart.checkout.fail"
           args: {
               reason: "insufficient_stock"
               details: "bound.item"
           }
       }
   }
   ```

4. **File compiles without errors**
   - `nysm compile ./specs` succeeds
   - `nysm validate ./specs` passes

5. **Documentation comments explain patterns**
   - Each sync rule has a comment explaining its purpose
   - Multi-binding pattern is explicitly documented
   - Error matching pattern is called out

6. **Demonstrates CRITICAL-1 pattern**
   - cart-inventory-reserve produces multiple invocations (one per cart item)
   - Each binding produces separate sync firing

7. **Demonstrates MEDIUM-2 pattern**
   - handle-insufficient-stock matches specific error variant
   - Error fields extracted into bindings

## Quick Reference

| Pattern | Key Rule |
|---------|----------|
| **CRITICAL-1** | Multi-binding sync: one completion → many invocations |
| **MEDIUM-2** | Error matching in when-clause with output case |
| **FR-2.1** | 3-clause sync rules (when → where → then) |
| **HIGH-1** | Flow scoping by default |

## Tasks / Subtasks

- [ ] Task 1: Create cart-inventory.sync.cue file (AC: #1-3)
  - [ ] 1.1 Create file in specs/ directory
  - [ ] 1.2 Add file header with overview comment
  - [ ] 1.3 Define cart-inventory-reserve sync rule
  - [ ] 1.4 Define handle-insufficient-stock sync rule

- [ ] Task 2: Add documentation comments (AC: #5)
  - [ ] 2.1 Add overview comment explaining sync coordination
  - [ ] 2.2 Document multi-binding pattern
  - [ ] 2.3 Document error matching pattern
  - [ ] 2.4 Explain 3-clause structure

- [ ] Task 3: Validate compilation (AC: #4)
  - [ ] 3.1 Run nysm compile ./specs
  - [ ] 3.2 Run nysm validate ./specs
  - [ ] 3.3 Fix any compilation errors

- [ ] Task 4: Verify pattern demonstrations (AC: #6-7)
  - [ ] 4.1 Confirm multi-binding semantics
  - [ ] 4.2 Confirm error matching semantics
  - [ ] 4.3 Verify flow scoping

## Dev Notes

### Complete cart-inventory.sync.cue File

```cue
// cart-inventory.sync.cue
//
// Synchronization rules coordinating Cart and Inventory concepts.
//
// This file demonstrates the NYSM sync pattern:
//   when (action completions) →
//   where (state queries + bindings) →
//   then (invocations)
//
// Key patterns demonstrated:
// 1. Multi-binding: One checkout → many reserve invocations (one per cart item)
// 2. Error matching: InsufficientStock error → fail checkout
// 3. Flow scoping: All syncs use scope: "flow" (default)

// Happy path: When cart checkout succeeds, reserve inventory for each item
//
// PATTERN: Multi-binding (CRITICAL-1)
// The where clause returns multiple bindings (one per CartItem).
// Each binding produces a separate Inventory.reserve invocation.
// This is how "for each item in cart, reserve inventory" works.
sync "cart-inventory-reserve" {
    // Scope: "flow" means only match records in same flow
    scope: "flow"

    // When: Trigger on successful cart checkout
    when: {
        action: "Cart.checkout"
        event: "completed"
        case: "Success"  // Only fire for success case
        bind: { cart_id: "result.cart_id" }
    }

    // Where: Query all items in the cart
    // This returns a SET of bindings, one per cart item.
    // Each binding gets its own sync firing and invocation.
    where: {
        from: "CartItem"
        filter: "flow_token == bound.flow_token"
        bind: {
            item_id: "item_id"
            quantity: "quantity"
        }
    }

    // Then: For EACH binding, invoke Inventory.reserve
    // If there are 3 items in cart, this fires 3 times.
    then: {
        action: "Inventory.reserve"
        args: {
            item_id: "bound.item_id"
            quantity: "bound.quantity"
        }
    }
}

// Error path: When inventory reserve fails, propagate error to checkout
//
// PATTERN: Error matching (MEDIUM-2)
// The when clause matches a specific error variant (InsufficientStock).
// Error fields are extracted into bindings for use in then clause.
sync "handle-insufficient-stock" {
    scope: "flow"

    // When: Trigger on InsufficientStock error from Inventory.reserve
    when: {
        action: "Inventory.reserve"
        event: "completed"
        case: "InsufficientStock"  // Match specific error variant
        bind: {
            item: "result.item"
            available: "result.available"
            requested: "result.requested"
        }
    }

    // Where: No query needed - error bindings from when clause are sufficient
    where: {}

    // Then: Fail the checkout with error details
    then: {
        action: "Cart.checkout.fail"
        args: {
            reason: "insufficient_stock"
            details: "bound.item"
            available: "bound.available"
            requested: "bound.requested"
        }
    }
}

// Optional: Release inventory on cart abandonment
//
// PATTERN: 3-clause coordination with cleanup
// Demonstrates synchronization in the opposite direction.
sync "cart-abandon-release" {
    scope: "flow"

    // When: Cart is abandoned (user cancels, timeout, etc.)
    when: {
        action: "Cart.abandon"
        event: "completed"
        case: "Success"
        bind: { cart_id: "result.cart_id" }
    }

    // Where: Find all reserved inventory for this cart
    where: {
        from: "InventoryReservation"
        filter: "cart_id == bound.cart_id"
        bind: {
            item_id: "item_id"
            quantity: "quantity"
        }
    }

    // Then: Release each reservation
    then: {
        action: "Inventory.release"
        args: {
            item_id: "bound.item_id"
            quantity: "bound.quantity"
        }
    }
}
```

### Pattern Documentation

#### Multi-Binding Pattern (CRITICAL-1)

The `cart-inventory-reserve` sync demonstrates the multi-binding pattern:

1. **When:** One completion (Cart.checkout)
2. **Where:** Query returns N bindings (one per cart item)
3. **Then:** N invocations generated (one Inventory.reserve per item)

**Idempotency:** Each binding gets unique hash, tracked in sync_firings:
```sql
UNIQUE(completion_id, sync_id, binding_hash)
```

**Example:**
- Cart has 3 items: widget (qty 2), gadget (qty 1), doodad (qty 5)
- Cart.checkout completes
- Where clause returns 3 bindings
- Engine generates 3 Inventory.reserve invocations
- 3 sync_firings records created (one per binding)

#### Error Matching Pattern (MEDIUM-2)

The `handle-insufficient-stock` sync demonstrates error matching:

1. **When clause matches output case:** `case: "InsufficientStock"`
2. **Error fields extracted:** `result.item`, `result.available`, `result.requested`
3. **Bindings available in then:** Use `bound.item`, etc.

**Why no transactions?**
- Errors are explicit, typed action outputs
- Syncs match on error cases and propagate/handle them
- No rollback needed - errors handled via coordination

#### 3-Clause Structure

Every sync rule has:

```
when:   Action completion pattern (trigger)
  ├─ action:     Which action to watch
  ├─ event:      "completed" or "invoked"
  ├─ case:       Output case to match (optional)
  └─ bind:       Extract values from result

where:  State query + bindings (optional)
  ├─ from:       State table to query
  ├─ filter:     SQL WHERE clause expression
  └─ bind:       Extract values from query results

then:   Invocation to generate
  ├─ action:     Which action to invoke
  └─ args:       Arguments using bound variables
```

### Compilation Validation

```bash
# Validate sync rules compile correctly
$ nysm compile ./specs
✓ Compiled 3 concepts, 3 syncs

Concepts:
  Cart: 3 actions, 1 operational principle
  Inventory: 2 actions, 1 operational principle
  Web: 2 actions

Syncs:
  cart-inventory-reserve: Cart.checkout → Inventory.reserve (multi-binding)
  handle-insufficient-stock: Inventory.reserve[InsufficientStock] → Cart.checkout.fail
  cart-abandon-release: Cart.abandon → Inventory.release (multi-binding)

# Validate against schema
$ nysm validate ./specs
✓ All specs valid
```

### Test Examples

#### Unit Test: Verify Sync Compilation

```go
func TestCartInventorySyncCompilation(t *testing.T) {
    ctx := cuecontext.New()

    // Load cart-inventory.sync.cue
    v := ctx.CompileFile("specs/cart-inventory.sync.cue")
    require.NoError(t, v.Err())

    // Parse cart-inventory-reserve sync
    reserveSync := v.LookupPath(cue.ParsePath(`sync."cart-inventory-reserve"`))
    rule, err := compiler.CompileSync(reserveSync)

    require.NoError(t, err)
    assert.Equal(t, "cart-inventory-reserve", rule.ID)
    assert.Equal(t, "flow", rule.Scope.Mode)

    // Verify when clause
    assert.Equal(t, "Cart.checkout", rule.When.ActionRef)
    assert.Equal(t, "completed", rule.When.EventType)
    assert.Equal(t, "Success", rule.When.OutputCase)

    // Verify where clause exists
    assert.NotNil(t, rule.Where)
    assert.Equal(t, "CartItem", rule.Where.Source)

    // Verify then clause
    assert.Equal(t, "Inventory.reserve", rule.Then.ActionRef)
    assert.Contains(t, rule.Then.Args, "item_id")
    assert.Contains(t, rule.Then.Args, "quantity")
}

func TestHandleInsufficientStockCompilation(t *testing.T) {
    ctx := cuecontext.New()

    // Load cart-inventory.sync.cue
    v := ctx.CompileFile("specs/cart-inventory.sync.cue")
    require.NoError(t, v.Err())

    // Parse handle-insufficient-stock sync
    errorSync := v.LookupPath(cue.ParsePath(`sync."handle-insufficient-stock"`))
    rule, err := compiler.CompileSync(errorSync)

    require.NoError(t, err)

    // Verify error matching (MEDIUM-2)
    assert.Equal(t, "Inventory.reserve", rule.When.ActionRef)
    assert.Equal(t, "InsufficientStock", rule.When.OutputCase)

    // Verify error field bindings
    assert.Contains(t, rule.When.Bindings, "item")
    assert.Contains(t, rule.When.Bindings, "available")
    assert.Contains(t, rule.When.Bindings, "requested")

    // Verify no where clause (error bindings sufficient)
    assert.Nil(t, rule.Where)
}
```

#### Integration Test: Multi-Binding Execution

```go
func TestCartInventoryMultiBinding(t *testing.T) {
    // Setup: Create test database with 3 cart items
    store := testutil.NewTestStore(t)
    engine := testutil.NewTestEngine(t, store)

    // Load specs
    specs := loadSpecs(t, "specs/cart.concept.cue", "specs/inventory.concept.cue")
    syncs := loadSpecs(t, "specs/cart-inventory.sync.cue")
    engine.RegisterSpecs(specs)
    engine.RegisterSyncs(syncs)

    // Create flow with fixed token for determinism
    flowToken := "test-flow-123"

    // Add 3 items to cart (creates CartItem state records)
    engine.Invoke(flowToken, "Cart.addItem", map[string]any{
        "item_id": "widget",
        "quantity": 2,
    })
    engine.Invoke(flowToken, "Cart.addItem", map[string]any{
        "item_id": "gadget",
        "quantity": 1,
    })
    engine.Invoke(flowToken, "Cart.addItem", map[string]any{
        "item_id": "doodad",
        "quantity": 5,
    })

    // Invoke checkout - should trigger sync
    result := engine.Invoke(flowToken, "Cart.checkout", map[string]any{})
    require.Equal(t, "Success", result.Case)

    // Verify multi-binding: 3 Inventory.reserve invocations generated
    trace := store.ReadFlow(flowToken)
    reserveInvocations := filterByAction(trace, "Inventory.reserve")

    assert.Len(t, reserveInvocations, 3, "Should generate 3 reserve invocations")

    // Verify each item reserved
    assertInvocationExists(t, reserveInvocations, "widget", 2)
    assertInvocationExists(t, reserveInvocations, "gadget", 1)
    assertInvocationExists(t, reserveInvocations, "doodad", 5)

    // Verify provenance: Each reserve invocation has edge back to checkout completion
    checkout := findCompletion(trace, "Cart.checkout")
    for _, inv := range reserveInvocations {
        edges := store.ReadProvenance(inv.ID)
        require.Len(t, edges, 1)
        assert.Equal(t, checkout.ID, edges[0].CompletionID)
        assert.Equal(t, "cart-inventory-reserve", edges[0].SyncID)
    }
}
```

#### Integration Test: Error Matching

```go
func TestCartInventoryErrorMatching(t *testing.T) {
    store := testutil.NewTestStore(t)
    engine := testutil.NewTestEngine(t, store)

    // Load specs
    specs := loadSpecs(t, "specs/cart.concept.cue", "specs/inventory.concept.cue")
    syncs := loadSpecs(t, "specs/cart-inventory.sync.cue")
    engine.RegisterSpecs(specs)
    engine.RegisterSyncs(syncs)

    flowToken := "test-flow-456"

    // Setup: Inventory has only 2 units of widget
    engine.Invoke(flowToken, "Inventory.setStock", map[string]any{
        "item_id": "widget",
        "quantity": 2,
    })

    // Add 5 units to cart (more than available)
    engine.Invoke(flowToken, "Cart.addItem", map[string]any{
        "item_id": "widget",
        "quantity": 5,
    })

    // Checkout triggers reserve, which fails with InsufficientStock
    result := engine.Invoke(flowToken, "Cart.checkout", map[string]any{})

    // Verify error propagation via sync
    trace := store.ReadFlow(flowToken)

    // 1. Cart.checkout completes successfully (triggers sync)
    checkout := findCompletion(trace, "Cart.checkout")
    assert.Equal(t, "Success", checkout.OutputCase)

    // 2. Inventory.reserve invoked via sync
    reserve := findInvocation(trace, "Inventory.reserve")
    assert.NotNil(t, reserve)

    // 3. Inventory.reserve completes with InsufficientStock error
    reserveCompletion := findCompletion(trace, "Inventory.reserve")
    assert.Equal(t, "InsufficientStock", reserveCompletion.OutputCase)
    assert.Equal(t, "widget", reserveCompletion.Result["item"])
    assert.Equal(t, int64(2), reserveCompletion.Result["available"])
    assert.Equal(t, int64(5), reserveCompletion.Result["requested"])

    // 4. handle-insufficient-stock sync fires, triggering Cart.checkout.fail
    failInvocation := findInvocation(trace, "Cart.checkout.fail")
    assert.NotNil(t, failInvocation)
    assert.Equal(t, "insufficient_stock", failInvocation.Args["reason"])

    // Verify provenance: fail invocation triggered by reserve error
    edges := store.ReadProvenance(failInvocation.ID)
    require.Len(t, edges, 1)
    assert.Equal(t, reserveCompletion.ID, edges[0].CompletionID)
    assert.Equal(t, "handle-insufficient-stock", edges[0].SyncID)
}
```

### File List

Files to create/modify:

1. `specs/cart-inventory.sync.cue` - Demo sync rules
2. `specs/cart.concept.cue` - May need Cart.checkout.fail action
3. `specs/inventory.concept.cue` - May need InsufficientStock error case

### Relationship to Other Stories

- **Story 7.8:** Demo concept specs that these syncs coordinate
- **Story 1.7:** SyncRule parser validates these specs
- **Story 3.3-3.4:** Engine matches when-clauses with output cases
- **Story 4.4-4.5:** Engine executes where-clauses and bindings

### Story Completion Checklist

- [ ] cart-inventory.sync.cue file created
- [ ] cart-inventory-reserve sync rule defined
- [ ] handle-insufficient-stock sync rule defined
- [ ] cart-abandon-release sync rule defined (optional)
- [ ] Documentation comments explain patterns
- [ ] Multi-binding pattern demonstrated (CRITICAL-1)
- [ ] Error matching pattern demonstrated (MEDIUM-2)
- [ ] File compiles without errors
- [ ] File validates against schema
- [ ] All tests pass
- [ ] `go vet ./internal/...` passes

### References

- [Source: docs/epics.md#Story 7.9] - Story definition
- [Source: docs/architecture.md#Sync Rules] - Sync rule architecture
- [Source: docs/prd.md#FR-2.1] - 3-clause sync rules
- [Source: docs/prd.md#Appendix A] - Canonical demo scenarios
- [Source: docs/sprint-artifacts/1-7-cue-sync-rule-parser.md] - SyncRule IR representation

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Validation History

- Initial creation: create-story workflow
- Pattern validation: CRITICAL-1 (multi-binding), MEDIUM-2 (error matching)

### Completion Notes

- Three sync rules demonstrate key patterns:
  1. cart-inventory-reserve: Multi-binding (1 → many)
  2. handle-insufficient-stock: Error matching with typed error fields
  3. cart-abandon-release: Cleanup/reverse coordination (optional)
- All sync rules use flow scoping (default)
- Documentation comments explain each pattern
- Integration tests verify end-to-end behavior
