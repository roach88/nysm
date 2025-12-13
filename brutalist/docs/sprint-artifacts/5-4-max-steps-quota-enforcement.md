# Story 5.4: Max-Steps Quota Enforcement

Status: ready-for-dev

## Story

As a **developer preventing runaway flows**,
I want **a maximum steps quota per flow**,
So that **non-terminating patterns are caught and flows terminate gracefully**.

## Acceptance Criteria

1. **QuotaEnforcer in `internal/engine/quota.go`**
   ```go
   type QuotaEnforcer struct {
       maxSteps int
       current  int
   }

   func NewQuotaEnforcer(maxSteps int) *QuotaEnforcer {
       return &QuotaEnforcer{
           maxSteps: maxSteps,
           current:  0,
       }
   }

   func (q *QuotaEnforcer) Check(flowToken string) error {
       q.current++
       if q.current > q.maxSteps {
           return &StepsExceededError{
               FlowToken: flowToken,
               Steps:     q.current,
               Limit:     q.maxSteps,
           }
       }
       return nil
   }

   func (q *QuotaEnforcer) Reset() {
       q.current = 0
   }

   func (q *QuotaEnforcer) Current() int {
       return q.current
   }
   ```
   - Per-flow step counter
   - Configurable max steps limit
   - Check() increments and validates
   - Reset() for new flows

2. **StepsExceededError type**
   ```go
   type StepsExceededError struct {
       FlowToken string
       Steps     int
       Limit     int
   }

   func (e *StepsExceededError) Error() string {
       return fmt.Sprintf("flow %s exceeded max steps quota: %d steps > %d limit",
           e.FlowToken, e.Steps, e.Limit)
   }

   func (e *StepsExceededError) RuntimeError() string {
       return "StepsExceededError"
   }
   ```
   - Typed error for quota violations
   - Includes diagnostic information
   - FlowToken, current steps, and limit
   - Implements error interface

3. **Engine integration in `internal/engine/engine.go`**
   ```go
   type Engine struct {
       // ... existing fields
       maxSteps int  // Default: 1000
       quotas   map[string]*QuotaEnforcer  // Per-flow quota enforcers
   }

   func New(store *store.Store, specs []ir.ConceptSpec, syncs []ir.SyncRule, opts ...EngineOption) *Engine {
       e := &Engine{
           store:    store,
           clock:    NewClock(),
           specs:    specs,
           syncs:    syncs,
           queue:    newEventQueue(),
           maxSteps: 1000,  // Default quota
           quotas:   make(map[string]*QuotaEnforcer),
       }
       for _, opt := range opts {
           opt(e)
       }
       return e
   }

   // EngineOption allows configuration of engine parameters
   type EngineOption func(*Engine)

   func WithMaxSteps(maxSteps int) EngineOption {
       return func(e *Engine) {
           e.maxSteps = maxSteps
       }
   }
   ```
   - MaxSteps field with default 1000
   - Per-flow quota tracking in map
   - Configurable via WithMaxSteps option

4. **Quota enforcement in processCompletion()**
   ```go
   func (e *Engine) processCompletion(ctx context.Context, comp *ir.Completion) error {
       flowToken := comp.FlowToken

       // Get or create quota enforcer for this flow
       quota, exists := e.quotas[flowToken]
       if !exists {
           quota = NewQuotaEnforcer(e.maxSteps)
           e.quotas[flowToken] = quota
       }

       // Check quota before processing sync rules
       if err := quota.Check(flowToken); err != nil {
           // Log quota exceeded
           slog.Error("max steps quota exceeded",
               "flow_token", flowToken,
               "steps", quota.Current(),
               "limit", e.maxSteps,
               "event", "quota_exceeded",
           )
           return err
       }

       // ... continue with sync rule processing
   }
   ```
   - Check quota on every completion
   - Create enforcer lazily per flow
   - Error terminates flow gracefully
   - Logged with diagnostic info

5. **Flow cleanup in Engine**
   ```go
   func (e *Engine) cleanupFlow(flowToken string) {
       delete(e.quotas, flowToken)
   }
   ```
   - Remove quota enforcer when flow completes
   - Prevents quota map memory leak

6. **Configurable default quota**
   ```go
   engine := engine.New(
       store,
       specs,
       syncs,
       engine.WithMaxSteps(5000),  // Custom limit
   )
   ```
   - Default 1000 for typical flows
   - Configurable for specific needs
   - Documented in engine package

7. **Tests verify quota enforcement**
   - Flow exceeding max steps terminates with StepsExceededError
   - Flow within limits completes normally
   - Custom limit respected (e.g., 10 steps)
   - Error includes flow token, step count, and limit
   - Multiple concurrent flows have independent quotas

## Quick Reference

| Pattern | Key Rule |
|---------|----------|
| **CRITICAL-3** | Max-steps quota enforcement (prevents runaway flows) |
| **CP-2** | Per-flow quota tracking (not global) |
| **NFR-2.3** | Flow isolation - quotas are per-flow, not shared |

## Tasks / Subtasks

- [ ] Task 1: Create QuotaEnforcer in `internal/engine/quota.go` (AC: #1)
  - [ ] 1.1 Create `internal/engine/quota.go`
  - [ ] 1.2 Define QuotaEnforcer struct with maxSteps and current
  - [ ] 1.3 Implement NewQuotaEnforcer(maxSteps int) constructor
  - [ ] 1.4 Implement Check(flowToken string) error method
  - [ ] 1.5 Implement Reset() method
  - [ ] 1.6 Implement Current() int getter
  - [ ] 1.7 Write tests for QuotaEnforcer (within limit, exceeds limit, reset)

- [ ] Task 2: Create StepsExceededError type (AC: #2)
  - [ ] 2.1 Define StepsExceededError struct in `quota.go`
  - [ ] 2.2 Implement Error() string method
  - [ ] 2.3 Add RuntimeError() string method for typed errors
  - [ ] 2.4 Write tests for error formatting

- [ ] Task 3: Add maxSteps to Engine (AC: #3)
  - [ ] 3.1 Add maxSteps int field to Engine struct
  - [ ] 3.2 Add quotas map[string]*QuotaEnforcer field
  - [ ] 3.3 Initialize maxSteps to 1000 in New()
  - [ ] 3.4 Initialize quotas map in New()
  - [ ] 3.5 Define EngineOption func(*Engine) type
  - [ ] 3.6 Implement WithMaxSteps(maxSteps int) EngineOption
  - [ ] 3.7 Apply options in New() constructor
  - [ ] 3.8 Write tests for engine options

- [ ] Task 4: Integrate quota enforcement in processCompletion() (AC: #4)
  - [ ] 4.1 Get or create quota enforcer for flow
  - [ ] 4.2 Call quota.Check() before sync rule processing
  - [ ] 4.3 Log quota exceeded errors with slog
  - [ ] 4.4 Return StepsExceededError to terminate flow
  - [ ] 4.5 Write tests for quota enforcement during sync processing

- [ ] Task 5: Add flow cleanup (AC: #5)
  - [ ] 5.1 Implement cleanupFlow(flowToken string) method
  - [ ] 5.2 Call cleanupFlow() when flow completes (terminal state)
  - [ ] 5.3 Write tests for quota map cleanup (no memory leak)

- [ ] Task 6: Write comprehensive integration tests (AC: #7)
  - [ ] 6.1 Test flow exceeding max steps (default 1000)
  - [ ] 6.2 Test flow within limits completes normally
  - [ ] 6.3 Test custom limit (WithMaxSteps(10))
  - [ ] 6.4 Test error includes diagnostic info (flow token, steps, limit)
  - [ ] 6.5 Test multiple concurrent flows have independent quotas
  - [ ] 6.6 Test quota reset between flows

- [ ] Task 7: Update documentation
  - [ ] 7.1 Add quota enforcement to `internal/engine/doc.go`
  - [ ] 7.2 Document WithMaxSteps option in engine package
  - [ ] 7.3 Add example usage to tests

## Dev Notes

### Max-Steps Quota vs Cycle Detection

**Difference from Story 5.3 (Cycle Detection):**

| Aspect | Cycle Detection (5.3) | Max-Steps Quota (5.4) |
|--------|----------------------|----------------------|
| **Pattern** | Cyclic (A → B → A) | Linear (A → B → C → ... → Z) |
| **Detection** | Same (sync_id, binding_hash) fires twice | Total step count exceeds limit |
| **Use Case** | Infinite loops from self-triggering rules | Runaway flows with many distinct steps |
| **Granularity** | Per sync rule + binding | Per flow total |
| **Error Type** | CycleDetectedError | StepsExceededError |

**Both are complementary:**
- Cycle detection catches recursive patterns early
- Max-steps quota catches linear explosions (e.g., 1000 distinct invocations)
- Together they guarantee termination (CRITICAL-3)

### Quota Enforcement Implementation

```go
// internal/engine/quota.go
package engine

import (
    "fmt"
)

// QuotaEnforcer tracks the number of sync rule firings per flow
// and enforces a maximum steps limit.
//
// Each flow has its own QuotaEnforcer instance. The quota is checked
// on every completion before evaluating sync rules.
//
// This prevents runaway flows where many distinct sync rules fire
// in sequence (linear explosion), as opposed to cyclic patterns
// caught by cycle detection.
type QuotaEnforcer struct {
    maxSteps int  // Maximum allowed steps for this flow
    current  int  // Current step count
}

// NewQuotaEnforcer creates a new quota enforcer with the given limit.
//
// maxSteps: Maximum number of sync rule firings allowed per flow.
// Typical default: 1000 (configurable via engine.WithMaxSteps())
func NewQuotaEnforcer(maxSteps int) *QuotaEnforcer {
    return &QuotaEnforcer{
        maxSteps: maxSteps,
        current:  0,
    }
}

// Check increments the step counter and validates against the limit.
//
// Returns StepsExceededError if the quota is exceeded.
// This should be called before processing each completion.
func (q *QuotaEnforcer) Check(flowToken string) error {
    q.current++
    if q.current > q.maxSteps {
        return &StepsExceededError{
            FlowToken: flowToken,
            Steps:     q.current,
            Limit:     q.maxSteps,
        }
    }
    return nil
}

// Reset resets the step counter to 0.
// Used when starting a new flow with the same enforcer (rare).
func (q *QuotaEnforcer) Reset() {
    q.current = 0
}

// Current returns the current step count.
// Used for logging and diagnostics.
func (q *QuotaEnforcer) Current() int {
    return q.current
}

// StepsExceededError is returned when a flow exceeds the max steps quota.
type StepsExceededError struct {
    FlowToken string  // The flow that exceeded the quota
    Steps     int     // Number of steps taken
    Limit     int     // Maximum allowed steps
}

// Error implements the error interface.
func (e *StepsExceededError) Error() string {
    return fmt.Sprintf("flow %s exceeded max steps quota: %d steps > %d limit",
        e.FlowToken, e.Steps, e.Limit)
}

// RuntimeError returns the error type for matching.
// Used by error-handling sync rules.
func (e *StepsExceededError) RuntimeError() string {
    return "StepsExceededError"
}
```

### Engine Integration

```go
// internal/engine/engine.go (additions)

type Engine struct {
    store    *store.Store
    compiler queryir.Compiler
    flowGen  FlowTokenGenerator
    clock    *Clock
    specs    []ir.ConceptSpec
    syncs    []ir.SyncRule
    queue    *eventQueue

    // Quota enforcement
    maxSteps int                          // Default: 1000
    quotas   map[string]*QuotaEnforcer    // Per-flow quota trackers
}

// EngineOption allows configuration of engine parameters.
type EngineOption func(*Engine)

// WithMaxSteps sets the maximum steps quota per flow.
//
// Default: 1000 steps
// Use WithMaxSteps(5000) for flows with many distinct actions.
// Use WithMaxSteps(10) for testing quota enforcement.
func WithMaxSteps(maxSteps int) EngineOption {
    return func(e *Engine) {
        e.maxSteps = maxSteps
    }
}

func New(store *store.Store, specs []ir.ConceptSpec, syncs []ir.SyncRule, opts ...EngineOption) *Engine {
    e := &Engine{
        store:    store,
        clock:    NewClock(),
        specs:    specs,
        syncs:    syncs,
        queue:    newEventQueue(),
        maxSteps: 1000,  // Default quota
        quotas:   make(map[string]*QuotaEnforcer),
    }
    for _, opt := range opts {
        opt(e)
    }
    return e
}

func (e *Engine) processCompletion(ctx context.Context, comp *ir.Completion) error {
    flowToken := comp.FlowToken

    // Get or create quota enforcer for this flow
    quota, exists := e.quotas[flowToken]
    if !exists {
        quota = NewQuotaEnforcer(e.maxSteps)
        e.quotas[flowToken] = quota
    }

    // Check quota before processing sync rules
    if err := quota.Check(flowToken); err != nil {
        // Log quota exceeded with diagnostic info
        slog.Error("max steps quota exceeded",
            "flow_token", flowToken,
            "completion_id", comp.ID,
            "steps", quota.Current(),
            "limit", e.maxSteps,
            "event", "quota_exceeded",
        )
        return fmt.Errorf("quota enforcement failed: %w", err)
    }

    // ... continue with sync rule processing
    // (Story 3.3 - match sync rules)
    // (Story 4.4 - execute where-clauses)
    // (Story 4.5 - generate invocations)

    return nil
}

// cleanupFlow removes the quota enforcer for a completed flow.
// This prevents the quotas map from growing unbounded.
//
// Should be called when a flow reaches terminal state:
// - All actions completed
// - No pending invocations
// - No sync rules would fire
func (e *Engine) cleanupFlow(flowToken string) {
    delete(e.quotas, flowToken)
}
```

### Test Examples

```go
// internal/engine/quota_test.go
package engine

import (
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestQuotaEnforcer_WithinLimit(t *testing.T) {
    quota := NewQuotaEnforcer(10)

    // First 10 checks should succeed
    for i := 1; i <= 10; i++ {
        err := quota.Check("flow1")
        assert.NoError(t, err)
        assert.Equal(t, i, quota.Current())
    }

    // 11th check should fail
    err := quota.Check("flow1")
    require.Error(t, err)

    var stepsErr *StepsExceededError
    require.ErrorAs(t, err, &stepsErr)
    assert.Equal(t, "flow1", stepsErr.FlowToken)
    assert.Equal(t, 11, stepsErr.Steps)
    assert.Equal(t, 10, stepsErr.Limit)
}

func TestQuotaEnforcer_Reset(t *testing.T) {
    quota := NewQuotaEnforcer(5)

    // Use up quota
    for i := 0; i < 5; i++ {
        quota.Check("flow1")
    }
    assert.Equal(t, 5, quota.Current())

    // Reset
    quota.Reset()
    assert.Equal(t, 0, quota.Current())

    // Can check again
    err := quota.Check("flow1")
    assert.NoError(t, err)
    assert.Equal(t, 1, quota.Current())
}

func TestStepsExceededError_Error(t *testing.T) {
    err := &StepsExceededError{
        FlowToken: "flow-abc-123",
        Steps:     1001,
        Limit:     1000,
    }

    expected := "flow flow-abc-123 exceeded max steps quota: 1001 steps > 1000 limit"
    assert.Equal(t, expected, err.Error())
}

func TestStepsExceededError_RuntimeError(t *testing.T) {
    err := &StepsExceededError{
        FlowToken: "flow1",
        Steps:     100,
        Limit:     50,
    }

    assert.Equal(t, "StepsExceededError", err.RuntimeError())
}
```

```go
// internal/engine/engine_quota_test.go
package engine

import (
    "context"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/tyler/nysm/internal/ir"
    "github.com/tyler/nysm/internal/store"
)

func TestEngine_WithMaxSteps(t *testing.T) {
    dir := t.TempDir()
    st, err := store.Open(dir + "/test.db")
    require.NoError(t, err)
    defer st.Close()

    // Create engine with custom max steps
    engine := New(st, nil, nil, WithMaxSteps(50))

    assert.Equal(t, 50, engine.maxSteps)
}

func TestEngine_QuotaEnforcement_Exceeds(t *testing.T) {
    dir := t.TempDir()
    st, err := store.Open(dir + "/test.db")
    require.NoError(t, err)
    defer st.Close()

    // Create engine with low quota for testing
    engine := New(st, nil, nil, WithMaxSteps(3))

    ctx := context.Background()
    flowToken := "test-flow-1"

    // Process 3 completions - should succeed
    for i := 1; i <= 3; i++ {
        comp := &ir.Completion{
            ID:        fmt.Sprintf("comp%d", i),
            FlowToken: flowToken,
        }
        err := engine.processCompletion(ctx, comp)
        assert.NoError(t, err, "completion %d should succeed", i)
    }

    // 4th completion should fail (exceeds quota)
    comp4 := &ir.Completion{
        ID:        "comp4",
        FlowToken: flowToken,
    }
    err = engine.processCompletion(ctx, comp4)
    require.Error(t, err)

    var stepsErr *StepsExceededError
    require.ErrorAs(t, err, &stepsErr)
    assert.Equal(t, flowToken, stepsErr.FlowToken)
    assert.Equal(t, 4, stepsErr.Steps)
    assert.Equal(t, 3, stepsErr.Limit)
}

func TestEngine_QuotaEnforcement_WithinLimit(t *testing.T) {
    dir := t.TempDir()
    st, err := store.Open(dir + "/test.db")
    require.NoError(t, err)
    defer st.Close()

    engine := New(st, nil, nil, WithMaxSteps(100))

    ctx := context.Background()
    flowToken := "test-flow-2"

    // Process 50 completions - all should succeed
    for i := 1; i <= 50; i++ {
        comp := &ir.Completion{
            ID:        fmt.Sprintf("comp%d", i),
            FlowToken: flowToken,
        }
        err := engine.processCompletion(ctx, comp)
        assert.NoError(t, err, "completion %d should succeed", i)
    }

    // Verify quota tracker exists
    quota, exists := engine.quotas[flowToken]
    require.True(t, exists)
    assert.Equal(t, 50, quota.Current())
}

func TestEngine_QuotaEnforcement_IndependentFlows(t *testing.T) {
    dir := t.TempDir()
    st, err := store.Open(dir + "/test.db")
    require.NoError(t, err)
    defer st.Close()

    engine := New(st, nil, nil, WithMaxSteps(5))

    ctx := context.Background()
    flow1 := "flow-1"
    flow2 := "flow-2"

    // Process 5 completions for flow1
    for i := 1; i <= 5; i++ {
        comp := &ir.Completion{
            ID:        fmt.Sprintf("flow1-comp%d", i),
            FlowToken: flow1,
        }
        err := engine.processCompletion(ctx, comp)
        assert.NoError(t, err)
    }

    // Process 5 completions for flow2 (should also succeed - independent quota)
    for i := 1; i <= 5; i++ {
        comp := &ir.Completion{
            ID:        fmt.Sprintf("flow2-comp%d", i),
            FlowToken: flow2,
        }
        err := engine.processCompletion(ctx, comp)
        assert.NoError(t, err)
    }

    // Both flows should have independent quotas
    assert.Equal(t, 5, engine.quotas[flow1].Current())
    assert.Equal(t, 5, engine.quotas[flow2].Current())

    // 6th completion for flow1 should fail
    comp := &ir.Completion{
        ID:        "flow1-comp6",
        FlowToken: flow1,
    }
    err = engine.processCompletion(ctx, comp)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "quota enforcement failed")
}

func TestEngine_CleanupFlow(t *testing.T) {
    dir := t.TempDir()
    st, err := store.Open(dir + "/test.db")
    require.NoError(t, err)
    defer st.Close()

    engine := New(st, nil, nil)

    ctx := context.Background()
    flowToken := "test-flow-cleanup"

    // Process a completion to create quota enforcer
    comp := &ir.Completion{
        ID:        "comp1",
        FlowToken: flowToken,
    }
    err = engine.processCompletion(ctx, comp)
    require.NoError(t, err)

    // Verify quota exists
    _, exists := engine.quotas[flowToken]
    require.True(t, exists)

    // Cleanup flow
    engine.cleanupFlow(flowToken)

    // Verify quota removed
    _, exists = engine.quotas[flowToken]
    assert.False(t, exists)
}

func TestEngine_DefaultMaxSteps(t *testing.T) {
    dir := t.TempDir()
    st, err := store.Open(dir + "/test.db")
    require.NoError(t, err)
    defer st.Close()

    // Create engine without options
    engine := New(st, nil, nil)

    // Should have default of 1000
    assert.Equal(t, 1000, engine.maxSteps)
}
```

### Configuration Examples

```go
// Example 1: Default quota (1000 steps)
engine := engine.New(store, specs, syncs)

// Example 2: High quota for complex flows
engine := engine.New(
    store,
    specs,
    syncs,
    engine.WithMaxSteps(5000),
)

// Example 3: Low quota for testing
engine := engine.New(
    store,
    specs,
    syncs,
    engine.WithMaxSteps(10),
)

// Example 4: Multiple options
engine := engine.New(
    store,
    specs,
    syncs,
    engine.WithMaxSteps(2000),
    engine.WithFlowGenerator(fixedFlowGen),
    engine.WithClock(deterministicClock),
)
```

### File List

Files to create:

1. `internal/engine/quota.go` - QuotaEnforcer and StepsExceededError
2. `internal/engine/quota_test.go` - QuotaEnforcer unit tests
3. `internal/engine/engine_quota_test.go` - Engine integration tests

Files to modify:

1. `internal/engine/engine.go` - Add maxSteps field, quotas map, WithMaxSteps option, quota enforcement in processCompletion(), cleanupFlow()
2. `internal/engine/doc.go` - Document quota enforcement

### Relationship to Other Stories

**Dependencies:**
- Story 3.1 (Single-Writer Event Loop) - Required for Engine struct and processCompletion()
- Story 3.2 (Sync Rule Registration) - Required for sync evaluation
- Story 5.1 (Binding-Level Idempotency) - Required for understanding sync firing semantics
- Story 5.3 (Cycle Detection per Flow) - Complementary safety mechanism

**Enables:**
- Story 5.5 (Compile-Time Cycle Analysis) - Static analysis can warn about potential quota violations
- Story 6.2 (Test Execution Engine) - Test scenarios can verify quota enforcement
- Story 7.4 (Run Command) - CLI can configure max steps via flags

**Complements:**
- Story 5.3 (Cycle Detection) - Together they guarantee termination (CRITICAL-3)
  - Cycle detection catches recursive patterns (A → B → A)
  - Max-steps quota catches linear explosions (A → B → C → ... → Z)

### Story Completion Checklist

- [ ] `internal/engine/quota.go` created with QuotaEnforcer and StepsExceededError
- [ ] QuotaEnforcer implements Check(), Reset(), Current() methods
- [ ] StepsExceededError implements Error() and RuntimeError() methods
- [ ] Engine struct has maxSteps int field (default 1000)
- [ ] Engine struct has quotas map[string]*QuotaEnforcer field
- [ ] EngineOption type defined
- [ ] WithMaxSteps(maxSteps int) EngineOption implemented
- [ ] Engine.New() applies options correctly
- [ ] processCompletion() checks quota before sync processing
- [ ] cleanupFlow() removes quota enforcer from map
- [ ] Quota enforcement logs diagnostic info with slog
- [ ] Unit tests verify QuotaEnforcer behavior
- [ ] Integration tests verify flow termination on quota exceeded
- [ ] Integration tests verify flows within limits complete normally
- [ ] Integration tests verify custom limits respected
- [ ] Integration tests verify independent quotas for concurrent flows
- [ ] Integration tests verify quota cleanup prevents memory leak
- [ ] All tests pass (`go test ./internal/engine/...`)
- [ ] `go build ./...` succeeds
- [ ] `go vet ./internal/engine` passes
- [ ] Documentation updated in engine package

### References

- [Source: docs/epics.md#Story 5.4] - Story definition and acceptance criteria
- [Source: docs/architecture.md#CRITICAL-3] - Sync engine termination semantics
- [Source: docs/architecture.md#CP-2] - Per-flow quota tracking (not global)
- [Source: docs/prd.md#NFR-2.3] - Flow isolation requirement
- [Source: docs/epics.md#Story 5.3] - Cycle detection (complementary pattern)

## Dev Agent Record

### Agent Model Used

Claude Sonnet 4.5 (claude-sonnet-4-5-20250929)

### Validation History

- Initial creation: 2025-12-12

### Completion Notes

- Max-steps quota enforcement is CRITICAL-3 - guarantees termination
- Complementary to cycle detection (Story 5.3):
  - Cycle detection catches recursive patterns (A → B → A)
  - Max-steps quota catches linear explosions (A → B → C → ... → Z)
- Per-flow quota tracking ensures flow isolation (NFR-2.3)
- Default 1000 steps is reasonable for typical flows
- Configurable via WithMaxSteps() for specific needs
- StepsExceededError includes diagnostic info for debugging
- Quota map cleanup prevents memory leak
- Error logging with slog provides visibility
- Independent quotas for concurrent flows
- RuntimeError() method supports error-handling sync rules (MEDIUM-2)
