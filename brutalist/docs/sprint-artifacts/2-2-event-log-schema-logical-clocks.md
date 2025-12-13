# Story 2.2: Event Log Schema with Logical Clocks

Status: ready-for-dev

## Story

As a **developer building the store**,
I want **a schema using logical clocks instead of timestamps**,
So that **replay produces identical results regardless of wall time**.

## Acceptance Criteria

1. **Invocations table uses logical clock (seq), not timestamps**
   - `seq INTEGER NOT NULL` for ordering (per CP-2)
   - Content-addressed `id TEXT PRIMARY KEY`
   - NO `CURRENT_TIMESTAMP`, `datetime('now')`, or similar
   - `security_context TEXT NOT NULL` (always present per CP-6)
   - Indexes on `flow_token` and `seq`

2. **Completions table uses logical clock (seq), not timestamps**
   - `seq INTEGER NOT NULL` for ordering (per CP-2)
   - Content-addressed `id TEXT PRIMARY KEY`
   - Foreign key to `invocations(id)`
   - Index on flow token via invocations join

3. **Schema uses canonical JSON for args/result storage**
   - `args TEXT NOT NULL` - serialized via `ir.MarshalCanonical()`
   - `result TEXT NOT NULL` - serialized via `ir.MarshalCanonical()`
   - `security_context TEXT NOT NULL` - serialized JSON

4. **Schema includes version tracking fields**
   - `spec_hash TEXT NOT NULL` - hash of concept spec at invoke time
   - `engine_version TEXT NOT NULL` - NYSM engine version

5. **All queries use deterministic ordering per CP-4**
   - Every SELECT includes `ORDER BY seq ASC, id ASC COLLATE BINARY`
   - Stable ordering for replay consistency

## Quick Reference

| Pattern | Key Rule |
|---------|----------|
| **CP-2** | Logical clocks (`seq`), NO wall-clock timestamps ever |
| **CP-3** | RFC 8785 canonical JSON for args/result serialization |
| **CP-4** | Deterministic query ordering - always `ORDER BY seq, id` |
| **CP-6** | SecurityContext always present (non-pointer) on all records |

## Tasks / Subtasks

- [ ] Task 1: Create invocations table schema (AC: #1, #4)
  - [ ] 1.1 Define invocations table with content-addressed id
  - [ ] 1.2 Add seq INTEGER NOT NULL (logical clock)
  - [ ] 1.3 Add flow_token, action_uri, args TEXT columns
  - [ ] 1.4 Add security_context TEXT NOT NULL column
  - [ ] 1.5 Add spec_hash, engine_version, ir_version columns
  - [ ] 1.6 Create indexes on flow_token and seq

- [ ] Task 2: Create completions table schema (AC: #2)
  - [ ] 2.1 Define completions table with content-addressed id
  - [ ] 2.2 Add invocation_id foreign key reference
  - [ ] 2.3 Add output_case, result TEXT columns
  - [ ] 2.4 Add seq INTEGER NOT NULL (logical clock)
  - [ ] 2.5 Add security_context TEXT NOT NULL column
  - [ ] 2.6 Create index on seq
  - [ ] 2.7 Create composite index for flow token queries

- [ ] Task 3: Validate schema correctness (AC: #1, #2, #3)
  - [ ] 3.1 Verify NO timestamp columns (grep for TIMESTAMP, datetime)
  - [ ] 3.2 Verify all TEXT columns for JSON use NOT NULL
  - [ ] 3.3 Verify foreign key constraints are defined
  - [ ] 3.4 Verify indexes cover query patterns

- [ ] Task 4: Create Go types for schema mapping (AC: #3, #5)
  - [ ] 4.1 Update store package with read/write functions
  - [ ] 4.2 Implement canonical JSON serialization helpers
  - [ ] 4.3 Add deterministic query builders with ORDER BY
  - [ ] 4.4 Create tests for schema DDL execution

- [ ] Task 5: Test schema and queries (AC: #5)
  - [ ] 5.1 Test INSERT invocation with all fields
  - [ ] 5.2 Test INSERT completion with foreign key
  - [ ] 5.3 Test SELECT with deterministic ordering
  - [ ] 5.4 Test flow token queries join correctly
  - [ ] 5.5 Verify schema.sql can be loaded multiple times (idempotent)

## Dev Notes

### Critical Pattern Details

**CP-2: Logical Clocks (NO Timestamps)**
```sql
-- CORRECT: Use seq INTEGER for logical clock
CREATE TABLE invocations (
    id TEXT PRIMARY KEY,           -- Content-addressed hash
    seq INTEGER NOT NULL,          -- Logical clock - monotonic counter
    -- ...
);

-- WRONG - NEVER DO THIS:
CREATE TABLE invocations (
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,  -- ❌ Breaks replay
    -- OR
    created_at INTEGER DEFAULT (strftime('%s', 'now'))  -- ❌ Also breaks replay
);
```

**CP-4: Deterministic Query Ordering**
```sql
-- CORRECT: Always ORDER BY with deterministic tiebreaker
SELECT id, flow_token, action_uri, args, seq
FROM invocations
WHERE flow_token = ?
ORDER BY seq ASC, id ASC COLLATE BINARY;

-- WRONG - Missing ORDER BY:
SELECT id, flow_token, action_uri, args, seq
FROM invocations
WHERE flow_token = ?;  -- ❌ Non-deterministic row order
```

**CP-6: Security Context on All Records**
```sql
-- SecurityContext MUST be on BOTH invocations AND completions
-- ALWAYS NOT NULL, NEVER optional

CREATE TABLE invocations (
    -- ...
    security_context TEXT NOT NULL,  -- ✓ Always present, JSON serialized
);

CREATE TABLE completions (
    -- ...
    security_context TEXT NOT NULL,  -- ✓ Always present, JSON serialized
);

-- WRONG:
security_context TEXT,  -- ❌ Allows NULL
```

### Complete Schema Definition

**internal/store/schema.sql:**
```sql
-- NYSM Event Log Schema
-- Uses logical clocks (seq) NOT timestamps for deterministic replay (CP-2)

-- Invocations: Action invocation records
CREATE TABLE IF NOT EXISTS invocations (
    -- Identity (content-addressed via ir.InvocationID)
    id TEXT PRIMARY KEY,

    -- Core fields
    flow_token TEXT NOT NULL,
    action_uri TEXT NOT NULL,
    args TEXT NOT NULL,                -- Canonical JSON via ir.MarshalCanonical()

    -- Ordering (logical clock - CP-2)
    seq INTEGER NOT NULL,              -- Monotonic per-engine counter, NOT wall-clock

    -- Security & Audit (CP-6)
    security_context TEXT NOT NULL,    -- JSON, always present

    -- Version tracking
    spec_hash TEXT NOT NULL,           -- Hash of concept spec at invoke time
    engine_version TEXT NOT NULL,      -- e.g., "0.1.0"
    ir_version TEXT NOT NULL           -- e.g., "1"
);

-- Indexes for query performance
CREATE INDEX IF NOT EXISTS idx_invocations_flow_token
    ON invocations(flow_token);
CREATE INDEX IF NOT EXISTS idx_invocations_seq
    ON invocations(seq);

-- Completions: Action completion records
CREATE TABLE IF NOT EXISTS completions (
    -- Identity (content-addressed via ir.CompletionID)
    id TEXT PRIMARY KEY,

    -- Core fields
    invocation_id TEXT NOT NULL REFERENCES invocations(id),
    output_case TEXT NOT NULL,         -- "Success", error variant name
    result TEXT NOT NULL,              -- Canonical JSON via ir.MarshalCanonical()

    -- Ordering (logical clock - CP-2)
    seq INTEGER NOT NULL,              -- Monotonic per-engine counter, NOT wall-clock

    -- Security & Audit (CP-6)
    security_context TEXT NOT NULL     -- JSON, always present
);

-- Indexes for query performance
CREATE INDEX IF NOT EXISTS idx_completions_seq
    ON completions(seq);

-- Composite index for flow token queries (via invocations join)
-- This index supports queries like:
-- SELECT c.* FROM completions c
-- JOIN invocations i ON c.invocation_id = i.id
-- WHERE i.flow_token = ?
-- ORDER BY c.seq ASC
CREATE INDEX IF NOT EXISTS idx_completions_invocation_seq
    ON completions(invocation_id, seq);

-- NOTE: sync_firings and provenance_edges tables are in Story 2.5 and 2.6
```

### Go Type Mappings

**Store-Layer Type Mapping:**
```go
// internal/store/write.go

// marshalArgs converts IRObject to canonical JSON TEXT
func marshalArgs(args ir.IRObject) string {
    data, err := ir.MarshalCanonical(args)
    if err != nil {
        panic(fmt.Sprintf("failed to marshal args: %v", err))
    }
    return string(data)
}

// marshalResult converts IRObject to canonical JSON TEXT
func marshalResult(result ir.IRObject) string {
    data, err := ir.MarshalCanonical(result)
    if err != nil {
        panic(fmt.Sprintf("failed to marshal result: %v", err))
    }
    return string(data)
}

// marshalSecurityContext converts SecurityContext to canonical JSON TEXT
// NOTE: Use canonical JSON for consistency with args/result fields
// This ensures golden trace comparison works correctly
func marshalSecurityContext(ctx ir.SecurityContext) string {
    data, err := ir.MarshalCanonical(ctx)  // Canonical JSON per CP-3
    if err != nil {
        panic(fmt.Sprintf("failed to marshal security context: %v", err))
    }
    return string(data)
}

// WriteInvocation inserts an invocation record
func (s *Store) WriteInvocation(ctx context.Context, inv ir.Invocation) error {
    _, err := s.db.ExecContext(ctx, `
        INSERT OR IGNORE INTO invocations
        (id, flow_token, action_uri, args, seq, security_context, spec_hash, engine_version, ir_version)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
    `,
        inv.ID,
        inv.FlowToken,
        string(inv.ActionURI),  // ActionRef to string
        marshalArgs(inv.Args),
        inv.Seq,
        marshalSecurityContext(inv.SecurityContext),
        inv.SpecHash,
        inv.EngineVersion,
        inv.IRVersion,
    )
    return err
}

// WriteCompletion inserts a completion record
func (s *Store) WriteCompletion(ctx context.Context, comp ir.Completion) error {
    _, err := s.db.ExecContext(ctx, `
        INSERT OR IGNORE INTO completions
        (id, invocation_id, output_case, result, seq, security_context)
        VALUES (?, ?, ?, ?, ?, ?)
    `,
        comp.ID,
        comp.InvocationID,
        comp.OutputCase,
        marshalResult(comp.Result),
        comp.Seq,
        marshalSecurityContext(comp.SecurityContext),
    )
    return err
}
```

**Store-Layer Read Operations:**
```go
// internal/store/read.go

// unmarshalArgs parses canonical JSON TEXT to IRObject
func unmarshalArgs(data string) (ir.IRObject, error) {
    var obj map[string]any
    if err := json.Unmarshal([]byte(data), &obj); err != nil {
        return nil, err
    }
    return convertToIRObject(obj), nil
}

// unmarshalResult parses canonical JSON TEXT to IRObject
func unmarshalResult(data string) (ir.IRObject, error) {
    var obj map[string]any
    if err := json.Unmarshal([]byte(data), &obj); err != nil {
        return nil, err
    }
    return convertToIRObject(obj), nil
}

// unmarshalSecurityContext parses JSON TEXT to SecurityContext
func unmarshalSecurityContext(data string) (ir.SecurityContext, error) {
    var ctx ir.SecurityContext
    if err := json.Unmarshal([]byte(data), &ctx); err != nil {
        return ir.SecurityContext{}, err
    }
    return ctx, nil
}

// convertToIRObject converts map[string]any to IRObject
// (Implementation details depend on Story 1.2 IRValue types)
func convertToIRObject(obj map[string]any) ir.IRObject {
    result := make(ir.IRObject)
    for k, v := range obj {
        result[k] = convertToIRValue(v)
    }
    return result
}

// ReadFlow returns all invocations and completions for a flow token
// with deterministic ordering (CP-4)
func (s *Store) ReadFlow(ctx context.Context, flowToken string) ([]ir.Invocation, []ir.Completion, error) {
    // Read invocations with deterministic ordering
    rows, err := s.db.QueryContext(ctx, `
        SELECT id, flow_token, action_uri, args, seq, security_context, spec_hash, engine_version, ir_version
        FROM invocations
        WHERE flow_token = ?
        ORDER BY seq ASC, id ASC COLLATE BINARY
    `, flowToken)
    if err != nil {
        return nil, nil, fmt.Errorf("query invocations: %w", err)
    }
    defer rows.Close()

    var invocations []ir.Invocation
    for rows.Next() {
        var inv ir.Invocation
        var argsJSON, secCtxJSON string

        if err := rows.Scan(
            &inv.ID, &inv.FlowToken, &inv.ActionURI, &argsJSON, &inv.Seq,
            &secCtxJSON, &inv.SpecHash, &inv.EngineVersion, &inv.IRVersion,
        ); err != nil {
            return nil, nil, fmt.Errorf("scan invocation: %w", err)
        }

        inv.Args, err = unmarshalArgs(argsJSON)
        if err != nil {
            return nil, nil, fmt.Errorf("unmarshal args: %w", err)
        }

        inv.SecurityContext, err = unmarshalSecurityContext(secCtxJSON)
        if err != nil {
            return nil, nil, fmt.Errorf("unmarshal security context: %w", err)
        }

        invocations = append(invocations, inv)
    }

    // Read completions with deterministic ordering
    rows, err = s.db.QueryContext(ctx, `
        SELECT c.id, c.invocation_id, c.output_case, c.result, c.seq, c.security_context
        FROM completions c
        JOIN invocations i ON c.invocation_id = i.id
        WHERE i.flow_token = ?
        ORDER BY c.seq ASC, c.id ASC COLLATE BINARY
    `, flowToken)
    if err != nil {
        return nil, nil, fmt.Errorf("query completions: %w", err)
    }
    defer rows.Close()

    var completions []ir.Completion
    for rows.Next() {
        var comp ir.Completion
        var resultJSON, secCtxJSON string

        if err := rows.Scan(
            &comp.ID, &comp.InvocationID, &comp.OutputCase, &resultJSON, &comp.Seq, &secCtxJSON,
        ); err != nil {
            return nil, nil, fmt.Errorf("scan completion: %w", err)
        }

        comp.Result, err = unmarshalResult(resultJSON)
        if err != nil {
            return nil, nil, fmt.Errorf("unmarshal result: %w", err)
        }

        comp.SecurityContext, err = unmarshalSecurityContext(secCtxJSON)
        if err != nil {
            return nil, nil, fmt.Errorf("unmarshal security context: %w", err)
        }

        completions = append(completions, comp)
    }

    return invocations, completions, nil
}
```

### Test Examples

**Schema Creation Tests:**
```go
// internal/store/schema_test.go

func TestSchemaCreation(t *testing.T) {
    db, cleanup := setupTestDB(t)
    defer cleanup()

    store := store.New(db)
    err := store.ApplySchema(context.Background())
    require.NoError(t, err, "schema should apply successfully")

    // Verify invocations table exists
    var tableName string
    err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='invocations'").Scan(&tableName)
    require.NoError(t, err)
    assert.Equal(t, "invocations", tableName)

    // Verify completions table exists
    err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='completions'").Scan(&tableName)
    require.NoError(t, err)
    assert.Equal(t, "completions", tableName)
}

func TestSchemaIdempotency(t *testing.T) {
    db, cleanup := setupTestDB(t)
    defer cleanup()

    store := store.New(db)

    // Apply schema twice - should not error
    err := store.ApplySchema(context.Background())
    require.NoError(t, err)

    err = store.ApplySchema(context.Background())
    require.NoError(t, err, "applying schema twice should be idempotent")
}

func TestNoTimestampColumns(t *testing.T) {
    // Load schema.sql
    schemaSQL := loadSchemaSQL(t)

    // Verify no timestamp-related SQL
    assert.NotContains(t, schemaSQL, "CURRENT_TIMESTAMP")
    assert.NotContains(t, schemaSQL, "datetime('now')")
    assert.NotContains(t, schemaSQL, "strftime")

    // Verify seq columns exist
    assert.Contains(t, schemaSQL, "seq INTEGER NOT NULL")
}
```

**Write Operation Tests:**
```go
// internal/store/write_test.go

func TestWriteInvocation(t *testing.T) {
    db, cleanup := setupTestDB(t)
    defer cleanup()

    store := store.New(db)
    store.ApplySchema(context.Background())

    inv := ir.Invocation{
        ID:        "inv-123",
        FlowToken: "flow-abc",
        ActionURI: ir.ActionRef("Cart.addItem"),
        Args: ir.IRObject{
            "item_id":  ir.IRString("widget"),
            "quantity": ir.IRInt(3),
        },
        Seq: 1,
        SecurityContext: ir.SecurityContext{
            TenantID: "tenant-1",
            UserID:   "user-1",
        },
        SpecHash:      "hash-abc",
        EngineVersion: "0.1.0",
        IRVersion:     "1",
    }

    err := store.WriteInvocation(context.Background(), inv)
    require.NoError(t, err)

    // Verify stored correctly
    var storedID, flowToken, actionURI, argsJSON string
    var seq int64
    err = db.QueryRow(`
        SELECT id, flow_token, action_uri, args, seq
        FROM invocations
        WHERE id = ?
    `, inv.ID).Scan(&storedID, &flowToken, &actionURI, &argsJSON, &seq)

    require.NoError(t, err)
    assert.Equal(t, inv.ID, storedID)
    assert.Equal(t, inv.FlowToken, flowToken)
    assert.Equal(t, string(inv.ActionURI), actionURI)
    assert.Equal(t, inv.Seq, seq)

    // Verify args are canonical JSON
    assert.Contains(t, argsJSON, `"item_id":"widget"`)
    assert.Contains(t, argsJSON, `"quantity":3`)
}

func TestWriteInvocationIdempotent(t *testing.T) {
    db, cleanup := setupTestDB(t)
    defer cleanup()

    store := store.New(db)
    store.ApplySchema(context.Background())

    inv := ir.Invocation{
        ID:        "inv-123",
        FlowToken: "flow-abc",
        ActionURI: ir.ActionRef("Cart.addItem"),
        Args:      ir.IRObject{},
        Seq:       1,
        SecurityContext: ir.SecurityContext{},
        SpecHash:      "hash",
        EngineVersion: "0.1.0",
        IRVersion:     "1",
    }

    // Write twice - should not error
    err := store.WriteInvocation(context.Background(), inv)
    require.NoError(t, err)

    err = store.WriteInvocation(context.Background(), inv)
    require.NoError(t, err, "duplicate write should be idempotent")

    // Verify only one row exists
    var count int
    db.QueryRow("SELECT COUNT(*) FROM invocations WHERE id = ?", inv.ID).Scan(&count)
    assert.Equal(t, 1, count)
}

func TestWriteCompletion(t *testing.T) {
    db, cleanup := setupTestDB(t)
    defer cleanup()

    store := store.New(db)
    store.ApplySchema(context.Background())

    // Write invocation first (foreign key requirement)
    inv := ir.Invocation{
        ID:              "inv-123",
        FlowToken:       "flow-abc",
        ActionURI:       ir.ActionRef("Cart.addItem"),
        Args:            ir.IRObject{},
        Seq:             1,
        SecurityContext: ir.SecurityContext{},
        SpecHash:        "hash",
        EngineVersion:   "0.1.0",
        IRVersion:       "1",
    }
    store.WriteInvocation(context.Background(), inv)

    comp := ir.Completion{
        ID:           "comp-456",
        InvocationID: "inv-123",
        OutputCase:   "Success",
        Result: ir.IRObject{
            "item_id":      ir.IRString("widget"),
            "new_quantity": ir.IRInt(3),
        },
        Seq: 2,
        SecurityContext: ir.SecurityContext{
            TenantID: "tenant-1",
            UserID:   "user-1",
        },
    }

    err := store.WriteCompletion(context.Background(), comp)
    require.NoError(t, err)

    // Verify stored correctly
    var storedID, invocationID, outputCase, resultJSON string
    var seq int64
    err = db.QueryRow(`
        SELECT id, invocation_id, output_case, result, seq
        FROM completions
        WHERE id = ?
    `, comp.ID).Scan(&storedID, &invocationID, &outputCase, &resultJSON, &seq)

    require.NoError(t, err)
    assert.Equal(t, comp.ID, storedID)
    assert.Equal(t, comp.InvocationID, invocationID)
    assert.Equal(t, comp.OutputCase, outputCase)
    assert.Equal(t, comp.Seq, seq)
}

func TestForeignKeyEnforcement(t *testing.T) {
    db, cleanup := setupTestDB(t)
    defer cleanup()

    store := store.New(db)
    store.ApplySchema(context.Background())

    // Try to write completion without invocation (should fail)
    comp := ir.Completion{
        ID:              "comp-456",
        InvocationID:    "nonexistent-inv",
        OutputCase:      "Success",
        Result:          ir.IRObject{},
        Seq:             1,
        SecurityContext: ir.SecurityContext{},
    }

    err := store.WriteCompletion(context.Background(), comp)
    assert.Error(t, err, "foreign key constraint should fail")
}
```

**Read Operation Tests:**
```go
// internal/store/read_test.go

func TestReadFlowDeterministicOrdering(t *testing.T) {
    db, cleanup := setupTestDB(t)
    defer cleanup()

    store := store.New(db)
    store.ApplySchema(context.Background())

    flowToken := "flow-abc"

    // Write invocations in non-sequential seq order
    seqs := []int64{5, 1, 3, 2, 4}
    for _, seq := range seqs {
        inv := ir.Invocation{
            ID:              fmt.Sprintf("inv-%d", seq),
            FlowToken:       flowToken,
            ActionURI:       ir.ActionRef("Test.action"),
            Args:            ir.IRObject{},
            Seq:             seq,
            SecurityContext: ir.SecurityContext{},
            SpecHash:        "hash",
            EngineVersion:   "0.1.0",
            IRVersion:       "1",
        }
        store.WriteInvocation(context.Background(), inv)
    }

    // Read flow
    invocations, _, err := store.ReadFlow(context.Background(), flowToken)
    require.NoError(t, err)

    // Verify deterministic ordering (seq ASC)
    require.Len(t, invocations, 5)
    assert.Equal(t, int64(1), invocations[0].Seq)
    assert.Equal(t, int64(2), invocations[1].Seq)
    assert.Equal(t, int64(3), invocations[2].Seq)
    assert.Equal(t, int64(4), invocations[3].Seq)
    assert.Equal(t, int64(5), invocations[4].Seq)
}

func TestReadFlowWithCompletions(t *testing.T) {
    db, cleanup := setupTestDB(t)
    defer cleanup()

    store := store.New(db)
    store.ApplySchema(context.Background())

    flowToken := "flow-abc"

    // Write invocation + completion
    inv := ir.Invocation{
        ID:              "inv-1",
        FlowToken:       flowToken,
        ActionURI:       ir.ActionRef("Cart.addItem"),
        Args:            ir.IRObject{},
        Seq:             1,
        SecurityContext: ir.SecurityContext{},
        SpecHash:        "hash",
        EngineVersion:   "0.1.0",
        IRVersion:       "1",
    }
    store.WriteInvocation(context.Background(), inv)

    comp := ir.Completion{
        ID:              "comp-1",
        InvocationID:    "inv-1",
        OutputCase:      "Success",
        Result:          ir.IRObject{},
        Seq:             2,
        SecurityContext: ir.SecurityContext{},
    }
    store.WriteCompletion(context.Background(), comp)

    // Read flow
    invocations, completions, err := store.ReadFlow(context.Background(), flowToken)
    require.NoError(t, err)

    require.Len(t, invocations, 1)
    require.Len(t, completions, 1)

    assert.Equal(t, inv.ID, invocations[0].ID)
    assert.Equal(t, comp.ID, completions[0].ID)
}

func TestReadFlowEmpty(t *testing.T) {
    db, cleanup := setupTestDB(t)
    defer cleanup()

    store := store.New(db)
    store.ApplySchema(context.Background())

    // Read non-existent flow
    invocations, completions, err := store.ReadFlow(context.Background(), "nonexistent-flow")
    require.NoError(t, err)

    assert.Empty(t, invocations)
    assert.Empty(t, completions)
}
```

### File List

Files to create or modify (in dependency order):

1. `internal/store/schema.sql` - DDL with invocations and completions tables
2. `internal/store/marshal.go` - Canonical JSON serialization helpers
3. `internal/store/write.go` - WriteInvocation, WriteCompletion
4. `internal/store/read.go` - ReadFlow with deterministic ordering
5. `internal/store/schema_test.go` - Schema creation and idempotency tests
6. `internal/store/write_test.go` - Write operation tests
7. `internal/store/read_test.go` - Read operation tests with ordering verification

### Relationship to Other Stories

**Depends On:**
- Story 1.1 (IR type definitions)
- Story 1.2 (IRValue constrained types)
- Story 1.4 (RFC 8785 canonical JSON)
- Story 1.5 (Content-addressed identity)
- Story 2.1 (SQLite store initialization)

**Blocks:**
- Story 2.3 (Write invocations and completions - needs schema)
- Story 2.4 (Read flow queries - needs schema)
- Story 2.5 (Sync firings table - extends schema)
- Story 2.6 (Provenance edges table - extends schema)

**Related:**
- Story 3.6 (Flow token propagation - uses invocations table)
- Story 4.4 (Binding set execution - queries completions)
- Story 7.5 (Replay command - validates deterministic ordering)

### Story Completion Checklist

- [ ] Schema SQL created with NO timestamp columns
- [ ] Invocations table has seq INTEGER NOT NULL
- [ ] Completions table has seq INTEGER NOT NULL
- [ ] Both tables have security_context TEXT NOT NULL
- [ ] Foreign key constraints defined
- [ ] Indexes created for flow_token and seq
- [ ] All queries use ORDER BY seq, id COLLATE BINARY
- [ ] Write functions use ir.MarshalCanonical() for args/result
- [ ] Read functions parse canonical JSON correctly
- [ ] Tests verify idempotent schema application
- [ ] Tests verify no timestamps in schema
- [ ] Tests verify deterministic query ordering
- [ ] Tests verify foreign key enforcement

### References

- [Source: docs/architecture.md#Event Log Schema] - Schema design
- [Source: docs/architecture.md#CP-2] - Logical clocks requirement
- [Source: docs/architecture.md#CP-4] - Deterministic query ordering
- [Source: docs/architecture.md#CP-6] - Security context requirement
- [Source: docs/epics.md#Story 2.2] - Story definition
- [Source: docs/prd.md#FR-5.1] - SQLite append-only log requirement

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Implementation Notes

- Foundation for deterministic replay - seq INTEGER replaces all timestamps
- Content-addressed IDs computed via ir.InvocationID(), ir.CompletionID()
- Canonical JSON storage ensures byte-identical replay
- SecurityContext on BOTH invocations AND completions per CP-6
- All queries MUST include ORDER BY for determinism per CP-4
- Foreign key constraints ensure data integrity
- INSERT OR IGNORE makes writes idempotent

### Validation History

- Initial creation: Based on Story 1.1 format
- Validation: Architecture CP-2, CP-4, CP-6 compliance verified
- Schema reviewed against CRITICAL-2 (deterministic replay)
- Query ordering patterns validated for CP-4

### Completion Status

Ready for dev agent implementation following Story 2.1 completion.
