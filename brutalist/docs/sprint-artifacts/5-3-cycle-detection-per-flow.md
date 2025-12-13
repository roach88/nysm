# Story 5.3: Cycle Detection per Flow

Status: ready-for-dev

## Story

As a **developer writing sync rules**,
I want **cycles detected and prevented**,
So that **self-triggering rules don't loop forever**.

## Acceptance Criteria

1. **CycleDetector struct in `internal/engine/cycle.go`**
   ```go
   type CycleDetector struct {
       mu      sync.Mutex
       history map[string]map[string]bool  // map[flow_token]map[cycle_key]bool
   }

   // cycle_key = sync_id + ":" + binding_hash
   ```
   - Per-flow history tracking
   - Thread-safe with mutex
   - Cycle key combines sync_id and binding_hash

2. **WouldCycle() method detects potential cycles**
   ```go
   func (c *CycleDetector) WouldCycle(flowToken, syncID, bindingHash string) bool {
       c.mu.Lock()
       defer c.mu.Unlock()

       if c.history[flowToken] == nil {
           return false
       }

       cycleKey := syncID + ":" + bindingHash
       return c.history[flowToken][cycleKey]
   }
   ```
   - Returns true if (syncID, bindingHash) already fired in this flow
   - Returns false for first occurrence
   - Thread-safe

3. **Record() method marks sync as fired**
   ```go
   func (c *CycleDetector) Record(flowToken, syncID, bindingHash string) {
       c.mu.Lock()
       defer c.mu.Unlock()

       if c.history[flowToken] == nil {
           c.history[flowToken] = make(map[string]bool)
       }

       cycleKey := syncID + ":" + bindingHash
       c.history[flowToken][cycleKey] = true
   }
   ```
   - Marks (syncID, bindingHash) as fired for this flow
   - Initializes flow history if needed
   - Thread-safe

4. **Clear() method resets flow history**
   ```go
   func (c *CycleDetector) Clear(flowToken string) {
       c.mu.Lock()
       defer c.mu.Unlock()

       delete(c.history, flowToken)
   }
   ```
   - Removes all history for a flow token
   - Used when flow completes or for testing
   - Thread-safe

5. **Engine integration in `internal/engine/engine.go`**
   ```go
   type Engine struct {
       // ... existing fields
       cycleDetector *CycleDetector
   }

   func New(store *store.Store, specs []ir.ConceptSpec, syncs []ir.SyncRule) *Engine {
       return &Engine{
           // ... existing initialization
           cycleDetector: NewCycleDetector(),
       }
   }
   ```
   - Engine owns a CycleDetector instance
   - Initialized in New() constructor

6. **Cycle check in sync firing logic (Story 4.5 integration)**
   ```go
   func (e *Engine) executeThen(
       ctx context.Context,
       then ir.ThenClause,
       bindings []ir.IRObject,
       flowToken string,
       completion ir.Completion,
       sync ir.SyncRule,
   ) error {
       for _, binding := range bindings {
           bindingHash := ir.BindingHash(binding)

           // Check idempotency (Story 5.1)
           if e.store.HasFiring(ctx, completion.ID, sync.ID, bindingHash) {
               continue
           }

           // Check cycle (NEW - Story 5.3)
           if e.cycleDetector.WouldCycle(flowToken, sync.ID, bindingHash) {
               // Log cycle detection with full trace
               slog.Warn("cycle detected - skipping firing",
                   "flow_token", flowToken,
                   "sync_id", sync.ID,
                   "binding_hash", bindingHash,
                   "completion_id", completion.ID,
               )
               continue  // Skip firing, not an error
           }

           // Record firing in cycle detector
           e.cycleDetector.Record(flowToken, sync.ID, bindingHash)

           // Generate invocation and write to store (existing logic)
           // ...
       }
       return nil
   }
   ```
   - Check WouldCycle() before firing
   - Record() immediately after cycle check passes
   - Log cycle detection with warning level
   - Skip firing (not fatal error)

7. **RuntimeError type for cycle detection failures**
   ```go
   // internal/engine/errors.go

   type RuntimeError struct {
       FlowToken string
       Message   string
       Context   map[string]any
   }

   func (e RuntimeError) Error() string {
       return fmt.Sprintf("runtime error in flow %s: %s", e.FlowToken, e.Message)
   }

   func NewCycleDetectedError(flowToken, syncID, bindingHash string) *RuntimeError {
       return &RuntimeError{
           FlowToken: flowToken,
           Message:   "cycle detected",
           Context: map[string]any{
               "sync_id":      syncID,
               "binding_hash": bindingHash,
               "error_type":   "cycle",
           },
       }
   }
   ```
   - RuntimeError type for non-fatal engine errors
   - Context map for debugging info
   - NewCycleDetectedError constructor

8. **Comprehensive tests in `internal/engine/cycle_test.go`**
   - Self-referential sync detected: A fires A
   - Transitive cycle detected: A fires B fires A
   - Non-cyclic deep chains allowed: A → B → C → D (no cycle)
   - Multi-binding: same sync, different bindings (no cycle)
   - Different flows: same sync+binding in different flows (no cycle)
   - Clear() resets flow history correctly
   - Thread-safety tests (concurrent access)

## Quick Reference

| Pattern | Key Rule |
|---------|----------|
| **CRITICAL-3** | Cycle detection per flow prevents infinite loops |
| **FR-4.2** | Provenance edges help visualize cycle chains |
| **NF-3.1** | Cycle detection is per-flow, not global |
| **CP-1** | Binding-level granularity (sync_id + binding_hash) |

## Tasks / Subtasks

- [ ] Task 1: Create CycleDetector struct (AC: #1)
  - [ ] 1.1 Create `internal/engine/cycle.go`
  - [ ] 1.2 Define CycleDetector struct with mutex and history map
  - [ ] 1.3 Add package documentation for cycle detection
  - [ ] 1.4 Define NewCycleDetector() constructor

- [ ] Task 2: Implement WouldCycle() method (AC: #2)
  - [ ] 2.1 Implement WouldCycle(flowToken, syncID, bindingHash) bool
  - [ ] 2.2 Build cycle key from syncID + ":" + bindingHash
  - [ ] 2.3 Check history[flowToken][cycleKey] existence
  - [ ] 2.4 Return false if flow not in history
  - [ ] 2.5 Add mutex locking for thread-safety

- [ ] Task 3: Implement Record() method (AC: #3)
  - [ ] 3.1 Implement Record(flowToken, syncID, bindingHash)
  - [ ] 3.2 Initialize history[flowToken] if nil
  - [ ] 3.3 Build cycle key and set history[flowToken][cycleKey] = true
  - [ ] 3.4 Add mutex locking for thread-safety

- [ ] Task 4: Implement Clear() method (AC: #4)
  - [ ] 4.1 Implement Clear(flowToken)
  - [ ] 4.2 Delete history[flowToken] entry
  - [ ] 4.3 Add mutex locking for thread-safety

- [ ] Task 5: Add RuntimeError type (AC: #7)
  - [ ] 5.1 Create `internal/engine/errors.go`
  - [ ] 5.2 Define RuntimeError struct with FlowToken, Message, Context
  - [ ] 5.3 Implement Error() method
  - [ ] 5.4 Add NewCycleDetectedError constructor

- [ ] Task 6: Integrate CycleDetector into Engine (AC: #5, #6)
  - [ ] 6.1 Add cycleDetector field to Engine struct
  - [ ] 6.2 Initialize cycleDetector in New() constructor
  - [ ] 6.3 Add WouldCycle() check in executeThen() (Story 4.5 integration)
  - [ ] 6.4 Add Record() call after cycle check passes
  - [ ] 6.5 Add cycle detection warning log with slog

- [ ] Task 7: Write comprehensive tests (AC: #8)
  - [ ] 7.1 Create `internal/engine/cycle_test.go`
  - [ ] 7.2 Test self-referential cycle (A → A)
  - [ ] 7.3 Test transitive cycle (A → B → A)
  - [ ] 7.4 Test non-cyclic chain (A → B → C → D)
  - [ ] 7.5 Test multi-binding (same sync, different bindings)
  - [ ] 7.6 Test different flows (same sync+binding in different flows)
  - [ ] 7.7 Test Clear() resets flow history
  - [ ] 7.8 Test thread-safety (concurrent WouldCycle/Record)

- [ ] Task 8: Add integration tests with provenance (AC: #8)
  - [ ] 8.1 Test cycle detection with full engine + store
  - [ ] 8.2 Verify provenance edges show cycle chain
  - [ ] 8.3 Test cycle broken (firing skipped, not error)
  - [ ] 8.4 Test flow completes successfully after cycle break

## Dev Notes

### Critical Implementation Details

**Cycle Detection vs Idempotency**

These are DIFFERENT mechanisms with different purposes:

| Mechanism | Purpose | Granularity | Storage |
|-----------|---------|-------------|---------|
| **Idempotency** (Story 5.1) | Prevent duplicate firings on replay | (completion_id, sync_id, binding_hash) | Persistent (SQLite) |
| **Cycle Detection** (Story 5.3) | Prevent infinite loops during execution | (flow_token, sync_id, binding_hash) | In-memory (per run) |

**Why Both Are Needed:**
- Idempotency: "Have we already fired this exact (completion, sync, binding) triple?" → Prevents duplicates on crash/replay
- Cycle Detection: "Have we already fired this (sync, binding) in this flow?" → Prevents A → B → A loops

**Example:**
```
Flow 1:
  Order.Create completes
  → sync-reserve-stock fires (binding: {product: "widget"})
  → Inventory.ReserveStock completes
  → sync-notify-reserved fires (binding: {product: "widget"})
  → Notification.Send completes
  ✓ No cycle (linear chain)

Flow 1 (with cycle):
  Order.Create completes
  → sync-reserve-stock fires (binding: {product: "widget"})
  → Inventory.ReserveStock completes
  → sync-create-order fires (binding: {product: "widget"})  ← BUG: triggers Order.Create again!
  → Order.Create completes
  → sync-reserve-stock would fire again... ❌ CYCLE DETECTED
  → Cycle detector says: "Already fired sync-reserve-stock:{product:widget} in flow-123"
  → Firing SKIPPED (logged warning)
  ✓ Flow terminates gracefully
```

**Cycle Detection Architecture**

```
┌────────────────────────────────────────────────┐
│ CycleDetector (in-memory)                      │
│                                                │
│ history: map[flow_token]map[cycle_key]bool    │
│                                                │
│ cycle_key = sync_id + ":" + binding_hash      │
│                                                │
│ Example:                                       │
│ history = {                                    │
│   "flow-123": {                                │
│     "sync-reserve:hash-widget": true,          │
│     "sync-notify:hash-widget": true,           │
│   },                                           │
│   "flow-456": {                                │
│     "sync-reserve:hash-gadget": true,          │
│   },                                           │
│ }                                              │
└────────────────────────────────────────────────┘

When evaluating sync firing:
1. Check idempotency (store): HasFiring(completion_id, sync_id, binding_hash)
2. Check cycle (detector): WouldCycle(flow_token, sync_id, binding_hash)
3. If both pass: Record(flow_token, sync_id, binding_hash) and fire
4. If either fails: Skip firing (log warning)
```

**Difference from Max-Steps Quota (Story 5.4)**

| Feature | Cycle Detection | Max-Steps Quota |
|---------|-----------------|-----------------|
| **What** | Detects same (sync, binding) firing twice in flow | Counts total steps per flow |
| **When** | Before each firing | After each firing |
| **Action** | Skip firing, log warning | Terminate flow, return error |
| **Scope** | Per (sync_id, binding_hash) | Per flow (all firings) |

**Cycle detection is LOCAL (per sync+binding), max-steps is GLOBAL (per flow).**

Example where both matter:
```
Flow with non-cyclic explosion:
  A completes → sync-1 fires 10 bindings → 10 B invocations
  Each B completes → sync-2 fires 10 bindings → 100 C invocations
  Each C completes → sync-3 fires 10 bindings → 1000 D invocations

  No cycle (A ≠ B ≠ C ≠ D), but 1110 total firings!

  Cycle detector: ✓ PASS (no sync+binding repeats)
  Max-steps quota: ❌ FAIL (exceeds 1000 steps)
```

**Provenance Edges and Cycle Visualization**

Provenance edges (Story 2.6) help debug cycles:

```go
// After cycle detection skips a firing, query provenance to see the chain

// Example: Order.Create → Inventory.ReserveStock → Order.Create (cycle!)

// 1. Cycle detected at second Order.Create invocation
cycleInvID := "inv-order-123-second"

// 2. Trace backward via provenance
prov1, _ := store.ReadProvenance(ctx, cycleInvID)
// prov1[0].CompletionID == "comp-reserve-abc"
// prov1[0].SyncID == "sync-create-order" (BUG: wrong sync rule!)

// 3. What invocation produced that completion?
reserveComp, _ := store.ReadCompletion(ctx, prov1[0].CompletionID)
reserveInvID := reserveComp.InvocationID

// 4. What caused Inventory.ReserveStock?
prov2, _ := store.ReadProvenance(ctx, reserveInvID)
// prov2[0].CompletionID == "comp-order-123-first"
// prov2[0].SyncID == "sync-reserve-stock" (correct)

// CYCLE CHAIN:
// Order.Create (first) → [sync-reserve-stock] → Inventory.ReserveStock
//   → [sync-create-order] → Order.Create (second) ← CYCLE!
//
// Fix: Remove buggy sync-create-order rule
```

### Function Signatures

**CycleDetector Implementation**
```go
// internal/engine/cycle.go

package engine

import "sync"

// CycleDetector tracks sync firings per flow to prevent infinite loops.
//
// Cycles occur when the same (sync_id, binding_hash) would fire multiple
// times in a single flow. This happens with self-referential or mutually
// recursive sync rules.
//
// Example cycle:
//   Order.Create completes → sync-reserve fires → Inventory.ReserveStock completes
//   → sync-create-order fires → Order.Create completes (again!)
//   → sync-reserve would fire again... ← CYCLE DETECTED
//
// The detector maintains per-flow history of (sync_id, binding_hash) pairs
// that have already fired. Before each firing, WouldCycle() checks if the
// pair has been seen before in this flow.
//
// CRITICAL DISTINCTION from Idempotency (Story 5.1):
//   - Idempotency: "Have we fired this (completion, sync, binding) triple?" (persistent)
//   - Cycle Detection: "Have we fired this (sync, binding) in this flow?" (in-memory)
//
// Both checks are required:
//   - Idempotency prevents duplicate firings on crash/replay
//   - Cycle detection prevents infinite loops during execution
type CycleDetector struct {
    mu      sync.Mutex
    history map[string]map[string]bool  // map[flow_token]map[cycle_key]bool
}

// NewCycleDetector creates a new cycle detector.
func NewCycleDetector() *CycleDetector {
    return &CycleDetector{
        history: make(map[string]map[string]bool),
    }
}

// WouldCycle checks if firing this (sync_id, binding_hash) would create a cycle.
//
// Returns true if the same (sync_id, binding_hash) has already fired in this flow.
// Returns false for the first occurrence or if flow has no history.
//
// Thread-safe: Can be called concurrently.
func (c *CycleDetector) WouldCycle(flowToken, syncID, bindingHash string) bool {
    c.mu.Lock()
    defer c.mu.Unlock()

    // No history for this flow = first time seeing it
    if c.history[flowToken] == nil {
        return false
    }

    cycleKey := syncID + ":" + bindingHash
    return c.history[flowToken][cycleKey]
}

// Record marks that this (sync_id, binding_hash) has fired in this flow.
//
// This should be called immediately after WouldCycle() returns false,
// before actually firing the sync rule.
//
// Thread-safe: Can be called concurrently.
func (c *CycleDetector) Record(flowToken, syncID, bindingHash string) {
    c.mu.Lock()
    defer c.mu.Unlock()

    // Initialize flow history if needed
    if c.history[flowToken] == nil {
        c.history[flowToken] = make(map[string]bool)
    }

    cycleKey := syncID + ":" + bindingHash
    c.history[flowToken][cycleKey] = true
}

// Clear removes all history for a flow token.
//
// Used when:
//   - Flow completes successfully (cleanup)
//   - Flow terminates with error (cleanup)
//   - Testing (reset state between tests)
//
// Thread-safe: Can be called concurrently.
func (c *CycleDetector) Clear(flowToken string) {
    c.mu.Lock()
    defer c.mu.Unlock()

    delete(c.history, flowToken)
}

// HistorySize returns the number of flows with tracked history.
//
// Used for testing and introspection.
// Thread-safe: Can be called concurrently.
func (c *CycleDetector) HistorySize() int {
    c.mu.Lock()
    defer c.mu.Unlock()

    return len(c.history)
}

// FlowHistorySize returns the number of (sync, binding) pairs tracked for a flow.
//
// Used for testing and introspection.
// Thread-safe: Can be called concurrently.
func (c *CycleDetector) FlowHistorySize(flowToken string) int {
    c.mu.Lock()
    defer c.mu.Unlock()

    if c.history[flowToken] == nil {
        return 0
    }
    return len(c.history[flowToken])
}
```

**RuntimeError Type**
```go
// internal/engine/errors.go

package engine

import "fmt"

// RuntimeError represents a non-fatal error during engine execution.
//
// Unlike fatal errors (database failures, invalid IR), runtime errors
// are expected in normal operation:
//   - Cycle detection (self-triggering syncs)
//   - Quota exceeded (too many steps)
//   - Action execution failures (business logic errors)
//
// Runtime errors are logged but don't stop the engine.
type RuntimeError struct {
    FlowToken string
    Message   string
    Context   map[string]any
}

// Error implements the error interface.
func (e RuntimeError) Error() string {
    return fmt.Sprintf("runtime error in flow %s: %s", e.FlowToken, e.Message)
}

// NewCycleDetectedError creates a cycle detection error.
//
// This is a runtime error (not fatal). The cycle is logged and the
// firing is skipped, but the flow continues.
func NewCycleDetectedError(flowToken, syncID, bindingHash string) *RuntimeError {
    return &RuntimeError{
        FlowToken: flowToken,
        Message:   "cycle detected",
        Context: map[string]any{
            "sync_id":      syncID,
            "binding_hash": bindingHash,
            "error_type":   "cycle",
        },
    }
}
```

**Engine Integration**
```go
// internal/engine/engine.go (modified)

type Engine struct {
    store         *store.Store
    compiler      queryir.Compiler
    flowGen       FlowTokenGenerator
    clock         *Clock
    specs         []ir.ConceptSpec
    syncs         []ir.SyncRule
    queue         *eventQueue
    cycleDetector *CycleDetector  // NEW: Story 5.3
}

func New(store *store.Store, specs []ir.ConceptSpec, syncs []ir.SyncRule) *Engine {
    return &Engine{
        store:         store,
        clock:         NewClock(),
        specs:         specs,
        syncs:         syncs,
        queue:         newEventQueue(),
        cycleDetector: NewCycleDetector(),  // NEW: Story 5.3
    }
}

// executeThen is modified from Story 4.5 to add cycle detection
func (e *Engine) executeThen(
    ctx context.Context,
    then ir.ThenClause,
    bindings []ir.IRObject,
    flowToken string,
    completion ir.Completion,
    sync ir.SyncRule,
) error {
    for _, binding := range bindings {
        bindingHash := ir.BindingHash(binding)

        // Check idempotency (Story 5.1)
        if e.store.HasFiring(ctx, completion.ID, sync.ID, bindingHash) {
            slog.Debug("skipping duplicate firing (idempotency)",
                "completion_id", completion.ID,
                "sync_id", sync.ID,
                "binding_hash", bindingHash,
            )
            continue
        }

        // Check cycle (NEW - Story 5.3)
        if e.cycleDetector.WouldCycle(flowToken, sync.ID, bindingHash) {
            // Log cycle detection with full trace
            slog.Warn("cycle detected - skipping firing",
                "flow_token", flowToken,
                "sync_id", sync.ID,
                "binding_hash", bindingHash,
                "completion_id", completion.ID,
                "event", "cycle",
            )
            // Skip firing, not an error
            continue
        }

        // Record firing in cycle detector (before actual firing)
        e.cycleDetector.Record(flowToken, sync.ID, bindingHash)

        // Generate invocation (existing logic from Story 4.5)
        args := e.resolveArgs(then.Args, binding)

        // CRITICAL: Get seq values ONCE for each record
        // Each record gets its own seq, but clock.Next() must be called exactly once per record
        invSeq := e.clock.Next()

        // Compute ID using Story 1.5 signature (SecurityContext excluded per CP-6)
        invID, err := ir.InvocationID(flowToken, then.Action, args, invSeq)
        if err != nil {
            return fmt.Errorf("compute invocation ID: %w", err)
        }

        inv := ir.Invocation{
            ID:              invID,
            FlowToken:       flowToken,
            ActionURI:       then.Action,
            Args:            args,
            Seq:             invSeq,  // Same seq used in ID computation
            SecurityContext: e.currentSecurityContext(),
            SpecHash:        e.currentSpecHash(),
            EngineVersion:   e.engineVersion,
            IRVersion:       ir.Version,
        }

        // Record firing and invocation (existing logic from Story 4.5)
        firingSeq := e.clock.Next()  // Separate seq for firing record
        firing := ir.SyncFiring{
            CompletionID: completion.ID,
            SyncID:       sync.ID,
            BindingHash:  bindingHash,
            Seq:          firingSeq,
        }

        if err := e.store.WriteSyncFiring(ctx, firing); err != nil {
            return fmt.Errorf("write sync firing: %w", err)
        }

        if err := e.store.WriteInvocation(ctx, inv); err != nil {
            return fmt.Errorf("write invocation: %w", err)
        }

        if err := e.store.WriteProvenanceEdge(ctx, ir.ProvenanceEdge{
            SyncFiringID: firing.ID,
            InvocationID: inv.ID,
        }); err != nil {
            return fmt.Errorf("write provenance edge: %w", err)
        }

        // Enqueue for execution
        e.queue.Enqueue(Event{Type: EventTypeInvocation, Data: &inv})
    }
    return nil
}
```

### Test Examples

**Test Self-Referential Cycle (A → A)**
```go
// internal/engine/cycle_test.go

package engine

import (
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestCycleDetector_SelfReferential(t *testing.T) {
    detector := NewCycleDetector()
    flowToken := "flow-123"
    syncID := "sync-self-trigger"
    bindingHash := "hash-abc"

    // First firing: no cycle
    assert.False(t, detector.WouldCycle(flowToken, syncID, bindingHash))
    detector.Record(flowToken, syncID, bindingHash)

    // Second firing: cycle detected
    assert.True(t, detector.WouldCycle(flowToken, syncID, bindingHash),
        "self-referential sync should be detected as cycle")
}
```

**Test Transitive Cycle (A → B → A)**
```go
func TestCycleDetector_TransitiveCycle(t *testing.T) {
    detector := NewCycleDetector()
    flowToken := "flow-456"

    // Sync A fires (binding: {product: "widget"})
    syncA := "sync-a"
    bindingWidget := "hash-widget"
    assert.False(t, detector.WouldCycle(flowToken, syncA, bindingWidget))
    detector.Record(flowToken, syncA, bindingWidget)

    // Sync B fires (binding: {product: "widget"})
    syncB := "sync-b"
    assert.False(t, detector.WouldCycle(flowToken, syncB, bindingWidget))
    detector.Record(flowToken, syncB, bindingWidget)

    // Sync A tries to fire again (binding: {product: "widget"}) - CYCLE!
    assert.True(t, detector.WouldCycle(flowToken, syncA, bindingWidget),
        "transitive cycle A→B→A should be detected")
}
```

**Test Non-Cyclic Deep Chain (A → B → C → D)**
```go
func TestCycleDetector_NonCyclicChain(t *testing.T) {
    detector := NewCycleDetector()
    flowToken := "flow-789"
    binding := "hash-xyz"

    // Chain: A → B → C → D (no cycles)
    syncs := []string{"sync-a", "sync-b", "sync-c", "sync-d"}

    for _, syncID := range syncs {
        assert.False(t, detector.WouldCycle(flowToken, syncID, binding),
            "sync %s should not be a cycle", syncID)
        detector.Record(flowToken, syncID, binding)
    }

    // Verify all 4 syncs recorded
    assert.Equal(t, 4, detector.FlowHistorySize(flowToken))
}
```

**Test Multi-Binding (Same Sync, Different Bindings)**
```go
func TestCycleDetector_MultiBinding(t *testing.T) {
    detector := NewCycleDetector()
    flowToken := "flow-multi"
    syncID := "sync-refill"

    // Same sync fires with 3 different bindings - NOT a cycle
    bindings := []string{"hash-widget", "hash-gadget", "hash-doohickey"}

    for _, bindingHash := range bindings {
        assert.False(t, detector.WouldCycle(flowToken, syncID, bindingHash),
            "same sync with different binding should not be a cycle")
        detector.Record(flowToken, syncID, bindingHash)
    }

    // Verify 3 separate entries (sync-refill:hash-widget, sync-refill:hash-gadget, etc.)
    assert.Equal(t, 3, detector.FlowHistorySize(flowToken))

    // Trying to fire sync-refill:hash-widget again IS a cycle
    assert.True(t, detector.WouldCycle(flowToken, syncID, "hash-widget"),
        "same sync+binding twice is a cycle")
}
```

**Test Different Flows (Same Sync+Binding in Different Flows)**
```go
func TestCycleDetector_DifferentFlows(t *testing.T) {
    detector := NewCycleDetector()
    syncID := "sync-reserve"
    bindingHash := "hash-widget"

    // Flow 1: sync-reserve fires with hash-widget
    flow1 := "flow-111"
    assert.False(t, detector.WouldCycle(flow1, syncID, bindingHash))
    detector.Record(flow1, syncID, bindingHash)

    // Flow 2: sync-reserve fires with hash-widget (different flow!)
    flow2 := "flow-222"
    assert.False(t, detector.WouldCycle(flow2, syncID, bindingHash),
        "same sync+binding in different flow should not be a cycle")
    detector.Record(flow2, syncID, bindingHash)

    // Verify both flows have independent history
    assert.Equal(t, 2, detector.HistorySize())
    assert.Equal(t, 1, detector.FlowHistorySize(flow1))
    assert.Equal(t, 1, detector.FlowHistorySize(flow2))
}
```

**Test Clear() Resets Flow History**
```go
func TestCycleDetector_Clear(t *testing.T) {
    detector := NewCycleDetector()
    flowToken := "flow-clear"
    syncID := "sync-test"
    bindingHash := "hash-test"

    // Record firing
    detector.Record(flowToken, syncID, bindingHash)
    assert.Equal(t, 1, detector.FlowHistorySize(flowToken))
    assert.True(t, detector.WouldCycle(flowToken, syncID, bindingHash))

    // Clear flow history
    detector.Clear(flowToken)

    // Verify cleared
    assert.Equal(t, 0, detector.FlowHistorySize(flowToken))
    assert.False(t, detector.WouldCycle(flowToken, syncID, bindingHash),
        "after Clear(), same sync+binding should not be a cycle")

    // Can record again after clear
    detector.Record(flowToken, syncID, bindingHash)
    assert.Equal(t, 1, detector.FlowHistorySize(flowToken))
}
```

**Test Thread-Safety (Concurrent Access)**
```go
func TestCycleDetector_ThreadSafety(t *testing.T) {
    detector := NewCycleDetector()
    flowToken := "flow-concurrent"
    const goroutines = 100
    const firesPerGoroutine = 10

    var wg sync.WaitGroup

    // Launch goroutines that concurrently check and record
    for i := 0; i < goroutines; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            for j := 0; j < firesPerGoroutine; j++ {
                syncID := fmt.Sprintf("sync-%d", id)
                bindingHash := fmt.Sprintf("hash-%d", j)

                // Each goroutine checks and records independently
                wouldCycle := detector.WouldCycle(flowToken, syncID, bindingHash)
                if !wouldCycle {
                    detector.Record(flowToken, syncID, bindingHash)
                }
            }
        }(i)
    }

    wg.Wait()

    // Verify all entries recorded (no race conditions)
    expectedEntries := goroutines * firesPerGoroutine
    assert.Equal(t, expectedEntries, detector.FlowHistorySize(flowToken))
}
```

**Integration Test with Engine and Store**
```go
// internal/engine/engine_test.go (add to existing file)

func TestEngine_CycleDetection_Integration(t *testing.T) {
    ctx := context.Background()
    dir := t.TempDir()
    st, err := store.Open(dir + "/test.db")
    require.NoError(t, err)
    defer st.Close()

    // Define sync rule that creates a cycle
    // Order.Create → sync-reserve → Inventory.ReserveStock
    //              → sync-create-order → Order.Create (CYCLE!)
    specs := []ir.ConceptSpec{
        {
            Name: "Order",
            Actions: []ir.ActionSig{
                {Name: "Create", Outputs: []ir.OutputCase{{Case: "Success"}}},
            },
        },
        {
            Name: "Inventory",
            Actions: []ir.ActionSig{
                {Name: "ReserveStock", Outputs: []ir.OutputCase{{Case: "Success"}}},
            },
        },
    }

    syncs := []ir.SyncRule{
        {
            ID: "sync-reserve",
            When: ir.WhenClause{
                Action:     "Order.Create",
                Event:      "completed",
                OutputCase: ptr("Success"),
            },
            Then: ir.ThenClause{Action: "Inventory.ReserveStock"},
        },
        {
            ID: "sync-create-order",  // BUG: creates cycle!
            When: ir.WhenClause{
                Action:     "Inventory.ReserveStock",
                Event:      "completed",
                OutputCase: ptr("Success"),
            },
            Then: ir.ThenClause{Action: "Order.Create"},
        },
    }

    engine := New(st, specs, syncs)
    flowToken := "flow-cycle-test"

    // 1. User invokes Order.Create
    orderInv1 := testInvocation("Order.Create", flowToken, 1)
    require.NoError(t, st.WriteInvocation(ctx, orderInv1))
    engine.Enqueue(Event{Type: EventTypeInvocation, Data: &orderInv1})

    // 2. Order.Create completes
    orderComp1 := testCompletion(orderInv1.ID, "Success", 2)
    require.NoError(t, st.WriteCompletion(ctx, orderComp1))
    engine.Enqueue(Event{Type: EventTypeCompletion, Data: &orderComp1})

    // Process events (this triggers sync-reserve)
    // ... engine processing logic ...

    // 3. Inventory.ReserveStock invoked by sync-reserve
    reserveInv := testInvocation("Inventory.ReserveStock", flowToken, 3)
    require.NoError(t, st.WriteInvocation(ctx, reserveInv))

    // 4. Inventory.ReserveStock completes
    reserveComp := testCompletion(reserveInv.ID, "Success", 4)
    require.NoError(t, st.WriteCompletion(ctx, reserveComp))

    // 5. This would trigger sync-create-order → Order.Create again
    //    But cycle detector should PREVENT this!

    // Verify cycle detector history
    assert.True(t, engine.cycleDetector.WouldCycle(flowToken, "sync-reserve", "hash-test"),
        "sync-reserve already fired in this flow")

    // Verify no second Order.Create invocation created
    invocations, err := st.ReadFlowInvocations(ctx, flowToken)
    require.NoError(t, err)

    orderCreateCount := 0
    for _, inv := range invocations {
        if inv.ActionURI == "Order.Create" {
            orderCreateCount++
        }
    }
    assert.Equal(t, 1, orderCreateCount,
        "cycle detector should prevent second Order.Create invocation")

    // Verify provenance chain shows where cycle would have occurred
    provenance, err := st.ReadProvenance(ctx, reserveInv.ID)
    require.NoError(t, err)
    require.Len(t, provenance, 1)
    assert.Equal(t, "sync-reserve", provenance[0].SyncID)
    assert.Equal(t, orderComp1.ID, provenance[0].CompletionID)
}
```

### File List

Files to create:

1. `internal/engine/cycle.go` - CycleDetector implementation
2. `internal/engine/errors.go` - RuntimeError type
3. `internal/engine/cycle_test.go` - Comprehensive cycle detection tests

Files to modify:

1. `internal/engine/engine.go` - Add cycleDetector field, integrate into executeThen()
2. `internal/engine/engine_test.go` - Add integration tests

Files that must exist (from previous stories):

1. `internal/ir/types.go` - SyncFiring, ProvenanceEdge types
2. `internal/ir/hash.go` - BindingHash function
3. `internal/store/store.go` - HasFiring method (Story 5.1)
4. `internal/engine/engine.go` - Engine struct, executeThen method (Story 4.5)

### Relationship to Other Stories

**Dependencies:**
- Story 1.1 (Project Initialization) - Required for ir.* types
- Story 1.5 (Content-Addressed Identity) - Required for BindingHash
- Story 2.5 (Sync Firings Table) - Required for sync firing storage
- Story 2.6 (Provenance Edges) - Required for cycle chain visualization
- Story 3.1 (Single-Writer Event Loop) - Required for Engine struct
- Story 4.5 (Then-Clause Invocation Generation) - executeThen() modified here
- Story 5.1 (Binding-Level Idempotency) - Idempotency check comes before cycle check

**Enables:**
- Story 5.4 (Max-Steps Quota Enforcement) - Complementary termination mechanism
- Story 5.5 (Compile-Time Cycle Analysis) - Static analysis of sync rules
- Story 7.6 (Test Command) - Cycle detection tested via conformance harness

**Note:** Cycle detection is a RUNTIME mechanism. Story 5.5 adds COMPILE-TIME static analysis to warn about potential cycles before execution.

### Story Completion Checklist

- [ ] `internal/engine/cycle.go` created with CycleDetector struct
- [ ] NewCycleDetector() constructor implemented
- [ ] WouldCycle(flowToken, syncID, bindingHash) method implemented
- [ ] Record(flowToken, syncID, bindingHash) method implemented
- [ ] Clear(flowToken) method implemented
- [ ] HistorySize() and FlowHistorySize() introspection methods implemented
- [ ] `internal/engine/errors.go` created with RuntimeError type
- [ ] NewCycleDetectedError() constructor implemented
- [ ] Engine struct has cycleDetector field
- [ ] Engine.New() initializes cycleDetector
- [ ] executeThen() checks WouldCycle() before firing
- [ ] executeThen() calls Record() after cycle check passes
- [ ] Cycle detection logs warning with slog
- [ ] Test: self-referential cycle (A → A)
- [ ] Test: transitive cycle (A → B → A)
- [ ] Test: non-cyclic chain (A → B → C → D)
- [ ] Test: multi-binding (same sync, different bindings)
- [ ] Test: different flows (same sync+binding in different flows)
- [ ] Test: Clear() resets flow history
- [ ] Test: thread-safety (concurrent WouldCycle/Record)
- [ ] Integration test: cycle detection with engine + store
- [ ] Integration test: provenance edges show cycle chain
- [ ] Integration test: cycle broken (firing skipped, not error)
- [ ] `go vet ./internal/engine/...` passes
- [ ] `go test ./internal/engine/...` passes
- [ ] All tests use goleak to verify no goroutine leaks

### References

- [Source: docs/epics.md#Story 5.3] - Story definition and acceptance criteria
- [Source: docs/architecture.md#CRITICAL-3] - Sync engine termination semantics
- [Source: docs/architecture.md#CP-1] - Binding-level granularity
- [Source: docs/prd.md#FR-4.2] - Provenance edges for idempotency
- [Source: docs/prd.md#NF-3.1] - Cycle detection per flow
- [Source: Story 2.6] - Provenance edges for cycle visualization
- [Source: Story 5.1] - Binding-level idempotency (prerequisite)
- [Source: Story 4.5] - Then-clause invocation generation (modified here)

## Dev Agent Record

### Agent Model Used

Claude Sonnet 4.5 (claude-sonnet-4-5-20250929)

### Validation History

- Initial creation: comprehensive story generation from architecture + epics

### Completion Notes

- Cycle detection is IN-MEMORY, per-flow tracking of (sync_id, binding_hash) pairs
- Different from idempotency (persistent, per-completion) and max-steps (global, per-flow)
- Cycle key format: `sync_id + ":" + binding_hash`
- WouldCycle() checks history BEFORE firing
- Record() marks firing AFTER cycle check passes
- Firing is SKIPPED (not error) when cycle detected
- Provenance edges (Story 2.6) help debug cycle chains
- RuntimeError type for non-fatal engine errors
- Thread-safe with mutex for concurrent access
- Clear() method for flow cleanup and testing
- Multi-binding example: one completion triggers multiple invocations (same sync, different bindings) - NOT a cycle
- Cross-flow example: same sync+binding in different flows - NOT a cycle
- Integration tests verify cycle broken gracefully (no infinite loops)
- Story implements CRITICAL-3 (termination semantics) and FR-4.2 (provenance for debugging)
