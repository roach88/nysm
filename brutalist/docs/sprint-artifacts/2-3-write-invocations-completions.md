# Story 2.3: Write Invocations and Completions

Status: done

## Story

As a **developer using the store**,
I want **to write invocations and completions to the event log**,
So that **all actions are durably recorded**.

## Acceptance Criteria

1. **WriteInvocation function in `internal/store/write.go`**
   - Signature: `func (s *Store) WriteInvocation(ctx context.Context, inv ir.Invocation) error`
   - Content-addressed `id` computed via `ir.InvocationID()`
   - `args` serialized via `ir.MarshalCanonical()`
   - `security_context` serialized via `ir.MarshalCanonical()`
   - INSERT OR IGNORE for idempotent writes

2. **WriteCompletion function in `internal/store/write.go`**
   - Signature: `func (s *Store) WriteCompletion(ctx context.Context, comp ir.Completion) error`
   - Content-addressed `id` computed via `ir.CompletionID()`
   - `result` serialized via `ir.MarshalCanonical()`
   - `security_context` serialized via `ir.MarshalCanonical()`
   - INSERT OR IGNORE for idempotent writes

3. **All SQL uses parameterized queries** (never string interpolation) per HIGH-3
   - Uses `db.ExecContext(ctx, query, args...)`
   - NO `fmt.Sprintf` or string concatenation in SQL
   - All values passed as `?` placeholders

4. **Duplicate IDs handled gracefully**
   - `INSERT OR IGNORE` allows idempotent replay
   - Returns nil error for duplicate writes (same content-addressed ID)
   - No panic on constraint violations

5. **Error handling follows Go best practices**
   - Non-nil errors returned for database failures
   - Context cancellation respected
   - Clear error messages include context

6. **Comprehensive tests in `internal/store/write_test.go`**
   - Normal case: fresh write succeeds
   - Idempotent case: duplicate write succeeds (no-op)
   - Error case: invalid data returns error
   - Transaction rollback on partial failure
   - Context cancellation handling

## Quick Reference

| Pattern | Key Rule |
|---------|----------|
| **CP-2** | Logical clock (`seq`), NO wall-clock timestamps |
| **CP-5** | Content-addressed IDs via `InvocationID()` / `CompletionID()` |
| **HIGH-3** | Parameterized queries only - never string interpolation |

## Tasks / Subtasks

- [ ] Task 1: Implement WriteInvocation (AC: #1, #3)
  - [ ] 1.1 Create `internal/store/write.go`
  - [ ] 1.2 Implement `WriteInvocation(ctx, inv)` function
  - [ ] 1.3 Use parameterized SQL with INSERT OR IGNORE
  - [ ] 1.4 Serialize `args` and `security_context` via `ir.MarshalCanonical()`
  - [ ] 1.5 Handle errors and context cancellation

- [ ] Task 2: Implement WriteCompletion (AC: #2, #3)
  - [ ] 2.1 Implement `WriteCompletion(ctx, comp)` function
  - [ ] 2.2 Use parameterized SQL with INSERT OR IGNORE
  - [ ] 2.3 Serialize `result` and `security_context` via `ir.MarshalCanonical()`
  - [ ] 2.4 Handle foreign key constraint (invocation_id must exist)

- [ ] Task 3: Write comprehensive tests (AC: #4, #5, #6)
  - [ ] 3.1 Create `internal/store/write_test.go`
  - [ ] 3.2 Test normal writes (fresh invocations and completions)
  - [ ] 3.3 Test idempotent writes (duplicate IDs)
  - [ ] 3.4 Test error cases (invalid foreign keys, nil context)
  - [ ] 3.5 Test context cancellation
  - [ ] 3.6 Test transaction behavior

- [ ] Task 4: Verify parameterized SQL (AC: #3)
  - [ ] 4.1 Code review: verify NO `fmt.Sprintf` in SQL
  - [ ] 4.2 Code review: verify all values use `?` placeholders
  - [ ] 4.3 Test: assert on exact SQL strings

## Dev Notes

### Critical Implementation Details

**Parameterized SQL - HIGH-3 Security Pattern**
```go
// CORRECT: Parameterized query
_, err := s.db.ExecContext(ctx, `
    INSERT OR IGNORE INTO invocations
    (id, flow_token, action_uri, args, seq, security_context, spec_hash, engine_version, ir_version)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
`, inv.ID, inv.FlowToken, string(inv.ActionURI),
   argsJSON, inv.Seq, securityJSON,
   inv.SpecHash, inv.EngineVersion, inv.IRVersion)

// WRONG: String interpolation - SQL injection risk
query := fmt.Sprintf("INSERT INTO invocations VALUES ('%s', '%s', ...)", inv.ID, inv.FlowToken) // ❌ NEVER
```

**Content-Addressed IDs**
```go
// IDs MUST be computed before calling Write functions
// WriteInvocation/WriteCompletion do NOT compute IDs
inv := ir.Invocation{
    FlowToken: flowToken,
    ActionURI: "Inventory.ReserveStock",
    Args: ir.IRObject{
        "product_id": ir.IRString("p123"),
        "quantity": ir.IRInt(5),
    },
    Seq: clock.Next(),
    SecurityContext: ir.SecurityContext{
        TenantID: "tenant1",
        UserID: "user1",
        Permissions: []string{"inventory.write"},
    },
    SpecHash: specHash,
    EngineVersion: ir.EngineVersion,
    IRVersion: ir.IRVersion,
}

// Compute content-addressed ID using Story 1.5 signature
// IMPORTANT: SecurityContext is EXCLUDED from InvocationID (CP-6)
// This enables replay flexibility and decouples identity from auth context
inv.ID, err = ir.InvocationID(inv.FlowToken, inv.ActionURI, inv.Args, inv.Seq)
if err != nil {
    return fmt.Errorf("failed to compute invocation ID: %w", err)
}

// Now write to store
if err := store.WriteInvocation(ctx, inv); err != nil {
    return err
}
```

**Idempotent Writes with INSERT OR IGNORE**
```go
// INSERT OR IGNORE allows replay without errors
// Same content-addressed ID → same record → no-op on duplicate
_, err := s.db.ExecContext(ctx, `
    INSERT OR IGNORE INTO invocations
    (id, flow_token, action_uri, args, seq, security_context, spec_hash, engine_version, ir_version)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
`, ...)

// Returns nil error even if row already exists
// This is CORRECT for deterministic replay
```

### Function Signatures

**WriteInvocation Implementation**
```go
// internal/store/write.go
package store

import (
    "context"
    "fmt"

    "github.com/tyler/nysm/internal/ir"
)

// WriteInvocation writes an invocation to the event log.
// Duplicate IDs (same content-addressed hash) are ignored (idempotent).
// The invocation ID MUST be pre-computed via ir.InvocationID().
func (s *Store) WriteInvocation(ctx context.Context, inv ir.Invocation) error {
    // Validate invocation has ID
    if inv.ID == "" {
        return fmt.Errorf("invocation ID is required (must be pre-computed via ir.InvocationID)")
    }

    // Serialize args to canonical JSON
    argsJSON, err := ir.MarshalCanonical(inv.Args)
    if err != nil {
        return fmt.Errorf("marshal invocation args: %w", err)
    }

    // Serialize security context to canonical JSON
    securityJSON, err := ir.MarshalCanonical(inv.SecurityContext)
    if err != nil {
        return fmt.Errorf("marshal security context: %w", err)
    }

    // INSERT OR IGNORE for idempotent writes
    _, err = s.db.ExecContext(ctx, `
        INSERT OR IGNORE INTO invocations
        (id, flow_token, action_uri, args, seq, security_context, spec_hash, engine_version, ir_version)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
    `, inv.ID, inv.FlowToken, string(inv.ActionURI),
       string(argsJSON), inv.Seq, string(securityJSON),
       inv.SpecHash, inv.EngineVersion, inv.IRVersion)

    if err != nil {
        return fmt.Errorf("insert invocation %s: %w", inv.ID, err)
    }

    return nil
}
```

**WriteCompletion Implementation**
```go
// WriteCompletion writes a completion to the event log.
// Duplicate IDs (same content-addressed hash) are ignored (idempotent).
// The completion ID MUST be pre-computed via ir.CompletionID().
// The invocation_id MUST reference an existing invocation (foreign key).
func (s *Store) WriteCompletion(ctx context.Context, comp ir.Completion) error {
    // Validate completion has ID
    if comp.ID == "" {
        return fmt.Errorf("completion ID is required (must be pre-computed via ir.CompletionID)")
    }

    // Validate invocation reference
    if comp.InvocationID == "" {
        return fmt.Errorf("completion.invocation_id is required")
    }

    // Serialize result to canonical JSON
    resultJSON, err := ir.MarshalCanonical(comp.Result)
    if err != nil {
        return fmt.Errorf("marshal completion result: %w", err)
    }

    // Serialize security context to canonical JSON
    securityJSON, err := ir.MarshalCanonical(comp.SecurityContext)
    if err != nil {
        return fmt.Errorf("marshal security context: %w", err)
    }

    // INSERT OR IGNORE for idempotent writes
    // Foreign key constraint ensures invocation_id exists
    _, err = s.db.ExecContext(ctx, `
        INSERT OR IGNORE INTO completions
        (id, invocation_id, output_case, result, seq, security_context)
        VALUES (?, ?, ?, ?, ?, ?)
    `, comp.ID, comp.InvocationID, comp.OutputCase,
       string(resultJSON), comp.Seq, string(securityJSON))

    if err != nil {
        return fmt.Errorf("insert completion %s: %w", comp.ID, err)
    }

    return nil
}
```

### Error Handling Patterns

**Validation Errors**
```go
// Validate required fields before database operations
if inv.ID == "" {
    return fmt.Errorf("invocation ID is required")
}
if comp.InvocationID == "" {
    return fmt.Errorf("completion.invocation_id is required")
}
```

**Serialization Errors**
```go
// MarshalCanonical can fail if data contains invalid types
argsJSON, err := ir.MarshalCanonical(inv.Args)
if err != nil {
    return fmt.Errorf("marshal invocation args: %w", err)
}
```

**Database Errors**
```go
// Database errors wrapped with context
_, err = s.db.ExecContext(ctx, query, args...)
if err != nil {
    return fmt.Errorf("insert invocation %s: %w", inv.ID, err)
}
```

**Foreign Key Violations**
```go
// WriteCompletion references invocation_id (foreign key)
// If invocation doesn't exist, SQLite returns FOREIGN KEY constraint error
// This is NOT an idempotency case - it's a genuine error
_, err = s.db.ExecContext(ctx, `INSERT OR IGNORE INTO completions ...`, ...)
if err != nil {
    // Check if foreign key error
    if strings.Contains(err.Error(), "FOREIGN KEY") {
        return fmt.Errorf("invocation %s not found: %w", comp.InvocationID, err)
    }
    return fmt.Errorf("insert completion %s: %w", comp.ID, err)
}
```

### Transaction Considerations

**Single-Record Writes**
```go
// WriteInvocation and WriteCompletion are single-record operations
// No explicit transaction needed for single INSERT
// SQLite ensures atomicity of single statements
```

**Multi-Record Operations (Future Stories)**
```go
// Sync firings + provenance edges require transactions (Story 2.5)
tx, err := s.db.BeginTx(ctx, nil)
if err != nil {
    return err
}
defer tx.Rollback() // Safe to call even after commit

if err := writeSyncFiring(tx, firing); err != nil {
    return err
}
if err := writeProvenance(tx, edge); err != nil {
    return err
}

return tx.Commit()
```

### Test Examples

**Test Normal Write**
```go
func TestWriteInvocation_Success(t *testing.T) {
    ctx := context.Background()
    store := setupTestStore(t)

    inv := ir.Invocation{
        FlowToken: "flow-123",
        ActionURI: "Test.Action",
        Args: ir.IRObject{
            "arg1": ir.IRString("value1"),
        },
        Seq: 1,
        SecurityContext: ir.SecurityContext{
            TenantID: "tenant1",
            UserID: "user1",
        },
        SpecHash: "hash123",
        EngineVersion: ir.EngineVersion,
        IRVersion: ir.IRVersion,
    }

    // Compute content-addressed ID
    inv.ID = ir.InvocationID(inv)

    // Write invocation
    err := store.WriteInvocation(ctx, inv)
    require.NoError(t, err)

    // Verify written to database
    var count int
    err = store.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM invocations WHERE id = ?", inv.ID).Scan(&count)
    require.NoError(t, err)
    assert.Equal(t, 1, count)
}
```

**Test Idempotent Write**
```go
func TestWriteInvocation_Idempotent(t *testing.T) {
    ctx := context.Background()
    store := setupTestStore(t)

    inv := ir.Invocation{
        FlowToken: "flow-123",
        ActionURI: "Test.Action",
        Args: ir.IRObject{"arg1": ir.IRString("value1")},
        Seq: 1,
        SecurityContext: ir.SecurityContext{TenantID: "tenant1"},
        SpecHash: "hash123",
        EngineVersion: ir.EngineVersion,
        IRVersion: ir.IRVersion,
    }
    inv.ID = ir.InvocationID(inv)

    // Write first time
    err := store.WriteInvocation(ctx, inv)
    require.NoError(t, err)

    // Write second time (same ID)
    err = store.WriteInvocation(ctx, inv)
    require.NoError(t, err) // Should succeed (no-op)

    // Verify only one row exists
    var count int
    err = store.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM invocations WHERE id = ?", inv.ID).Scan(&count)
    require.NoError(t, err)
    assert.Equal(t, 1, count, "duplicate write should not create second row")
}
```

**Test Foreign Key Violation**
```go
func TestWriteCompletion_InvalidInvocationID(t *testing.T) {
    ctx := context.Background()
    store := setupTestStore(t)

    comp := ir.Completion{
        InvocationID: "nonexistent-inv-id",
        OutputCase: "Success",
        Result: ir.IRObject{"result": ir.IRString("done")},
        Seq: 2,
        SecurityContext: ir.SecurityContext{TenantID: "tenant1"},
    }
    comp.ID = ir.CompletionID(comp)

    // Write should fail - invocation doesn't exist
    err := store.WriteCompletion(ctx, comp)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "FOREIGN KEY", "should fail with foreign key error")
}
```

**Test Context Cancellation**
```go
func TestWriteInvocation_ContextCancelled(t *testing.T) {
    store := setupTestStore(t)

    inv := ir.Invocation{
        FlowToken: "flow-123",
        ActionURI: "Test.Action",
        Args: ir.IRObject{"arg1": ir.IRString("value1")},
        Seq: 1,
        SecurityContext: ir.SecurityContext{TenantID: "tenant1"},
        SpecHash: "hash123",
        EngineVersion: ir.EngineVersion,
        IRVersion: ir.IRVersion,
    }
    inv.ID = ir.InvocationID(inv)

    // Create cancelled context
    ctx, cancel := context.WithCancel(context.Background())
    cancel() // Cancel immediately

    // Write should respect cancellation
    err := store.WriteInvocation(ctx, inv)
    require.Error(t, err)
    assert.Equal(t, context.Canceled, err) // Should be context error
}
```

**Test Missing ID Validation**
```go
func TestWriteInvocation_MissingID(t *testing.T) {
    ctx := context.Background()
    store := setupTestStore(t)

    inv := ir.Invocation{
        FlowToken: "flow-123",
        ActionURI: "Test.Action",
        Args: ir.IRObject{"arg1": ir.IRString("value1")},
        Seq: 1,
        SecurityContext: ir.SecurityContext{TenantID: "tenant1"},
        // ID not set (missing)
    }

    err := store.WriteInvocation(ctx, inv)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "invocation ID is required")
}
```

**Test Canonical JSON Serialization**
```go
func TestWriteInvocation_CanonicalJSONSerialization(t *testing.T) {
    ctx := context.Background()
    store := setupTestStore(t)

    inv := ir.Invocation{
        FlowToken: "flow-123",
        ActionURI: "Test.Action",
        Args: ir.IRObject{
            "z_field": ir.IRString("last"),  // Key order will be sorted
            "a_field": ir.IRString("first"),
        },
        Seq: 1,
        SecurityContext: ir.SecurityContext{
            TenantID: "tenant1",
            UserID: "user1",
            Permissions: []string{"read", "write"},
        },
        SpecHash: "hash123",
        EngineVersion: ir.EngineVersion,
        IRVersion: ir.IRVersion,
    }
    inv.ID = ir.InvocationID(inv)

    err := store.WriteInvocation(ctx, inv)
    require.NoError(t, err)

    // Retrieve args as JSON from database
    var argsJSON string
    err = store.db.QueryRowContext(ctx, "SELECT args FROM invocations WHERE id = ?", inv.ID).Scan(&argsJSON)
    require.NoError(t, err)

    // Verify canonical JSON (keys sorted, no whitespace)
    assert.Equal(t, `{"a_field":"first","z_field":"last"}`, argsJSON)
}
```

**Test WriteCompletion Success**
```go
func TestWriteCompletion_Success(t *testing.T) {
    ctx := context.Background()
    store := setupTestStore(t)

    // First write invocation (required for foreign key)
    inv := ir.Invocation{
        FlowToken: "flow-123",
        ActionURI: "Test.Action",
        Args: ir.IRObject{"arg1": ir.IRString("value1")},
        Seq: 1,
        SecurityContext: ir.SecurityContext{TenantID: "tenant1"},
        SpecHash: "hash123",
        EngineVersion: ir.EngineVersion,
        IRVersion: ir.IRVersion,
    }
    inv.ID = ir.InvocationID(inv)
    err := store.WriteInvocation(ctx, inv)
    require.NoError(t, err)

    // Now write completion
    comp := ir.Completion{
        InvocationID: inv.ID,
        OutputCase: "Success",
        Result: ir.IRObject{"result": ir.IRString("done")},
        Seq: 2,
        SecurityContext: ir.SecurityContext{TenantID: "tenant1"},
    }
    comp.ID = ir.CompletionID(comp)

    err = store.WriteCompletion(ctx, comp)
    require.NoError(t, err)

    // Verify written to database
    var count int
    err = store.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM completions WHERE id = ?", comp.ID).Scan(&count)
    require.NoError(t, err)
    assert.Equal(t, 1, count)
}
```

### File List

Files to create:

1. `internal/store/write.go` - Write functions
2. `internal/store/write_test.go` - Comprehensive tests

Files to reference (must exist from previous stories):

1. `internal/ir/types.go` - Invocation, Completion types
2. `internal/ir/canonical.go` - MarshalCanonical function
3. `internal/ir/hash.go` - InvocationID, CompletionID functions
4. `internal/store/store.go` - Store struct with db field

### Story Completion Checklist

- [ ] WriteInvocation function implemented
- [ ] WriteCompletion function implemented
- [ ] All SQL uses parameterized queries (no string interpolation)
- [ ] INSERT OR IGNORE for idempotent writes
- [ ] Content-addressed IDs pre-computed (not generated in Write functions)
- [ ] Args/Result serialized via MarshalCanonical
- [ ] SecurityContext serialized via MarshalCanonical
- [ ] Error handling includes context wrapping
- [ ] Tests for normal writes pass
- [ ] Tests for idempotent writes pass
- [ ] Tests for error cases pass
- [ ] Tests for context cancellation pass
- [ ] `go vet ./internal/store/...` passes
- [ ] `go test ./internal/store/...` passes

### References

- [Source: docs/architecture.md#SQLite Configuration] - Database setup
- [Source: docs/architecture.md#HIGH-3] - Parameterized queries only
- [Source: docs/architecture.md#CP-2] - Logical clock, no timestamps
- [Source: docs/architecture.md#Store Interface] - WriteInvocation/WriteCompletion signatures
- [Source: docs/epics.md#Story 2.3] - Story definition
- [Source: docs/epics.md#Story 2.2] - Schema (completions table)
- [Source: Story 1.4] - MarshalCanonical function
- [Source: Story 1.5] - InvocationID/CompletionID functions

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Validation History

- Initial creation: comprehensive story generation

### Completion Notes

- Write functions are THIN - they do NOT compute IDs or validate business logic
- ID computation happens in ir package (Story 1.5)
- INSERT OR IGNORE enables deterministic replay (same ID = no-op)
- Parameterized SQL is MANDATORY per HIGH-3 (SQL injection prevention)
- Foreign key on completion.invocation_id ensures referential integrity
- SecurityContext MUST be serialized (always present per CP-6)
- No transactions needed for single-record writes (SQLite guarantees atomicity)
- Error wrapping provides context for debugging (invocation/completion ID)
