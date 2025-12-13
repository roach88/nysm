# Story 4.5: Then-Clause Invocation Generation

Status: ready-for-dev

## Story

As a **developer building sync execution**,
I want **then-clauses to generate invocations from bindings**,
So that **follow-on actions are triggered correctly**.

## Acceptance Criteria

1. **executeThen function in `internal/engine/executor.go`**
   ```go
   func (e *Engine) executeThen(
       ctx context.Context,
       then ir.ThenClause,
       bindings []ir.IRObject,
       flowToken string,
       completion ir.Completion,
       sync ir.SyncRule,
   ) error {
       // Generates ONE invocation PER binding
       // Implements CP-1 binding-level idempotency
       // Records sync_firing, invocation, provenance_edge
       // Enqueues invocation for execution
   }
   ```
   - Signature exactly as shown (no modifications)
   - Takes bindings slice (from where-clause execution)
   - Processes each binding independently
   - Returns error if any write fails

2. **One invocation generated per binding (FR-2.4)**
   - `for _, binding := range bindings` loop
   - Each iteration generates exactly one invocation
   - Empty bindings slice = no invocations (valid, not error)
   - Multiple bindings = multiple invocations (critical for multi-binding syncs)

3. **Binding hash idempotency check (CP-1)**
   ```go
   bindingHash := ir.BindingHash(binding)
   hasFired, err := e.store.HasFiring(ctx, completion.ID, sync.ID, bindingHash)
   if err != nil {
       return err
   }
   if hasFired {
       continue  // Skip - already processed
   }
   ```
   - Compute hash BEFORE checking (deterministic)
   - HasFiring returns `(bool, error)` - handle both
   - Skip binding if already fired (idempotent replay)
   - Continue to next binding on skip (not return)

4. **resolveArgs argument template substitution**
   ```go
   func (e *Engine) resolveArgs(argTemplates ir.IRObject, bindings ir.IRObject) (ir.IRObject, error) {
       // Substitutes "bound.varName" references with binding values
       // Returns error if binding variable not found
       // Preserves IRValue types
   }
   ```
   - Supports `"bound.varName"` syntax in arg templates
   - Extracts variable name by stripping `"bound."` prefix
   - Looks up variable in bindings IRObject
   - Returns error if variable not found (clear message)
   - Literal values passed through unchanged

5. **Record sync_firing, invocation, provenance_edge**
   ```go
   // 1. Create sync firing record
   firing := ir.SyncFiring{
       CompletionID: completion.ID,
       SyncID:       sync.ID,
       BindingHash:  bindingHash,
       Seq:          e.clock.Next(),
   }
   firingID, err := e.store.WriteSyncFiring(ctx, firing)

   // 2. Create invocation
   inv := ir.Invocation{
       ID:        ir.InvocationID(inv),  // Content-addressed
       FlowToken: flowToken,
       ActionURI: then.Action,
       Args:      resolvedArgs,
       Seq:       e.clock.Next(),
       // ... other required fields
   }
   err = e.store.WriteInvocation(ctx, inv)

   // 3. Link firing → invocation
   edge := ir.ProvenanceEdge{
       SyncFiringID: firingID,
       InvocationID: inv.ID,
   }
   err = e.store.WriteProvenanceEdge(ctx, edge)
   ```
   - Write order: firing → invocation → provenance_edge
   - Each write must succeed before next (atomic per binding)
   - Return error if any write fails
   - Clock increments for each record (seq ordering)

6. **Enqueue invocation for execution**
   ```go
   e.queue.Enqueue(Event{
       Type: EventTypeInvocation,
       Data: &inv,
   })
   ```
   - After all writes succeed
   - Before moving to next binding
   - Queue takes ownership of invocation

7. **Comprehensive tests in `internal/engine/executor_test.go`**
   - Test: single binding generates one invocation
   - Test: multiple bindings generate multiple invocations
   - Test: empty bindings generate zero invocations
   - Test: resolveArgs substitutes bound.varName correctly
   - Test: resolveArgs passes literal values unchanged
   - Test: resolveArgs returns error on missing variable
   - Test: idempotency check skips already-fired bindings
   - Test: all records written (firing, invocation, edge)
   - Test: invocation enqueued after successful write
   - Test: error handling (store write failures)

## Quick Reference

| Pattern | Key Rule |
|---------|----------|
| **FR-2.4** | Execute then-clause to generate invocations from bindings |
| **CP-1** | Binding-level idempotency via (completion_id, sync_id, binding_hash) |
| **FR-4.1** | Record provenance: firing → invocation |
| **CP-4** | Content-addressed invocation IDs with domain separation |

## Tasks / Subtasks

- [ ] Task 1: Implement resolveArgs helper (AC: #4)
  - [ ] 1.1 Create `internal/engine/executor.go`
  - [ ] 1.2 Implement resolveArgs function signature
  - [ ] 1.3 Iterate over argTemplates IRObject
  - [ ] 1.4 Check for "bound." prefix in string values
  - [ ] 1.5 Strip prefix and lookup variable in bindings
  - [ ] 1.6 Return error if variable not found
  - [ ] 1.7 Preserve IRValue types for literal values
  - [ ] 1.8 Build and return resolved args IRObject

- [ ] Task 2: Implement executeThen core loop (AC: #1, #2)
  - [ ] 2.1 Implement executeThen function signature
  - [ ] 2.2 Add for-loop over bindings slice
  - [ ] 2.3 Handle empty bindings slice (no-op, no error)
  - [ ] 2.4 Add context cancellation check in loop
  - [ ] 2.5 Call resolveArgs for each binding
  - [ ] 2.6 Handle resolveArgs errors

- [ ] Task 3: Implement idempotency check (AC: #3)
  - [ ] 3.1 Compute binding hash with ir.BindingHash(binding)
  - [ ] 3.2 Call store.HasFiring with completion.ID, sync.ID, bindingHash
  - [ ] 3.3 Handle HasFiring error (return immediately)
  - [ ] 3.4 Skip binding if hasFired is true (continue loop)
  - [ ] 3.5 Add logging for skipped bindings (debug level)

- [ ] Task 4: Implement record writes (AC: #5)
  - [ ] 4.1 Create SyncFiring struct with all required fields
  - [ ] 4.2 Call store.WriteSyncFiring, capture firing ID
  - [ ] 4.3 Create Invocation struct with resolved args
  - [ ] 4.4 Compute invocation ID via ir.InvocationID
  - [ ] 4.5 Call store.WriteInvocation
  - [ ] 4.6 Create ProvenanceEdge with firing ID and invocation ID
  - [ ] 4.7 Call store.WriteProvenanceEdge
  - [ ] 4.8 Handle errors for each write operation

- [ ] Task 5: Implement invocation enqueueing (AC: #6)
  - [ ] 5.1 Create Event struct with EventTypeInvocation
  - [ ] 5.2 Set Event.Data to invocation pointer
  - [ ] 5.3 Call e.queue.Enqueue(event)
  - [ ] 5.4 Enqueue AFTER all writes succeed
  - [ ] 5.5 Enqueue BEFORE continuing to next binding

- [ ] Task 6: Write comprehensive tests (AC: #7)
  - [ ] 6.1 Create `internal/engine/executor_test.go`
  - [ ] 6.2 Test resolveArgs with bound.varName substitution
  - [ ] 6.3 Test resolveArgs with literal values
  - [ ] 6.4 Test resolveArgs error on missing variable
  - [ ] 6.5 Test executeThen with single binding
  - [ ] 6.6 Test executeThen with multiple bindings (3+)
  - [ ] 6.7 Test executeThen with empty bindings slice
  - [ ] 6.8 Test idempotency skip (HasFiring returns true)
  - [ ] 6.9 Test record writes (verify firing, invocation, edge)
  - [ ] 6.10 Test invocation enqueued correctly
  - [ ] 6.11 Test error handling (store failures)
  - [ ] 6.12 Test context cancellation

- [ ] Task 7: Verify integration with engine loop
  - [ ] 7.1 Verify executeThen signature matches engine loop needs
  - [ ] 7.2 Document executeThen contract in godoc
  - [ ] 7.3 Add usage examples in documentation

## Dev Notes

### Critical Implementation Details

**executeThen Implementation**
```go
// internal/engine/executor.go
package engine

import (
    "context"
    "fmt"

    "github.com/tyler/nysm/internal/ir"
)

// executeThen generates invocations from then-clause bindings.
//
// For each binding:
// 1. Check idempotency (skip if already fired)
// 2. Resolve arg templates with binding values
// 3. Generate invocation with content-addressed ID
// 4. Write sync_firing, invocation, provenance_edge
// 5. Enqueue invocation for execution
//
// Empty bindings slice is valid (generates zero invocations).
// Multiple bindings generate multiple invocations (critical for multi-binding syncs).
//
// Implements FR-2.4 (execute then-clause to generate invocations from bindings).
// Implements CP-1 (binding-level idempotency via binding hash).
func (e *Engine) executeThen(
    ctx context.Context,
    then ir.ThenClause,
    bindings []ir.IRObject,
    flowToken string,
    completion ir.Completion,
    sync ir.SyncRule,
) error {
    // Handle empty bindings (valid - no invocations generated)
    if len(bindings) == 0 {
        return nil
    }

    // Process each binding independently
    for _, binding := range bindings {
        // Check context cancellation
        if err := ctx.Err(); err != nil {
            return fmt.Errorf("context cancelled: %w", err)
        }

        // Compute binding hash for idempotency (CP-1)
        bindingHash := ir.BindingHash(binding)

        // Check if this binding already fired (idempotent replay)
        hasFired, err := e.store.HasFiring(ctx, completion.ID, sync.ID, bindingHash)
        if err != nil {
            return fmt.Errorf("check firing for binding: %w", err)
        }
        if hasFired {
            // Skip - already processed (replay scenario)
            continue
        }

        // Resolve arg templates with binding values
        resolvedArgs, err := e.resolveArgs(then.Args, binding)
        if err != nil {
            return fmt.Errorf("resolve args for binding: %w", err)
        }

        // Generate invocation
        seq := e.clock.Next()
        inv := ir.Invocation{
            FlowToken:       flowToken,
            ActionURI:       then.Action,
            Args:            resolvedArgs,
            Seq:             seq,
            SecurityContext: ir.IRObject{}, // TODO: Propagate from parent invocation
            SpecHash:        "",            // TODO: Compute from action spec
            EngineVersion:   ir.EngineVersion,
            IRVersion:       ir.IRVersion,
        }
        // Compute content-addressed ID
        inv.ID = ir.InvocationID(inv)

        // Write sync firing record
        firing := ir.SyncFiring{
            CompletionID: completion.ID,
            SyncID:       sync.ID,
            BindingHash:  bindingHash,
            Seq:          e.clock.Next(),
        }
        firingID, err := e.store.WriteSyncFiring(ctx, firing)
        if err != nil {
            return fmt.Errorf("write sync firing: %w", err)
        }

        // Write invocation
        if err := e.store.WriteInvocation(ctx, inv); err != nil {
            return fmt.Errorf("write invocation: %w", err)
        }

        // Write provenance edge (firing → invocation)
        edge := ir.ProvenanceEdge{
            SyncFiringID: firingID,
            InvocationID: inv.ID,
        }
        if err := e.store.WriteProvenanceEdge(ctx, edge); err != nil {
            return fmt.Errorf("write provenance edge: %w", err)
        }

        // Enqueue invocation for execution
        e.queue.Enqueue(Event{
            Type: EventTypeInvocation,
            Data: &inv,
        })
    }

    return nil
}
```

**resolveArgs Implementation**
```go
// resolveArgs substitutes binding variables into then-clause arg templates.
//
// Arg templates support "bound.varName" syntax for binding substitution:
// - "bound.item_id" → looks up "item_id" in bindings
// - Literal IRValues (IRString, IRInt, etc.) passed through unchanged
//
// Returns error if binding variable not found.
// All-or-nothing: partial substitution not allowed.
//
// Example:
//   argTemplates = {"product": "bound.item_id", "quantity": IRInt(1)}
//   bindings = {"item_id": IRString("widget-x")}
//   result = {"product": IRString("widget-x"), "quantity": IRInt(1)}
func (e *Engine) resolveArgs(argTemplates ir.IRObject, bindings ir.IRObject) (ir.IRObject, error) {
    resolvedArgs := make(ir.IRObject, len(argTemplates))

    for key, template := range argTemplates {
        // Check if template is a string with "bound." prefix
        if strTemplate, ok := template.(ir.IRString); ok {
            templateStr := string(strTemplate)
            if len(templateStr) > 6 && templateStr[:6] == "bound." {
                // Extract variable name (strip "bound." prefix)
                varName := templateStr[6:]

                // Look up variable in bindings
                value, exists := bindings[varName]
                if !exists {
                    return nil, fmt.Errorf("binding variable %q not found (referenced in arg %q)", varName, key)
                }

                // Substitute binding value
                resolvedArgs[key] = value
                continue
            }
        }

        // Not a bound reference - pass literal value through
        resolvedArgs[key] = template
    }

    return resolvedArgs, nil
}
```

### Binding Hash Idempotency (CP-1)

**Why Binding-Level Idempotency is CRITICAL:**

Consider a sync rule that processes cart items:
```cue
sync "reserve-items" {
  when: Cart.checkout.completed {
    bind: { cart_id: result.cart_id }
  }
  where: CartItems {
    filter: "cart_id = bound.cart_id"
    bind: { item_id: item_id, qty: quantity }
  }
  then: Inventory.reserve(item: bound.item_id, qty: bound.qty)
}
```

**Scenario:** Cart contains 3 items.

**Expected Behavior:**
1. Where-clause returns 3 bindings (one per item)
2. executeThen generates 3 invocations (one per binding)
3. All 3 items reserved

**Without binding-level idempotency:**
```go
// BROKEN: Only checking (completion_id, sync_id)
hasFired := store.HasFiring(completion.ID, sync.ID)  // ❌ WRONG
if hasFired {
    return  // Skip entire sync
}
```

With this approach:
1. First binding fires → sync_firing inserted ✓
2. Second binding fires → HasFiring returns true ❌
3. Third binding fires → HasFiring returns true ❌
4. **Result:** Only 1 of 3 items reserved. Cart checkout succeeds but inventory is corrupted.

**With binding-level idempotency (CP-1):**
```go
// CORRECT: Check (completion_id, sync_id, binding_hash)
for _, binding := range bindings {
    bindingHash := ir.BindingHash(binding)
    hasFired := store.HasFiring(completion.ID, sync.ID, bindingHash)  // ✓ CORRECT
    if hasFired {
        continue  // Skip this binding only
    }
    // Generate invocation for this binding...
}
```

With this approach:
1. First binding fires → (completion, sync, hash_1) inserted ✓
2. Second binding fires → (completion, sync, hash_2) inserted ✓
3. Third binding fires → (completion, sync, hash_3) inserted ✓
4. **Result:** All 3 items reserved correctly.

**Replay Scenario (Crash Recovery):**

After a crash, the engine replays the Cart.checkout completion:
1. Where-clause re-executes → produces same 3 bindings (deterministic query ordering)
2. Binding hashes re-computed → produce same hash values (canonical JSON)
3. HasFiring checks → all return true (already fired)
4. All 3 bindings skipped (idempotent replay)
5. **Result:** No duplicate reservations. State identical to pre-crash.

### Argument Template Resolution

**resolveArgs Examples**

**Example 1: Simple Binding Substitution**
```go
// Then-clause template
then := ir.ThenClause{
    Action: "Inventory.reserve",
    Args: ir.IRObject{
        "product":  ir.IRString("bound.item_id"),
        "quantity": ir.IRString("bound.qty"),
    },
}

// Binding from where-clause
binding := ir.IRObject{
    "item_id": ir.IRString("widget-x"),
    "qty":     ir.IRInt(5),
}

// Resolved args
resolvedArgs, err := e.resolveArgs(then.Args, binding)
// resolvedArgs = {
//     "product":  IRString("widget-x"),
//     "quantity": IRInt(5),
// }
```

**Example 2: Mixed Literal and Bound Values**
```go
// Template with both literal and bound values
then := ir.ThenClause{
    Action: "Notification.send",
    Args: ir.IRObject{
        "to":       ir.IRString("bound.customer_email"),
        "template": ir.IRString("order_confirmation"),  // Literal
        "order_id": ir.IRString("bound.order_id"),
    },
}

binding := ir.IRObject{
    "customer_email": ir.IRString("alice@example.com"),
    "order_id":       ir.IRString("ord-123"),
}

resolvedArgs, _ := e.resolveArgs(then.Args, binding)
// resolvedArgs = {
//     "to":       IRString("alice@example.com"),
//     "template": IRString("order_confirmation"),  // Passed through
//     "order_id": IRString("ord-123"),
// }
```

**Example 3: Nested IRObject Values**
```go
// Binding contains nested object
binding := ir.IRObject{
    "customer": ir.IRObject{
        "id":    ir.IRString("cust-123"),
        "email": ir.IRString("alice@example.com"),
    },
    "order_id": ir.IRString("ord-456"),
}

// Template binds entire nested object
then := ir.ThenClause{
    Action: "Order.Create",
    Args: ir.IRObject{
        "customer_info": ir.IRString("bound.customer"),  // Binds entire object
        "order_id":      ir.IRString("bound.order_id"),
    },
}

resolvedArgs, _ := e.resolveArgs(then.Args, binding)
// resolvedArgs = {
//     "customer_info": IRObject{
//         "id":    IRString("cust-123"),
//         "email": IRString("alice@example.com"),
//     },
//     "order_id": IRString("ord-456"),
// }
```

**Example 4: Missing Binding Variable (Error)**
```go
// Template references variable not in binding
then := ir.ThenClause{
    Action: "Inventory.reserve",
    Args: ir.IRObject{
        "product": ir.IRString("bound.item_id"),
        "qty":     ir.IRString("bound.quantity"),  // Variable not in binding
    },
}

binding := ir.IRObject{
    "item_id": ir.IRString("widget-x"),
    // "quantity" missing!
}

resolvedArgs, err := e.resolveArgs(then.Args, binding)
// err = "binding variable \"quantity\" not found (referenced in arg \"qty\")"
// resolvedArgs = nil
```

### Write Order and Error Handling

**Write Order is CRITICAL:**

```go
// 1. Write sync_firing FIRST (captures firing ID)
firingID, err := e.store.WriteSyncFiring(ctx, firing)
if err != nil {
    return err  // No cleanup needed - nothing written yet
}

// 2. Write invocation SECOND
err = e.store.WriteInvocation(ctx, inv)
if err != nil {
    return err  // sync_firing orphaned but harmless (no provenance edge)
}

// 3. Write provenance_edge LAST (links firing → invocation)
edge := ir.ProvenanceEdge{
    SyncFiringID: firingID,
    InvocationID: inv.ID,
}
err = e.store.WriteProvenanceEdge(ctx, edge)
if err != nil {
    return err  // sync_firing and invocation orphaned but recoverable
}

// 4. Enqueue invocation AFTER all writes succeed
e.queue.Enqueue(Event{Type: EventTypeInvocation, Data: &inv})
```

**Why This Order:**

1. **sync_firing first** - Captures binding hash for idempotency. If write fails, nothing is orphaned.
2. **invocation second** - If write fails, sync_firing exists but has no provenance edge (harmless orphan).
3. **provenance_edge last** - Links firing to invocation. If write fails, both firing and invocation are orphaned but can be detected and recovered.
4. **enqueue last** - Only enqueue if all writes succeeded. Prevents processing invocations with incomplete provenance.

**Error Recovery:**

If a write fails mid-binding:
- Previous bindings: Successfully written and enqueued ✓
- Current binding: Partially written (firing exists, invocation may exist, no edge)
- Next bindings: Not processed ✗

On replay:
- Previous bindings: HasFiring returns true → skipped (idempotent) ✓
- Current binding: HasFiring returns true (firing exists) → skipped ✓
- Next bindings: HasFiring returns false → processed ✓

**Result:** Replay completes the sync rule execution without duplicates.

### Test Examples

**Test: Single Binding Generates One Invocation**
```go
func TestExecuteThen_SingleBinding(t *testing.T) {
    ctx := context.Background()
    store, engine := setupTestEngine(t)

    // Setup: completion that matches sync rule
    inv := testInvocation("Cart.checkout", 1)
    store.WriteInvocation(ctx, inv)
    comp := testCompletion(inv.ID, "Success", 2)
    comp.Result = ir.IRObject{"cart_id": ir.IRString("cart-123")}
    store.WriteCompletion(ctx, comp)

    // Sync rule with single binding
    sync := ir.SyncRule{
        ID: "sync-test",
        Then: ir.ThenClause{
            Action: "Inventory.reserve",
            Args: ir.IRObject{
                "cart": ir.IRString("bound.cart_id"),
            },
        },
    }

    // Single binding from where-clause
    bindings := []ir.IRObject{
        {"cart_id": ir.IRString("cart-123")},
    }

    // Execute then-clause
    err := engine.executeThen(ctx, sync.Then, bindings, "flow-1", comp, sync)
    require.NoError(t, err)

    // Verify invocation created
    invs, _ := store.ReadInvocations(ctx, "flow-1")
    require.Len(t, invs, 2, "should have original + generated invocation")
    assert.Equal(t, "Inventory.reserve", string(invs[1].ActionURI))
    assert.Equal(t, ir.IRString("cart-123"), invs[1].Args["cart"])

    // Verify sync firing created
    firings := readAllSyncFirings(t, store)
    require.Len(t, firings, 1)
    assert.Equal(t, comp.ID, firings[0].CompletionID)
    assert.Equal(t, "sync-test", firings[0].SyncID)

    // Verify provenance edge created
    prov, _ := store.ReadProvenance(ctx, invs[1].ID)
    require.Len(t, prov, 1)
    assert.Equal(t, comp.ID, prov[0].CompletionID)
}
```

**Test: Multiple Bindings Generate Multiple Invocations**
```go
func TestExecuteThen_MultipleBindings(t *testing.T) {
    ctx := context.Background()
    store, engine := setupTestEngine(t)

    // Setup completion
    inv := testInvocation("Cart.checkout", 1)
    store.WriteInvocation(ctx, inv)
    comp := testCompletion(inv.ID, "Success", 2)
    store.WriteCompletion(ctx, comp)

    // Sync rule
    sync := ir.SyncRule{
        ID: "sync-reserve-items",
        Then: ir.ThenClause{
            Action: "Inventory.reserve",
            Args: ir.IRObject{
                "item": ir.IRString("bound.item_id"),
                "qty":  ir.IRString("bound.quantity"),
            },
        },
    }

    // Three bindings (three cart items)
    bindings := []ir.IRObject{
        {"item_id": ir.IRString("widget"), "quantity": ir.IRInt(10)},
        {"item_id": ir.IRString("gadget"), "quantity": ir.IRInt(5)},
        {"item_id": ir.IRString("doohickey"), "quantity": ir.IRInt(7)},
    }

    // Execute then-clause
    err := engine.executeThen(ctx, sync.Then, bindings, "flow-1", comp, sync)
    require.NoError(t, err)

    // Verify 3 invocations created
    invs, _ := store.ReadInvocations(ctx, "flow-1")
    require.Len(t, invs, 4, "should have original + 3 generated invocations")

    // Verify each invocation has correct args
    expectedItems := []string{"widget", "gadget", "doohickey"}
    expectedQtys := []int64{10, 5, 7}

    for i := 0; i < 3; i++ {
        inv := invs[i+1]  // Skip original invocation
        assert.Equal(t, "Inventory.reserve", string(inv.ActionURI))
        assert.Equal(t, ir.IRString(expectedItems[i]), inv.Args["item"])
        assert.Equal(t, ir.IRInt(expectedQtys[i]), inv.Args["qty"])
    }

    // Verify 3 sync firings created (different binding hashes)
    firings := readAllSyncFirings(t, store)
    require.Len(t, firings, 3)

    // All firings reference same completion and sync
    for _, firing := range firings {
        assert.Equal(t, comp.ID, firing.CompletionID)
        assert.Equal(t, "sync-reserve-items", firing.SyncID)
    }

    // Verify all binding hashes are different
    hashes := make(map[string]bool)
    for _, firing := range firings {
        hashes[firing.BindingHash] = true
    }
    assert.Len(t, hashes, 3, "binding hashes must be unique")

    // Verify 3 provenance edges created
    for i := 0; i < 3; i++ {
        prov, _ := store.ReadProvenance(ctx, invs[i+1].ID)
        require.Len(t, prov, 1)
        assert.Equal(t, comp.ID, prov[0].CompletionID)
    }
}
```

**Test: Empty Bindings Generate Zero Invocations**
```go
func TestExecuteThen_EmptyBindings(t *testing.T) {
    ctx := context.Background()
    store, engine := setupTestEngine(t)

    // Setup completion
    inv := testInvocation("Cart.checkout", 1)
    store.WriteInvocation(ctx, inv)
    comp := testCompletion(inv.ID, "Success", 2)
    store.WriteCompletion(ctx, comp)

    // Sync rule
    sync := ir.SyncRule{
        ID: "sync-test",
        Then: ir.ThenClause{
            Action: "Inventory.reserve",
            Args:   ir.IRObject{},
        },
    }

    // Empty bindings (where-clause returned nothing)
    bindings := []ir.IRObject{}

    // Execute then-clause
    err := engine.executeThen(ctx, sync.Then, bindings, "flow-1", comp, sync)
    require.NoError(t, err, "empty bindings should not error")

    // Verify no invocations created
    invs, _ := store.ReadInvocations(ctx, "flow-1")
    require.Len(t, invs, 1, "should only have original invocation")

    // Verify no sync firings created
    firings := readAllSyncFirings(t, store)
    assert.Empty(t, firings)
}
```

**Test: Idempotency Check Skips Already-Fired Bindings**
```go
func TestExecuteThen_IdempotencySkip(t *testing.T) {
    ctx := context.Background()
    store, engine := setupTestEngine(t)

    // Setup completion
    inv := testInvocation("Cart.checkout", 1)
    store.WriteInvocation(ctx, inv)
    comp := testCompletion(inv.ID, "Success", 2)
    store.WriteCompletion(ctx, comp)

    // Sync rule
    sync := ir.SyncRule{
        ID: "sync-test",
        Then: ir.ThenClause{
            Action: "Inventory.reserve",
            Args: ir.IRObject{
                "item": ir.IRString("bound.item_id"),
            },
        },
    }

    // Three bindings
    bindings := []ir.IRObject{
        {"item_id": ir.IRString("widget")},
        {"item_id": ir.IRString("gadget")},
        {"item_id": ir.IRString("doohickey")},
    }

    // First execution - all bindings fire
    err := engine.executeThen(ctx, sync.Then, bindings, "flow-1", comp, sync)
    require.NoError(t, err)

    // Verify 3 invocations created
    invs1, _ := store.ReadInvocations(ctx, "flow-1")
    require.Len(t, invs1, 4, "should have original + 3 generated")

    // Second execution (replay scenario) - all bindings skipped
    err = engine.executeThen(ctx, sync.Then, bindings, "flow-1", comp, sync)
    require.NoError(t, err)

    // Verify no new invocations created
    invs2, _ := store.ReadInvocations(ctx, "flow-1")
    require.Len(t, invs2, 4, "should still have only 4 invocations")

    // Verify invocation IDs unchanged (exact same invocations)
    for i, inv1 := range invs1 {
        assert.Equal(t, inv1.ID, invs2[i].ID, "invocation IDs must match")
    }

    // Verify sync firings count unchanged
    firings := readAllSyncFirings(t, store)
    require.Len(t, firings, 3, "should still have only 3 firings")
}
```

**Test: resolveArgs Substitutes Bound Variables**
```go
func TestResolveArgs_BoundVariableSubstitution(t *testing.T) {
    engine := setupTestEngine(t)

    argTemplates := ir.IRObject{
        "product":  ir.IRString("bound.item_id"),
        "quantity": ir.IRString("bound.qty"),
        "priority": ir.IRString("high"),  // Literal value
    }

    bindings := ir.IRObject{
        "item_id": ir.IRString("widget-x"),
        "qty":     ir.IRInt(10),
    }

    resolvedArgs, err := engine.resolveArgs(argTemplates, bindings)
    require.NoError(t, err)

    expected := ir.IRObject{
        "product":  ir.IRString("widget-x"),
        "quantity": ir.IRInt(10),
        "priority": ir.IRString("high"),  // Unchanged
    }

    assert.Equal(t, expected, resolvedArgs)
}
```

**Test: resolveArgs Returns Error on Missing Variable**
```go
func TestResolveArgs_MissingVariable(t *testing.T) {
    engine := setupTestEngine(t)

    argTemplates := ir.IRObject{
        "product": ir.IRString("bound.item_id"),
        "qty":     ir.IRString("bound.quantity"),  // Not in bindings
    }

    bindings := ir.IRObject{
        "item_id": ir.IRString("widget-x"),
        // "quantity" missing!
    }

    resolvedArgs, err := engine.resolveArgs(argTemplates, bindings)
    require.Error(t, err)
    assert.Nil(t, resolvedArgs)
    assert.Contains(t, err.Error(), "quantity")
    assert.Contains(t, err.Error(), "not found")
}
```

**Test: resolveArgs Preserves IRValue Types**
```go
func TestResolveArgs_PreservesTypes(t *testing.T) {
    engine := setupTestEngine(t)

    argTemplates := ir.IRObject{
        "str":    ir.IRString("bound.str_val"),
        "int":    ir.IRString("bound.int_val"),
        "bool":   ir.IRString("bound.bool_val"),
        "obj":    ir.IRString("bound.obj_val"),
        "arr":    ir.IRString("bound.arr_val"),
        "literal": ir.IRInt(42),  // Literal
    }

    bindings := ir.IRObject{
        "str_val":  ir.IRString("hello"),
        "int_val":  ir.IRInt(123),
        "bool_val": ir.IRBool(true),
        "obj_val":  ir.IRObject{"nested": ir.IRString("value")},
        "arr_val":  ir.IRArray{ir.IRInt(1), ir.IRInt(2)},
    }

    resolvedArgs, err := engine.resolveArgs(argTemplates, bindings)
    require.NoError(t, err)

    // Verify types preserved
    assert.IsType(t, ir.IRString(""), resolvedArgs["str"])
    assert.IsType(t, ir.IRInt(0), resolvedArgs["int"])
    assert.IsType(t, ir.IRBool(false), resolvedArgs["bool"])
    assert.IsType(t, ir.IRObject{}, resolvedArgs["obj"])
    assert.IsType(t, ir.IRArray{}, resolvedArgs["arr"])
    assert.IsType(t, ir.IRInt(0), resolvedArgs["literal"])

    // Verify values
    assert.Equal(t, ir.IRString("hello"), resolvedArgs["str"])
    assert.Equal(t, ir.IRInt(123), resolvedArgs["int"])
    assert.Equal(t, ir.IRBool(true), resolvedArgs["bool"])
    assert.Equal(t, ir.IRInt(42), resolvedArgs["literal"])
}
```

### File List

Files to create:

1. `internal/engine/executor.go` - executeThen and resolveArgs functions
2. `internal/engine/executor_test.go` - Comprehensive tests

Files to modify:

1. `internal/engine/engine.go` - May need to export executeThen if used by loop

Files that must exist (from previous stories):

1. `internal/ir/hash.go` - BindingHash function (Story 2.5)
2. `internal/ir/types.go` - Invocation, Completion, SyncRule types (Story 1.1)
3. `internal/ir/clause.go` - ThenClause type (Story 1.1)
4. `internal/ir/store_types.go` - SyncFiring, ProvenanceEdge types (Story 1.1)
5. `internal/store/store.go` - HasFiring, WriteSyncFiring, WriteInvocation, WriteProvenanceEdge (Stories 2.3, 2.5, 2.6)
6. `internal/engine/clock.go` - Clock for seq generation (Story 2.2)
7. `internal/engine/queue.go` - Event queue (Story 3.1)

### Story Completion Checklist

- [ ] executeThen function implemented
- [ ] resolveArgs function implemented
- [ ] One invocation generated per binding
- [ ] Empty bindings handled (no-op, no error)
- [ ] Binding hash computed before idempotency check
- [ ] HasFiring check skips already-fired bindings
- [ ] Arg templates with "bound.varName" substituted
- [ ] Literal arg values passed through unchanged
- [ ] Error on missing binding variable
- [ ] sync_firing record written with correct fields
- [ ] Invocation record written with content-addressed ID
- [ ] ProvenanceEdge record written linking firing → invocation
- [ ] Invocation enqueued after successful writes
- [ ] Write order: firing → invocation → edge → enqueue
- [ ] Error handling returns immediately on write failure
- [ ] Context cancellation checked in loop
- [ ] Test: single binding generates one invocation
- [ ] Test: multiple bindings generate multiple invocations
- [ ] Test: empty bindings generate zero invocations
- [ ] Test: idempotency skip works correctly
- [ ] Test: resolveArgs substitutes bound variables
- [ ] Test: resolveArgs preserves IRValue types
- [ ] Test: resolveArgs errors on missing variable
- [ ] Test: all records written (firing, invocation, edge)
- [ ] Test: invocation enqueued correctly
- [ ] Test: error handling for store failures
- [ ] `go vet ./internal/engine/...` passes
- [ ] `go test ./internal/engine/...` passes

### References

- [Source: docs/epics.md#Story 4.5] - Story definition and acceptance criteria
- [Source: docs/prd.md#FR-2.4] - Execute then-clause to generate invocations from bindings
- [Source: docs/architecture.md#CP-1] - Binding-level idempotency pattern
- [Source: docs/architecture.md#CP-2] - Content-addressed identity
- [Source: Story 2.5] - sync_firings table and binding hash
- [Source: Story 2.6] - provenance_edges table
- [Source: Story 3.1] - Event queue for invocation enqueueing
- [Source: Story 3.6] - Flow token propagation (prerequisite)

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Validation History

- Initial creation: comprehensive story generation from epics.md and existing story patterns

### Completion Notes

- **executeThen is the core sync execution primitive** - Takes bindings from where-clause and generates invocations
- **Binding-level idempotency is CRITICAL** - Without it, multi-binding syncs break (only first binding fires)
- **One invocation per binding** - Multi-binding syncs (e.g., process each cart item) depend on this
- **resolveArgs template substitution** - Supports "bound.varName" syntax for binding variable references
- **Write order matters** - firing → invocation → edge → enqueue ensures atomic per-binding execution
- **Idempotency enables replay** - Crash recovery re-executes then-clause without duplicates
- **Empty bindings are valid** - Where-clause can return zero results (no invocations generated)
- **Error handling is fail-fast** - Return immediately on write failure, previous bindings already processed
- **Context cancellation checked** - Long binding loops can be cancelled gracefully
- **Provenance tracking** - Every generated invocation has a provenance edge linking to its cause
- **Type preservation** - resolveArgs preserves IRValue types from bindings (no type coercion)
- **Clear error messages** - Missing binding variables report both variable name and arg key
- **Story implements FR-2.4** - Complete then-clause execution with binding-level idempotency (CP-1)
