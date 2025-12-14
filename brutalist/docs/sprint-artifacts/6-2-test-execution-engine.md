# Story 6.2: Test Execution Engine

Status: done

## Story

As a **developer running conformance tests**,
I want **scenarios executed with deterministic clock and flow tokens**,
So that **tests are reproducible**.

## Acceptance Criteria

1. **harness.Run(scenario) function signature in `internal/harness/harness.go`**
   ```go
   type Harness struct {
       store    *store.Store
       engine   *engine.Engine
       clock    *testutil.DeterministicClock
       flowGen  *testutil.FixedFlowGenerator
       logger   *slog.Logger
   }

   type Result struct {
       Pass    bool
       Trace   []ir.Event
       Errors  []string
       State   map[string]interface{} // Final state tables
   }

   func Run(scenario *Scenario) (*Result, error)
   ```
   - Returns Result with pass/fail, trace, and errors
   - Scenario loaded from YAML (Story 6.1 dependency)

2. **Fresh in-memory database per test in `internal/harness/harness.go`**
   ```go
   func Run(scenario *Scenario) (*Result, error) {
       // Create fresh in-memory SQLite database
       st, err := store.Open(":memory:")
       if err != nil {
           return nil, fmt.Errorf("failed to create in-memory store: %w", err)
       }
       defer st.Close()

       // Apply schema
       if err := st.Migrate(); err != nil {
           return nil, fmt.Errorf("failed to migrate schema: %w", err)
       }

       // Initialize harness with fresh store
       h := &Harness{
           store:   st,
           clock:   testutil.NewDeterministicClock(),
           flowGen: testutil.NewFixedFlowGenerator(scenario.FlowToken),
           logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
       }

       // ... rest of execution
   }
   ```
   - Each test isolated with fresh database
   - No state pollution between tests

3. **DeterministicClock and FixedFlowGenerator in `internal/testutil/`**
   ```go
   // internal/testutil/clock.go
   type DeterministicClock struct {
       mu  sync.Mutex
       seq int64
   }

   func NewDeterministicClock() *DeterministicClock {
       return &DeterministicClock{seq: 0}
   }

   func (c *DeterministicClock) Next() int64 {
       c.mu.Lock()
       defer c.mu.Unlock()
       c.seq++
       return c.seq
   }

   func (c *DeterministicClock) Reset() {
       c.mu.Lock()
       defer c.mu.Unlock()
       c.seq = 0
   }

   // internal/testutil/flow.go
   type FixedFlowGenerator struct {
       token string
   }

   func NewFixedFlowGenerator(token string) *FixedFlowGenerator {
       return &FixedFlowGenerator{token: token}
   }

   func (g *FixedFlowGenerator) Generate() string {
       return g.token
   }
   ```
   - DeterministicClock starts at 0, increments monotonically
   - FixedFlowGenerator returns same token every time
   - Reset() allows test reuse

4. **Setup step execution in `internal/harness/harness.go`**
   ```go
   func (h *Harness) executeSetup(ctx context.Context, setup []SetupStep) error {
       for i, step := range setup {
           // Generate flow token ONCE for this invocation
           flowToken := h.flowGen.Generate()

           // Get seq ONCE and reuse for both ID computation and record field
           // CRITICAL: clock.Next() must be called exactly once per record
           invSeq := h.clock.Next()

           // Compute ID using Story 1.5 signature (SecurityContext excluded per CP-6)
           invID, err := ir.InvocationID(flowToken, step.Action, step.Args, invSeq)
           if err != nil {
               return fmt.Errorf("setup step %d: failed to compute invocation ID: %w", i, err)
           }

           inv := ir.Invocation{
               ID:              invID,
               FlowToken:       flowToken,
               ActionURI:       step.Action,
               Args:            step.Args,
               Seq:             invSeq,  // Same seq used in ID computation
               SecurityContext: ir.SecurityContext{},
               SpecHash:        h.specHash,
               EngineVersion:   "test",
           }

           // Write invocation to store
           if err := h.store.WriteInvocation(ctx, inv); err != nil {
               return fmt.Errorf("setup step %d: failed to write invocation: %w", i, err)
           }

           // Execute action (stub for now - Story 6.x will implement)
           // For now, assume all setup steps succeed

           // Get completion seq ONCE
           compSeq := h.clock.Next()
           result := ir.IRObject{}  // Empty for setup

           compID, err := ir.CompletionID(inv.ID, "Success", result, compSeq)
           if err != nil {
               return fmt.Errorf("setup step %d: failed to compute completion ID: %w", i, err)
           }

           comp := ir.Completion{
               ID:           compID,
               InvocationID: inv.ID,
               OutputCase:   "Success",
               Result:       result,
               Seq:          compSeq,  // Same seq used in ID computation
           }

           if err := h.store.WriteCompletion(ctx, comp); err != nil {
               return fmt.Errorf("setup step %d: failed to write completion: %w", i, err)
           }
       }
       return nil
   }
   ```
   - Setup steps executed before flow steps
   - Uses deterministic clock and flow generator
   - Writes to event log for replay

5. **Flow step execution with expect validation in `internal/harness/harness.go`**
   ```go
   func (h *Harness) executeFlow(ctx context.Context, flow []FlowStep) (*Result, error) {
       result := &Result{
           Pass:   true,
           Trace:  []ir.Event{},
           Errors: []string{},
           State:  make(map[string]interface{}),
       }

       for i, step := range flow {
           // Generate flow token and seq ONCE (CRITICAL: avoid double clock.Next())
           flowToken := h.flowGen.Generate()
           invSeq := h.clock.Next()

           // Compute ID using Story 1.5 signature
           invID, err := ir.InvocationID(flowToken, step.Invoke, step.Args, invSeq)
           if err != nil {
               return nil, fmt.Errorf("flow step %d: failed to compute invocation ID: %w", i, err)
           }

           inv := ir.Invocation{
               ID:              invID,
               FlowToken:       flowToken,
               ActionURI:       step.Invoke,
               Args:            step.Args,
               Seq:             invSeq,  // Same seq used in ID computation
               SecurityContext: ir.SecurityContext{},
               SpecHash:        h.specHash,
               EngineVersion:   "test",
           }

           // Enqueue to engine
           h.engine.Enqueue(engine.Event{
               Type: engine.EventTypeInvocation,
               Data: &inv,
           })

           // Wait for completion (blocking - test is deterministic)
           // TODO: Story 6.x - implement synchronous execution for tests
           comp := h.waitForCompletion(ctx, inv.ID)

           // Validate against expect clause
           if step.Expect != nil {
               if comp.OutputCase != step.Expect.Case {
                   err := fmt.Errorf("step %d: expected case %q, got %q",
                       i, step.Expect.Case, comp.OutputCase)
                   result.Errors = append(result.Errors, err.Error())
                   result.Pass = false
               }

               // Validate result fields
               for key, expectedVal := range step.Expect.Result {
                   actualVal, ok := comp.Result[key]
                   if !ok {
                       err := fmt.Errorf("step %d: missing result field %q", i, key)
                       result.Errors = append(result.Errors, err.Error())
                       result.Pass = false
                       continue
                   }
                   if !ir.ValueEqual(expectedVal, actualVal) {
                       err := fmt.Errorf("step %d: field %q expected %v, got %v",
                           i, key, expectedVal, actualVal)
                       result.Errors = append(result.Errors, err.Error())
                       result.Pass = false
                   }
               }
           }

           // Add to trace
           result.Trace = append(result.Trace, inv, comp)
       }

       return result, nil
   }
   ```
   - Each flow step generates invocation
   - Waits for completion synchronously (deterministic)
   - Validates expect clause against actual completion
   - Collects errors without fail-fast
   - Builds trace for assertions

6. **Result struct with pass/fail and trace in `internal/harness/types.go`**
   ```go
   type Result struct {
       Pass    bool              // Overall test pass/fail
       Trace   []ir.Event        // Complete invocation/completion trace
       Errors  []string          // Validation error messages
       State   map[string]interface{} // Final state tables (for assertions)
   }

   // Event is a union type for invocations and completions
   type Event interface {
       eventMarker()
   }

   // Invocation and Completion implement Event via ir types
   ```
   - Pass = true if all expect clauses match and all assertions pass
   - Trace contains all invocations/completions in order
   - Errors collected for reporting
   - State captured for final state assertions

## Quick Reference

| Pattern | Key Rule |
|---------|----------|
| **CP-2** | Logical clock (seq) for deterministic ordering |
| **CP-3** | Deterministic test helpers (clock, flow generator) |
| **FR-6.2** | Run scenarios with assertions on action traces |
| **NFR-2.1** | Deterministic replay (same scenario twice = identical results) |

## Tasks / Subtasks

- [ ] Task 1: Create harness package structure (AC: #1)
  - [ ] 1.1 Create `internal/harness/` directory
  - [ ] 1.2 Create `internal/harness/doc.go` with package documentation
  - [ ] 1.3 Create `internal/harness/types.go` with Result, Event types
  - [ ] 1.4 Create `internal/harness/harness.go` with Harness struct

- [ ] Task 2: Create testutil package with deterministic helpers (AC: #3)
  - [ ] 2.1 Create `internal/testutil/` directory
  - [ ] 2.2 Create `internal/testutil/clock.go` with DeterministicClock
  - [ ] 2.3 Create `internal/testutil/flow.go` with FixedFlowGenerator
  - [ ] 2.4 Write tests for DeterministicClock (monotonicity, reset)
  - [ ] 2.5 Write tests for FixedFlowGenerator (fixed token)

- [ ] Task 3: Implement Run() function with fresh database (AC: #2)
  - [ ] 3.1 Implement Run(scenario) function signature
  - [ ] 3.2 Create in-memory SQLite database (":memory:")
  - [ ] 3.3 Apply schema migration
  - [ ] 3.4 Initialize Harness with fresh store, clock, flowGen
  - [ ] 3.5 Write tests for fresh database isolation

- [ ] Task 4: Implement executeSetup() (AC: #4)
  - [ ] 4.1 Create executeSetup(ctx, setup) method
  - [ ] 4.2 Loop over setup steps
  - [ ] 4.3 Generate invocations with deterministic clock and flow token
  - [ ] 4.4 Write invocations to store
  - [ ] 4.5 Generate success completions (stub - assume all succeed)
  - [ ] 4.6 Write completions to store
  - [ ] 4.7 Write tests for setup execution (order, determinism)

- [ ] Task 5: Implement executeFlow() with expect validation (AC: #5)
  - [ ] 5.1 Create executeFlow(ctx, flow) method
  - [ ] 5.2 Loop over flow steps
  - [ ] 5.3 Generate invocations with deterministic clock and flow token
  - [ ] 5.4 Enqueue invocations to engine
  - [ ] 5.5 Wait for completions synchronously (waitForCompletion stub)
  - [ ] 5.6 Validate expect clause (output case, result fields)
  - [ ] 5.7 Collect errors without fail-fast
  - [ ] 5.8 Build trace (append invocations and completions)
  - [ ] 5.9 Write tests for flow execution (expect validation, error collection)

- [ ] Task 6: Implement Result struct and helpers (AC: #6)
  - [ ] 6.1 Define Result struct in types.go
  - [ ] 6.2 Add Pass, Trace, Errors, State fields
  - [ ] 6.3 Implement ValueEqual() helper for result comparison
  - [ ] 6.4 Write tests for Result struct (pass/fail logic)

- [ ] Task 7: Write comprehensive integration tests (AC: all)
  - [ ] 7.1 Test same scenario twice produces identical results (determinism)
  - [ ] 7.2 Test failed expectation reported with details
  - [ ] 7.3 Test setup steps executed before flow
  - [ ] 7.4 Test fresh database per test (no state pollution)
  - [ ] 7.5 Test trace includes all invocations and completions

## Dev Notes

### Harness Execution Architecture

```
┌────────────────────────────────────────────────┐
│ harness.Run(scenario)                          │
│                                                │
│ 1. Create fresh in-memory database             │
│ 2. Apply schema migration                      │
│ 3. Initialize deterministic helpers            │
│    - DeterministicClock (seq starts at 0)      │
│    - FixedFlowGenerator (same token)           │
│ 4. Load concept specs and sync rules           │
│ 5. Create Engine with test helpers             │
└────────────────────────────────────────────────┘
                      │
                      ▼
┌────────────────────────────────────────────────┐
│ executeSetup(setup steps)                      │
│                                                │
│ For each setup step:                           │
│   - Generate invocation (deterministic)        │
│   - Write to event log                         │
│   - Generate success completion (stub)         │
│   - Write completion to event log              │
└────────────────────────────────────────────────┘
                      │
                      ▼
┌────────────────────────────────────────────────┐
│ executeFlow(flow steps)                        │
│                                                │
│ For each flow step:                            │
│   - Generate invocation (deterministic)        │
│   - Enqueue to engine                          │
│   - Wait for completion (synchronous)          │
│   - Validate expect clause:                    │
│     * Check output case matches                │
│     * Check result fields match                │
│   - Collect errors (don't fail-fast)           │
│   - Add invocation/completion to trace         │
└────────────────────────────────────────────────┘
                      │
                      ▼
┌────────────────────────────────────────────────┐
│ Return Result                                  │
│                                                │
│ - Pass: true if all expects match              │
│ - Trace: all invocations/completions           │
│ - Errors: collected validation failures        │
│ - State: final state tables (for assertions)   │
└────────────────────────────────────────────────┘
```

### Deterministic Test Helpers

**DeterministicClock:**
- Starts at seq 0
- Each Next() increments seq monotonically
- Reset() resets to 0 (for test reuse)
- Thread-safe with mutex
- Implements same interface as engine.Clock

**FixedFlowGenerator:**
- Returns same flow token every time
- Configured with token at construction
- Implements engine.FlowTokenGenerator interface
- Enables golden trace snapshots (same token = same hash)

**Why Deterministic?**
- Same scenario run twice produces byte-identical event log
- Golden snapshots don't drift
- Replay always produces same results
- Test failures are reproducible

### Package Documentation

```go
// internal/harness/doc.go
// Package harness implements the NYSM conformance test runner.
//
// The harness executes test scenarios defined in YAML, validates operational
// principles, and generates golden trace snapshots.
//
// ARCHITECTURE:
//
// Test Isolation:
// Each test scenario runs in a fresh in-memory SQLite database. This ensures:
// - No state pollution between tests
// - Fast test execution (no disk I/O)
// - Simple cleanup (database discarded after test)
//
// Deterministic Execution:
// The harness uses deterministic test helpers:
// - DeterministicClock: Monotonic seq counter (no wall time)
// - FixedFlowGenerator: Same flow token every run
// This ensures the same scenario produces identical results on every run.
//
// Expect Validation:
// Each flow step can have an expect clause:
//   expect:
//     case: Success
//     result: { item_id: "widget", new_quantity: 3 }
//
// The harness validates:
// - Output case matches expected case
// - Result fields match expected values
// - Errors collected without fail-fast
//
// Trace Collection:
// All invocations and completions are collected in a trace:
// - Trace used for assertions (Story 6.3)
// - Trace compared to golden snapshots (Story 6.6)
// - Trace enables "why did this happen?" debugging
//
// CRITICAL PATTERNS:
//
// CP-2: Logical Clock
// All events stamped with DeterministicClock.Next().
// NEVER use wall-clock timestamps in tests.
//
// FR-6.2: Run scenarios with assertions
// The harness executes scenarios and validates expect clauses.
// This is the foundation for operational principle validation.
package harness
```

### Harness Implementation

```go
// internal/harness/harness.go
package harness

import (
    "context"
    "fmt"
    "io"
    "log/slog"

    "github.com/tyler/nysm/internal/engine"
    "github.com/tyler/nysm/internal/ir"
    "github.com/tyler/nysm/internal/store"
    "github.com/tyler/nysm/internal/testutil"
)

// Harness is the test execution engine.
// It runs scenarios with deterministic clock and flow tokens.
type Harness struct {
    store    *store.Store
    engine   *engine.Engine
    clock    *testutil.DeterministicClock
    flowGen  *testutil.FixedFlowGenerator
    logger   *slog.Logger
    specHash string // Hash of concept specs (for invocations)
}

// Run executes a test scenario and returns the result.
//
// Each scenario runs in a fresh in-memory database for isolation.
// Deterministic helpers ensure reproducible results.
//
// Execution flow:
// 1. Create fresh in-memory database
// 2. Load and compile concept specs and sync rules
// 3. Execute setup steps
// 4. Execute flow steps with expect validation
// 5. Return result with pass/fail, trace, and errors
func Run(scenario *Scenario) (*Result, error) {
    // Create fresh in-memory SQLite database
    st, err := store.Open(":memory:")
    if err != nil {
        return nil, fmt.Errorf("failed to create in-memory store: %w", err)
    }
    defer st.Close()

    // Apply schema migration
    if err := st.Migrate(); err != nil {
        return nil, fmt.Errorf("failed to migrate schema: %w", err)
    }

    // Initialize deterministic helpers
    clock := testutil.NewDeterministicClock()
    flowGen := testutil.NewFixedFlowGenerator(scenario.FlowToken)

    // TODO: Story 6.1 - Load and compile specs from scenario.Specs
    // For now, use empty specs
    specs := []ir.ConceptSpec{}
    syncs := []ir.SyncRule{}
    specHash := "test-spec-hash"

    // Create engine with test helpers
    eng := engine.New(st, specs, syncs)
    // TODO: Story 3.6 - Inject clock and flowGen into engine
    // For now, engine uses its own clock/flowGen

    // Initialize harness
    h := &Harness{
        store:    st,
        engine:   eng,
        clock:    clock,
        flowGen:  flowGen,
        logger:   slog.New(slog.NewTextHandler(io.Discard, nil)), // Suppress logs in tests
        specHash: specHash,
    }

    ctx := context.Background()

    // Execute setup steps
    if err := h.executeSetup(ctx, scenario.Setup); err != nil {
        return nil, fmt.Errorf("failed to execute setup: %w", err)
    }

    // Execute flow steps
    result, err := h.executeFlow(ctx, scenario.Flow)
    if err != nil {
        return nil, fmt.Errorf("failed to execute flow: %w", err)
    }

    return result, nil
}

// executeSetup runs all setup steps.
//
// Setup steps are executed sequentially before the flow.
// Each step generates an invocation and completion (assuming success).
func (h *Harness) executeSetup(ctx context.Context, setup []SetupStep) error {
    for i, step := range setup {
        // Generate flow token ONCE for this invocation
        flowToken := h.flowGen.Generate()

        // Get seq ONCE and reuse for both ID computation and record field
        // CRITICAL: clock.Next() must be called exactly once per record
        invSeq := h.clock.Next()

        // Compute ID using Story 1.5 signature (SecurityContext excluded per CP-6)
        invID, err := ir.InvocationID(flowToken, step.Action, step.Args, invSeq)
        if err != nil {
            return fmt.Errorf("setup step %d: failed to compute invocation ID: %w", i, err)
        }

        inv := ir.Invocation{
            ID:              invID,
            FlowToken:       flowToken,
            ActionURI:       step.Action,
            Args:            step.Args,
            Seq:             invSeq,  // Same seq used in ID computation
            SecurityContext: ir.SecurityContext{},
            SpecHash:        h.specHash,
            EngineVersion:   "test",
        }

        // Write invocation to store
        if err := h.store.WriteInvocation(ctx, inv); err != nil {
            return fmt.Errorf("setup step %d: failed to write invocation: %w", i, err)
        }

        // Execute action (stub for now - Story 6.x will implement)
        // For now, assume all setup steps succeed

        // Get completion seq ONCE
        compSeq := h.clock.Next()
        result := ir.IRObject{}  // Empty for setup

        compID, err := ir.CompletionID(inv.ID, "Success", result, compSeq)
        if err != nil {
            return fmt.Errorf("setup step %d: failed to compute completion ID: %w", i, err)
        }

        comp := ir.Completion{
            ID:           compID,
            InvocationID: inv.ID,
            OutputCase:   "Success",
            Result:       result,
            Seq:          compSeq,  // Same seq used in ID computation
        }

        if err := h.store.WriteCompletion(ctx, comp); err != nil {
            return fmt.Errorf("setup step %d: failed to write completion: %w", i, err)
        }

        h.logger.Info("setup step completed",
            "step", i,
            "action", step.Action,
            "invocation_id", inv.ID,
            "completion_id", comp.ID,
        )
    }
    return nil
}

// executeFlow runs all flow steps and validates expect clauses.
//
// Each step:
// 1. Generates invocation
// 2. Enqueues to engine
// 3. Waits for completion
// 4. Validates expect clause
// 5. Collects errors without fail-fast
// 6. Builds trace
func (h *Harness) executeFlow(ctx context.Context, flow []FlowStep) (*Result, error) {
    result := &Result{
        Pass:   true,
        Trace:  []ir.Event{},
        Errors: []string{},
        State:  make(map[string]interface{}),
    }

    for i, step := range flow {
        // Generate flow token and seq ONCE (CRITICAL: avoid double clock.Next())
        flowToken := h.flowGen.Generate()
        invSeq := h.clock.Next()

        // Compute ID using Story 1.5 signature (SecurityContext excluded per CP-6)
        invID, err := ir.InvocationID(flowToken, step.Invoke, step.Args, invSeq)
        if err != nil {
            return nil, fmt.Errorf("flow step %d: failed to compute invocation ID: %w", i, err)
        }

        inv := ir.Invocation{
            ID:              invID,
            FlowToken:       flowToken,
            ActionURI:       step.Invoke,
            Args:            step.Args,
            Seq:             invSeq,  // Same seq used in ID computation
            SecurityContext: ir.SecurityContext{},
            SpecHash:        h.specHash,
            EngineVersion:   "test",
        }

        // Write invocation to store
        if err := h.store.WriteInvocation(ctx, inv); err != nil {
            return nil, fmt.Errorf("flow step %d: failed to write invocation: %w", i, err)
        }

        // Enqueue to engine
        h.engine.Enqueue(engine.Event{
            Type: engine.EventTypeInvocation,
            Data: &inv,
        })

        // Wait for completion (blocking - test is deterministic)
        // TODO: Story 6.x - implement synchronous execution for tests
        comp := h.waitForCompletion(ctx, inv.ID)

        // Validate against expect clause
        if step.Expect != nil {
            if comp.OutputCase != step.Expect.Case {
                err := fmt.Errorf("step %d: expected case %q, got %q",
                    i, step.Expect.Case, comp.OutputCase)
                result.Errors = append(result.Errors, err.Error())
                result.Pass = false
            }

            // Validate result fields
            for key, expectedVal := range step.Expect.Result {
                actualVal, ok := comp.Result[key]
                if !ok {
                    err := fmt.Errorf("step %d: missing result field %q", i, key)
                    result.Errors = append(result.Errors, err.Error())
                    result.Pass = false
                    continue
                }
                if !ir.ValueEqual(expectedVal, actualVal) {
                    err := fmt.Errorf("step %d: field %q expected %v, got %v",
                        i, key, expectedVal, actualVal)
                    result.Errors = append(result.Errors, err.Error())
                    result.Pass = false
                }
            }
        }

        // Add to trace
        result.Trace = append(result.Trace, inv, comp)

        h.logger.Info("flow step completed",
            "step", i,
            "action", step.Invoke,
            "invocation_id", inv.ID,
            "completion_id", comp.ID,
            "output_case", comp.OutputCase,
        )
    }

    return result, nil
}

// waitForCompletion waits for a completion to be written to the store.
//
// This is a blocking call that polls the store until the completion appears.
// In a real implementation, this would use a channel or event notification.
//
// TODO: Story 6.x - implement proper synchronous execution for tests
func (h *Harness) waitForCompletion(ctx context.Context, invocationID string) ir.Completion {
    // Stub implementation - assume completion is already written
    // In real implementation, this would poll store or use event notification

    // Get seq ONCE and reuse for both ID computation and record field
    compSeq := h.clock.Next()
    result := ir.IRObject{}

    compID, err := ir.CompletionID(invocationID, "Success", result, compSeq)
    if err != nil {
        // In stub, panic is acceptable; real impl would return error
        panic(fmt.Errorf("failed to compute completion ID: %w", err))
    }

    comp := ir.Completion{
        ID:           compID,
        InvocationID: invocationID,
        OutputCase:   "Success",
        Result:       result,
        Seq:          compSeq,  // Same seq used in ID computation
    }
    return comp
}
```

### Types Implementation

```go
// internal/harness/types.go
package harness

import (
    "github.com/tyler/nysm/internal/ir"
)

// Result is the outcome of a test scenario execution.
type Result struct {
    Pass   bool              // Overall test pass/fail
    Trace  []ir.Event        // Complete invocation/completion trace
    Errors []string          // Validation error messages
    State  map[string]interface{} // Final state tables (for assertions)
}

// Scenario is a test scenario loaded from YAML (Story 6.1).
type Scenario struct {
    Name      string
    FlowToken string
    Specs     []string    // Paths to concept/sync specs
    Setup     []SetupStep
    Flow      []FlowStep
}

// SetupStep is a setup action to execute before the flow.
type SetupStep struct {
    Action string
    Args   ir.IRObject
}

// FlowStep is a flow action with optional expect clause.
type FlowStep struct {
    Invoke string
    Args   ir.IRObject
    Expect *ExpectClause // nil if no expectation
}

// ExpectClause defines expected completion behavior.
type ExpectClause struct {
    Case   string      // Expected output case (e.g., "Success")
    Result ir.IRObject // Expected result fields
}
```

### Test Utility Implementations

```go
// internal/testutil/clock.go
package testutil

import "sync"

// DeterministicClock provides a thread-safe monotonic logical clock for tests.
//
// Unlike engine.Clock, DeterministicClock can be reset for test reuse.
// This enables the same test scenario to run multiple times with identical seq values.
type DeterministicClock struct {
    mu  sync.Mutex
    seq int64
}

// NewDeterministicClock creates a new deterministic clock starting at 0.
//
// The first call to Next() returns 1.
func NewDeterministicClock() *DeterministicClock {
    return &DeterministicClock{seq: 0}
}

// Next increments and returns the next sequence number.
//
// Thread-safe: Can be called from multiple goroutines.
// Monotonic: Always returns seq+1, never decreases.
func (c *DeterministicClock) Next() int64 {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.seq++
    return c.seq
}

// Current returns the current sequence number without incrementing.
//
// Thread-safe: Can be called from multiple goroutines.
func (c *DeterministicClock) Current() int64 {
    c.mu.Lock()
    defer c.mu.Unlock()
    return c.seq
}

// Reset resets the clock to 0.
//
// Used for test reuse. After Reset(), the next call to Next() returns 1.
func (c *DeterministicClock) Reset() {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.seq = 0
}
```

```go
// internal/testutil/flow.go
package testutil

// FixedFlowGenerator generates the same flow token every time.
//
// This enables deterministic test execution and golden snapshot comparison.
// The same scenario with the same FixedFlowGenerator produces byte-identical event logs.
type FixedFlowGenerator struct {
    token string
}

// NewFixedFlowGenerator creates a new fixed flow token generator.
//
// The token is typically set in the scenario YAML:
//   flow_token: "test-flow-00000000-0000-0000-0000-000000000001"
func NewFixedFlowGenerator(token string) *FixedFlowGenerator {
    return &FixedFlowGenerator{token: token}
}

// Generate returns the fixed flow token.
//
// Implements engine.FlowTokenGenerator interface.
func (g *FixedFlowGenerator) Generate() string {
    return g.token
}
```

## Test Examples

### Example 1: Same scenario twice produces identical results

```go
// internal/harness/harness_test.go
package harness

import (
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/tyler/nysm/internal/ir"
)

func TestRun_Deterministic(t *testing.T) {
    scenario := &Scenario{
        Name:      "test_determinism",
        FlowToken: "test-flow-00000000-0000-0000-0000-000000000001",
        Setup:     []SetupStep{},
        Flow: []FlowStep{
            {
                Invoke: "Cart.addItem",
                Args: ir.IRObject{
                    "item_id":  ir.IRString("widget"),
                    "quantity": ir.IRInt(3),
                },
                Expect: &ExpectClause{
                    Case: "Success",
                    Result: ir.IRObject{
                        "item_id":      ir.IRString("widget"),
                        "new_quantity": ir.IRInt(3),
                    },
                },
            },
        },
    }

    // Run scenario twice
    result1, err := Run(scenario)
    require.NoError(t, err)
    require.True(t, result1.Pass)

    result2, err := Run(scenario)
    require.NoError(t, err)
    require.True(t, result2.Pass)

    // Verify traces are identical
    require.Equal(t, len(result1.Trace), len(result2.Trace))
    for i := range result1.Trace {
        // Compare event IDs (content-addressed, so identical inputs = identical IDs)
        switch e1 := result1.Trace[i].(type) {
        case ir.Invocation:
            e2, ok := result2.Trace[i].(ir.Invocation)
            require.True(t, ok)
            assert.Equal(t, e1.ID, e2.ID, "invocation IDs should match")
            assert.Equal(t, e1.Seq, e2.Seq, "seq should match")
        case ir.Completion:
            e2, ok := result2.Trace[i].(ir.Completion)
            require.True(t, ok)
            assert.Equal(t, e1.ID, e2.ID, "completion IDs should match")
            assert.Equal(t, e1.Seq, e2.Seq, "seq should match")
        }
    }
}
```

### Example 2: Failed expectation reported with details

```go
func TestRun_ExpectValidation_Failure(t *testing.T) {
    scenario := &Scenario{
        Name:      "test_expect_failure",
        FlowToken: "test-flow-00000000-0000-0000-0000-000000000002",
        Setup:     []SetupStep{},
        Flow: []FlowStep{
            {
                Invoke: "Cart.addItem",
                Args: ir.IRObject{
                    "item_id":  ir.IRString("widget"),
                    "quantity": ir.IRInt(3),
                },
                Expect: &ExpectClause{
                    Case: "Success",
                    Result: ir.IRObject{
                        "item_id":      ir.IRString("widget"),
                        "new_quantity": ir.IRInt(5), // WRONG - expecting 5 but got 3
                    },
                },
            },
        },
    }

    result, err := Run(scenario)
    require.NoError(t, err)

    // Test should fail
    assert.False(t, result.Pass)

    // Verify error message is descriptive
    require.Len(t, result.Errors, 1)
    assert.Contains(t, result.Errors[0], "field \"new_quantity\" expected 5, got 3")
}
```

### Example 3: Setup steps executed before flow

```go
func TestRun_SetupSteps(t *testing.T) {
    scenario := &Scenario{
        Name:      "test_setup",
        FlowToken: "test-flow-00000000-0000-0000-0000-000000000003",
        Setup: []SetupStep{
            {
                Action: "Inventory.setStock",
                Args: ir.IRObject{
                    "item_id":  ir.IRString("widget"),
                    "quantity": ir.IRInt(10),
                },
            },
        },
        Flow: []FlowStep{
            {
                Invoke: "Inventory.reserve",
                Args: ir.IRObject{
                    "item_id":  ir.IRString("widget"),
                    "quantity": ir.IRInt(3),
                },
                Expect: &ExpectClause{
                    Case: "Success",
                },
            },
        },
    }

    result, err := Run(scenario)
    require.NoError(t, err)
    require.True(t, result.Pass)

    // Verify trace shows setup before flow
    // Trace: [setup_inv, setup_comp, flow_inv, flow_comp]
    require.Len(t, result.Trace, 4)

    // First two events are setup
    setupInv, ok := result.Trace[0].(ir.Invocation)
    require.True(t, ok)
    assert.Equal(t, "Inventory.setStock", setupInv.ActionURI)

    setupComp, ok := result.Trace[1].(ir.Completion)
    require.True(t, ok)
    assert.Equal(t, setupInv.ID, setupComp.InvocationID)

    // Next two events are flow
    flowInv, ok := result.Trace[2].(ir.Invocation)
    require.True(t, ok)
    assert.Equal(t, "Inventory.reserve", flowInv.ActionURI)

    flowComp, ok := result.Trace[3].(ir.Completion)
    require.True(t, ok)
    assert.Equal(t, flowInv.ID, flowComp.InvocationID)
}
```

## File List

Files to create:

1. `internal/harness/doc.go` - Package documentation
2. `internal/harness/types.go` - Result, Scenario, SetupStep, FlowStep, ExpectClause types
3. `internal/harness/harness.go` - Harness struct, Run(), executeSetup(), executeFlow(), waitForCompletion()
4. `internal/harness/harness_test.go` - Tests for determinism, expect validation, setup execution
5. `internal/testutil/clock.go` - DeterministicClock with Reset()
6. `internal/testutil/flow.go` - FixedFlowGenerator
7. `internal/testutil/clock_test.go` - Tests for DeterministicClock
8. `internal/testutil/flow_test.go` - Tests for FixedFlowGenerator

## Relationship to Other Stories

**Dependencies:**
- Story 1.1 (Project Initialization & IR Type Definitions) - Required for ir.* types
- Story 2.1 (SQLite Store Initialization) - Required for store.Open()
- Story 2.2 (Event Log Schema) - Required for store operations
- Story 3.1 (Single-Writer Event Loop) - Required for engine.Engine, engine.Event
- Story 6.1 (Scenario Definition Format) - Required for Scenario, SetupStep, FlowStep types

**Enables:**
- Story 6.3 (Trace Assertions) - Uses Result.Trace for assertions
- Story 6.4 (Final State Assertions) - Uses Result.State for state validation
- Story 6.5 (Operational Principle Validation) - Runs principles as scenarios
- Story 6.6 (Golden Trace Snapshots) - Compares Result.Trace to golden files

**Partial Dependencies:**
- Story 3.6 (Flow Token Generation) - FixedFlowGenerator implements FlowTokenGenerator interface
- Story 3.5 (Invocation Execution) - waitForCompletion() will use engine execution when available

## Story Completion Checklist

- [ ] `internal/harness/` directory created
- [ ] `internal/harness/doc.go` written with package documentation
- [ ] `internal/harness/types.go` implements Result, Scenario, SetupStep, FlowStep, ExpectClause
- [ ] `internal/harness/harness.go` implements Harness struct and methods
- [ ] Run() creates fresh in-memory database per test
- [ ] executeSetup() executes setup steps before flow
- [ ] executeFlow() executes flow steps with expect validation
- [ ] waitForCompletion() stub implemented (synchronous execution)
- [ ] Result struct has Pass, Trace, Errors, State fields
- [ ] `internal/testutil/clock.go` implements DeterministicClock
- [ ] `internal/testutil/flow.go` implements FixedFlowGenerator
- [ ] All tests pass (`go test ./internal/harness/... ./internal/testutil/...`)
- [ ] `go build ./...` succeeds
- [ ] `go vet ./internal/harness ./internal/testutil` passes
- [ ] Test: Same scenario twice produces identical results
- [ ] Test: Failed expectation reported with details
- [ ] Test: Setup steps executed before flow
- [ ] Test: Fresh database per test (no state pollution)
- [ ] Test: Trace includes all invocations and completions in order

## References

- [Source: docs/architecture.md#Harness package structure] - Test execution architecture
- [Source: docs/architecture.md#CP-2] - Logical clocks (seq), not timestamps
- [Source: docs/architecture.md#CP-3] - Deterministic test helpers
- [Source: docs/epics.md#Story 6.2] - Story definition and acceptance criteria
- [Source: docs/prd.md#FR-6.2] - Run scenarios with assertions on action traces
- [Source: docs/prd.md#NFR-2.1] - Deterministic replay requirement
- [Source: docs/sprint-artifacts/3-1-single-writer-event-loop.md] - Engine event processing pattern

## Dev Agent Record

### Agent Model Used

Claude Sonnet 4.5 (claude-sonnet-4-5-20250929)

### Validation History

- Initial creation: 2025-12-12

### Completion Notes

- Foundation story for Epic 6 - all harness stories depend on this execution engine
- Fresh in-memory database per test ensures isolation
- Deterministic helpers (DeterministicClock, FixedFlowGenerator) ensure reproducible results
- executeSetup() runs setup steps before flow (state initialization)
- executeFlow() validates expect clauses without fail-fast (collects all errors)
- Result struct provides pass/fail, trace, and errors for assertions and golden comparison
- waitForCompletion() is a stub - will be implemented when engine execution is available
- FixedFlowGenerator implements engine.FlowTokenGenerator interface for injection
- DeterministicClock can be reset for test reuse
- Trace includes all invocations and completions in order for assertions (Story 6.3)
- Next stories (6.3-6.6) will use Result.Trace and Result.State for assertions and golden comparison
