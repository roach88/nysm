# Story 2.7: Crash Recovery and Replay

Status: done

## Story

As a **developer running NYSM**,
I want **the engine to recover from crashes and replay identically**,
So that **durability guarantees are upheld and no work is lost**.

## Acceptance Criteria

1. **DetectIncompleteFlows function identifies crashed flows**
   ```go
   func (s *Store) DetectIncompleteFlows(ctx context.Context) ([]string, error)
   ```
   - Returns list of flow tokens with invocations but no terminal completion
   - Terminal completion = completion for invocation with no outbound provenance edges
   - Ordered by oldest seq first (FIFO recovery)

2. **ReplayFlow function implements deterministic replay**
   ```go
   func (s *Store) ReplayFlow(ctx context.Context, flowToken string) (*ReplayResult, error)

   type ReplayResult struct {
       FlowToken        string
       InvocationsFound int
       CompletionsFound int
       SyncFiringsFound int
       LastSeq          int64
       Status           FlowStatus  // "incomplete" or "complete"
   }

   type FlowStatus string
   const (
       FlowIncomplete FlowStatus = "incomplete"
       FlowComplete   FlowStatus = "complete"
   )
   ```

3. **Replay produces IDENTICAL results due to determinism**
   - Content-addressed IDs: same inputs → same ID (Story 1.5)
   - Seq restoration: seq values loaded from database, not regenerated
   - Binding hash idempotency: UNIQUE(completion_id, sync_id, binding_hash) prevents duplicate firings
   - Query ordering: ORDER BY seq ASC, id ASC COLLATE BINARY ensures same order

4. **DetectIncompleteFlows uses deterministic query ordering**
   ```sql
   -- Query must be deterministic per CP-4
   SELECT DISTINCT flow_token
   FROM invocations i
   WHERE NOT EXISTS (
       SELECT 1 FROM completions c
       WHERE c.invocation_id = i.id
       AND NOT EXISTS (
           SELECT 1 FROM provenance_edges pe
           JOIN sync_firings sf ON pe.sync_firing_id = sf.id
           WHERE sf.completion_id = c.id
       )
   )
   ORDER BY MIN(i.seq) ASC
   ```

5. **ReplayFlow reconstructs flow state from event log**
   - Reads all invocations, completions, sync_firings for flow_token
   - Returns summary statistics (counts, last seq, status)
   - Does NOT re-execute actions (read-only analysis)
   - Validates referential integrity (all completion.invocation_id exist)

6. **Comprehensive tests in `internal/store/replay_test.go`**
   - Test: DetectIncompleteFlows finds crashed flow
   - Test: DetectIncompleteFlows ignores complete flows
   - Test: ReplayFlow returns correct statistics
   - Test: Replay determinism - same flow token → identical results
   - Test: Multi-crash recovery (crash twice, replay twice, same result)
   - Test: Empty database returns empty list

## Quick Reference

| Pattern | Key Rule |
|---------|----------|
| **FR-4.3** | Enable crash/restart replay with identical results |
| **FR-5.3** | Support crash recovery and replay |
| **CP-2** | Seq is logical clock (restored from DB, not regenerated) |
| **CP-4** | All queries MUST have ORDER BY with deterministic tiebreaker |
| **CP-5** | Content-addressed IDs ensure same inputs → same ID |

## Tasks / Subtasks

- [ ] Task 1: Implement DetectIncompleteFlows (AC: #1, #4)
  - [ ] 1.1 Create `internal/store/replay.go`
  - [ ] 1.2 Define query to find flows with invocations but no terminal completions
  - [ ] 1.3 Use ORDER BY MIN(seq) ASC for FIFO recovery order
  - [ ] 1.4 Return empty slice if no incomplete flows found
  - [ ] 1.5 Use parameterized SQL with ? placeholders

- [ ] Task 2: Implement ReplayFlow (AC: #2, #3, #5)
  - [ ] 2.1 Define ReplayResult and FlowStatus types
  - [ ] 2.2 Read all invocations for flow_token (ORDER BY seq ASC, id ASC)
  - [ ] 2.3 Read all completions for flow_token (ORDER BY seq ASC, id ASC)
  - [ ] 2.4 Read all sync_firings for flow_token
  - [ ] 2.5 Compute statistics (counts, last seq)
  - [ ] 2.6 Determine flow status (incomplete vs complete)
  - [ ] 2.7 Validate referential integrity

- [ ] Task 3: Ensure deterministic replay (AC: #3)
  - [ ] 3.1 Verify all queries use ORDER BY seq ASC, id ASC
  - [ ] 3.2 Document how content-addressed IDs enable determinism
  - [ ] 3.3 Document how seq restoration works
  - [ ] 3.4 Document how binding hash prevents duplicate firings

- [ ] Task 4: Write comprehensive tests (AC: #6)
  - [ ] 4.1 Create `internal/store/replay_test.go`
  - [ ] 4.2 Test DetectIncompleteFlows with crashed flow
  - [ ] 4.3 Test DetectIncompleteFlows with complete flow
  - [ ] 4.4 Test DetectIncompleteFlows ordering (oldest first)
  - [ ] 4.5 Test ReplayFlow returns correct statistics
  - [ ] 4.6 Test replay determinism (replay twice, compare results)
  - [ ] 4.7 Test multi-crash scenario (crash → replay → crash → replay)
  - [ ] 4.8 Test empty database edge case
  - [ ] 4.9 Test missing invocation references (orphaned completions)

## Dev Notes

### Key Insight: Why Replay is Deterministic

Replay produces IDENTICAL results across multiple runs because of three guarantees:

1. **Content-Addressed IDs (Story 1.5)**
   ```go
   // Same inputs → Same ID (always)
   inv1 := ir.Invocation{FlowToken: "flow-1", ActionURI: "Cart.addItem", Args: args, Seq: 1}
   id1 := ir.InvocationID(inv1) // Hash of canonical JSON

   inv2 := ir.Invocation{FlowToken: "flow-1", ActionURI: "Cart.addItem", Args: args, Seq: 1}
   id2 := ir.InvocationID(inv2)

   // id1 == id2 ALWAYS (content-addressed)
   ```

2. **Seq Restoration (Not Regeneration)**
   ```go
   // WRONG: Regenerating seq breaks determinism
   newSeq := clock.Next() // Seq = 5 on first run, Seq = 8 on replay ❌

   // CORRECT: Restore seq from database
   invocation := readInvocationFromDB(id)
   seq := invocation.Seq // Seq = 5 on both runs ✅
   ```

3. **Binding Hash Idempotency (Story 2.5)**
   ```sql
   -- UNIQUE constraint prevents duplicate sync firings
   UNIQUE(completion_id, sync_id, binding_hash)

   -- If sync rule fires again with same binding, INSERT OR IGNORE succeeds (no-op)
   -- This makes replay idempotent: same binding values → same firing decision
   ```

### Recovery Algorithm

**Step 1: Detect Incomplete Flows**
```go
// Find flows with invocations but no terminal completion
// Terminal = completion with no outbound provenance edges
incompleteFlows, err := store.DetectIncompleteFlows(ctx)
```

**Step 2: Replay Each Flow**
```go
for _, flowToken := range incompleteFlows {
    result, err := store.ReplayFlow(ctx, flowToken)
    if err != nil {
        log.Errorf("replay failed for flow %s: %v", flowToken, err)
        continue
    }

    if result.Status == FlowIncomplete {
        log.Infof("flow %s still incomplete after replay (pending external action?)", flowToken)
    } else {
        log.Infof("flow %s recovered: %d invocations, %d completions",
            flowToken, result.InvocationsFound, result.CompletionsFound)
    }
}
```

**Step 3: Resume Normal Processing**
```go
// After recovery, engine continues normal operation
// Incomplete flows may be waiting for external action completions
```

### DetectIncompleteFlows Implementation

```go
// internal/store/replay.go

package store

import (
    "context"
    "database/sql"
    "fmt"
)

// DetectIncompleteFlows finds flows with invocations but no terminal completion.
// A terminal completion is one that has no outbound provenance edges (no follow-on syncs).
// Returns flow tokens ordered by oldest seq first (FIFO recovery).
func (s *Store) DetectIncompleteFlows(ctx context.Context) ([]string, error) {
    // Find flows where:
    // 1. Invocations exist
    // 2. No terminal completion exists (completion with no outbound provenance)
    //
    // Terminal completion = completion that didn't trigger any sync rules
    // (i.e., no sync_firings referencing it, or all sync_firings have no provenance edges)
    //
    // CRITICAL: Must use GROUP BY and ORDER BY for deterministic results (CP-4)
    query := `
        SELECT i.flow_token, MIN(i.seq) as min_seq
        FROM invocations i
        WHERE NOT EXISTS (
            SELECT 1 FROM completions c
            WHERE c.invocation_id = i.id
            AND NOT EXISTS (
                SELECT 1 FROM sync_firings sf
                JOIN provenance_edges pe ON pe.sync_firing_id = sf.id
                WHERE sf.completion_id = c.id
            )
        )
        GROUP BY i.flow_token
        ORDER BY min_seq ASC
    `

    rows, err := s.db.QueryContext(ctx, query)
    if err != nil {
        return nil, fmt.Errorf("query incomplete flows: %w", err)
    }
    defer rows.Close()

    var flowTokens []string
    for rows.Next() {
        var flowToken string
        var minSeq int64
        if err := rows.Scan(&flowToken, &minSeq); err != nil {
            return nil, fmt.Errorf("scan flow token: %w", err)
        }
        flowTokens = append(flowTokens, flowToken)
    }

    if err := rows.Err(); err != nil {
        return nil, fmt.Errorf("iterate rows: %w", err)
    }

    return flowTokens, nil
}
```

### ReplayFlow Implementation

```go
// FlowStatus indicates whether a flow is complete or still has pending work
type FlowStatus string

const (
    FlowIncomplete FlowStatus = "incomplete"
    FlowComplete   FlowStatus = "complete"
)

// ReplayResult contains statistics about a replayed flow
type ReplayResult struct {
    FlowToken        string     `json:"flow_token"`
    InvocationsFound int        `json:"invocations_found"`
    CompletionsFound int        `json:"completions_found"`
    SyncFiringsFound int        `json:"sync_firings_found"`
    LastSeq          int64      `json:"last_seq"`
    Status           FlowStatus `json:"status"`
}

// ReplayFlow reconstructs flow state from the event log.
// This is a READ-ONLY operation - it does NOT re-execute actions.
// Returns statistics about the flow and whether it's complete or incomplete.
func (s *Store) ReplayFlow(ctx context.Context, flowToken string) (*ReplayResult, error) {
    result := &ReplayResult{
        FlowToken: flowToken,
        Status:    FlowIncomplete, // Assume incomplete until proven otherwise
    }

    // Count invocations
    invCount, lastInvSeq, err := s.countInvocationsByFlow(ctx, flowToken)
    if err != nil {
        return nil, fmt.Errorf("count invocations: %w", err)
    }
    result.InvocationsFound = invCount

    // Count completions
    compCount, lastCompSeq, err := s.countCompletionsByFlow(ctx, flowToken)
    if err != nil {
        return nil, fmt.Errorf("count completions: %w", err)
    }
    result.CompletionsFound = compCount

    // Count sync firings
    firingCount, lastFiringSeq, err := s.countSyncFiringsByFlow(ctx, flowToken)
    if err != nil {
        return nil, fmt.Errorf("count sync firings: %w", err)
    }
    result.SyncFiringsFound = firingCount

    // Determine last seq (max across all record types)
    result.LastSeq = maxInt64(lastInvSeq, lastCompSeq, lastFiringSeq)

    // Check if flow is complete (has terminal completion)
    isComplete, err := s.isFlowComplete(ctx, flowToken)
    if err != nil {
        return nil, fmt.Errorf("check flow status: %w", err)
    }
    if isComplete {
        result.Status = FlowComplete
    }

    return result, nil
}

// countInvocationsByFlow returns count and last seq for invocations in a flow
func (s *Store) countInvocationsByFlow(ctx context.Context, flowToken string) (count int, lastSeq int64, err error) {
    query := `
        SELECT COUNT(*), COALESCE(MAX(seq), 0)
        FROM invocations
        WHERE flow_token = ?
    `
    err = s.db.QueryRowContext(ctx, query, flowToken).Scan(&count, &lastSeq)
    if err != nil {
        return 0, 0, fmt.Errorf("count invocations: %w", err)
    }
    return count, lastSeq, nil
}

// countCompletionsByFlow returns count and last seq for completions in a flow
func (s *Store) countCompletionsByFlow(ctx context.Context, flowToken string) (count int, lastSeq int64, err error) {
    query := `
        SELECT COUNT(*), COALESCE(MAX(c.seq), 0)
        FROM completions c
        JOIN invocations i ON c.invocation_id = i.id
        WHERE i.flow_token = ?
    `
    err = s.db.QueryRowContext(ctx, query, flowToken).Scan(&count, &lastSeq)
    if err != nil {
        return 0, 0, fmt.Errorf("count completions: %w", err)
    }
    return count, lastSeq, nil
}

// countSyncFiringsByFlow returns count and last seq for sync firings in a flow
func (s *Store) countSyncFiringsByFlow(ctx context.Context, flowToken string) (count int, lastSeq int64, err error) {
    query := `
        SELECT COUNT(*), COALESCE(MAX(sf.seq), 0)
        FROM sync_firings sf
        JOIN completions c ON sf.completion_id = c.id
        JOIN invocations i ON c.invocation_id = i.id
        WHERE i.flow_token = ?
    `
    err = s.db.QueryRowContext(ctx, query, flowToken).Scan(&count, &lastSeq)
    if err != nil {
        return 0, 0, fmt.Errorf("count sync firings: %w", err)
    }
    return count, lastSeq, nil
}

// isFlowComplete checks if flow has at least one terminal completion
// (completion with no outbound provenance edges)
func (s *Store) isFlowComplete(ctx context.Context, flowToken string) (bool, error) {
    query := `
        SELECT EXISTS (
            SELECT 1 FROM completions c
            JOIN invocations i ON c.invocation_id = i.id
            WHERE i.flow_token = ?
            AND NOT EXISTS (
                SELECT 1 FROM sync_firings sf
                JOIN provenance_edges pe ON pe.sync_firing_id = sf.id
                WHERE sf.completion_id = c.id
            )
        )
    `
    var exists bool
    err := s.db.QueryRowContext(ctx, query, flowToken).Scan(&exists)
    if err != nil {
        return false, fmt.Errorf("check terminal completion: %w", err)
    }
    return exists, nil
}

// maxInt64 returns the maximum of three int64 values
func maxInt64(a, b, c int64) int64 {
    max := a
    if b > max {
        max = b
    }
    if c > max {
        max = c
    }
    return max
}
```

### Test Examples

**Test: Detect Incomplete Flows**
```go
func TestDetectIncompleteFlows_FindsCrashedFlow(t *testing.T) {
    ctx := context.Background()
    store := setupTestStore(t)

    // Write invocation (simulating crash before completion)
    inv := ir.Invocation{
        FlowToken: "crashed-flow-1",
        ActionURI: "Cart.checkout",
        Args: ir.IRObject{"cart_id": ir.IRString("cart-123")},
        Seq: 1,
        SecurityContext: ir.SecurityContext{TenantID: "tenant1"},
        SpecHash: "hash123",
        EngineVersion: ir.EngineVersion,
        IRVersion: ir.IRVersion,
    }
    inv.ID = ir.InvocationID(inv)

    err := store.WriteInvocation(ctx, inv)
    require.NoError(t, err)

    // Detect incomplete flows
    incompleteFlows, err := store.DetectIncompleteFlows(ctx)
    require.NoError(t, err)

    // Should find the crashed flow
    assert.Contains(t, incompleteFlows, "crashed-flow-1")
}
```

**Test: Ignore Complete Flows**
```go
func TestDetectIncompleteFlows_IgnoresCompleteFlows(t *testing.T) {
    ctx := context.Background()
    store := setupTestStore(t)

    // Write invocation
    inv := ir.Invocation{
        FlowToken: "complete-flow-1",
        ActionURI: "Cart.checkout",
        Args: ir.IRObject{"cart_id": ir.IRString("cart-123")},
        Seq: 1,
        SecurityContext: ir.SecurityContext{TenantID: "tenant1"},
        SpecHash: "hash123",
        EngineVersion: ir.EngineVersion,
        IRVersion: ir.IRVersion,
    }
    inv.ID = ir.InvocationID(inv)
    err := store.WriteInvocation(ctx, inv)
    require.NoError(t, err)

    // Write terminal completion (no sync firings)
    comp := ir.Completion{
        InvocationID: inv.ID,
        OutputCase: "Success",
        Result: ir.IRObject{"status": ir.IRString("completed")},
        Seq: 2,
        SecurityContext: ir.SecurityContext{TenantID: "tenant1"},
    }
    comp.ID = ir.CompletionID(comp)
    err = store.WriteCompletion(ctx, comp)
    require.NoError(t, err)

    // Detect incomplete flows
    incompleteFlows, err := store.DetectIncompleteFlows(ctx)
    require.NoError(t, err)

    // Should NOT find the complete flow
    assert.NotContains(t, incompleteFlows, "complete-flow-1")
}
```

**Test: Replay Determinism**
```go
func TestReplayFlow_DeterministicResults(t *testing.T) {
    ctx := context.Background()
    store := setupTestStore(t)

    // Setup: Write flow data
    flowToken := "deterministic-flow-1"

    inv := ir.Invocation{
        FlowToken: flowToken,
        ActionURI: "Inventory.reserveStock",
        Args: ir.IRObject{"product_id": ir.IRString("p123"), "quantity": ir.IRInt(5)},
        Seq: 1,
        SecurityContext: ir.SecurityContext{TenantID: "tenant1"},
        SpecHash: "hash123",
        EngineVersion: ir.EngineVersion,
        IRVersion: ir.IRVersion,
    }
    inv.ID = ir.InvocationID(inv)
    err := store.WriteInvocation(ctx, inv)
    require.NoError(t, err)

    comp := ir.Completion{
        InvocationID: inv.ID,
        OutputCase: "Success",
        Result: ir.IRObject{"reserved": ir.IRBool(true)},
        Seq: 2,
        SecurityContext: ir.SecurityContext{TenantID: "tenant1"},
    }
    comp.ID = ir.CompletionID(comp)
    err = store.WriteCompletion(ctx, comp)
    require.NoError(t, err)

    // Replay flow multiple times
    result1, err := store.ReplayFlow(ctx, flowToken)
    require.NoError(t, err)

    result2, err := store.ReplayFlow(ctx, flowToken)
    require.NoError(t, err)

    result3, err := store.ReplayFlow(ctx, flowToken)
    require.NoError(t, err)

    // All replays should return IDENTICAL results
    assert.Equal(t, result1, result2, "replay 1 and 2 should be identical")
    assert.Equal(t, result2, result3, "replay 2 and 3 should be identical")

    // Verify specific values
    assert.Equal(t, flowToken, result1.FlowToken)
    assert.Equal(t, 1, result1.InvocationsFound)
    assert.Equal(t, 1, result1.CompletionsFound)
    assert.Equal(t, int64(2), result1.LastSeq)
}
```

**Test: Multi-Crash Recovery**
```go
func TestReplayFlow_MultiCrashRecovery(t *testing.T) {
    ctx := context.Background()
    store := setupTestStore(t)

    flowToken := "multi-crash-flow"

    // First run: Write invocation, crash before completion
    inv1 := ir.Invocation{
        FlowToken: flowToken,
        ActionURI: "Payment.process",
        Args: ir.IRObject{"amount": ir.IRInt(100)},
        Seq: 1,
        SecurityContext: ir.SecurityContext{TenantID: "tenant1"},
        SpecHash: "hash123",
        EngineVersion: ir.EngineVersion,
        IRVersion: ir.IRVersion,
    }
    inv1.ID = ir.InvocationID(inv1)
    err := store.WriteInvocation(ctx, inv1)
    require.NoError(t, err)

    // First replay
    replay1, err := store.ReplayFlow(ctx, flowToken)
    require.NoError(t, err)
    assert.Equal(t, FlowIncomplete, replay1.Status)
    assert.Equal(t, 1, replay1.InvocationsFound)
    assert.Equal(t, 0, replay1.CompletionsFound)

    // Second run: Add completion, crash before sync firing
    comp1 := ir.Completion{
        InvocationID: inv1.ID,
        OutputCase: "Success",
        Result: ir.IRObject{"transaction_id": ir.IRString("txn-123")},
        Seq: 2,
        SecurityContext: ir.SecurityContext{TenantID: "tenant1"},
    }
    comp1.ID = ir.CompletionID(comp1)
    err = store.WriteCompletion(ctx, comp1)
    require.NoError(t, err)

    // Second replay
    replay2, err := store.ReplayFlow(ctx, flowToken)
    require.NoError(t, err)
    assert.Equal(t, FlowComplete, replay2.Status)
    assert.Equal(t, 1, replay2.InvocationsFound)
    assert.Equal(t, 1, replay2.CompletionsFound)

    // Third replay should give IDENTICAL results to second replay
    replay3, err := store.ReplayFlow(ctx, flowToken)
    require.NoError(t, err)
    assert.Equal(t, replay2, replay3, "replay after recovery should be deterministic")
}
```

**Test: Content-Addressed ID Determinism**
```go
func TestReplayFlow_ContentAddressedIDDeterminism(t *testing.T) {
    ctx := context.Background()
    store1 := setupTestStore(t) // First database
    store2 := setupTestStore(t) // Second database (independent)

    flowToken := "deterministic-id-flow"

    // Create identical invocation in both databases
    inv := ir.Invocation{
        FlowToken: flowToken,
        ActionURI: "Cart.addItem",
        Args: ir.IRObject{
            "product_id": ir.IRString("p456"),
            "quantity": ir.IRInt(3),
        },
        Seq: 1,
        SecurityContext: ir.SecurityContext{TenantID: "tenant1"},
        SpecHash: "hash123",
        EngineVersion: ir.EngineVersion,
        IRVersion: ir.IRVersion,
    }

    // Compute ID once
    inv.ID = ir.InvocationID(inv)
    id1 := inv.ID

    // Write to first database
    err := store1.WriteInvocation(ctx, inv)
    require.NoError(t, err)

    // Write to second database (same data, independent DB)
    err = store2.WriteInvocation(ctx, inv)
    require.NoError(t, err)

    // Read from both databases
    result1, err := store1.ReplayFlow(ctx, flowToken)
    require.NoError(t, err)

    result2, err := store2.ReplayFlow(ctx, flowToken)
    require.NoError(t, err)

    // Content-addressed IDs ensure identical results across databases
    assert.Equal(t, result1, result2, "content-addressed IDs produce identical replay results")

    // Verify ID is stable
    inv1Read, err := store1.ReadInvocation(ctx, id1)
    require.NoError(t, err)

    inv2Read, err := store2.ReadInvocation(ctx, id1)
    require.NoError(t, err)

    assert.Equal(t, inv1Read.ID, inv2Read.ID, "IDs must be identical")
    assert.Equal(t, id1, inv1Read.ID, "ID must match original")
}
```

**Test: Seq Restoration**
```go
func TestReplayFlow_SeqRestoration(t *testing.T) {
    ctx := context.Background()
    store := setupTestStore(t)

    flowToken := "seq-restore-flow"

    // Write invocations with specific seq values
    for seq := int64(1); seq <= 5; seq++ {
        inv := ir.Invocation{
            FlowToken: flowToken,
            ActionURI: "Test.action",
            Args: ir.IRObject{"step": ir.IRInt(seq)},
            Seq: seq,
            SecurityContext: ir.SecurityContext{TenantID: "tenant1"},
            SpecHash: "hash123",
            EngineVersion: ir.EngineVersion,
            IRVersion: ir.IRVersion,
        }
        inv.ID = ir.InvocationID(inv)
        err := store.WriteInvocation(ctx, inv)
        require.NoError(t, err)
    }

    // Replay multiple times
    for i := 0; i < 10; i++ {
        result, err := store.ReplayFlow(ctx, flowToken)
        require.NoError(t, err)

        // LastSeq should ALWAYS be 5 (restored from DB, not regenerated)
        assert.Equal(t, int64(5), result.LastSeq,
            "seq must be restored from database, not regenerated (iteration %d)", i)
    }
}
```

### File List

Files to create:

1. `internal/store/replay.go` - Recovery and replay functions
2. `internal/store/replay_test.go` - Comprehensive replay tests

Files to reference (must exist from previous stories):

1. `internal/ir/types.go` - Invocation, Completion types
2. `internal/ir/hash.go` - InvocationID, CompletionID functions
3. `internal/store/store.go` - Store struct
4. `internal/store/read.go` - ReadInvocation, ReadCompletion functions
5. `internal/store/schema.sql` - Database schema with provenance_edges

### Story Completion Checklist

- [ ] DetectIncompleteFlows function implemented
- [ ] DetectIncompleteFlows uses deterministic ordering (MIN(seq) ASC)
- [ ] ReplayFlow function implemented
- [ ] ReplayResult and FlowStatus types defined
- [ ] ReplayFlow computes correct statistics
- [ ] ReplayFlow determines flow status (incomplete vs complete)
- [ ] All queries use parameterized SQL (no string interpolation)
- [ ] All queries use ORDER BY with deterministic tiebreaker
- [ ] Tests verify replay determinism (multiple runs → same result)
- [ ] Tests verify content-addressed ID stability
- [ ] Tests verify seq restoration (not regeneration)
- [ ] Tests verify multi-crash recovery
- [ ] Tests verify incomplete flow detection
- [ ] Tests verify complete flow ignored by detector
- [ ] `go test ./internal/store/...` passes
- [ ] `go vet ./internal/store/...` passes

### Determinism Verification

**Three pillars of deterministic replay:**

1. **Content-Addressed IDs (Story 1.5)**
   - Hash of canonical JSON ensures same inputs → same ID
   - Test: Same invocation in two databases → identical IDs

2. **Seq Restoration**
   - Seq values loaded from database, NOT regenerated
   - Test: Replay 10 times → LastSeq always the same

3. **Binding Hash Idempotency (Story 2.5)**
   - UNIQUE(completion_id, sync_id, binding_hash) prevents duplicate firings
   - Test: Replay with sync rules → same sync_firings count

**Critical Tests:**

```go
// Test 1: Replay produces byte-identical results
func TestReplayDeterminism_ByteIdentical(t *testing.T) {
    // Write flow data once
    // Replay 100 times
    // Assert all ReplayResult structs are Equal()
}

// Test 2: Content-addressed IDs are stable
func TestContentAddressedIDStability(t *testing.T) {
    // Same invocation data
    // Compute ID in two separate processes
    // Assert IDs are identical
}

// Test 3: Seq is restored, not regenerated
func TestSeqRestoration(t *testing.T) {
    // Write invocations with seq 1, 2, 3
    // Replay multiple times
    // Assert LastSeq = 3 every time (not 4, 5, 6...)
}

// Test 4: Multi-crash recovery converges
func TestMultiCrashRecovery(t *testing.T) {
    // Crash 1: Invocation only
    // Replay 1: Status = Incomplete
    // Crash 2: Add completion
    // Replay 2: Status = Complete
    // Replay 3: Identical to Replay 2
}
```

### Relationship to Other Stories

**Dependencies:**
- Story 1.5 (Content-Addressed Identity) - Required for deterministic IDs
- Story 2.1 (SQLite Store) - Required for database
- Story 2.2 (Event Log Schema) - Required for tables and seq column
- Story 2.3 (Write Invocations/Completions) - Required for writes
- Story 2.4 (Read Flow) - Required for reading flow data
- Story 2.5 (Sync Firings) - Required for understanding terminal completions
- Story 2.6 (Provenance Edges) - Required for detecting incomplete flows

**Enables:**
- Story 3.1 (Single-Writer Event Loop) - Engine can recover and resume
- Story 7.5 (Manual Replay Command) - User-facing replay functionality
- All future work - Crash recovery is foundational for durability

**Note:** This is the FINAL story in Epic 2 (Durable Event Store). After this, the engine has full crash recovery with deterministic replay guarantees.

### References

- [Source: docs/prd.md#FR-4.3] - Enable crash/restart replay with identical results
- [Source: docs/prd.md#FR-5.3] - Support crash recovery and replay
- [Source: docs/epics.md#Story 2.7] - Story definition and acceptance criteria
- [Source: docs/architecture.md#CP-2] - Logical Identity and Time
- [Source: docs/architecture.md#CP-4] - Deterministic Query Results
- [Source: docs/architecture.md#CRITICAL-2] - Deterministic Replay Requirements
- [Source: Story 1.5] - Content-addressed ID computation
- [Source: Story 2.2] - Logical clock (seq) instead of timestamps
- [Source: Story 2.5] - Binding hash for idempotent sync firings

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Validation History

- Initial creation: comprehensive story generation for crash recovery

### Completion Notes

- Crash recovery is READ-ONLY analysis (does NOT re-execute actions)
- Determinism relies on three guarantees: content-addressed IDs, seq restoration, binding hash idempotency
- DetectIncompleteFlows uses FIFO ordering (oldest seq first) for predictable recovery
- Terminal completion = completion with no outbound provenance edges
- Replay can run multiple times safely (idempotent)
- Multi-crash scenarios are supported (crash → replay → crash → replay → same result)
- Empty database is a valid edge case (returns empty list)
- This completes Epic 2 - all durability guarantees are now in place
- Next epic (Epic 3: Sync Engine Core) can rely on crash recovery for safety
