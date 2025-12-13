# Story 2.6: Provenance Edges Table

Status: ready-for-dev

## Story

As a **developer tracing causality**,
I want **a provenance_edges table linking firings to invocations**,
So that **I can answer "why did this happen?"**.

## Acceptance Criteria

1. **Provenance edges table created in schema (AC: #1)**
   - Table: `provenance_edges` with columns: `id`, `sync_firing_id`, `invocation_id`
   - `id` is INTEGER PRIMARY KEY (auto-increment for store FK only)
   - `sync_firing_id` references `sync_firings(id)` with FOREIGN KEY constraint
   - `invocation_id` references `invocations(id)` with FOREIGN KEY constraint
   - `UNIQUE(sync_firing_id)` ensures each firing produces exactly one invocation

2. **Index on invocation_id for backward tracing (AC: #2)**
   - `CREATE INDEX idx_provenance_invocation ON provenance_edges(invocation_id)`
   - Enables efficient "what caused this invocation?" queries
   - Index required for ReadProvenance performance

3. **WriteProvenanceEdge function in `internal/store/write.go` (AC: #3)**
   - Signature: `func (s *Store) WriteProvenanceEdge(ctx context.Context, edge ir.ProvenanceEdge) error`
   - Validates `sync_firing_id` and `invocation_id` are non-zero/non-empty
   - Uses parameterized SQL (HIGH-3 security pattern)
   - Returns error if foreign key constraints violated
   - Respects context cancellation

4. **ReadProvenance function for backward queries (AC: #4)**
   - Signature: `func (s *Store) ReadProvenance(ctx context.Context, invocationID string) ([]ir.ProvenanceEdge, error)`
   - Queries: "What caused this invocation?"
   - Returns all provenance edges for the given invocation
   - Joins with sync_firings to include completion_id and sync_id
   - Returns empty slice (not error) if no provenance found
   - Results ordered by `seq` for determinism

5. **ReadTriggered function for forward queries (AC: #5)**
   - Signature: `func (s *Store) ReadTriggered(ctx context.Context, completionID string) ([]ir.Invocation, error)`
   - Queries: "What did this completion trigger?"
   - Joins provenance_edges → sync_firings → invocations
   - Returns all invocations triggered by the completion (via any sync rule)
   - Results ordered by invocation `seq` for determinism
   - Returns empty slice (not error) if nothing triggered

6. **Comprehensive tests in `internal/store/provenance_test.go` (AC: #6)**
   - Normal case: write edge, read provenance, read triggered
   - Error case: foreign key violations
   - Edge case: invocation with no provenance (user-initiated)
   - Edge case: completion that triggered nothing
   - Multi-binding case: single completion triggers multiple invocations
   - Ordering determinism tests

## Quick Reference

| Pattern | Key Rule |
|---------|----------|
| **FR-4.1** | Record provenance edges: (completion) -[sync-id]-> (invocation) |
| **FR-5.2** | Store provenance with query support |
| **NFR-1.3** | Queryable provenance enables "why did this happen?" debugging |
| **HIGH-3** | Parameterized queries only - never string interpolation |

## Tasks / Subtasks

- [ ] Task 1: Add provenance_edges table to schema (AC: #1, #2)
  - [ ] 1.1 Add `CREATE TABLE provenance_edges` to `internal/store/schema.sql`
  - [ ] 1.2 Add `CREATE INDEX idx_provenance_invocation` to schema
  - [ ] 1.3 Verify foreign key constraints reference correct tables
  - [ ] 1.4 Verify `UNIQUE(sync_firing_id)` constraint present

- [ ] Task 2: Implement WriteProvenanceEdge (AC: #3)
  - [ ] 2.1 Add `WriteProvenanceEdge(ctx, edge)` to `internal/store/write.go`
  - [ ] 2.2 Use parameterized SQL with INSERT
  - [ ] 2.3 Validate sync_firing_id and invocation_id are present
  - [ ] 2.4 Handle foreign key constraint errors with clear messages
  - [ ] 2.5 Respect context cancellation

- [ ] Task 3: Implement ReadProvenance (AC: #4)
  - [ ] 3.1 Create `internal/store/read_provenance.go` or add to `read.go`
  - [ ] 3.2 Implement `ReadProvenance(ctx, invocationID)` function
  - [ ] 3.3 Join provenance_edges with sync_firings to get full context
  - [ ] 3.4 Return empty slice for invocations with no provenance
  - [ ] 3.5 Order results by `seq` for determinism

- [ ] Task 4: Implement ReadTriggered (AC: #5)
  - [ ] 4.1 Implement `ReadTriggered(ctx, completionID)` function
  - [ ] 4.2 Join provenance_edges → sync_firings → invocations
  - [ ] 4.3 Return all invocations caused by the completion
  - [ ] 4.4 Order results by invocation seq for determinism
  - [ ] 4.5 Return empty slice for completions that triggered nothing

- [ ] Task 5: Write comprehensive tests (AC: #6)
  - [ ] 5.1 Create `internal/store/provenance_test.go`
  - [ ] 5.2 Test WriteProvenanceEdge normal case
  - [ ] 5.3 Test foreign key violations (nonexistent firing/invocation)
  - [ ] 5.4 Test ReadProvenance for user-initiated invocations (no provenance)
  - [ ] 5.5 Test ReadTriggered for terminal completions (triggered nothing)
  - [ ] 5.6 Test multi-binding scenario (one completion → many invocations)
  - [ ] 5.7 Test result ordering determinism

- [ ] Task 6: Verify and document provenance queries (AC: #4, #5)
  - [ ] 6.1 Document query patterns in code comments
  - [ ] 6.2 Verify all SQL uses `ORDER BY` with tiebreaker (CP-4)
  - [ ] 6.3 Verify all SQL uses parameterized queries (HIGH-3)
  - [ ] 6.4 Add example queries to story documentation

## Dev Notes

### Critical Implementation Details

**Schema Definition**
```sql
-- internal/store/schema.sql (add to existing file)

-- Provenance Edges: Link sync firings to generated invocations
-- This enables "why did this happen?" causality queries
CREATE TABLE IF NOT EXISTS provenance_edges (
    id INTEGER PRIMARY KEY,           -- Auto-increment (store FK only)
    sync_firing_id INTEGER NOT NULL REFERENCES sync_firings(id),
    invocation_id TEXT NOT NULL REFERENCES invocations(id),
    UNIQUE(sync_firing_id)            -- Each firing produces exactly one invocation
);

-- Index for backward queries: "what caused this invocation?"
CREATE INDEX IF NOT EXISTS idx_provenance_invocation
    ON provenance_edges(invocation_id);
```

**Parameterized SQL - HIGH-3 Security Pattern**
```go
// CORRECT: Parameterized query
_, err := s.db.ExecContext(ctx, `
    INSERT INTO provenance_edges (sync_firing_id, invocation_id)
    VALUES (?, ?)
`, edge.SyncFiringID, edge.InvocationID)

// WRONG: String interpolation - SQL injection risk
query := fmt.Sprintf("INSERT INTO provenance_edges VALUES (%d, '%s')",
    edge.SyncFiringID, edge.InvocationID) // ❌ NEVER
```

**ProvenanceEdge Type (Already Defined in Story 1.1)**
```go
// internal/ir/store_types.go - already exists

// ProvenanceEdge links a sync firing to its generated invocation (store-layer)
type ProvenanceEdge struct {
    ID           int64  `json:"id"`             // Auto-increment (store FK)
    SyncFiringID int64  `json:"sync_firing_id"`
    InvocationID string `json:"invocation_id"`  // Content-addressed
}
```

### Function Signatures

**WriteProvenanceEdge Implementation**
```go
// internal/store/write.go (add to existing file)

// WriteProvenanceEdge records that a sync firing produced an invocation.
// This enables causality tracing: "why did this invocation happen?"
//
// Foreign key constraints ensure:
// - sync_firing_id references an existing sync_firings(id)
// - invocation_id references an existing invocations(id)
//
// The UNIQUE(sync_firing_id) constraint ensures each firing produces
// exactly one invocation (1:1 relationship).
func (s *Store) WriteProvenanceEdge(ctx context.Context, edge ir.ProvenanceEdge) error {
    // Validate required fields
    if edge.SyncFiringID == 0 {
        return fmt.Errorf("sync_firing_id is required")
    }
    if edge.InvocationID == "" {
        return fmt.Errorf("invocation_id is required")
    }

    // Insert provenance edge (parameterized SQL)
    _, err := s.db.ExecContext(ctx, `
        INSERT INTO provenance_edges (sync_firing_id, invocation_id)
        VALUES (?, ?)
    `, edge.SyncFiringID, edge.InvocationID)

    if err != nil {
        // Check for foreign key violations
        if strings.Contains(err.Error(), "FOREIGN KEY") {
            return fmt.Errorf("invalid reference: sync_firing_id=%d or invocation_id=%s not found: %w",
                edge.SyncFiringID, edge.InvocationID, err)
        }
        // Check for duplicate sync_firing_id
        if strings.Contains(err.Error(), "UNIQUE") {
            return fmt.Errorf("sync_firing_id %d already has a provenance edge: %w",
                edge.SyncFiringID, err)
        }
        return fmt.Errorf("insert provenance edge: %w", err)
    }

    return nil
}
```

**ReadProvenance Implementation (Backward Query)**
```go
// internal/store/read.go or read_provenance.go

// ReadProvenance answers: "What caused this invocation?"
// Returns all sync firings that produced this invocation (typically one).
//
// Returns empty slice (not error) if invocation has no provenance
// (e.g., user-initiated invocations have no provenance edges).
//
// Results include:
// - ProvenanceEdge with ID, SyncFiringID, InvocationID
// - Associated SyncFiring data (CompletionID, SyncID, BindingHash, Seq)
//
// Results ordered by seq for deterministic replay.
func (s *Store) ReadProvenance(ctx context.Context, invocationID string) ([]ProvenanceRecord, error) {
    if invocationID == "" {
        return nil, fmt.Errorf("invocation_id is required")
    }

    rows, err := s.db.QueryContext(ctx, `
        SELECT
            pe.id,
            pe.sync_firing_id,
            pe.invocation_id,
            sf.completion_id,
            sf.sync_id,
            sf.binding_hash,
            sf.seq
        FROM provenance_edges pe
        JOIN sync_firings sf ON pe.sync_firing_id = sf.id
        WHERE pe.invocation_id = ?
        ORDER BY sf.seq ASC
    `, invocationID)
    if err != nil {
        return nil, fmt.Errorf("query provenance for invocation %s: %w", invocationID, err)
    }
    defer rows.Close()

    var records []ProvenanceRecord
    for rows.Next() {
        var rec ProvenanceRecord
        err := rows.Scan(
            &rec.EdgeID,
            &rec.SyncFiringID,
            &rec.InvocationID,
            &rec.CompletionID,
            &rec.SyncID,
            &rec.BindingHash,
            &rec.Seq,
        )
        if err != nil {
            return nil, fmt.Errorf("scan provenance record: %w", err)
        }
        records = append(records, rec)
    }

    if err := rows.Err(); err != nil {
        return nil, fmt.Errorf("iterate provenance rows: %w", err)
    }

    return records, nil
}

// ProvenanceRecord combines provenance edge with sync firing context
type ProvenanceRecord struct {
    EdgeID       int64  // provenance_edges.id
    SyncFiringID int64  // sync_firings.id
    InvocationID string // invocations.id (what was triggered)
    CompletionID string // completions.id (what triggered it)
    SyncID       string // sync rule ID
    BindingHash  string // binding values hash
    Seq          int64  // logical clock
}
```

**ReadTriggered Implementation (Forward Query)**
```go
// ReadTriggered answers: "What did this completion trigger?"
// Returns all invocations produced by sync firings from this completion.
//
// Returns empty slice (not error) if completion triggered nothing
// (e.g., terminal completions or no matching sync rules).
//
// A single completion can trigger multiple invocations if:
// - Multiple sync rules match the completion
// - A single sync rule produces multiple bindings (multi-binding query)
//
// Results ordered by invocation seq for deterministic replay.
func (s *Store) ReadTriggered(ctx context.Context, completionID string) ([]ir.Invocation, error) {
    if completionID == "" {
        return nil, fmt.Errorf("completion_id is required")
    }

    rows, err := s.db.QueryContext(ctx, `
        SELECT
            i.id,
            i.flow_token,
            i.action_uri,
            i.args,
            i.seq,
            i.security_context,
            i.spec_hash,
            i.engine_version,
            i.ir_version
        FROM invocations i
        JOIN provenance_edges pe ON i.id = pe.invocation_id
        JOIN sync_firings sf ON pe.sync_firing_id = sf.id
        WHERE sf.completion_id = ?
        ORDER BY i.seq ASC, i.id ASC COLLATE BINARY
    `, completionID)
    if err != nil {
        return nil, fmt.Errorf("query triggered invocations for completion %s: %w", completionID, err)
    }
    defer rows.Close()

    var invocations []ir.Invocation
    for rows.Next() {
        var inv ir.Invocation
        var argsJSON, securityJSON string

        err := rows.Scan(
            &inv.ID,
            &inv.FlowToken,
            &inv.ActionURI,
            &argsJSON,
            &inv.Seq,
            &securityJSON,
            &inv.SpecHash,
            &inv.EngineVersion,
            &inv.IRVersion,
        )
        if err != nil {
            return nil, fmt.Errorf("scan invocation: %w", err)
        }

        // Deserialize args
        if err := ir.UnmarshalCanonical([]byte(argsJSON), &inv.Args); err != nil {
            return nil, fmt.Errorf("unmarshal invocation args: %w", err)
        }

        // Deserialize security context
        if err := ir.UnmarshalCanonical([]byte(securityJSON), &inv.SecurityContext); err != nil {
            return nil, fmt.Errorf("unmarshal security context: %w", err)
        }

        invocations = append(invocations, inv)
    }

    if err := rows.Err(); err != nil {
        return nil, fmt.Errorf("iterate triggered invocations: %w", err)
    }

    return invocations, nil
}
```

### Provenance Query Examples

**Example 1: User-Initiated Invocation (No Provenance)**
```go
// User calls Order.Create directly
// This invocation has NO provenance (nothing caused it)

inv := ir.Invocation{
    FlowToken: "flow-user-123",
    ActionURI: "Order.Create",
    Args: ir.IRObject{"product": ir.IRString("widget")},
    // ... other fields
}
inv.ID = ir.InvocationID(inv)
store.WriteInvocation(ctx, inv)

// Query provenance
provenance, err := store.ReadProvenance(ctx, inv.ID)
// provenance == []ProvenanceRecord{} (empty slice)
// This is CORRECT - user-initiated invocations have no cause
```

**Example 2: Simple Chain (One Completion Triggers One Invocation)**
```go
// Scenario: Order.Create completes → triggers Inventory.ReserveStock

// 1. User invokes Order.Create
orderInv := ir.Invocation{
    ID: "inv-order-123",
    FlowToken: "flow-order-456",
    ActionURI: "Order.Create",
    Args: ir.IRObject{"product": ir.IRString("widget"), "qty": ir.IRInt(5)},
    Seq: 1,
    // ...
}
store.WriteInvocation(ctx, orderInv)

// 2. Order.Create completes successfully
orderComp := ir.Completion{
    ID: "comp-order-123",
    InvocationID: orderInv.ID,
    OutputCase: "Success",
    Result: ir.IRObject{"order_id": ir.IRString("ord-789")},
    Seq: 2,
    // ...
}
store.WriteCompletion(ctx, orderComp)

// 3. Sync rule fires: when Order.Create completes, reserve stock
syncFiring := ir.SyncFiring{
    CompletionID: orderComp.ID,
    SyncID: "sync-reserve-stock",
    BindingHash: ir.BindingHash(ir.IRObject{
        "order_id": ir.IRString("ord-789"),
        "product": ir.IRString("widget"),
        "qty": ir.IRInt(5),
    }),
    Seq: 3,
}
// WriteSyncFiring returns syncFiring with auto-increment ID populated
result, err := store.WriteSyncFiring(ctx, syncFiring)
syncFiring.ID = result.LastInsertId() // e.g., 1

// 4. Sync generates invocation: Inventory.ReserveStock
reserveInv := ir.Invocation{
    FlowToken: orderInv.FlowToken, // Same flow!
    ActionURI: "Inventory.ReserveStock",
    Args: ir.IRObject{
        "product": ir.IRString("widget"),
        "quantity": ir.IRInt(5),
    },
    Seq: 4,
    // ...
}
reserveInv.ID = ir.InvocationID(reserveInv)
store.WriteInvocation(ctx, reserveInv)

// 5. Record provenance: firing → invocation
edge := ir.ProvenanceEdge{
    SyncFiringID: syncFiring.ID,
    InvocationID: reserveInv.ID,
}
store.WriteProvenanceEdge(ctx, edge)

// QUERY: "Why did Inventory.ReserveStock get invoked?"
provenance, err := store.ReadProvenance(ctx, reserveInv.ID)
// provenance == []ProvenanceRecord{{
//     SyncFiringID: 1,
//     InvocationID: "inv-reserve-...",
//     CompletionID: "comp-order-123",
//     SyncID: "sync-reserve-stock",
//     BindingHash: "hash-of-bindings",
//     Seq: 3,
// }}

// ANSWER: "Inventory.ReserveStock was triggered by Order.Create completion
//          via sync rule 'sync-reserve-stock' at seq=3"

// QUERY: "What did Order.Create completion trigger?"
triggered, err := store.ReadTriggered(ctx, orderComp.ID)
// triggered == []ir.Invocation{ reserveInv }

// ANSWER: "Order.Create completion triggered Inventory.ReserveStock"
```

**Example 3: Multi-Binding (One Completion Triggers Many Invocations)**
```go
// Scenario: ShipmentArrived completion triggers RefillStock for each item

// 1. Shipment arrives with 3 items
arrivalComp := ir.Completion{
    ID: "comp-arrival-123",
    OutputCase: "Success",
    Result: ir.IRObject{
        "items": ir.IRArray{
            ir.IRObject{"product": ir.IRString("widget"), "qty": ir.IRInt(100)},
            ir.IRObject{"product": ir.IRString("gadget"), "qty": ir.IRInt(50)},
            ir.IRObject{"product": ir.IRString("doohickey"), "qty": ir.IRInt(75)},
        },
    },
    Seq: 10,
    // ...
}
store.WriteCompletion(ctx, arrivalComp)

// 2. Sync rule fires with 3 bindings (one per item)
// Each binding produces a separate sync_firing and provenance_edge

bindings := []ir.IRObject{
    {"product": ir.IRString("widget"), "qty": ir.IRInt(100)},
    {"product": ir.IRString("gadget"), "qty": ir.IRInt(50)},
    {"product": ir.IRString("doohickey"), "qty": ir.IRInt(75)},
}

var refillInvs []ir.Invocation
for i, binding := range bindings {
    // 2a. Record sync firing for this binding
    firing := ir.SyncFiring{
        CompletionID: arrivalComp.ID,
        SyncID: "sync-refill-on-arrival",
        BindingHash: ir.BindingHash(binding),
        Seq: 11 + int64(i), // seq=11, 12, 13
    }
    result, _ := store.WriteSyncFiring(ctx, firing)
    firing.ID = result.LastInsertId()

    // 2b. Generate refill invocation
    refillInv := ir.Invocation{
        ActionURI: "Inventory.RefillStock",
        Args: binding,
        Seq: 11 + int64(i),
        // ...
    }
    refillInv.ID = ir.InvocationID(refillInv)
    store.WriteInvocation(ctx, refillInv)

    // 2c. Record provenance
    edge := ir.ProvenanceEdge{
        SyncFiringID: firing.ID,
        InvocationID: refillInv.ID,
    }
    store.WriteProvenanceEdge(ctx, edge)

    refillInvs = append(refillInvs, refillInv)
}

// QUERY: "What did ShipmentArrived completion trigger?"
triggered, err := store.ReadTriggered(ctx, arrivalComp.ID)
// triggered == []ir.Invocation{
//     {ActionURI: "Inventory.RefillStock", Args: {product: "widget", qty: 100}},
//     {ActionURI: "Inventory.RefillStock", Args: {product: "gadget", qty: 50}},
//     {ActionURI: "Inventory.RefillStock", Args: {product: "doohickey", qty: 75}},
// }

// ANSWER: "ShipmentArrived triggered 3 RefillStock invocations (one per item)"

// QUERY: "Why did RefillStock get called for 'gadget'?"
provenance, err := store.ReadProvenance(ctx, refillInvs[1].ID)
// provenance == []ProvenanceRecord{{
//     CompletionID: "comp-arrival-123",
//     SyncID: "sync-refill-on-arrival",
//     BindingHash: "hash-of-{product:gadget,qty:50}",
//     Seq: 12,
// }}

// ANSWER: "RefillStock for 'gadget' was triggered by ShipmentArrived completion
//          via sync rule 'sync-refill-on-arrival' with binding {product:gadget, qty:50}"
```

**Example 4: Causality Chain (Invocation → Completion → Firing → Invocation)**
```go
// Scenario: Order.Create → Inventory.ReserveStock → Notification.Send

// Trace backward from Notification.Send to root cause
notificationInvID := "inv-notify-xyz"

// Step 1: What caused Notification.Send?
prov1, _ := store.ReadProvenance(ctx, notificationInvID)
// prov1[0].CompletionID == "comp-reserve-abc" (Inventory.ReserveStock completed)
// prov1[0].SyncID == "sync-notify-on-reserve"

// Step 2: What invocation produced that completion?
reserveComp, _ := store.ReadCompletion(ctx, prov1[0].CompletionID)
reserveInvID := reserveComp.InvocationID

// Step 3: What caused Inventory.ReserveStock?
prov2, _ := store.ReadProvenance(ctx, reserveInvID)
// prov2[0].CompletionID == "comp-order-123" (Order.Create completed)
// prov2[0].SyncID == "sync-reserve-on-order"

// Step 4: What invocation produced that completion?
orderComp, _ := store.ReadCompletion(ctx, prov2[0].CompletionID)
orderInvID := orderComp.InvocationID

// Step 5: What caused Order.Create?
prov3, _ := store.ReadProvenance(ctx, orderInvID)
// prov3 == [] (empty - user-initiated)

// COMPLETE TRACE:
// User → Order.Create → (sync) → Inventory.ReserveStock → (sync) → Notification.Send
//        inv-order-123           inv-reserve-abc                    inv-notify-xyz
```

### Error Handling Patterns

**Foreign Key Violations**
```go
// WriteProvenanceEdge with nonexistent sync_firing_id
edge := ir.ProvenanceEdge{
    SyncFiringID: 999999, // Does not exist
    InvocationID: "inv-valid-123",
}
err := store.WriteProvenanceEdge(ctx, edge)
// err: "invalid reference: sync_firing_id=999999 or invocation_id=inv-valid-123 not found: FOREIGN KEY constraint failed"

// WriteProvenanceEdge with nonexistent invocation_id
edge := ir.ProvenanceEdge{
    SyncFiringID: 1,
    InvocationID: "inv-nonexistent", // Does not exist
}
err := store.WriteProvenanceEdge(ctx, edge)
// err: "invalid reference: sync_firing_id=1 or invocation_id=inv-nonexistent not found: FOREIGN KEY constraint failed"
```

**Duplicate Sync Firing ID**
```go
// Each sync_firing_id can only have ONE provenance edge
// UNIQUE(sync_firing_id) enforces this

// First edge succeeds
edge1 := ir.ProvenanceEdge{
    SyncFiringID: 1,
    InvocationID: "inv-first-123",
}
store.WriteProvenanceEdge(ctx, edge1) // OK

// Second edge with same sync_firing_id fails
edge2 := ir.ProvenanceEdge{
    SyncFiringID: 1, // Same firing
    InvocationID: "inv-second-456", // Different invocation
}
err := store.WriteProvenanceEdge(ctx, edge2)
// err: "sync_firing_id 1 already has a provenance edge: UNIQUE constraint failed"

// This is CORRECT - each firing produces exactly one invocation
```

**Empty Results (Not Errors)**
```go
// ReadProvenance for user-initiated invocation
provenance, err := store.ReadProvenance(ctx, "inv-user-initiated")
// provenance == []ProvenanceRecord{} (empty slice)
// err == nil

// ReadTriggered for terminal completion
triggered, err := store.ReadTriggered(ctx, "comp-terminal-123")
// triggered == []ir.Invocation{} (empty slice)
// err == nil

// Empty results are VALID - not all invocations have provenance,
// not all completions trigger follow-ons
```

### Test Examples

**Test Normal Provenance Write and Read**
```go
func TestProvenanceEdge_WriteAndRead(t *testing.T) {
    ctx := context.Background()
    store := setupTestStore(t)

    // Create invocation
    inv := testInvocation("Order.Create", 1)
    require.NoError(t, store.WriteInvocation(ctx, inv))

    // Create completion
    comp := testCompletion(inv.ID, "Success", 2)
    require.NoError(t, store.WriteCompletion(ctx, comp))

    // Create sync firing
    firing := ir.SyncFiring{
        CompletionID: comp.ID,
        SyncID: "sync-test",
        BindingHash: "hash123",
        Seq: 3,
    }
    result, err := store.WriteSyncFiring(ctx, firing)
    require.NoError(t, err)
    firing.ID = result.LastInsertId()

    // Create triggered invocation
    triggeredInv := testInvocation("Inventory.ReserveStock", 4)
    require.NoError(t, store.WriteInvocation(ctx, triggeredInv))

    // Write provenance edge
    edge := ir.ProvenanceEdge{
        SyncFiringID: firing.ID,
        InvocationID: triggeredInv.ID,
    }
    err = store.WriteProvenanceEdge(ctx, edge)
    require.NoError(t, err)

    // Read provenance backward
    provenance, err := store.ReadProvenance(ctx, triggeredInv.ID)
    require.NoError(t, err)
    require.Len(t, provenance, 1)
    assert.Equal(t, firing.ID, provenance[0].SyncFiringID)
    assert.Equal(t, comp.ID, provenance[0].CompletionID)
    assert.Equal(t, "sync-test", provenance[0].SyncID)

    // Read triggered forward
    triggered, err := store.ReadTriggered(ctx, comp.ID)
    require.NoError(t, err)
    require.Len(t, triggered, 1)
    assert.Equal(t, triggeredInv.ID, triggered[0].ID)
    assert.Equal(t, "Inventory.ReserveStock", string(triggered[0].ActionURI))
}
```

**Test Foreign Key Violations**
```go
func TestProvenanceEdge_ForeignKeyViolations(t *testing.T) {
    ctx := context.Background()
    store := setupTestStore(t)

    // Test nonexistent sync_firing_id
    edge := ir.ProvenanceEdge{
        SyncFiringID: 99999,
        InvocationID: "inv-valid-123",
    }
    err := store.WriteProvenanceEdge(ctx, edge)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "invalid reference")
    assert.Contains(t, err.Error(), "FOREIGN KEY")

    // Test nonexistent invocation_id
    // First create a valid sync firing
    inv := testInvocation("Test.Action", 1)
    store.WriteInvocation(ctx, inv)
    comp := testCompletion(inv.ID, "Success", 2)
    store.WriteCompletion(ctx, comp)
    firing := ir.SyncFiring{CompletionID: comp.ID, SyncID: "test", BindingHash: "h", Seq: 3}
    result, _ := store.WriteSyncFiring(ctx, firing)

    edge = ir.ProvenanceEdge{
        SyncFiringID: result.LastInsertId(),
        InvocationID: "inv-nonexistent",
    }
    err = store.WriteProvenanceEdge(ctx, edge)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "invalid reference")
}
```

**Test User-Initiated Invocation (No Provenance)**
```go
func TestReadProvenance_UserInitiated(t *testing.T) {
    ctx := context.Background()
    store := setupTestStore(t)

    // User-initiated invocation (no provenance)
    inv := testInvocation("Order.Create", 1)
    require.NoError(t, store.WriteInvocation(ctx, inv))

    // Read provenance - should return empty slice
    provenance, err := store.ReadProvenance(ctx, inv.ID)
    require.NoError(t, err)
    assert.Empty(t, provenance, "user-initiated invocations have no provenance")
}
```

**Test Terminal Completion (Triggered Nothing)**
```go
func TestReadTriggered_TerminalCompletion(t *testing.T) {
    ctx := context.Background()
    store := setupTestStore(t)

    // Create invocation and completion
    inv := testInvocation("Notification.Send", 1)
    require.NoError(t, store.WriteInvocation(ctx, inv))
    comp := testCompletion(inv.ID, "Success", 2)
    require.NoError(t, store.WriteCompletion(ctx, comp))

    // No sync firings created (terminal action)

    // Read triggered - should return empty slice
    triggered, err := store.ReadTriggered(ctx, comp.ID)
    require.NoError(t, err)
    assert.Empty(t, triggered, "terminal completions trigger nothing")
}
```

**Test Multi-Binding Scenario**
```go
func TestReadTriggered_MultiBinding(t *testing.T) {
    ctx := context.Background()
    store := setupTestStore(t)

    // Setup: one completion triggers 3 invocations (multi-binding)
    inv := testInvocation("ShipmentArrived", 1)
    store.WriteInvocation(ctx, inv)
    comp := testCompletion(inv.ID, "Success", 2)
    store.WriteCompletion(ctx, comp)

    // Create 3 sync firings (different bindings)
    products := []string{"widget", "gadget", "doohickey"}
    var triggeredInvs []ir.Invocation

    for i, product := range products {
        // Create sync firing
        firing := ir.SyncFiring{
            CompletionID: comp.ID,
            SyncID: "sync-refill",
            BindingHash: fmt.Sprintf("hash-%s", product),
            Seq: 3 + int64(i),
        }
        result, _ := store.WriteSyncFiring(ctx, firing)
        firing.ID = result.LastInsertId()

        // Create triggered invocation
        refillInv := testInvocation("Inventory.RefillStock", 3+int64(i))
        refillInv.Args = ir.IRObject{"product": ir.IRString(product)}
        store.WriteInvocation(ctx, refillInv)

        // Record provenance
        edge := ir.ProvenanceEdge{
            SyncFiringID: firing.ID,
            InvocationID: refillInv.ID,
        }
        store.WriteProvenanceEdge(ctx, edge)

        triggeredInvs = append(triggeredInvs, refillInv)
    }

    // Query: what did ShipmentArrived trigger?
    triggered, err := store.ReadTriggered(ctx, comp.ID)
    require.NoError(t, err)
    require.Len(t, triggered, 3, "should trigger 3 invocations")

    // Verify all 3 invocations returned
    invIDs := make(map[string]bool)
    for _, inv := range triggered {
        invIDs[inv.ID] = true
    }
    for _, expectedInv := range triggeredInvs {
        assert.True(t, invIDs[expectedInv.ID], "missing invocation %s", expectedInv.ID)
    }
}
```

**Test Result Ordering Determinism**
```go
func TestReadTriggered_OrderingDeterminism(t *testing.T) {
    ctx := context.Background()
    store := setupTestStore(t)

    // Create completion that triggers multiple invocations
    inv := testInvocation("BatchJob.Complete", 1)
    store.WriteInvocation(ctx, inv)
    comp := testCompletion(inv.ID, "Success", 2)
    store.WriteCompletion(ctx, comp)

    // Create 5 sync firings with same seq (must sort by ID as tiebreaker)
    expectedOrder := []string{}
    for i := 0; i < 5; i++ {
        firing := ir.SyncFiring{
            CompletionID: comp.ID,
            SyncID: fmt.Sprintf("sync-%d", i),
            BindingHash: fmt.Sprintf("hash-%d", i),
            Seq: 3, // Same seq!
        }
        result, _ := store.WriteSyncFiring(ctx, firing)
        firing.ID = result.LastInsertId()

        triggeredInv := testInvocation(fmt.Sprintf("Task.Run-%d", i), 3)
        store.WriteInvocation(ctx, triggeredInv)

        edge := ir.ProvenanceEdge{SyncFiringID: firing.ID, InvocationID: triggeredInv.ID}
        store.WriteProvenanceEdge(ctx, edge)

        expectedOrder = append(expectedOrder, triggeredInv.ID)
    }

    // Query multiple times - order MUST be deterministic
    for run := 0; run < 3; run++ {
        triggered, err := store.ReadTriggered(ctx, comp.ID)
        require.NoError(t, err)
        require.Len(t, triggered, 5)

        actualOrder := []string{}
        for _, inv := range triggered {
            actualOrder = append(actualOrder, inv.ID)
        }

        assert.Equal(t, expectedOrder, actualOrder,
            "run %d: order must be deterministic (seq ASC, id ASC COLLATE BINARY)", run)
    }
}
```

### File List

Files to modify:

1. `internal/store/schema.sql` - Add provenance_edges table and index
2. `internal/store/write.go` - Add WriteProvenanceEdge function
3. `internal/store/read.go` or `read_provenance.go` - Add ReadProvenance and ReadTriggered

Files to create:

1. `internal/store/provenance_test.go` - Comprehensive provenance tests

Files that must exist (from previous stories):

1. `internal/ir/store_types.go` - ProvenanceEdge type (Story 1.1)
2. `internal/ir/types.go` - Invocation, Completion types
3. `internal/store/store.go` - Store struct with db field
4. `internal/store/schema.sql` - sync_firings table (Story 2.5)

### Story Completion Checklist

- [ ] provenance_edges table added to schema
- [ ] idx_provenance_invocation index created
- [ ] UNIQUE(sync_firing_id) constraint present
- [ ] Foreign key constraints reference correct tables
- [ ] WriteProvenanceEdge function implemented
- [ ] ReadProvenance function implemented
- [ ] ReadTriggered function implemented
- [ ] All SQL uses parameterized queries (no string interpolation)
- [ ] All queries use ORDER BY with tiebreaker (CP-4)
- [ ] Empty results return empty slice, not error
- [ ] Foreign key violations return clear error messages
- [ ] Test: normal write and read
- [ ] Test: foreign key violations
- [ ] Test: user-initiated invocation (no provenance)
- [ ] Test: terminal completion (triggered nothing)
- [ ] Test: multi-binding scenario
- [ ] Test: result ordering determinism
- [ ] `go vet ./internal/store/...` passes
- [ ] `go test ./internal/store/...` passes

### References

- [Source: docs/prd.md#FR-4.1] - Record provenance edges requirement
- [Source: docs/prd.md#FR-5.2] - Store provenance with query support
- [Source: docs/prd.md#NFR-1.3] - Queryable provenance for debugging
- [Source: docs/architecture.md#Store Interface] - ReadProvenance signature
- [Source: docs/architecture.md#HIGH-3] - Parameterized queries only
- [Source: docs/architecture.md#CP-4] - Deterministic ordering
- [Source: docs/epics.md#Story 2.6] - Story definition
- [Source: docs/epics.md#Story 2.5] - sync_firings table (prerequisite)
- [Source: Story 1.1] - ProvenanceEdge type definition
- [Source: Story 2.2] - Event log schema foundation

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Validation History

- Initial creation: comprehensive story generation

### Completion Notes

- Provenance edges enable causality tracing: "why did this happen?"
- ReadProvenance queries backward: invocation → firing → completion (cause)
- ReadTriggered queries forward: completion → firing → invocation (effect)
- Each sync_firing_id maps to exactly ONE invocation (UNIQUE constraint)
- Multi-binding syncs create multiple provenance edges (one per binding)
- User-initiated invocations have NO provenance (empty slice, not error)
- Terminal completions trigger NOTHING (empty slice, not error)
- All queries use ORDER BY seq, id for deterministic results (CP-4)
- Foreign key constraints ensure referential integrity
- Parameterized SQL prevents injection attacks (HIGH-3)
- ProvenanceRecord type joins edge with sync firing context for rich debugging
- Story implements FR-4.1, FR-5.2, and NFR-1.3 (queryable provenance)
