# Story 5.1: Binding-Level Idempotency Enforcement

Status: done

## Story

As a **developer relying on idempotency**,
I want **sync firings to be idempotent at the binding level**,
So that **replay and recovery don't create duplicate invocations**.

## Acceptance Criteria

1. **HasFiring check before every sync firing**
   ```go
   func (s *Store) HasFiring(ctx context.Context, completionID, syncID, bindingHash string) (bool, error)
   ```
   - Returns true if firing exists, false otherwise
   - Query uses parameterized SQL: `SELECT COUNT(*) FROM sync_firings WHERE completion_id = ? AND sync_id = ? AND binding_hash = ?`
   - Returns error only on query failure, not on "not found"

2. **RecordFiring function atomically records firing + invocation**
   ```go
   func (e *Engine) RecordFiring(ctx context.Context, completion ir.Completion, sync ir.SyncRule, binding ir.IRObject, invocation ir.Invocation) error
   ```
   - Computes binding hash via `ir.BindingHash(binding)`
   - Writes sync_firing record with UNIQUE(completion_id, sync_id, binding_hash)
   - Writes invocation record
   - Writes provenance edge linking firing to invocation
   - All 3 writes in single transaction (if store supports) or sequential with rollback

3. **Idempotent INSERT for sync firings**
   - Uses `INSERT OR IGNORE` for graceful duplicate handling
   - Duplicate (completion_id, sync_id, binding_hash) does NOT error
   - Returns existing firing ID if duplicate detected

4. **Engine skips already-fired bindings during sync evaluation**
   ```go
   // In engine sync evaluation loop:
   for _, binding := range bindings {
       bindingHash := ir.BindingHash(binding)
       if e.store.HasFiring(ctx, completion.ID, sync.ID, bindingHash) {
           continue  // Skip - already processed
       }
       // Generate and record invocation...
   }
   ```

5. **Replay produces identical firings and invocations**
   - Same completion re-evaluated → same bindings (deterministic query)
   - Same bindings → same binding hashes (canonical JSON)
   - HasFiring returns true → all bindings skipped
   - Final state: no duplicate invocations, identical to initial run

6. **Multi-binding sync fires each binding exactly once**
   - Where-clause returns N bindings → N firings recorded
   - Each firing has different binding_hash
   - UNIQUE constraint allows all N inserts
   - Replay skips all N bindings

## Quick Reference

| Pattern | Key Rule |
|---------|----------|
| **CP-1** | CRITICAL: `UNIQUE(completion_id, sync_id, binding_hash)` - NOT `UNIQUE(completion_id, sync_id)` |
| **CP-2** | Logical clocks (`seq`), NO wall-clock timestamps |
| **CP-3** | RFC 8785 UTF-16 key ordering in canonical JSON |

## Tasks / Subtasks

- [ ] Task 1: Implement HasFiring query function (AC: #1)
  - [ ] 1.1 Add HasFiring to Store interface in `internal/store/store.go`
  - [ ] 1.2 Implement SELECT COUNT(*) with parameterized query
  - [ ] 1.3 Return (bool, error) - true if exists, false otherwise
  - [ ] 1.4 Add unit tests for existing/non-existing/partial-match cases

- [ ] Task 2: Implement RecordFiring in engine (AC: #2)
  - [ ] 2.1 Add RecordFiring method to Engine
  - [ ] 2.2 Compute binding hash via ir.BindingHash
  - [ ] 2.3 Write sync_firing record with INSERT OR IGNORE
  - [ ] 2.4 Write invocation record
  - [ ] 2.5 Write provenance edge linking firing→invocation
  - [ ] 2.6 Handle errors with proper rollback/cleanup

- [ ] Task 3: Integrate HasFiring check into sync evaluation (AC: #4)
  - [ ] 3.1 Locate sync evaluation loop in `internal/engine/engine.go`
  - [ ] 3.2 Add HasFiring check before generating invocation
  - [ ] 3.3 Skip binding if already fired (log skip event)
  - [ ] 3.4 Process binding if not fired (call RecordFiring)

- [ ] Task 4: Verify idempotent INSERT behavior (AC: #3)
  - [ ] 4.1 Test WriteSyncFiring with duplicate (completion, sync, binding)
  - [ ] 4.2 Verify INSERT OR IGNORE succeeds without error
  - [ ] 4.3 Verify existing ID returned
  - [ ] 4.4 Verify row count unchanged

- [ ] Task 5: Replay and multi-binding tests (AC: #5, #6)
  - [ ] 5.1 Integration test: sync with 3 bindings fires all 3
  - [ ] 5.2 Integration test: replay skips all 3 bindings
  - [ ] 5.3 Integration test: partial completion (2/3 fired) → replay completes
  - [ ] 5.4 Integration test: verify final state identical across runs

## Dev Notes

### Why Binding-Level Idempotency is CRITICAL

**CP-1 is the most important correctness pattern in NYSM.** Without it, multi-binding syncs break.

**The Problem Without binding_hash:**

Consider this sync rule:
```cue
sync "reserve-each-item" {
  when: Cart.checkout.completed {
    bind: { cart_id: result.cart_id }
  }
  where: {
    from: CartItems
    filter: cart_id == bound.cart_id
    bind: { item_id: item_id, quantity: quantity }
  }
  then: Inventory.reserve {
    args: { item: bound.item_id, qty: bound.quantity }
  }
}
```

**Scenario:** Cart contains 3 items.

**Expected:** Where-clause returns 3 bindings. Sync fires 3 times. 3 `Inventory.reserve` invocations.

**With WRONG schema (sync-level idempotency):**
```sql
UNIQUE(completion_id, sync_id)  -- ❌ BROKEN
```

1. First binding fires → sync_firing inserted ✓
2. Second binding fires → INSERT fails (duplicate completion_id, sync_id) ❌
3. Third binding fires → INSERT fails ❌
4. **Result:** Only 1 of 3 items reserved. Cart checkout succeeds but inventory corrupted.

**With CORRECT schema (binding-level idempotency):**
```sql
UNIQUE(completion_id, sync_id, binding_hash)  -- ✓ CORRECT
```

1. First binding fires → (completion, sync, hash_1) inserted ✓
2. Second binding fires → (completion, sync, hash_2) inserted ✓
3. Third binding fires → (completion, sync, hash_3) inserted ✓
4. **Result:** All 3 items reserved. Cart and inventory both consistent.

**Replay Scenario:**

After crash, engine replays Cart.checkout completion:
1. Where-clause re-executes → same 3 bindings (deterministic query ordering per CP-4)
2. Binding hashes re-computed → same values (canonical JSON per CP-3)
3. HasFiring checks → all return true
4. All 3 bindings skipped
5. **Result:** Replay identical. No duplicate reservations.

### Implementation Pattern

**HasFiring Query:**

```go
// internal/store/store.go

// HasFiring checks if a sync firing already exists for a specific binding.
// This is the primary idempotency check used by the engine.
// Returns (true, nil) if firing exists.
// Returns (false, nil) if firing does not exist.
// Returns (false, error) only on query failure.
func (s *Store) HasFiring(ctx context.Context, completionID, syncID, bindingHash string) (bool, error) {
    var count int
    err := s.db.QueryRowContext(ctx, `
        SELECT COUNT(*) FROM sync_firings
        WHERE completion_id = ? AND sync_id = ? AND binding_hash = ?
    `, completionID, syncID, bindingHash).Scan(&count)

    if err != nil {
        return false, fmt.Errorf("check firing: %w", err)
    }
    return count > 0, nil
}
```

**Engine Sync Evaluation Loop:**

```go
// internal/engine/engine.go

// evaluateSyncs processes all matching sync rules for a completion.
// Implements binding-level idempotency per CP-1.
func (e *Engine) evaluateSyncs(ctx context.Context, completion ir.Completion) error {
    for _, sync := range e.syncs {
        if !e.matches(sync.When, completion) {
            continue
        }

        // Execute where-clause to get bindings
        bindings, err := e.executeWhere(ctx, sync.Where, completion)
        if err != nil {
            return fmt.Errorf("execute where-clause: %w", err)
        }

        // Process each binding with idempotency check
        for _, binding := range bindings {
            if err := e.processBinding(ctx, completion, sync, binding); err != nil {
                return fmt.Errorf("process binding: %w", err)
            }
        }
    }
    return nil
}

// processBinding handles a single binding from a where-clause.
// Checks idempotency before generating invocation.
func (e *Engine) processBinding(ctx context.Context, completion ir.Completion, sync ir.SyncRule, binding ir.IRObject) error {
    // Compute binding hash for idempotency check
    bindingHash, err := ir.BindingHash(binding)
    if err != nil {
        return fmt.Errorf("compute binding hash: %w", err)
    }

    // Check if already fired (idempotency)
    fired, err := e.store.HasFiring(ctx, completion.ID, sync.ID, bindingHash)
    if err != nil {
        return fmt.Errorf("check firing: %w", err)
    }
    if fired {
        // Already processed - skip with log entry
        slog.Info("sync binding already fired (skipping)",
            "completion_id", completion.ID,
            "sync_id", sync.ID,
            "binding_hash", bindingHash,
            "event", "skip")
        return nil
    }

    // Generate invocation from then-clause
    invocation, err := e.generateInvocation(ctx, sync.Then, binding, completion.FlowToken)
    if err != nil {
        return fmt.Errorf("generate invocation: %w", err)
    }

    // Record firing + invocation atomically
    if err := e.recordFiring(ctx, completion, sync, binding, bindingHash, invocation); err != nil {
        return fmt.Errorf("record firing: %w", err)
    }

    // Log successful firing
    slog.Info("sync binding fired",
        "completion_id", completion.ID,
        "sync_id", sync.ID,
        "binding_hash", bindingHash,
        "invocation_id", invocation.ID,
        "event", "fire")

    return nil
}

// recordFiring atomically records sync_firing, invocation, and provenance edge.
// This is the core idempotency enforcement point.
// CRITICAL: All 3 writes MUST be in a single transaction for atomicity.
func (e *Engine) recordFiring(
    ctx context.Context,
    completion ir.Completion,
    sync ir.SyncRule,
    binding ir.IRObject,
    bindingHash string,
    invocation ir.Invocation,
) error {
    // Create sync firing record
    firing := ir.SyncFiring{
        CompletionID: completion.ID,
        SyncID:       sync.ID,
        BindingHash:  bindingHash,
        Seq:          e.clock.Next(),
    }

    // CRITICAL: Use transaction for atomic writes
    // All 3 operations must succeed or none persist
    return e.store.WithTransaction(ctx, func(tx store.Tx) error {
        // Write firing (INSERT OR IGNORE for idempotency)
        firingID, err := tx.WriteSyncFiring(ctx, firing)
        if err != nil {
            return fmt.Errorf("write sync firing: %w", err)
        }

        // Write invocation
        if err := tx.WriteInvocation(ctx, invocation); err != nil {
            return fmt.Errorf("write invocation: %w", err)
        }

        // Write provenance edge linking firing→invocation
        edge := ir.ProvenanceEdge{
            SyncFiringID: firingID,
            InvocationID: invocation.ID,
        }
        if err := tx.WriteProvenanceEdge(ctx, edge); err != nil {
            return fmt.Errorf("write provenance edge: %w", err)
        }

        return nil  // Commit transaction
    })
}
```

**WriteSyncFiring with INSERT OR IGNORE:**

```go
// internal/store/store.go

// WriteSyncFiring records a sync rule firing.
// Uses INSERT OR IGNORE for idempotent writes.
// Returns the firing ID (either newly inserted or existing).
func (s *Store) WriteSyncFiring(ctx context.Context, firing ir.SyncFiring) (int64, error) {
    result, err := s.db.ExecContext(ctx, `
        INSERT OR IGNORE INTO sync_firings
            (completion_id, sync_id, binding_hash, seq)
        VALUES (?, ?, ?, ?)
    `, firing.CompletionID, firing.SyncID, firing.BindingHash, firing.Seq)

    if err != nil {
        return 0, fmt.Errorf("write sync firing: %w", err)
    }

    // Check if INSERT was skipped (duplicate)
    rowsAffected, _ := result.RowsAffected()
    if rowsAffected == 0 {
        // Duplicate - query for existing ID
        var id int64
        err := s.db.QueryRowContext(ctx, `
            SELECT id FROM sync_firings
            WHERE completion_id = ? AND sync_id = ? AND binding_hash = ?
        `, firing.CompletionID, firing.SyncID, firing.BindingHash).Scan(&id)

        if err != nil {
            return 0, fmt.Errorf("query existing firing: %w", err)
        }
        return id, nil
    }

    // New insert succeeded
    id, err := result.LastInsertId()
    if err != nil {
        return 0, fmt.Errorf("get last insert id: %w", err)
    }
    return id, nil
}
```

### Test Examples

**Test: Same Completion, Different Bindings Fire Multiple Times**

```go
func TestSyncIdempotency_MultiBinding(t *testing.T) {
    // GIVEN: Cart with 3 items
    cart := setupCartWith3Items(t)

    // WHEN: Cart.checkout completes
    completion := engine.Complete(ctx, "Cart.checkout", IRObject{"cart_id": "cart-123"})

    // THEN: reserve-each-item sync fires 3 times
    firings := store.ReadFirings(ctx, completion.ID, "reserve-each-item")
    assert.Len(t, firings, 3, "Should fire once per cart item")

    // AND: Each firing has different binding_hash
    hashes := extractBindingHashes(firings)
    assert.Len(t, uniqueValues(hashes), 3, "All binding hashes should be unique")

    // AND: 3 Inventory.reserve invocations created
    invocations := store.ReadInvocationsByAction(ctx, "Inventory.reserve")
    assert.Len(t, invocations, 3, "Should create one invocation per binding")
}
```

**Test: Same Binding Fires Only Once**

```go
func TestSyncIdempotency_SingleBinding(t *testing.T) {
    // GIVEN: Cart with 1 item
    cart := setupCartWith1Item(t)

    // WHEN: Cart.checkout completes
    completion := engine.Complete(ctx, "Cart.checkout", IRObject{"cart_id": "cart-123"})

    // THEN: reserve-each-item sync fires 1 time
    firings := store.ReadFirings(ctx, completion.ID, "reserve-each-item")
    assert.Len(t, firings, 1)

    // WHEN: Engine re-evaluates same completion (simulating replay)
    engine.evaluateSyncs(ctx, completion)

    // THEN: No additional firing
    firings = store.ReadFirings(ctx, completion.ID, "reserve-each-item")
    assert.Len(t, firings, 1, "Should not fire again - idempotent")

    // AND: Still only 1 invocation
    invocations := store.ReadInvocationsByAction(ctx, "Inventory.reserve")
    assert.Len(t, invocations, 1)
}
```

**Test: Replay Produces Identical Firings**

```go
func TestSyncIdempotency_ReplayIdentical(t *testing.T) {
    // GIVEN: Complete flow with sync that fired 3 bindings
    flow1 := runCompleteFlow(t)
    firings1 := store.ReadAllFirings(ctx, flow1.Token)
    invocations1 := store.ReadAllInvocations(ctx, flow1.Token)

    // Record initial state
    assert.Len(t, firings1, 3, "Initial run should have 3 firings")
    assert.Len(t, invocations1, 5, "Initial run should have 5 total invocations")

    // WHEN: Engine replays the flow from scratch
    engine.Replay(ctx, flow1.Token)

    // THEN: No new firings added
    firings2 := store.ReadAllFirings(ctx, flow1.Token)
    assert.Len(t, firings2, 3, "Replay should not add firings")

    // AND: No new invocations added
    invocations2 := store.ReadAllInvocations(ctx, flow1.Token)
    assert.Len(t, invocations2, 5, "Replay should not add invocations")

    // AND: Firing records unchanged (same IDs, same binding hashes)
    assert.Equal(t, firings1, firings2, "Firings should be identical")

    // AND: Invocation records unchanged
    assert.Equal(t, invocations1, invocations2, "Invocations should be identical")
}
```

**Test: Partial Completion + Replay Finishes**

```go
func TestSyncIdempotency_PartialReplay(t *testing.T) {
    // GIVEN: Flow where sync should produce 3 bindings
    flow := setupFlowWithPartialSync(t)

    // AND: Only 2/3 bindings were fired (simulated crash)
    firings := store.ReadAllFirings(ctx, flow.Token)
    assert.Len(t, firings, 2, "Initial state: 2/3 bindings fired")

    // WHEN: Engine replays the flow
    engine.Replay(ctx, flow.Token)

    // THEN: Third binding fires
    firingsAfter := store.ReadAllFirings(ctx, flow.Token)
    assert.Len(t, firingsAfter, 3, "After replay: all 3 bindings fired")

    // AND: First 2 bindings skipped (HasFiring = true)
    // AND: Third binding processed (HasFiring = false)

    // AND: Final state has 3 invocations (2 existing + 1 new)
    invocations := store.ReadAllInvocations(ctx, flow.Token)
    assert.Len(t, invocations, 3)
}
```

**Test: Deterministic Binding Hash**

```go
func TestBindingHash_Deterministic(t *testing.T) {
    // GIVEN: Identical bindings created independently
    binding1 := ir.IRObject{
        "item_id":  ir.IRString("SKU-001"),
        "quantity": ir.IRInt(5),
    }

    binding2 := ir.IRObject{
        "item_id":  ir.IRString("SKU-001"),
        "quantity": ir.IRInt(5),
    }

    // WHEN: Hashes computed
    hash1, _ := ir.BindingHash(binding1)
    hash2, _ := ir.BindingHash(binding2)

    // THEN: Hashes are identical
    assert.Equal(t, hash1, hash2, "Same bindings must produce same hash")
}

func TestBindingHash_KeyOrderIndependent(t *testing.T) {
    // GIVEN: Bindings with different key insertion order
    binding1 := ir.IRObject{
        "a": ir.IRInt(1),
        "b": ir.IRString("test"),
        "c": ir.IRBool(true),
    }

    binding2 := ir.IRObject{
        "c": ir.IRBool(true),
        "a": ir.IRInt(1),
        "b": ir.IRString("test"),
    }

    // WHEN: Hashes computed
    hash1, _ := ir.BindingHash(binding1)
    hash2, _ := ir.BindingHash(binding2)

    // THEN: Hashes are identical (canonical JSON sorts keys)
    assert.Equal(t, hash1, hash2, "Key order must not affect hash")
}

func TestBindingHash_ValueSensitive(t *testing.T) {
    // GIVEN: Bindings differing only in one value
    binding1 := ir.IRObject{"item_id": ir.IRString("SKU-001")}
    binding2 := ir.IRObject{"item_id": ir.IRString("SKU-002")}

    // WHEN: Hashes computed
    hash1, _ := ir.BindingHash(binding1)
    hash2, _ := ir.BindingHash(binding2)

    // THEN: Hashes differ
    assert.NotEqual(t, hash1, hash2, "Different values must produce different hashes")
}
```

### File List

Files to create/modify:

1. `internal/store/store.go` - HasFiring function (modify)
2. `internal/engine/engine.go` - evaluateSyncs, processBinding, recordFiring (modify)
3. `internal/engine/engine_test.go` - Idempotency tests (modify)
4. `internal/store/store_test.go` - HasFiring tests (modify)
5. `internal/engine/replay_test.go` - Replay idempotency tests (new file)

### Relationship to Other Stories

- **Story 1.5:** Uses BindingHash function from content-addressed identity
- **Story 2.5:** Uses sync_firings table schema with binding_hash column
- **Story 4.5:** Then-clause invocation generation calls RecordFiring
- **Story 5.2:** Replay scenarios validate idempotency end-to-end

### Story Completion Checklist

- [ ] HasFiring query function implemented
- [ ] RecordFiring method in engine implemented
- [ ] HasFiring check integrated into sync evaluation loop
- [ ] INSERT OR IGNORE handles duplicates gracefully
- [ ] Multi-binding test passes (N bindings → N firings)
- [ ] Single-binding idempotency test passes
- [ ] Replay produces identical firings test passes
- [ ] Partial replay completes missing bindings test passes
- [ ] Binding hash determinism tests pass
- [ ] All unit tests pass
- [ ] All integration tests pass
- [ ] `go build ./...` succeeds
- [ ] `go vet ./...` passes
- [ ] `go test ./internal/engine/...` passes
- [ ] `go test ./internal/store/...` passes

### References

- [Source: docs/architecture.md#CRITICAL-1] - Idempotency schema blocks multi-invocation
- [Source: docs/architecture.md#CP-1] - Binding-Level Idempotency pattern
- [Source: docs/architecture.md#CP-2] - Logical Identity and Time
- [Source: docs/architecture.md#CP-3] - RFC 8785 UTF-16 key ordering
- [Source: docs/prd.md#FR-4.2] - Idempotency check via sync edges
- [Source: docs/epics.md#Story 5.1] - Story definition
- [Source: docs/sprint-artifacts/1-5-content-addressed-identity-with-domain-separation.md] - BindingHash implementation
- [Source: docs/sprint-artifacts/2-5-sync-firings-table-binding-hash.md] - sync_firings schema

## Dev Agent Record

### Agent Model Used

_To be filled during implementation_

### Validation History

_To be filled during implementation_

### Completion Notes

_To be filled during implementation_
