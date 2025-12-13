# Story 5.2: Idempotency in Replay Scenarios

Status: ready-for-dev

## Story

As a **developer recovering from crashes**,
I want **replay to skip already-fired sync bindings**,
So that **recovery produces identical results and doesn't create duplicate invocations**.

## Acceptance Criteria

1. **Replay skips already-fired sync bindings via HasFiring check**
   ```go
   func (s *Store) HasFiring(ctx context.Context, completionID, syncID, bindingHash string) bool
   ```
   - Returns true if (completion_id, sync_id, binding_hash) triple exists in sync_firings
   - Engine calls HasFiring before attempting to insert sync_firing
   - If HasFiring returns true, skip invocation generation (already processed)
   - If HasFiring returns false, proceed with sync firing and invocation

2. **Replay produces identical final state**
   - Given: Flow partially processed before crash (some syncs fired, some not)
   - When: Engine replays the flow from event log
   - Then:
     - Completions are re-processed
     - Sync rules are re-evaluated
     - Binding hashes are re-computed (deterministically via ir.BindingHash)
     - Already-fired bindings are skipped via HasFiring check
     - Only NEW bindings (not in sync_firings table) produce invocations
     - Final state (all tables) is identical to non-crash run

3. **Content-addressed IDs ensure same computation = same ID**
   ```go
   // Same binding values ALWAYS produce same hash (Story 1.5)
   binding1 := ir.IRObject{"item_id": ir.IRString("widget"), "quantity": ir.IRInt(3)}
   hash1 := ir.BindingHash(binding1) // "abc123..."

   binding2 := ir.IRObject{"item_id": ir.IRString("widget"), "quantity": ir.IRInt(3)}
   hash2 := ir.BindingHash(binding2) // "abc123..." (identical)

   // Different binding values produce different hashes
   binding3 := ir.IRObject{"item_id": ir.IRString("widget"), "quantity": ir.IRInt(5)}
   hash3 := ir.BindingHash(binding3) // "def456..." (different)
   ```

4. **No special replay mode needed**
   - Idempotency is structural (database UNIQUE constraint + HasFiring check)
   - Engine doesn't need to know if it's in "replay mode" vs "normal mode"
   - Same code path handles both initial execution and replay
   - Database enforces correctness via UNIQUE(completion_id, sync_id, binding_hash)

5. **Comprehensive tests in `internal/engine/replay_test.go`**
   - Test: Partial replay skips existing sync firings
   - Test: Full replay produces identical invocations/completions
   - Test: New bindings during replay DO fire
   - Test: Binding hash collision (same values, different order) → same skip
   - Test: Multi-crash scenario (crash → replay → crash → replay → same result)
   - Test: Replay with zero existing firings behaves like normal execution

## Quick Reference

| Pattern | Key Rule |
|---------|----------|
| **FR-4.2** | Support idempotency check via sync edges |
| **FR-4.3** | Enable crash/restart replay with identical results |
| **CP-1** | Binding-level idempotency via UNIQUE(completion_id, sync_id, binding_hash) |
| **CP-4** | Content-addressed IDs ensure deterministic hash computation |
| **Story 2.5** | sync_firings table with binding_hash column |
| **Story 5.1** | HasFiring function for idempotency check |

## Tasks / Subtasks

- [ ] Task 1: Document replay flow with HasFiring integration (AC: #1, #4)
  - [ ] 1.1 Create `internal/engine/replay.go` with replay documentation
  - [ ] 1.2 Document how engine processes completions during replay
  - [ ] 1.3 Document HasFiring check before sync firing
  - [ ] 1.4 Document skip vs fire decision logic
  - [ ] 1.5 Add examples showing skip/fire behavior

- [ ] Task 2: Implement replay test scenarios (AC: #2, #3, #5)
  - [ ] 2.1 Create `internal/engine/replay_test.go`
  - [ ] 2.2 Test: Partial replay skips existing firings
  - [ ] 2.3 Test: Full replay produces identical results
  - [ ] 2.4 Test: New bindings during replay fire correctly
  - [ ] 2.5 Test: Binding hash determinism (same values → same hash)
  - [ ] 2.6 Test: Multi-crash recovery scenario

- [ ] Task 3: Verify content-addressed ID stability (AC: #3)
  - [ ] 3.1 Test binding hash computation determinism
  - [ ] 3.2 Test invocation ID stability across replays
  - [ ] 3.3 Test completion ID stability across replays
  - [ ] 3.4 Verify ir.MarshalCanonical used for hashing

- [ ] Task 4: Integration tests for full replay cycle (AC: #2, #4)
  - [ ] 4.1 Test cart-inventory demo with crash mid-flow
  - [ ] 4.2 Test replay produces same final state
  - [ ] 4.3 Test replay doesn't duplicate invocations
  - [ ] 4.4 Test replay with multiple sync rules
  - [ ] 4.5 Test replay with multi-binding syncs

## Dev Notes

### Replay Flow Architecture

**Normal Execution:**
```
[Completion] → [Match Syncs] → [Execute Where] → [Bindings]
                                                      ↓
                                          [For each binding]
                                                      ↓
                                          [Check HasFiring?]
                                                      ↓
                                          NO → Fire sync → Write sync_firing
                                          YES → Skip (already processed)
```

**Replay (Identical Path):**
```
[Completion from DB] → [Match Syncs] → [Execute Where] → [Bindings]
                                                              ↓
                                                  [For each binding]
                                                              ↓
                                                  [Check HasFiring?]
                                                              ↓
                                                  NO → Fire sync → Write sync_firing
                                                  YES → Skip (already in DB)
```

### Key Insight: Structural Idempotency

Idempotency is NOT a separate "replay mode" - it's built into the engine's normal operation:

1. **Database Constraint**
   ```sql
   UNIQUE(completion_id, sync_id, binding_hash)
   ```
   Prevents duplicate (completion, sync, binding) triples at write time.

2. **HasFiring Check**
   ```go
   if store.HasFiring(ctx, completion.ID, sync.ID, bindingHash) {
       continue // Skip - already processed
   }
   ```
   Prevents duplicate invocation generation before write.

3. **Content-Addressed Binding Hash**
   ```go
   bindingHash := ir.BindingHash(binding)
   ```
   Same binding values always produce the same hash (deterministic via RFC 8785 canonical JSON).

Result: **Replay can't create duplicates even if it tries.** The database and HasFiring check enforce correctness.

### Why Content-Addressed IDs Matter

Content-addressed IDs ensure replay produces the same IDs for the same inputs:

**Invocation IDs:**
```go
// Original run
inv1 := ir.Invocation{
    FlowToken: "flow-1",
    ActionURI: "Inventory.reserve",
    Args: ir.IRObject{"item_id": ir.IRString("widget"), "qty": ir.IRInt(3)},
    Seq: 5,
}
id1 := ir.InvocationID(inv1) // Hash of canonical JSON

// Replay (same inputs)
inv2 := ir.Invocation{
    FlowToken: "flow-1",
    ActionURI: "Inventory.reserve",
    Args: ir.IRObject{"item_id": ir.IRString("widget"), "qty": ir.IRInt(3)},
    Seq: 5, // Restored from DB (Story 2.7)
}
id2 := ir.InvocationID(inv2) // IDENTICAL to id1

// id1 == id2 ALWAYS
```

**Binding Hashes:**
```go
// Original run
binding1 := ir.IRObject{"item_id": ir.IRString("widget"), "qty": ir.IRInt(3)}
hash1 := ir.BindingHash(binding1) // "abc123..."

// Replay (same where-clause result)
binding2 := ir.IRObject{"item_id": ir.IRString("widget"), "qty": ir.IRInt(3)}
hash2 := ir.BindingHash(binding2) // "abc123..." (IDENTICAL)

// HasFiring check uses hash2 to detect duplicate
exists := store.HasFiring(ctx, completionID, syncID, hash2)
// exists == true → Skip firing
```

### Replay Scenarios

**Scenario 1: Partial Replay (Crash Mid-Sync)**

```
Before Crash:
  [Completion C1] → Sync "cart-inventory"
                    → Where-clause returns 3 bindings: B1, B2, B3
                    → Fired B1 ✓ (wrote sync_firing)
                    → Fired B2 ✓ (wrote sync_firing)
                    → CRASH before firing B3

After Replay:
  [Completion C1] → Sync "cart-inventory"
                    → Where-clause returns 3 bindings: B1, B2, B3
                    → Check HasFiring(C1, "cart-inventory", hash(B1)) → true → Skip
                    → Check HasFiring(C1, "cart-inventory", hash(B2)) → true → Skip
                    → Check HasFiring(C1, "cart-inventory", hash(B3)) → false → Fire ✓

Result: Only B3 fires on replay. Final state identical to non-crash run.
```

**Scenario 2: Full Replay (No Previous Firings)**

```
Original Run:
  [Completion C1] → Sync "cart-inventory"
                    → Bindings: B1, B2, B3
                    → Fire B1, B2, B3 ✓

Replay (Empty DB):
  [Completion C1] → Sync "cart-inventory"
                    → Bindings: B1, B2, B3 (identical via deterministic query)
                    → Check HasFiring for each → all false
                    → Fire B1, B2, B3 ✓

Result: Identical sync_firings and invocations created.
```

**Scenario 3: New Bindings During Replay**

```
Before Crash:
  [Completion C1] → Where-clause returns bindings: B1, B2
                    → Fire B1, B2 ✓
  CRASH

Data Change:
  User manually adds row to state table (unusual but possible)

Replay:
  [Completion C1] → Where-clause returns bindings: B1, B2, B3 (new!)
                    → Check HasFiring(C1, sync, hash(B1)) → true → Skip
                    → Check HasFiring(C1, sync, hash(B2)) → true → Skip
                    → Check HasFiring(C1, sync, hash(B3)) → false → Fire ✓

Result: B3 fires (new data), B1/B2 skip (already processed).
Note: This scenario is rare - state should be derived from event log.
```

**Scenario 4: Multi-Crash Recovery**

```
Run 1:
  [Completion C1] → Fire B1 ✓
  CRASH

Replay 1:
  [Completion C1] → Fire B2 ✓
  CRASH

Replay 2:
  [Completion C1] → Fire B3 ✓
  Complete ✓

Replay 3:
  [Completion C1] → Skip B1, B2, B3 (all exist)
  No new firings

Result: Replay is idempotent across multiple crashes.
```

### Test Examples

**Test: Partial Replay Skips Existing Firings**
```go
func TestReplay_PartialReplaySkipsExistingFirings(t *testing.T) {
    ctx := context.Background()
    store := setupTestStore(t)
    engine := setupTestEngine(t, store)

    // Setup: Write completion that triggers sync with 3 bindings
    comp := ir.Completion{
        FlowToken: "flow-1",
        InvocationID: "inv-1",
        OutputCase: "Success",
        Result: ir.IRObject{"cart_id": ir.IRString("cart-123")},
        Seq: 2,
        SecurityContext: ir.SecurityContext{TenantID: "tenant1"},
    }
    comp.ID = ir.CompletionID(comp)
    err := store.WriteCompletion(ctx, comp)
    require.NoError(t, err)

    // Simulate crash after firing 2 of 3 bindings
    // (State table has 3 items, but only 2 sync_firings exist)

    // Write CartItem state (3 items)
    writeCartItem(t, store, "cart-123", "item-A", 1)
    writeCartItem(t, store, "cart-123", "item-B", 2)
    writeCartItem(t, store, "cart-123", "item-C", 3)

    // Write sync_firings for first 2 items (simulating partial execution)
    firing1 := ir.SyncFiring{
        CompletionID: comp.ID,
        SyncID: "cart-inventory",
        BindingHash: ir.BindingHash(ir.IRObject{"item_id": ir.IRString("item-A"), "qty": ir.IRInt(1)}),
        Seq: 3,
    }
    err = store.WriteSyncFiring(ctx, firing1)
    require.NoError(t, err)

    firing2 := ir.SyncFiring{
        CompletionID: comp.ID,
        SyncID: "cart-inventory",
        BindingHash: ir.BindingHash(ir.IRObject{"item_id": ir.IRString("item-B"), "qty": ir.IRInt(2)}),
        Seq: 4,
    }
    err = store.WriteSyncFiring(ctx, firing2)
    require.NoError(t, err)

    // CRASH before firing item-C

    // Replay: Process completion again
    err = engine.ProcessCompletion(ctx, comp)
    require.NoError(t, err)

    // Verify: Only 1 NEW sync_firing created (for item-C)
    firings, err := store.ReadSyncFiringsByCompletion(ctx, comp.ID)
    require.NoError(t, err)
    assert.Len(t, firings, 3, "should have 3 total firings")

    // Verify: First 2 firings unchanged (same IDs)
    assert.Equal(t, firing1.ID, firings[0].ID)
    assert.Equal(t, firing2.ID, firings[1].ID)

    // Verify: Third firing is new
    assert.Equal(t, "cart-inventory", firings[2].SyncID)
    assert.Equal(t,
        ir.BindingHash(ir.IRObject{"item_id": ir.IRString("item-C"), "qty": ir.IRInt(3)}),
        firings[2].BindingHash)
}
```

**Test: Full Replay Produces Identical Results**
```go
func TestReplay_FullReplayIdenticalToOriginal(t *testing.T) {
    ctx := context.Background()

    // Run 1: Original execution (empty DB)
    store1 := setupTestStore(t)
    engine1 := setupTestEngine(t, store1)

    comp := createTestCompletion(t, "flow-1")
    err := engine1.ProcessCompletion(ctx, comp)
    require.NoError(t, err)

    // Capture original state
    firings1, err := store1.ReadSyncFiringsByCompletion(ctx, comp.ID)
    require.NoError(t, err)

    invocations1, err := store1.ReadInvocationsByFlow(ctx, "flow-1")
    require.NoError(t, err)

    // Run 2: Replay from scratch (independent DB)
    store2 := setupTestStore(t)
    engine2 := setupTestEngine(t, store2)

    // Copy state tables and completion to second DB
    copyStateTablesAndCompletion(t, store1, store2, "flow-1")

    err = engine2.ProcessCompletion(ctx, comp)
    require.NoError(t, err)

    // Capture replay state
    firings2, err := store2.ReadSyncFiringsByCompletion(ctx, comp.ID)
    require.NoError(t, err)

    invocations2, err := store2.ReadInvocationsByFlow(ctx, "flow-1")
    require.NoError(t, err)

    // Verify: Identical results (same counts, same IDs, same hashes)
    assert.Equal(t, len(firings1), len(firings2), "same number of sync firings")
    assert.Equal(t, len(invocations1), len(invocations2), "same number of invocations")

    for i := range firings1 {
        // Content-addressed binding hashes are identical
        assert.Equal(t, firings1[i].BindingHash, firings2[i].BindingHash,
            "binding hash %d must be identical", i)
    }

    for i := range invocations1 {
        // Content-addressed invocation IDs are identical
        assert.Equal(t, invocations1[i].ID, invocations2[i].ID,
            "invocation ID %d must be identical", i)
    }
}
```

**Test: New Bindings During Replay Do Fire**
```go
func TestReplay_NewBindingsDuringReplayFire(t *testing.T) {
    ctx := context.Background()
    store := setupTestStore(t)
    engine := setupTestEngine(t, store)

    comp := createTestCompletion(t, "flow-1")

    // First execution: 2 items in cart
    writeCartItem(t, store, "cart-123", "item-A", 1)
    writeCartItem(t, store, "cart-123", "item-B", 2)

    err := engine.ProcessCompletion(ctx, comp)
    require.NoError(t, err)

    // Verify: 2 sync firings created
    firings1, err := store.ReadSyncFiringsByCompletion(ctx, comp.ID)
    require.NoError(t, err)
    assert.Len(t, firings1, 2)

    // User manually adds new item (simulating data change)
    writeCartItem(t, store, "cart-123", "item-C", 3)

    // Replay: Process completion again
    err = engine.ProcessCompletion(ctx, comp)
    require.NoError(t, err)

    // Verify: 3 sync firings now (1 new)
    firings2, err := store.ReadSyncFiringsByCompletion(ctx, comp.ID)
    require.NoError(t, err)
    assert.Len(t, firings2, 3, "should have 3 total firings (2 old + 1 new)")

    // Verify: First 2 firings unchanged
    assert.Equal(t, firings1[0].BindingHash, firings2[0].BindingHash)
    assert.Equal(t, firings1[1].BindingHash, firings2[1].BindingHash)

    // Verify: Third firing is new (item-C)
    newFiring := firings2[2]
    expectedHash := ir.BindingHash(ir.IRObject{"item_id": ir.IRString("item-C"), "qty": ir.IRInt(3)})
    assert.Equal(t, expectedHash, newFiring.BindingHash)
}
```

**Test: Binding Hash Collision (Same Values, Different Order)**
```go
func TestReplay_BindingHashDeterministic(t *testing.T) {
    // RFC 8785 canonical JSON ensures key ordering
    // Same keys/values → same hash regardless of input order

    binding1 := ir.IRObject{
        "item_id": ir.IRString("widget"),
        "quantity": ir.IRInt(5),
        "price": ir.IRInt(100),
    }

    binding2 := ir.IRObject{
        "price": ir.IRInt(100),       // Different order
        "item_id": ir.IRString("widget"),
        "quantity": ir.IRInt(5),
    }

    hash1 := ir.BindingHash(binding1)
    hash2 := ir.BindingHash(binding2)

    // Hashes MUST be identical (canonical JSON sorts keys)
    assert.Equal(t, hash1, hash2, "binding hash must be deterministic")
}
```

**Test: Multi-Crash Recovery**
```go
func TestReplay_MultiCrashRecovery(t *testing.T) {
    ctx := context.Background()
    store := setupTestStore(t)
    engine := setupTestEngine(t, store)

    comp := createTestCompletion(t, "flow-1")

    // State: 3 items in cart
    writeCartItem(t, store, "cart-123", "item-A", 1)
    writeCartItem(t, store, "cart-123", "item-B", 2)
    writeCartItem(t, store, "cart-123", "item-C", 3)

    // Crash 1: Process completion, crash after 1 firing
    // (manually create 1 firing to simulate partial execution)
    firing1 := ir.SyncFiring{
        CompletionID: comp.ID,
        SyncID: "cart-inventory",
        BindingHash: ir.BindingHash(ir.IRObject{"item_id": ir.IRString("item-A"), "qty": ir.IRInt(1)}),
        Seq: 3,
    }
    err := store.WriteSyncFiring(ctx, firing1)
    require.NoError(t, err)

    // Replay 1: Should fire B and C
    err = engine.ProcessCompletion(ctx, comp)
    require.NoError(t, err)

    firingsAfterReplay1, err := store.ReadSyncFiringsByCompletion(ctx, comp.ID)
    require.NoError(t, err)
    assert.Len(t, firingsAfterReplay1, 3, "should have 3 firings after replay 1")

    // Crash 2: Simulate another crash (no new firings, just testing idempotence)

    // Replay 2: Should skip all (already exist)
    err = engine.ProcessCompletion(ctx, comp)
    require.NoError(t, err)

    firingsAfterReplay2, err := store.ReadSyncFiringsByCompletion(ctx, comp.ID)
    require.NoError(t, err)
    assert.Len(t, firingsAfterReplay2, 3, "should still have 3 firings (no duplicates)")

    // Replay 3: Verify idempotence (replay N times → same result)
    for i := 0; i < 10; i++ {
        err = engine.ProcessCompletion(ctx, comp)
        require.NoError(t, err)

        firings, err := store.ReadSyncFiringsByCompletion(ctx, comp.ID)
        require.NoError(t, err)
        assert.Len(t, firings, 3, "replay %d: should always have 3 firings", i)
    }
}
```

**Test: Replay with Zero Existing Firings (Normal Execution)**
```go
func TestReplay_ZeroExistingFiringsBehavesNormally(t *testing.T) {
    ctx := context.Background()
    store := setupTestStore(t)
    engine := setupTestEngine(t, store)

    comp := createTestCompletion(t, "flow-1")

    // State: 2 items in cart
    writeCartItem(t, store, "cart-123", "item-A", 1)
    writeCartItem(t, store, "cart-123", "item-B", 2)

    // Process completion (empty sync_firings table)
    err := engine.ProcessCompletion(ctx, comp)
    require.NoError(t, err)

    // Verify: 2 sync firings created
    firings, err := store.ReadSyncFiringsByCompletion(ctx, comp.ID)
    require.NoError(t, err)
    assert.Len(t, firings, 2)

    // Verify: 2 invocations created
    invocations, err := store.ReadInvocationsByFlow(ctx, "flow-1")
    require.NoError(t, err)
    assert.Len(t, invocations, 2)

    // No difference from "normal" execution - replay is just normal execution with HasFiring checks
}
```

### File List

Files to create:
1. `internal/engine/replay.go` - Replay documentation and flow examples
2. `internal/engine/replay_test.go` - Comprehensive replay scenario tests

Files to reference (must exist from previous stories):
1. `internal/ir/hash.go` - BindingHash function (Story 1.5)
2. `internal/ir/json.go` - MarshalCanonical function (Story 1.4)
3. `internal/store/store.go` - Store interface
4. `internal/store/write.go` - WriteSyncFiring function (Story 2.5)
5. `internal/store/read.go` - ReadSyncFiringsByCompletion function
6. `internal/store/schema.sql` - UNIQUE(completion_id, sync_id, binding_hash) constraint
7. `internal/engine/engine.go` - ProcessCompletion function (Story 5.1)

### Story Completion Checklist

- [ ] Replay flow documented in `internal/engine/replay.go`
- [ ] HasFiring integration point documented
- [ ] Content-addressed ID determinism documented
- [ ] Structural idempotency explanation clear
- [ ] Test: Partial replay skips existing firings
- [ ] Test: Full replay produces identical results
- [ ] Test: New bindings during replay fire correctly
- [ ] Test: Binding hash determinism verified
- [ ] Test: Multi-crash recovery scenario
- [ ] Test: Zero existing firings behaves normally
- [ ] Test: Binding hash collision (same values, different order)
- [ ] Integration test with cart-inventory demo
- [ ] `go test ./internal/engine/...` passes
- [ ] `go vet ./internal/engine/...` passes
- [ ] All tests use deterministic assertions (no flakiness)

### Relationship to Other Stories

**Dependencies:**
- Story 1.4 (RFC 8785 Canonical JSON) - Required for deterministic binding hash
- Story 1.5 (Content-Addressed Identity) - Required for deterministic IDs
- Story 2.5 (Sync Firings Table) - Required for UNIQUE constraint and binding_hash column
- Story 2.7 (Crash Recovery and Replay) - Required for DetectIncompleteFlows and ReplayFlow
- Story 5.1 (Binding-Level Idempotency Enforcement) - Required for HasFiring function

**Enables:**
- Story 7.5 (Replay Command) - User can manually replay flows and verify determinism
- All future work - Replay safety is foundational for durability guarantees

**Note:** This is the FINAL story in Epic 5 (Idempotency & Cycle Safety). After this, the engine has complete idempotency guarantees across crash/recovery scenarios.

### References

- [Source: docs/prd.md#FR-4.2] - Support idempotency check via sync edges
- [Source: docs/prd.md#FR-4.3] - Enable crash/restart replay with identical results
- [Source: docs/epics.md#Story 5.2] - Story definition and acceptance criteria
- [Source: docs/architecture.md#CP-1] - Binding-Level Idempotency
- [Source: docs/architecture.md#CP-2] - Logical Identity and Time
- [Source: docs/architecture.md#CRITICAL-1] - Idempotency Schema Blocks Multi-Invocation
- [Source: docs/architecture.md#CRITICAL-2] - Deterministic Replay Requires Logical Identity
- [Source: Story 1.5] - Content-addressed ID computation
- [Source: Story 2.5] - Binding hash for idempotent sync firings
- [Source: Story 5.1] - HasFiring function implementation

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Validation History

- Initial creation: comprehensive story generation for replay idempotency scenarios

### Completion Notes

- Idempotency is STRUCTURAL (database constraint + HasFiring check), not a special "replay mode"
- Replay uses the same code path as normal execution (no branches)
- Content-addressed IDs ensure same inputs → same IDs across replays
- Binding hash is deterministic via RFC 8785 canonical JSON (sorted keys)
- HasFiring check is cheap (index lookup) - no performance penalty
- Multi-crash recovery is supported (replay N times → same result)
- New bindings during replay DO fire (rare but correct)
- Zero existing firings behaves identically to normal execution
- This completes Epic 5 - all idempotency guarantees are now in place
- Next epic (Epic 6: Conformance Harness) can rely on deterministic replay for golden testing
