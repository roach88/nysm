# Story 3.2: Sync Rule Registration and Declaration Order

Status: ready-for-dev

## Story

As a **developer using the engine**,
I want **sync rules evaluated in declaration order**,
So that **rule priority is explicit and predictable**.

## Acceptance Criteria

1. **Engine accepts sync rule registration via RegisterSyncs**
   - `func (e *Engine) RegisterSyncs(syncs []ir.SyncRule)` stores rules
   - Rules stored in the exact order provided
   - Order corresponds to declaration order from compiler

2. **Sync rules stored in declaration order**
   ```go
   type Engine struct {
       syncs []ir.SyncRule  // Declaration order preserved
       // ... other fields
   }
   ```
   - Syncs slice maintains insertion order
   - No sorting or reordering applied

3. **evaluateSyncs iterates in declaration order**
   ```go
   func (e *Engine) evaluateSyncs(ctx context.Context, comp ir.Completion) error {
       for _, sync := range e.syncs {  // Declaration order iteration
           if e.matches(sync.When, comp) {
               // Process this sync (stub for now)
           }
       }
       return nil
   }
   ```
   - Iteration uses range over syncs slice
   - Order guaranteed by slice iteration semantics

4. **Order preserved across engine restarts**
   - Determinism requirement: same input order = same evaluation order
   - Declaration order = order in compiled sync rule slice from compiler
   - No dependency on timestamps, insertion timestamps, or IDs

5. **RegisterSyncs validates input**
   - Accepts nil or empty slice without error
   - Accepts valid sync rules
   - Returns error for duplicate sync IDs within same registration

6. **evaluateSyncs stub implementation**
   - Function signature correct
   - Iterates all syncs in order
   - Calls matcher stub (no actual matching logic yet)
   - Returns nil (no error handling yet)

## Quick Reference

| Pattern | Key Rule |
|---------|----------|
| **CRITICAL-3** | Deterministic scheduling policy - sync rules evaluated in declaration order |
| **FR-2.1** | Support 3-clause sync rules: when → where → then |

## Tasks / Subtasks

- [ ] Task 1: Create engine package structure (AC: #1, #2)
  - [ ] 1.1 Create `internal/engine/` directory
  - [ ] 1.2 Create `internal/engine/doc.go` with package documentation
  - [ ] 1.3 Create `internal/engine/engine.go` with Engine struct

- [ ] Task 2: Implement Engine struct (AC: #2)
  - [ ] 2.1 Define Engine struct with syncs []ir.SyncRule field
  - [ ] 2.2 Add New() constructor function
  - [ ] 2.3 Document field purposes and invariants

- [ ] Task 3: Implement RegisterSyncs (AC: #1, #4, #5)
  - [ ] 3.1 Create RegisterSyncs method signature
  - [ ] 3.2 Store syncs slice in declaration order
  - [ ] 3.3 Validate for duplicate sync IDs
  - [ ] 3.4 Handle nil/empty slice cases
  - [ ] 3.5 Document determinism guarantees

- [ ] Task 4: Implement evaluateSyncs stub (AC: #3, #6)
  - [ ] 4.1 Create evaluateSyncs method signature
  - [ ] 4.2 Iterate syncs in declaration order
  - [ ] 4.3 Add matcher stub call (no real matching)
  - [ ] 4.4 Return nil (no error handling yet)
  - [ ] 4.5 Add TODO comments for future implementation

- [ ] Task 5: Create matcher stub (AC: #6)
  - [ ] 5.1 Create `internal/engine/matcher.go`
  - [ ] 5.2 Implement `matches(when ir.WhenClause, comp ir.Completion) bool` stub
  - [ ] 5.3 Return false for now (no matching logic)
  - [ ] 5.4 Add TODO comments for Story 3.3

- [ ] Task 6: Write comprehensive tests (all AC)
  - [ ] 6.1 Test RegisterSyncs stores rules in order
  - [ ] 6.2 Test evaluateSyncs iterates in order
  - [ ] 6.3 Test order preservation (multiple registrations)
  - [ ] 6.4 Test nil/empty slice handling
  - [ ] 6.5 Test duplicate sync ID detection
  - [ ] 6.6 Test evaluateSyncs with multiple syncs
  - [ ] 6.7 Test determinism (same input = same order)

## Dev Notes

### Critical Pattern Details

**CRITICAL-3: Deterministic Scheduling Policy**
```go
// CORRECT: Sync rules evaluated in declaration order
func (e *Engine) evaluateSyncs(ctx context.Context, comp ir.Completion) error {
    // Iteration over slice is deterministic - preserves declaration order
    for _, sync := range e.syncs {
        if e.matches(sync.When, comp) {
            // Process sync (Story 3.3+ will implement)
        }
    }
    return nil
}

// WRONG - map iteration is non-deterministic:
type Engine struct {
    syncs map[string]ir.SyncRule  // ❌ Map order not guaranteed
}
```

**Declaration Order Guarantee**
```go
// The compiler produces sync rules in declaration order
// This order MUST be preserved through registration and evaluation

// Example: CUE spec declares syncs in this order:
// 1. sync "reserve-inventory" { ... }
// 2. sync "send-confirmation" { ... }
// 3. sync "update-analytics" { ... }

// RegisterSyncs receives them in this order
syncs := []ir.SyncRule{
    {ID: "reserve-inventory", ...},
    {ID: "send-confirmation", ...},
    {ID: "update-analytics", ...},
}

// They MUST be evaluated in this exact order
```

### Engine Package Structure

**Package Documentation (internal/engine/doc.go):**
```go
// Package engine implements the NYSM sync execution engine.
//
// The engine is responsible for:
// - Evaluating sync rules against completions
// - Generating invocations based on bindings
// - Maintaining deterministic evaluation order
// - Enforcing termination constraints (cycle detection, quotas)
//
// CRITICAL PATTERNS:
//
// CRITICAL-3: Deterministic Scheduling Policy
// - Sync rules are evaluated in declaration order (order from compiler)
// - For each completion, matching syncs are processed sequentially
// - For each sync, bindings are processed in query result order
// - Generated invocations enqueue their completions back to the engine
//
// Concurrency Model:
// - Single-writer event loop for determinism
// - FIFO queue of pending completions
// - No parallelism in core evaluation loop
// - Actions may execute in parallel (I/O bound)
// - Completions enqueue back to single writer
//
// This package implements FR-2 (Synchronization System) and FR-3 (Flow Token System).
package engine
```

**Engine Struct (internal/engine/engine.go):**
```go
package engine

import (
    "context"
    "fmt"

    "github.com/tyler/nysm/internal/ir"
)

// Engine is the NYSM sync execution engine.
// It evaluates sync rules against completions and generates invocations.
//
// The engine maintains deterministic evaluation order:
// - Sync rules are evaluated in declaration order (CRITICAL-3)
// - Completions are processed in FIFO order
// - Bindings are processed in query result order
//
// INVARIANTS:
// - syncs slice order NEVER changes after RegisterSyncs
// - Sync IDs within syncs are unique
// - Evaluation is single-threaded for determinism
type Engine struct {
    // syncs contains registered sync rules in declaration order.
    // This order is preserved from compilation and MUST NOT be modified.
    // Iteration order determines evaluation priority.
    syncs []ir.SyncRule

    // TODO (Story 3.1): Add store, flowGen, clock, specs fields
}

// New creates a new Engine instance.
// The engine is ready to accept sync rule registration.
func New() *Engine {
    return &Engine{
        syncs: nil,  // Empty until RegisterSyncs called
    }
}

// RegisterSyncs registers sync rules with the engine in declaration order.
//
// The syncs slice order determines evaluation priority. Sync rules are
// evaluated in the exact order provided, which must match the declaration
// order from the CUE compiler.
//
// This function validates that all sync IDs are unique within the provided
// slice. Duplicate IDs result in an error.
//
// Passing nil or an empty slice is valid and clears any previously
// registered sync rules.
//
// CRITICAL: This order is deterministic and preserved across engine restarts.
// The same input order guarantees the same evaluation order.
func (e *Engine) RegisterSyncs(syncs []ir.SyncRule) error {
    if syncs == nil {
        e.syncs = nil
        return nil
    }

    // Validate uniqueness of sync IDs
    seen := make(map[string]bool, len(syncs))
    for _, sync := range syncs {
        if seen[sync.ID] {
            return fmt.Errorf("duplicate sync ID: %s", sync.ID)
        }
        seen[sync.ID] = true
    }

    // Store syncs in declaration order
    // Make a copy to prevent external mutation
    e.syncs = make([]ir.SyncRule, len(syncs))
    copy(e.syncs, syncs)

    return nil
}

// evaluateSyncs evaluates all registered sync rules against a completion.
//
// Sync rules are checked in declaration order (CRITICAL-3). For each
// matching sync, bindings are extracted and invocations are generated.
//
// This is a stub implementation for Story 3.2. Full matching, binding
// extraction, and invocation generation will be implemented in subsequent
// stories (3.3, 3.6, 3.7).
//
// TODO (Story 3.3): Implement when-clause matching
// TODO (Story 3.6): Implement binding extraction
// TODO (Story 3.7): Implement invocation generation
func (e *Engine) evaluateSyncs(ctx context.Context, comp ir.Completion) error {
    // Iterate syncs in declaration order (deterministic)
    for _, sync := range e.syncs {
        // Check if this sync matches the completion
        // Story 3.3 will implement real matching logic
        if e.matches(sync.When, comp) {
            // TODO (Story 3.6): Extract bindings from where-clause
            // TODO (Story 3.7): Generate invocations from then-clause
        }
    }

    return nil
}
```

**Matcher Stub (internal/engine/matcher.go):**
```go
package engine

import (
    "github.com/tyler/nysm/internal/ir"
)

// matches checks if a when-clause matches a completion.
//
// This is a stub implementation for Story 3.2. It always returns false.
// Real matching logic will be implemented in Story 3.3.
//
// The matcher will check:
// 1. Action URI matches when.Action
// 2. Event type is "completed" (for completions)
// 3. Output case matches if specified (nil = any)
//
// TODO (Story 3.3): Implement when-clause matching logic
func (e *Engine) matches(when ir.WhenClause, comp ir.Completion) bool {
    // Stub: Always return false for now
    // Story 3.3 will implement real matching
    return false
}
```

### Test Examples

```go
// internal/engine/engine_test.go
package engine

import (
    "context"
    "testing"

    "github.com/tyler/nysm/internal/ir"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
    e := New()
    require.NotNil(t, e)
    assert.Nil(t, e.syncs, "new engine should have no syncs")
}

func TestRegisterSyncs_Empty(t *testing.T) {
    e := New()

    t.Run("nil_slice", func(t *testing.T) {
        err := e.RegisterSyncs(nil)
        require.NoError(t, err)
        assert.Nil(t, e.syncs)
    })

    t.Run("empty_slice", func(t *testing.T) {
        err := e.RegisterSyncs([]ir.SyncRule{})
        require.NoError(t, err)
        assert.NotNil(t, e.syncs)
        assert.Empty(t, e.syncs)
    })
}

func TestRegisterSyncs_Order(t *testing.T) {
    e := New()

    syncs := []ir.SyncRule{
        {ID: "sync-1", Scope: "flow"},
        {ID: "sync-2", Scope: "flow"},
        {ID: "sync-3", Scope: "flow"},
    }

    err := e.RegisterSyncs(syncs)
    require.NoError(t, err)

    // Verify order is preserved
    require.Len(t, e.syncs, 3)
    assert.Equal(t, "sync-1", e.syncs[0].ID)
    assert.Equal(t, "sync-2", e.syncs[1].ID)
    assert.Equal(t, "sync-3", e.syncs[2].ID)
}

func TestRegisterSyncs_DuplicateID(t *testing.T) {
    e := New()

    syncs := []ir.SyncRule{
        {ID: "sync-1", Scope: "flow"},
        {ID: "sync-2", Scope: "flow"},
        {ID: "sync-1", Scope: "flow"},  // Duplicate
    }

    err := e.RegisterSyncs(syncs)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "duplicate sync ID: sync-1")
}

func TestRegisterSyncs_CopyPreventsExternalMutation(t *testing.T) {
    e := New()

    syncs := []ir.SyncRule{
        {ID: "sync-1", Scope: "flow"},
        {ID: "sync-2", Scope: "flow"},
    }

    err := e.RegisterSyncs(syncs)
    require.NoError(t, err)

    // Mutate original slice
    syncs[0].ID = "mutated"

    // Engine should be unaffected
    assert.Equal(t, "sync-1", e.syncs[0].ID, "engine syncs should be independent copy")
}

func TestRegisterSyncs_ReplacesPreviousRegistration(t *testing.T) {
    e := New()

    // First registration
    syncs1 := []ir.SyncRule{
        {ID: "sync-1", Scope: "flow"},
    }
    err := e.RegisterSyncs(syncs1)
    require.NoError(t, err)
    assert.Len(t, e.syncs, 1)

    // Second registration replaces
    syncs2 := []ir.SyncRule{
        {ID: "sync-2", Scope: "flow"},
        {ID: "sync-3", Scope: "flow"},
    }
    err = e.RegisterSyncs(syncs2)
    require.NoError(t, err)
    assert.Len(t, e.syncs, 2)
    assert.Equal(t, "sync-2", e.syncs[0].ID)
}

func TestEvaluateSyncs_IterationOrder(t *testing.T) {
    e := New()

    // Track iteration order
    var visitedIDs []string

    // Register syncs
    syncs := []ir.SyncRule{
        {ID: "first", Scope: "flow"},
        {ID: "second", Scope: "flow"},
        {ID: "third", Scope: "flow"},
    }
    err := e.RegisterSyncs(syncs)
    require.NoError(t, err)

    // Create a completion (will not match due to stub)
    comp := ir.Completion{
        ID:           "comp-1",
        InvocationID: "inv-1",
        OutputCase:   "Success",
        Result:       ir.IRObject{},
        Seq:          1,
        SecurityContext: ir.SecurityContext{
            TenantID: "tenant-1",
            UserID:   "user-1",
        },
    }

    // Note: We can't easily test iteration order without modifying the stub
    // This test just verifies evaluateSyncs runs without error
    err = e.evaluateSyncs(context.Background(), comp)
    require.NoError(t, err)

    // TODO (Story 3.3): Add matcher mock to verify iteration order
    _ = visitedIDs
}

func TestEvaluateSyncs_NoSyncs(t *testing.T) {
    e := New()

    comp := ir.Completion{
        ID:           "comp-1",
        InvocationID: "inv-1",
        OutputCase:   "Success",
        Result:       ir.IRObject{},
        Seq:          1,
        SecurityContext: ir.SecurityContext{
            TenantID: "tenant-1",
            UserID:   "user-1",
        },
    }

    err := e.evaluateSyncs(context.Background(), comp)
    require.NoError(t, err)
}

func TestEvaluateSyncs_ContextCancellation(t *testing.T) {
    e := New()

    syncs := []ir.SyncRule{
        {ID: "sync-1", Scope: "flow"},
    }
    err := e.RegisterSyncs(syncs)
    require.NoError(t, err)

    comp := ir.Completion{
        ID:           "comp-1",
        InvocationID: "inv-1",
        OutputCase:   "Success",
        Result:       ir.IRObject{},
        Seq:          1,
        SecurityContext: ir.SecurityContext{
            TenantID: "tenant-1",
            UserID:   "user-1",
        },
    }

    // Cancel context
    ctx, cancel := context.WithCancel(context.Background())
    cancel()

    // Should still run (no cancellation handling yet)
    err = e.evaluateSyncs(ctx, comp)
    require.NoError(t, err)

    // TODO (Story 3.8+): Add proper context cancellation handling
}

// TestDeclarationOrderDeterminism verifies that the same input order
// produces the same evaluation order (determinism guarantee)
func TestDeclarationOrderDeterminism(t *testing.T) {
    syncs := []ir.SyncRule{
        {ID: "alpha", Scope: "flow"},
        {ID: "beta", Scope: "flow"},
        {ID: "gamma", Scope: "flow"},
    }

    // Create two engines with same syncs
    e1 := New()
    e2 := New()

    err := e1.RegisterSyncs(syncs)
    require.NoError(t, err)

    err = e2.RegisterSyncs(syncs)
    require.NoError(t, err)

    // Both engines should have identical order
    require.Equal(t, len(e1.syncs), len(e2.syncs))
    for i := range e1.syncs {
        assert.Equal(t, e1.syncs[i].ID, e2.syncs[i].ID,
            "engines should have identical sync order at index %d", i)
    }
}
```

```go
// internal/engine/matcher_test.go
package engine

import (
    "testing"

    "github.com/tyler/nysm/internal/ir"
    "github.com/stretchr/testify/assert"
)

func TestMatches_Stub(t *testing.T) {
    e := New()

    when := ir.WhenClause{
        Action: ir.ActionRef("Cart.checkout"),
        Event:  "completed",
    }

    comp := ir.Completion{
        ID:           "comp-1",
        InvocationID: "inv-1",
        OutputCase:   "Success",
        Result:       ir.IRObject{},
        Seq:          1,
        SecurityContext: ir.SecurityContext{
            TenantID: "tenant-1",
            UserID:   "user-1",
        },
    }

    // Stub always returns false
    matched := e.matches(when, comp)
    assert.False(t, matched, "stub matcher should always return false")
}

// TODO (Story 3.3): Add real matcher tests
// - Test action URI matching
// - Test event type matching
// - Test output case matching (nil = any)
// - Test binding extraction
```

### File List

Files to create:

1. `internal/engine/doc.go` - Package documentation
2. `internal/engine/engine.go` - Engine struct, New(), RegisterSyncs(), evaluateSyncs()
3. `internal/engine/matcher.go` - matches() stub
4. `internal/engine/engine_test.go` - Comprehensive tests
5. `internal/engine/matcher_test.go` - Matcher stub tests

### Relationship to Other Stories

**Dependencies:**
- Story 1.1 (Project Initialization & IR Type Definitions) - Required for ir.SyncRule, ir.Completion types
- Story 3.1 (Engine Initialization & Configuration) - Provides Engine struct foundation (if 3.1 exists before 3.2)

**Enables:**
- Story 3.3 (When-Clause Matching) - Uses evaluateSyncs() and matches() stubs
- Story 3.4 (Output Case Matching for Errors) - Extends matcher
- Story 3.6 (Binding Extraction from Where-Clause) - Uses evaluateSyncs() loop
- Story 3.7 (Invocation Generation from Then-Clause) - Uses evaluateSyncs() loop

**Note:** This story establishes the deterministic evaluation order foundation. All subsequent engine stories build upon this ordering guarantee.

### Story Completion Checklist

- [ ] `internal/engine/` directory created
- [ ] `internal/engine/doc.go` written with CRITICAL-3 documentation
- [ ] `internal/engine/engine.go` implements Engine struct
- [ ] `internal/engine/engine.go` implements New() constructor
- [ ] `internal/engine/engine.go` implements RegisterSyncs() with validation
- [ ] `internal/engine/engine.go` implements evaluateSyncs() stub
- [ ] `internal/engine/matcher.go` implements matches() stub
- [ ] RegisterSyncs preserves declaration order
- [ ] RegisterSyncs validates duplicate sync IDs
- [ ] RegisterSyncs handles nil/empty slices
- [ ] evaluateSyncs iterates in declaration order
- [ ] All tests pass (`go test ./internal/engine/...`)
- [ ] `go build ./...` succeeds
- [ ] `go vet ./internal/engine` passes
- [ ] Test coverage > 90% for new code
- [ ] Order preservation verified across multiple registrations
- [ ] Determinism test passes (same input = same order)

### References

- [Source: docs/architecture.md#CRITICAL-3] - Deterministic scheduling policy
- [Source: docs/architecture.md#Concurrency Model] - Single-writer event loop
- [Source: docs/architecture.md#Complete Project Directory Structure] - engine/ package layout
- [Source: docs/architecture.md#Package Dependencies] - engine imports ir, store, queryir, querysql
- [Source: docs/epics.md#Story 3.2] - Acceptance criteria
- [Source: docs/prd.md#FR-2.1] - Sync rule requirements
- [Source: docs/architecture.md#Implementation Patterns] - Declaration order guarantees

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Validation History

- Initial creation: create-story workflow

### Completion Notes

- Implements CRITICAL-3 deterministic scheduling policy
- Declaration order preserved via slice iteration semantics
- RegisterSyncs validates uniqueness of sync IDs
- evaluateSyncs stub ready for Story 3.3 (when-clause matching)
- matches() stub ready for Story 3.3 implementation
- Order guaranteed deterministic: same input = same evaluation order
- No dependency on timestamps, IDs, or non-deterministic sorting
- Tests verify order preservation and determinism
- Engine struct ready for additional fields in Story 3.1
