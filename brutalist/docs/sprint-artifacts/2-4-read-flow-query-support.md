# Story 2.4: Read Flow and Query Support

Status: ready-for-dev

## Story

As a **developer querying the store**,
I want **to read all events for a flow token**,
So that **I can reconstruct flow state and debug issues**.

## Acceptance Criteria

1. **ReadFlow function returns all invocations and completions for a flow**
   ```go
   // Returns strongly-typed structs (not map[string]any)
   func (s *Store) ReadFlow(ctx context.Context, flowToken string) (*FlowData, error)

   type FlowData struct {
       Invocations []Invocation
       Completions []Completion
   }
   ```

2. **Results ordered by seq ASC with deterministic tiebreaker per CP-4**
   ```sql
   -- ALL queries MUST have ORDER BY with tiebreaker
   SELECT id, flow_token, action_uri, args, seq, security_context, spec_hash, engine_version
   FROM invocations
   WHERE flow_token = ?
   ORDER BY seq ASC, id ASC COLLATE BINARY
   ```

3. **ReadInvocation function reads single invocation by ID**
   ```go
   func (s *Store) ReadInvocation(ctx context.Context, invocationID string) (*Invocation, error)
   ```

4. **ReadCompletion function reads single completion by ID**
   ```go
   func (s *Store) ReadCompletion(ctx context.Context, completionID string) (*Completion, error)
   ```

5. **Completions are joined to their invocations in ReadFlow**
   - Each Completion includes its InvocationID for tracing
   - Both invocations and completions ordered by seq ASC, id ASC

6. **All queries use parameterized SQL (never string interpolation)**
   - Per HIGH-3 security requirement
   - All parameters passed via `?` placeholders
   - No `fmt.Sprintf` or string concatenation of SQL

7. **Query results unmarshal to strongly-typed Go structs**
   - Invocation struct with all fields from Story 1-1
   - Completion struct with all fields from Story 1-1
   - Args and Result fields unmarshal from canonical JSON to IRObject

## Quick Reference

| Pattern | Key Rule |
|---------|----------|
| **CP-4** | All queries MUST have ORDER BY with deterministic tiebreaker |
| **HIGH-3** | Parameterized queries only, no string interpolation |
| **CP-2** | Logical clock (seq) for ordering, not timestamps |
| **CP-5** | Strongly-typed structs (not map[string]any) |

## Tasks / Subtasks

- [ ] Task 1: Define FlowData result struct (AC: #1)
  - [ ] 1.1 Create `internal/store/read.go`
  - [ ] 1.2 Define FlowData struct with Invocations and Completions slices
  - [ ] 1.3 Add helper functions for JSON unmarshaling

- [ ] Task 2: Implement ReadFlow function (AC: #1, #2, #5)
  - [ ] 2.1 Query invocations with flow_token filter
  - [ ] 2.2 Apply ORDER BY seq ASC, id ASC COLLATE BINARY
  - [ ] 2.3 Query completions joined to invocations by flow_token
  - [ ] 2.4 Apply same ORDER BY to completions
  - [ ] 2.5 Use parameterized queries with ? placeholders
  - [ ] 2.6 Unmarshal JSON fields to IRObject

- [ ] Task 3: Implement ReadInvocation function (AC: #3)
  - [ ] 3.1 Query invocations WHERE id = ?
  - [ ] 3.2 Return error if not found
  - [ ] 3.3 Unmarshal args JSON to IRObject

- [ ] Task 4: Implement ReadCompletion function (AC: #4)
  - [ ] 4.1 Query completions WHERE id = ?
  - [ ] 4.2 Return error if not found
  - [ ] 4.3 Unmarshal result JSON to IRObject

- [ ] Task 5: Ensure parameterized queries (AC: #6)
  - [ ] 5.1 All queries use ? placeholders
  - [ ] 5.2 All parameters passed via ExecContext/QueryContext args
  - [ ] 5.3 NO fmt.Sprintf or string interpolation

- [ ] Task 6: Write comprehensive tests
  - [ ] 6.1 Test ReadFlow returns all invocations/completions for flow
  - [ ] 6.2 Test ReadFlow ordering is deterministic (seq ASC, id ASC)
  - [ ] 6.3 Test ReadFlow with multiple flows (isolation)
  - [ ] 6.4 Test ReadInvocation by ID
  - [ ] 6.5 Test ReadCompletion by ID
  - [ ] 6.6 Test ReadInvocation/ReadCompletion return errors for missing IDs
  - [ ] 6.7 Test JSON unmarshaling to IRObject

## Dev Notes

### CP-4: Deterministic Query Ordering

**CRITICAL:** ALL queries MUST include `ORDER BY` with a deterministic tiebreaker.

```go
// CORRECT: Explicit ordering with tiebreaker
const queryInvocationsByFlow = `
    SELECT id, flow_token, action_uri, args, seq, security_context, spec_hash, engine_version, ir_version
    FROM invocations
    WHERE flow_token = ?
    ORDER BY seq ASC, id ASC COLLATE BINARY
`

// WRONG: Missing ORDER BY
const badQuery = `
    SELECT * FROM invocations WHERE flow_token = ?
    -- ❌ No ORDER BY - non-deterministic results!
`

// WRONG: ORDER BY seq only (no tiebreaker)
const badQuery2 = `
    SELECT * FROM invocations WHERE flow_token = ?
    ORDER BY seq ASC
    -- ❌ When two records have same seq, order is undefined!
`
```

**Why tiebreaker is required:**
- Multiple invocations/completions can have the same `seq` value
- Without tiebreaker, SQLite can return them in any order
- This breaks deterministic replay and makes tests flaky
- `id ASC COLLATE BINARY` provides stable lexicographic ordering

### ReadFlow Implementation

```go
// internal/store/read.go

package store

import (
    "context"
    "database/sql"
    "encoding/json"
    "fmt"

    "github.com/yourusername/nysm/internal/ir"
)

// FlowData contains all invocations and completions for a flow token
type FlowData struct {
    Invocations []ir.Invocation `json:"invocations"`
    Completions []ir.Completion `json:"completions"`
}

// ReadFlow returns all invocations and completions for a flow token,
// ordered deterministically by seq ASC, id ASC per CP-4
func (s *Store) ReadFlow(ctx context.Context, flowToken string) (*FlowData, error) {
    // Query invocations
    invocations, err := s.readInvocationsByFlow(ctx, flowToken)
    if err != nil {
        return nil, fmt.Errorf("read invocations: %w", err)
    }

    // Query completions
    completions, err := s.readCompletionsByFlow(ctx, flowToken)
    if err != nil {
        return nil, fmt.Errorf("read completions: %w", err)
    }

    return &FlowData{
        Invocations: invocations,
        Completions: completions,
    }, nil
}

// readInvocationsByFlow queries invocations for a flow token
func (s *Store) readInvocationsByFlow(ctx context.Context, flowToken string) ([]ir.Invocation, error) {
    // MUST have ORDER BY with tiebreaker per CP-4
    query := `
        SELECT id, flow_token, action_uri, args, seq, security_context, spec_hash, engine_version, ir_version
        FROM invocations
        WHERE flow_token = ?
        ORDER BY seq ASC, id ASC COLLATE BINARY
    `

    rows, err := s.db.QueryContext(ctx, query, flowToken)
    if err != nil {
        return nil, fmt.Errorf("query invocations: %w", err)
    }
    defer rows.Close()

    var invocations []ir.Invocation
    for rows.Next() {
        var inv ir.Invocation
        var argsJSON, secCtxJSON string

        err := rows.Scan(
            &inv.ID,
            &inv.FlowToken,
            &inv.ActionURI,
            &argsJSON,
            &inv.Seq,
            &secCtxJSON,
            &inv.SpecHash,
            &inv.EngineVersion,
            &inv.IRVersion,
        )
        if err != nil {
            return nil, fmt.Errorf("scan invocation: %w", err)
        }

        // Unmarshal JSON fields to IRObject
        if err := json.Unmarshal([]byte(argsJSON), &inv.Args); err != nil {
            return nil, fmt.Errorf("unmarshal args: %w", err)
        }
        if err := json.Unmarshal([]byte(secCtxJSON), &inv.SecurityContext); err != nil {
            return nil, fmt.Errorf("unmarshal security_context: %w", err)
        }

        invocations = append(invocations, inv)
    }

    if err := rows.Err(); err != nil {
        return nil, fmt.Errorf("rows iteration: %w", err)
    }

    return invocations, nil
}

// readCompletionsByFlow queries completions for a flow token
func (s *Store) readCompletionsByFlow(ctx context.Context, flowToken string) ([]ir.Completion, error) {
    // Join to invocations to get flow_token, then order deterministically
    query := `
        SELECT c.id, c.invocation_id, c.output_case, c.result, c.seq, c.security_context
        FROM completions c
        JOIN invocations i ON c.invocation_id = i.id
        WHERE i.flow_token = ?
        ORDER BY c.seq ASC, c.id ASC COLLATE BINARY
    `

    rows, err := s.db.QueryContext(ctx, query, flowToken)
    if err != nil {
        return nil, fmt.Errorf("query completions: %w", err)
    }
    defer rows.Close()

    var completions []ir.Completion
    for rows.Next() {
        var comp ir.Completion
        var resultJSON, secCtxJSON string

        err := rows.Scan(
            &comp.ID,
            &comp.InvocationID,
            &comp.OutputCase,
            &resultJSON,
            &comp.Seq,
            &secCtxJSON,
        )
        if err != nil {
            return nil, fmt.Errorf("scan completion: %w", err)
        }

        // Unmarshal JSON fields
        if err := json.Unmarshal([]byte(resultJSON), &comp.Result); err != nil {
            return nil, fmt.Errorf("unmarshal result: %w", err)
        }
        if err := json.Unmarshal([]byte(secCtxJSON), &comp.SecurityContext); err != nil {
            return nil, fmt.Errorf("unmarshal security_context: %w", err)
        }

        completions = append(completions, comp)
    }

    if err := rows.Err(); err != nil {
        return nil, fmt.Errorf("rows iteration: %w", err)
    }

    return completions, nil
}

// ReadInvocation returns a single invocation by ID
func (s *Store) ReadInvocation(ctx context.Context, invocationID string) (*ir.Invocation, error) {
    query := `
        SELECT id, flow_token, action_uri, args, seq, security_context, spec_hash, engine_version, ir_version
        FROM invocations
        WHERE id = ?
    `

    var inv ir.Invocation
    var argsJSON, secCtxJSON string

    err := s.db.QueryRowContext(ctx, query, invocationID).Scan(
        &inv.ID,
        &inv.FlowToken,
        &inv.ActionURI,
        &argsJSON,
        &inv.Seq,
        &secCtxJSON,
        &inv.SpecHash,
        &inv.EngineVersion,
        &inv.IRVersion,
    )
    if err == sql.ErrNoRows {
        return nil, fmt.Errorf("invocation not found: %s", invocationID)
    }
    if err != nil {
        return nil, fmt.Errorf("query invocation: %w", err)
    }

    // Unmarshal JSON fields
    if err := json.Unmarshal([]byte(argsJSON), &inv.Args); err != nil {
        return nil, fmt.Errorf("unmarshal args: %w", err)
    }
    if err := json.Unmarshal([]byte(secCtxJSON), &inv.SecurityContext); err != nil {
        return nil, fmt.Errorf("unmarshal security_context: %w", err)
    }

    return &inv, nil
}

// ReadCompletion returns a single completion by ID
func (s *Store) ReadCompletion(ctx context.Context, completionID string) (*ir.Completion, error) {
    query := `
        SELECT id, invocation_id, output_case, result, seq, security_context
        FROM completions
        WHERE id = ?
    `

    var comp ir.Completion
    var resultJSON, secCtxJSON string

    err := s.db.QueryRowContext(ctx, query, completionID).Scan(
        &comp.ID,
        &comp.InvocationID,
        &comp.OutputCase,
        &resultJSON,
        &comp.Seq,
        &secCtxJSON,
    )
    if err == sql.ErrNoRows {
        return nil, fmt.Errorf("completion not found: %s", completionID)
    }
    if err != nil {
        return nil, fmt.Errorf("query completion: %w", err)
    }

    // Unmarshal JSON fields
    if err := json.Unmarshal([]byte(resultJSON), &comp.Result); err != nil {
        return nil, fmt.Errorf("unmarshal result: %w", err)
    }
    if err := json.Unmarshal([]byte(secCtxJSON), &comp.SecurityContext); err != nil {
        return nil, fmt.Errorf("unmarshal security_context: %w", err)
    }

    return &comp, nil
}
```

### HIGH-3: Parameterized Queries

**CRITICAL:** NEVER use string interpolation or `fmt.Sprintf` to build SQL queries.

```go
// CORRECT: Parameterized query
query := `SELECT * FROM invocations WHERE flow_token = ?`
rows, err := db.QueryContext(ctx, query, flowToken)

// WRONG: String interpolation (SQL injection risk!)
query := fmt.Sprintf("SELECT * FROM invocations WHERE flow_token = '%s'", flowToken)
rows, err := db.QueryContext(ctx, query)  // ❌ NEVER DO THIS
```

**Why parameterized queries:**
- Prevents SQL injection attacks
- SQLite can compile prepared statements once and reuse them
- Type safety - SQLite handles escaping and quoting
- Enforced by HIGH-3 security requirement

### Test Examples

```go
// internal/store/read_test.go

package store

import (
    "context"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "github.com/yourusername/nysm/internal/ir"
)

func TestReadFlow(t *testing.T) {
    ctx := context.Background()
    s := newTestStore(t)
    defer s.Close()

    flowToken := "flow-123"

    // Write test data
    inv1 := ir.Invocation{
        ID:        "inv-1",
        FlowToken: flowToken,
        ActionURI: ir.ActionRef("Cart.addItem"),
        Args: ir.IRObject{
            "item_id":  ir.IRString("item-1"),
            "quantity": ir.IRInt(2),
        },
        Seq:             1,
        SecurityContext: ir.SecurityContext{},
        SpecHash:        "spec-hash-1",
        EngineVersion:   ir.EngineVersion,
        IRVersion:       ir.IRVersion,
    }

    inv2 := ir.Invocation{
        ID:              "inv-2",
        FlowToken:       flowToken,
        ActionURI:       ir.ActionRef("Cart.checkout"),
        Args:            ir.IRObject{},
        Seq:             2,
        SecurityContext: ir.SecurityContext{},
        SpecHash:        "spec-hash-1",
        EngineVersion:   ir.EngineVersion,
        IRVersion:       ir.IRVersion,
    }

    require.NoError(t, s.WriteInvocation(ctx, inv1))
    require.NoError(t, s.WriteInvocation(ctx, inv2))

    comp1 := ir.Completion{
        ID:           "comp-1",
        InvocationID: "inv-1",
        OutputCase:   "Success",
        Result: ir.IRObject{
            "new_quantity": ir.IRInt(2),
        },
        Seq:             3,
        SecurityContext: ir.SecurityContext{},
    }

    require.NoError(t, s.WriteCompletion(ctx, comp1))

    // Read flow
    flowData, err := s.ReadFlow(ctx, flowToken)
    require.NoError(t, err)

    // Verify invocations
    assert.Len(t, flowData.Invocations, 2)
    assert.Equal(t, "inv-1", flowData.Invocations[0].ID)
    assert.Equal(t, "inv-2", flowData.Invocations[1].ID)

    // Verify completions
    assert.Len(t, flowData.Completions, 1)
    assert.Equal(t, "comp-1", flowData.Completions[0].ID)
    assert.Equal(t, "inv-1", flowData.Completions[0].InvocationID)
}

func TestReadFlow_DeterministicOrdering(t *testing.T) {
    ctx := context.Background()
    s := newTestStore(t)
    defer s.Close()

    flowToken := "flow-order-test"

    // Write invocations with SAME seq but different IDs
    // This tests the tiebreaker: ORDER BY seq ASC, id ASC COLLATE BINARY
    inv1 := ir.Invocation{
        ID:              "inv-zzz", // Lexicographically last
        FlowToken:       flowToken,
        ActionURI:       ir.ActionRef("Test.action"),
        Args:            ir.IRObject{},
        Seq:             1, // Same seq
        SecurityContext: ir.SecurityContext{},
        SpecHash:        "spec-hash",
        EngineVersion:   ir.EngineVersion,
        IRVersion:       ir.IRVersion,
    }

    inv2 := ir.Invocation{
        ID:              "inv-aaa", // Lexicographically first
        FlowToken:       flowToken,
        ActionURI:       ir.ActionRef("Test.action"),
        Args:            ir.IRObject{},
        Seq:             1, // Same seq
        SecurityContext: ir.SecurityContext{},
        SpecHash:        "spec-hash",
        EngineVersion:   ir.EngineVersion,
        IRVersion:       ir.IRVersion,
    }

    // Write in reverse order to verify ORDER BY is applied
    require.NoError(t, s.WriteInvocation(ctx, inv1))
    require.NoError(t, s.WriteInvocation(ctx, inv2))

    // Read multiple times to verify deterministic ordering
    for i := 0; i < 5; i++ {
        flowData, err := s.ReadFlow(ctx, flowToken)
        require.NoError(t, err)

        // Should ALWAYS be ordered: inv-aaa, inv-zzz (lexicographic on id)
        require.Len(t, flowData.Invocations, 2)
        assert.Equal(t, "inv-aaa", flowData.Invocations[0].ID, "iteration %d", i)
        assert.Equal(t, "inv-zzz", flowData.Invocations[1].ID, "iteration %d", i)
    }
}

func TestReadFlow_FlowIsolation(t *testing.T) {
    ctx := context.Background()
    s := newTestStore(t)
    defer s.Close()

    // Write invocations for different flows
    inv1 := ir.Invocation{
        ID:              "inv-1",
        FlowToken:       "flow-A",
        ActionURI:       ir.ActionRef("Test.action"),
        Args:            ir.IRObject{},
        Seq:             1,
        SecurityContext: ir.SecurityContext{},
        SpecHash:        "spec-hash",
        EngineVersion:   ir.EngineVersion,
        IRVersion:       ir.IRVersion,
    }

    inv2 := ir.Invocation{
        ID:              "inv-2",
        FlowToken:       "flow-B",
        ActionURI:       ir.ActionRef("Test.action"),
        Args:            ir.IRObject{},
        Seq:             2,
        SecurityContext: ir.SecurityContext{},
        SpecHash:        "spec-hash",
        EngineVersion:   ir.EngineVersion,
        IRVersion:       ir.IRVersion,
    }

    require.NoError(t, s.WriteInvocation(ctx, inv1))
    require.NoError(t, s.WriteInvocation(ctx, inv2))

    // Read flow-A
    flowDataA, err := s.ReadFlow(ctx, "flow-A")
    require.NoError(t, err)
    assert.Len(t, flowDataA.Invocations, 1)
    assert.Equal(t, "inv-1", flowDataA.Invocations[0].ID)

    // Read flow-B
    flowDataB, err := s.ReadFlow(ctx, "flow-B")
    require.NoError(t, err)
    assert.Len(t, flowDataB.Invocations, 1)
    assert.Equal(t, "inv-2", flowDataB.Invocations[0].ID)
}

func TestReadInvocation(t *testing.T) {
    ctx := context.Background()
    s := newTestStore(t)
    defer s.Close()

    inv := ir.Invocation{
        ID:              "inv-123",
        FlowToken:       "flow-test",
        ActionURI:       ir.ActionRef("Cart.addItem"),
        Args: ir.IRObject{
            "item_id": ir.IRString("item-1"),
        },
        Seq:             1,
        SecurityContext: ir.SecurityContext{TenantID: "tenant-1"},
        SpecHash:        "spec-hash",
        EngineVersion:   ir.EngineVersion,
        IRVersion:       ir.IRVersion,
    }

    require.NoError(t, s.WriteInvocation(ctx, inv))

    // Read by ID
    readInv, err := s.ReadInvocation(ctx, "inv-123")
    require.NoError(t, err)

    assert.Equal(t, inv.ID, readInv.ID)
    assert.Equal(t, inv.FlowToken, readInv.FlowToken)
    assert.Equal(t, inv.ActionURI, readInv.ActionURI)
    assert.Equal(t, ir.IRString("item-1"), readInv.Args["item_id"])
    assert.Equal(t, "tenant-1", readInv.SecurityContext.TenantID)
}

func TestReadInvocation_NotFound(t *testing.T) {
    ctx := context.Background()
    s := newTestStore(t)
    defer s.Close()

    // Read non-existent invocation
    _, err := s.ReadInvocation(ctx, "does-not-exist")
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "not found")
}

func TestReadCompletion(t *testing.T) {
    ctx := context.Background()
    s := newTestStore(t)
    defer s.Close()

    // Write invocation first (FK requirement)
    inv := ir.Invocation{
        ID:              "inv-1",
        FlowToken:       "flow-test",
        ActionURI:       ir.ActionRef("Cart.addItem"),
        Args:            ir.IRObject{},
        Seq:             1,
        SecurityContext: ir.SecurityContext{},
        SpecHash:        "spec-hash",
        EngineVersion:   ir.EngineVersion,
        IRVersion:       ir.IRVersion,
    }
    require.NoError(t, s.WriteInvocation(ctx, inv))

    // Write completion
    comp := ir.Completion{
        ID:           "comp-1",
        InvocationID: "inv-1",
        OutputCase:   "Success",
        Result: ir.IRObject{
            "new_quantity": ir.IRInt(5),
        },
        Seq:             2,
        SecurityContext: ir.SecurityContext{UserID: "user-1"},
    }
    require.NoError(t, s.WriteCompletion(ctx, comp))

    // Read by ID
    readComp, err := s.ReadCompletion(ctx, "comp-1")
    require.NoError(t, err)

    assert.Equal(t, comp.ID, readComp.ID)
    assert.Equal(t, comp.InvocationID, readComp.InvocationID)
    assert.Equal(t, comp.OutputCase, readComp.OutputCase)
    assert.Equal(t, ir.IRInt(5), readComp.Result["new_quantity"])
    assert.Equal(t, "user-1", readComp.SecurityContext.UserID)
}

func TestReadCompletion_NotFound(t *testing.T) {
    ctx := context.Background()
    s := newTestStore(t)
    defer s.Close()

    // Read non-existent completion
    _, err := s.ReadCompletion(ctx, "does-not-exist")
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "not found")
}

func TestReadFlow_JSONUnmarshaling(t *testing.T) {
    ctx := context.Background()
    s := newTestStore(t)
    defer s.Close()

    flowToken := "flow-json-test"

    // Write invocation with complex args
    inv := ir.Invocation{
        ID:        "inv-1",
        FlowToken: flowToken,
        ActionURI: ir.ActionRef("Test.complexAction"),
        Args: ir.IRObject{
            "string_field": ir.IRString("hello"),
            "int_field":    ir.IRInt(42),
            "bool_field":   ir.IRBool(true),
            "array_field":  ir.IRArray{ir.IRInt(1), ir.IRInt(2), ir.IRInt(3)},
            "object_field": ir.IRObject{
                "nested": ir.IRString("value"),
            },
        },
        Seq:             1,
        SecurityContext: ir.SecurityContext{},
        SpecHash:        "spec-hash",
        EngineVersion:   ir.EngineVersion,
        IRVersion:       ir.IRVersion,
    }
    require.NoError(t, s.WriteInvocation(ctx, inv))

    // Read and verify unmarshaling
    flowData, err := s.ReadFlow(ctx, flowToken)
    require.NoError(t, err)
    require.Len(t, flowData.Invocations, 1)

    args := flowData.Invocations[0].Args
    assert.Equal(t, ir.IRString("hello"), args["string_field"])
    assert.Equal(t, ir.IRInt(42), args["int_field"])
    assert.Equal(t, ir.IRBool(true), args["bool_field"])

    // Verify array
    arr, ok := args["array_field"].(ir.IRArray)
    require.True(t, ok)
    assert.Len(t, arr, 3)
    assert.Equal(t, ir.IRInt(1), arr[0])

    // Verify nested object
    obj, ok := args["object_field"].(ir.IRObject)
    require.True(t, ok)
    assert.Equal(t, ir.IRString("value"), obj["nested"])
}
```

### File List

Files to create/modify:

1. `internal/store/read.go` - ReadFlow, ReadInvocation, ReadCompletion functions
2. `internal/store/read_test.go` - Comprehensive tests

### Relationship to Other Stories

- **Story 2.1:** Uses Store.db connection opened in initialization
- **Story 2.2:** Queries the schema defined in Story 2.2
- **Story 2.3:** Reads records written by WriteInvocation/WriteCompletion
- **Story 1-1:** Uses Invocation and Completion structs from IR types
- **Story 1-2:** Uses IRObject and IRValue types for args/result fields

### Story Completion Checklist

- [ ] FlowData struct defined with Invocations and Completions slices
- [ ] ReadFlow function returns all invocations/completions for flow token
- [ ] ReadInvocation function reads single invocation by ID
- [ ] ReadCompletion function reads single completion by ID
- [ ] All queries use ORDER BY seq ASC, id ASC COLLATE BINARY
- [ ] All queries use parameterized SQL (? placeholders)
- [ ] JSON fields unmarshal to IRObject correctly
- [ ] Tests verify deterministic ordering across multiple calls
- [ ] Tests verify flow isolation (different flow tokens don't mix)
- [ ] Tests verify ReadInvocation/ReadCompletion return errors for missing IDs
- [ ] Tests verify complex JSON unmarshaling (arrays, nested objects)
- [ ] All tests pass
- [ ] `go vet ./internal/store/...` passes

### References

- [Source: docs/epics.md#Story 2.4] - Story definition
- [Source: docs/architecture.md#CP-4] - Deterministic query ordering
- [Source: docs/architecture.md#HIGH-3] - Parameterized queries only
- [Source: docs/architecture.md#CP-2] - Logical clocks (seq) for ordering
- [Source: docs/sprint-artifacts/1-1-project-initialization-ir-type-definitions.md] - Invocation and Completion struct definitions

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Validation History

- Initial creation: 2025-12-12 - Story file created based on Epic 2.4 requirements

### Completion Notes

- Implements FR-5.1 (SQLite-backed append-only log query support)
- Critical dependency for debugging and replay (FR-5.3)
- Enforces CP-4 (deterministic ordering) and HIGH-3 (parameterized queries)
- Returns strongly-typed structs per CP-5 (not map[string]any)
- Foundation for provenance queries and flow tracing
