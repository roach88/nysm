# Story 7.10: Demo Scenarios and Golden Traces

Status: done

## Story

As a **developer validating the demo**,
I want **test scenarios with golden traces**,
So that **the demo is verified and serves as test fixtures**.

## Acceptance Criteria

1. **Two demo scenarios in `testdata/scenarios/`**
   - `cart_checkout_success.yaml` - Successful checkout flow from PRD Appendix A
   - `cart_checkout_insufficient_stock.yaml` - Error handling flow from PRD Appendix A

2. **cart_checkout_success scenario validates:**
   - Cart addItem → checkout → Inventory reserve chain
   - Provenance edge: checkout-completion → reserve-invocation
   - Final state: cart empty (or cleared), inventory reduced by reserved quantity
   - Sync rule fires correctly with correct bindings

3. **cart_checkout_insufficient_stock scenario validates:**
   - Cart addItem → checkout → Inventory reserve fails
   - Error case `InsufficientStock` properly matched and propagated
   - No partial state changes (inventory unchanged, cart preserves items)
   - Error-matching sync rules work correctly

4. **Golden traces in `testdata/golden/`**
   - `cart_checkout_success.golden` - Complete trace with all invocations, completions, sync firings, provenance edges
   - `cart_checkout_insufficient_stock.golden` - Complete error flow trace
   - Both use canonical JSON (RFC 8785) for stability

5. **All demo scenarios pass when run:**
   ```bash
   go test ./internal/harness -run TestDemoScenarios
   ```
   - No failures or assertion errors
   - Golden files match exactly

6. **End-to-end MVP validation:**
   - Demo specs from Story 7.8 compile correctly
   - Sync rules from Story 7.9 execute as expected
   - Complete invocation/completion chain visible in trace
   - Provenance edges show causality
   - Deterministic replay produces identical results

## Quick Reference

| Pattern | Key Rule |
|---------|----------|
| **FR-6.4** | Generate golden trace snapshots |
| **CP-2** | Logical clocks for deterministic ordering |
| **CP-3** | RFC 8785 canonical JSON |
| **PRD Appendix A** | Canonical demo scenarios define expected behavior |

## Tasks / Subtasks

- [ ] Task 1: Create cart_checkout_success scenario (AC: #1, #2)
  - [ ] 1.1 Create `testdata/scenarios/cart_checkout_success.yaml`
  - [ ] 1.2 Define specs list (cart, inventory, web, cart-inventory sync)
  - [ ] 1.3 Add setup section to initialize inventory stock
  - [ ] 1.4 Define flow: addItem → checkout
  - [ ] 1.5 Add assertions: trace_contains, final_state, trace_order, provenance

- [ ] Task 2: Create cart_checkout_insufficient_stock scenario (AC: #1, #3)
  - [ ] 2.1 Create `testdata/scenarios/cart_checkout_insufficient_stock.yaml`
  - [ ] 2.2 Define specs list (same as success scenario)
  - [ ] 2.3 Add setup with insufficient stock
  - [ ] 2.4 Define flow: addItem → checkout (expecting error)
  - [ ] 2.5 Add assertions: error case matching, no state changes

- [ ] Task 3: Generate golden traces (AC: #4)
  - [ ] 3.1 Run scenarios with deterministic test setup
  - [ ] 3.2 Generate `cart_checkout_success.golden` using -update flag
  - [ ] 3.3 Generate `cart_checkout_insufficient_stock.golden` using -update flag
  - [ ] 3.4 Verify golden files use canonical JSON format
  - [ ] 3.5 Commit golden files to git

- [ ] Task 4: Write test harness integration (AC: #5)
  - [ ] 4.1 Create `internal/harness/demo_test.go`
  - [ ] 4.2 Implement TestDemoScenarios test function
  - [ ] 4.3 Load both demo scenarios
  - [ ] 4.4 Run with golden comparison
  - [ ] 4.5 Verify all assertions pass

- [ ] Task 5: End-to-end validation (AC: #6)
  - [ ] 5.1 Verify demo specs from Story 7.8 compile
  - [ ] 5.2 Verify sync rules from Story 7.9 execute
  - [ ] 5.3 Verify complete trace chain visible
  - [ ] 5.4 Verify provenance edges correct
  - [ ] 5.5 Run deterministic replay and verify identical results

- [ ] Task 6: Documentation and examples (AC: all)
  - [ ] 6.1 Add README in testdata/scenarios/ explaining demo scenarios
  - [ ] 6.2 Document how to run demo tests
  - [ ] 6.3 Document how to update golden files when needed
  - [ ] 6.4 Add comments in scenario files explaining each step

## Dev Notes

### Cart Checkout Success Scenario

```yaml
# testdata/scenarios/cart_checkout_success.yaml
name: cart_checkout_success
description: "Successful checkout triggers inventory reservation (PRD Appendix A)"

# Specs from Stories 7.8 and 7.9
specs:
  - specs/cart.concept.cue
  - specs/inventory.concept.cue
  - specs/web.concept.cue
  - specs/cart-inventory.sync.cue

# Fixed flow token for deterministic golden comparison
flow_token: "01936f8a-1234-7abc-8901-123456789abc"

# Setup: Initialize inventory with stock
setup:
  - action: Inventory.setStock
    args:
      item_id: "widget"
      quantity: 10

# Main flow: Add item and checkout
flow:
  - invoke: Cart.addItem
    args:
      item_id: "widget"
      quantity: 3
    expect:
      case: Success
      result:
        item_id: "widget"
        new_quantity: 3

  - invoke: Cart.checkout
    args: {}
    expect:
      case: Success
      result:
        cart_id: "cart-1"

assertions:
  # Verify sync rule fired and generated reserve invocation
  - type: trace_contains
    action: Inventory.reserve
    args:
      item_id: "widget"
      quantity: 3

  # Verify final inventory state (10 - 3 = 7)
  - type: final_state
    table: inventory
    where:
      item_id: "widget"
    expect:
      quantity: 7

  # Verify action execution order
  - type: trace_order
    actions:
      - Cart.addItem
      - Cart.checkout
      - Inventory.reserve

  # Verify exactly one reserve invocation
  - type: trace_count
    action: Inventory.reserve
    count: 1

  # Verify cart is empty or cleared after checkout
  - type: final_state
    table: cart_items
    where:
      item_id: "widget"
    expect:
      quantity: 0  # Or row doesn't exist

  # Additional validation: reserve completed successfully
  - type: trace_contains
    action: Inventory.reserve
    args:
      output_case: Success
```

### Cart Checkout Insufficient Stock Scenario

```yaml
# testdata/scenarios/cart_checkout_insufficient_stock.yaml
name: cart_checkout_insufficient_stock
description: "Checkout with insufficient stock triggers error handling (PRD Appendix A)"

specs:
  - specs/cart.concept.cue
  - specs/inventory.concept.cue
  - specs/web.concept.cue
  - specs/cart-inventory.sync.cue

# Fixed flow token for deterministic golden comparison
flow_token: "01936f8a-5678-7def-8901-987654321abc"

# Setup: Initialize inventory with insufficient stock
setup:
  - action: Inventory.setStock
    args:
      item_id: "widget"
      quantity: 2

# Main flow: Add more items than available, attempt checkout
flow:
  - invoke: Cart.addItem
    args:
      item_id: "widget"
      quantity: 3
    expect:
      case: Success
      result:
        item_id: "widget"
        new_quantity: 3

  - invoke: Cart.checkout
    args: {}
    expect:
      case: CheckoutFailed
      result:
        reason: "insufficient_stock"

assertions:
  # Verify reserve was attempted
  - type: trace_contains
    action: Inventory.reserve
    args:
      item_id: "widget"
      quantity: 3

  # Verify reserve failed with InsufficientStock error case
  - type: trace_contains
    action: Inventory.reserve
    args:
      output_case: InsufficientStock
      available: 2
      requested: 3

  # Verify inventory unchanged (no partial state)
  - type: final_state
    table: inventory
    where:
      item_id: "widget"
    expect:
      quantity: 2

  # Verify cart still has items (failed checkout preserves cart)
  - type: final_state
    table: cart_items
    where:
      item_id: "widget"
    expect:
      quantity: 3

  # Verify action execution order (including error flow)
  - type: trace_order
    actions:
      - Cart.addItem
      - Cart.checkout
      - Inventory.reserve

  # Verify error propagation sync rule fired
  - type: trace_contains
    action: Cart.checkout
    args:
      output_case: CheckoutFailed
```

### Golden Trace Format

Golden files capture the complete trace including:

1. **All Invocations:**
   - Content-addressed ID
   - Flow token
   - Action URI (e.g., `nysm://demo/Cart/addItem@1.0.0`)
   - Arguments (canonical JSON)
   - Seq (logical clock)
   - Security context

2. **All Completions:**
   - Content-addressed ID
   - Invocation ID reference
   - Output case (Success, InsufficientStock, etc.)
   - Result fields (canonical JSON)
   - Seq (logical clock)

3. **All Sync Firings:**
   - Completion ID reference
   - Sync ID (e.g., "cart-inventory-reserve")
   - Binding hash (for idempotency)
   - Seq (logical clock)

4. **All Provenance Edges:**
   - Sync firing ID
   - Invocation ID (the invocation produced by this firing)

**Example golden file structure:**

```json
{
  "completions": [
    {
      "id": "sha256:abc123...",
      "invocation_id": "sha256:def456...",
      "output_case": "Success",
      "result": {
        "item_id": "widget",
        "new_quantity": 3
      },
      "seq": 2
    },
    {
      "id": "sha256:ghi789...",
      "invocation_id": "sha256:jkl012...",
      "output_case": "Success",
      "result": {
        "cart_id": "cart-1"
      },
      "seq": 4
    },
    {
      "id": "sha256:mno345...",
      "invocation_id": "sha256:pqr678...",
      "output_case": "Success",
      "result": {
        "reservation_id": "res-1"
      },
      "seq": 6
    }
  ],
  "invocations": [
    {
      "action_uri": "nysm://demo/Cart/addItem@1.0.0",
      "args": {
        "item_id": "widget",
        "quantity": 3
      },
      "flow_token": "01936f8a-1234-7abc-8901-123456789abc",
      "id": "sha256:def456...",
      "security_context": {
        "permissions": [],
        "tenant_id": "",
        "user_id": ""
      },
      "seq": 1
    },
    {
      "action_uri": "nysm://demo/Cart/checkout@1.0.0",
      "args": {},
      "flow_token": "01936f8a-1234-7abc-8901-123456789abc",
      "id": "sha256:jkl012...",
      "security_context": {
        "permissions": [],
        "tenant_id": "",
        "user_id": ""
      },
      "seq": 3
    },
    {
      "action_uri": "nysm://demo/Inventory/reserve@1.0.0",
      "args": {
        "item_id": "widget",
        "quantity": 3
      },
      "flow_token": "01936f8a-1234-7abc-8901-123456789abc",
      "id": "sha256:pqr678...",
      "security_context": {
        "permissions": [],
        "tenant_id": "",
        "user_id": ""
      },
      "seq": 5
    }
  ],
  "provenance_edges": [
    {
      "invocation_id": "sha256:pqr678...",
      "sync_firing_id": 1
    }
  ],
  "scenario_name": "cart_checkout_success",
  "sync_firings": [
    {
      "binding_hash": "sha256:stu901...",
      "completion_id": "sha256:ghi789...",
      "id": 1,
      "seq": 5,
      "sync_id": "cart-inventory-reserve"
    }
  ]
}
```

### Test Harness Integration

```go
// internal/harness/demo_test.go
package harness

import (
    "testing"

    "github.com/stretchr/testify/require"
    "github.com/tyler/nysm/internal/testutil"
)

// TestDemoScenarios validates the canonical demo scenarios from PRD Appendix A.
// These scenarios serve as:
// 1. End-to-end validation of the NYSM MVP
// 2. Reference implementation examples
// 3. Regression test fixtures
func TestDemoScenarios(t *testing.T) {
    tests := []struct {
        name         string
        scenarioPath string
    }{
        {
            name:         "cart_checkout_success",
            scenarioPath: "testdata/scenarios/cart_checkout_success.yaml",
        },
        {
            name:         "cart_checkout_insufficient_stock",
            scenarioPath: "testdata/scenarios/cart_checkout_insufficient_stock.yaml",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Load scenario
            scenario, err := LoadScenario(tt.scenarioPath)
            require.NoError(t, err, "failed to load scenario")

            // Create harness with deterministic test helpers
            clock := testutil.NewDeterministicClock()

            // Use flow token from scenario for determinism
            var flowGen FlowTokenGenerator
            if scenario.FlowToken != "" {
                flowGen = testutil.NewFixedFlowGenerator(scenario.FlowToken)
            } else {
                flowGen = testutil.NewFixedFlowGenerator("test-flow-default")
            }

            h := NewHarness(
                WithClock(clock),
                WithFlowGenerator(flowGen),
            )

            // Run scenario with golden comparison
            err = h.RunWithGolden(t, scenario)
            require.NoError(t, err, "scenario execution or golden comparison failed")
        })
    }
}

// TestDemoScenariosReplay validates deterministic replay.
// Running the same scenario twice should produce identical golden files.
func TestDemoScenariosReplay(t *testing.T) {
    scenarioPath := "testdata/scenarios/cart_checkout_success.yaml"

    scenario, err := LoadScenario(scenarioPath)
    require.NoError(t, err)

    // First run
    clock1 := testutil.NewDeterministicClock()
    flowGen1 := testutil.NewFixedFlowGenerator(scenario.FlowToken)
    h1 := NewHarness(
        WithClock(clock1),
        WithFlowGenerator(flowGen1),
    )
    result1, err := h1.Run(scenario)
    require.NoError(t, err)

    // Second run with identical setup
    clock2 := testutil.NewDeterministicClock()
    flowGen2 := testutil.NewFixedFlowGenerator(scenario.FlowToken)
    h2 := NewHarness(
        WithClock(clock2),
        WithFlowGenerator(flowGen2),
    )
    result2, err := h2.Run(scenario)
    require.NoError(t, err)

    // Both runs should produce identical traces
    require.Equal(t, result1.Invocations, result2.Invocations, "invocations must be identical")
    require.Equal(t, result1.Completions, result2.Completions, "completions must be identical")
    require.Equal(t, result1.SyncFirings, result2.SyncFirings, "sync firings must be identical")
    require.Equal(t, result1.ProvenanceEdges, result2.ProvenanceEdges, "provenance edges must be identical")
}
```

### Scenario README

```markdown
# testdata/scenarios/README.md

# Demo Scenarios

This directory contains the canonical demo scenarios from PRD Appendix A.

## Scenarios

### cart_checkout_success.yaml
Validates the happy path: adding items to cart and successfully checking out.

**Flow:**
1. Setup: Initialize inventory with 10 widgets
2. Cart.addItem(widget, qty=3) → Success
3. Cart.checkout() → Success
4. Sync rule fires: cart-inventory-reserve
5. Inventory.reserve(widget, qty=3) → Success

**Validates:**
- Sync rule execution (when → where → then)
- Provenance edge creation (checkout → reserve)
- State updates (inventory reduced, cart cleared)
- Action ordering (addItem → checkout → reserve)

### cart_checkout_insufficient_stock.yaml
Validates error handling when inventory stock is insufficient.

**Flow:**
1. Setup: Initialize inventory with only 2 widgets
2. Cart.addItem(widget, qty=3) → Success
3. Cart.checkout() → CheckoutFailed
4. Sync rule fires: cart-inventory-reserve
5. Inventory.reserve(widget, qty=3) → InsufficientStock
6. Error-matching sync propagates error back to cart

**Validates:**
- Error case matching in sync rules
- Error propagation (inventory → cart)
- No partial state changes (inventory unchanged, cart preserves items)
- Typed error outputs (InsufficientStock with available/requested fields)

## Running Demo Tests

```bash
# Run all demo scenarios
go test ./internal/harness -run TestDemoScenarios

# Run specific scenario
go test ./internal/harness -run TestDemoScenarios/cart_checkout_success

# Update golden files (only when behavior changes intentionally)
go test ./internal/harness -run TestDemoScenarios -update
```

## Deterministic Testing

All demo scenarios use:
- Fixed flow tokens (specified in YAML)
- Deterministic logical clocks (testutil.DeterministicClock)
- In-memory SQLite database (isolated per test)

This ensures golden files are stable across runs for reliable regression testing.

## Golden Files

Golden traces are stored in `testdata/golden/`:
- `cart_checkout_success.golden` - Complete trace for success scenario
- `cart_checkout_insufficient_stock.golden` - Complete trace for error scenario

Golden files use RFC 8785 canonical JSON with:
- Sorted keys (UTF-16 code unit ordering)
- Logical clocks (seq) instead of timestamps
- Content-addressed IDs for determinism
```

### File List

Files to create:

1. `testdata/scenarios/cart_checkout_success.yaml` - Success scenario definition
2. `testdata/scenarios/cart_checkout_insufficient_stock.yaml` - Error scenario definition
3. `testdata/scenarios/README.md` - Scenario documentation
4. `internal/harness/demo_test.go` - Demo scenario tests
5. `testdata/golden/cart_checkout_success.golden` - Generated golden trace (via -update)
6. `testdata/golden/cart_checkout_insufficient_stock.golden` - Generated golden trace (via -update)

### Relationship to Other Stories

**Dependencies:**
- Story 7.8 (Demo Concept Specs) - Provides cart.concept.cue, inventory.concept.cue, web.concept.cue
- Story 7.9 (Demo Sync Rules) - Provides cart-inventory.sync.cue
- Story 6.1 (Scenario Definition Format) - Defines YAML scenario format
- Story 6.6 (Golden Trace Snapshots) - Provides RunWithGolden implementation
- Story 6.2 (Test Execution Engine) - Provides harness.Run() function
- Story 6.3 (Trace Assertions) - Implements trace_contains, trace_order, trace_count
- Story 6.4 (Final State Assertions) - Implements final_state assertions
- Story 1.4 (RFC 8785) - Provides MarshalCanonical for golden files
- Story 2.2 (Logical Clocks) - Ensures deterministic seq values in traces
- Story 3.5 (Flow Token Generation) - Uses fixed flow tokens from scenarios

**Validates:**
- Epic 1 (Foundation & IR Core) - Specs compile to IR correctly
- Epic 2 (Durable Event Store) - Events persisted and queryable
- Epic 3 (Sync Engine Core) - Sync rules fire correctly
- Epic 4 (Query & Binding System) - Where-clauses produce correct bindings
- Epic 5 (Idempotency & Cycle Safety) - No duplicate firings
- Epic 6 (Conformance Harness) - All assertion types work
- Epic 7 (CLI & Demo) - Complete end-to-end MVP functionality

**This is the final integration point for the entire NYSM MVP.**

### Story Completion Checklist

- [ ] `testdata/scenarios/cart_checkout_success.yaml` created with all sections
- [ ] `testdata/scenarios/cart_checkout_insufficient_stock.yaml` created with all sections
- [ ] `testdata/scenarios/README.md` written with usage instructions
- [ ] `internal/harness/demo_test.go` implements TestDemoScenarios
- [ ] Demo scenarios reference correct spec files from Stories 7.8 and 7.9
- [ ] Setup sections initialize inventory stock correctly
- [ ] Flow sections define complete invocation sequences
- [ ] Assertions validate sync rule execution
- [ ] Assertions validate provenance edges
- [ ] Assertions validate final state
- [ ] Assertions validate action ordering
- [ ] Error scenario validates InsufficientStock error case
- [ ] Error scenario validates no partial state changes
- [ ] Fixed flow tokens specified for deterministic golden comparison
- [ ] Golden files generated with `go test ./internal/harness -update`
- [ ] `testdata/golden/cart_checkout_success.golden` committed to git
- [ ] `testdata/golden/cart_checkout_insufficient_stock.golden` committed to git
- [ ] Golden files use canonical JSON (RFC 8785)
- [ ] All demo tests pass without -update flag
- [ ] Deterministic replay test passes (identical results on re-run)
- [ ] Complete invocation/completion chain visible in golden traces
- [ ] Provenance edges show causality in golden traces
- [ ] All tests pass (`go test ./internal/harness/...`)
- [ ] `go build ./...` succeeds
- [ ] `go vet ./internal/harness` passes
- [ ] End-to-end MVP validation complete

### References

- [Source: docs/epics.md#Story 7.10] - Story definition and acceptance criteria
- [Source: docs/prd.md#Appendix A] - Canonical demo scenarios specification
- [Source: docs/architecture.md#CP-2] - Logical clocks for deterministic ordering
- [Source: docs/architecture.md#CP-3] - RFC 8785 canonical JSON
- [Source: docs/sprint-artifacts/6-1-scenario-definition-format.md] - Scenario YAML format
- [Source: docs/sprint-artifacts/6-6-golden-trace-snapshots.md] - Golden file generation and comparison

## Dev Agent Record

### Agent Model Used

Claude Sonnet 4.5 (claude-sonnet-4-5-20250929)

### Validation History

- Initial creation: 2025-12-12

### Completion Notes

- Demo scenarios validate the complete NYSM MVP end-to-end
- Success scenario demonstrates sync rule execution with correct bindings
- Error scenario demonstrates typed error outputs and error-matching sync rules
- Golden traces capture complete causality chain (invocations → completions → sync firings → provenance edges)
- Fixed flow tokens ensure golden files are stable across runs
- Scenarios serve as both validation tests and reference examples
- Golden files committed to git provide regression testing
- Deterministic replay ensures NYSM's core correctness guarantees
- This story completes Epic 7 and validates all previous epics
- After this story, NYSM MVP is fully implemented and validated
