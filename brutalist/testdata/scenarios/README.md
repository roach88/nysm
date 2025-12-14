# Demo Scenarios

This directory contains the canonical demo scenarios from PRD Appendix A.
These scenarios serve as end-to-end validation of the NYSM MVP.

## Scenarios

### cart_checkout_success.yaml

Validates the happy path: adding items to cart and successfully checking out.

**Flow:**
1. Setup: Initialize inventory with 10 widgets
2. Cart.addItem(widget, qty=3) → Success
3. Cart.checkout() → Success
4. (Future) Sync rule fires: cart-inventory-reserve
5. (Future) Inventory.reserve(widget, qty=3) → Success

**Validates:**
- Scenario format and parsing
- Setup step execution
- Flow step execution with expect clauses
- Assertion evaluation (trace_contains, trace_order, trace_count, final_state)
- Deterministic trace generation

### cart_checkout_insufficient_stock.yaml

Validates error handling when inventory stock is insufficient.

**Flow:**
1. Setup: Initialize inventory with only 2 widgets
2. Cart.addItem(widget, qty=5) → Success
3. Cart.checkout() → CheckoutFailed
4. (Future) Sync rule fires: cart-inventory-reserve
5. (Future) Inventory.reserve fails with InsufficientStock

**Validates:**
- Error case output matching
- Typed error variants
- No partial state changes
- Error propagation patterns

## Running Demo Tests

```bash
# Run all demo scenario tests
go test ./internal/harness -run TestDemo -v

# Run specific scenario test
go test ./internal/harness -run TestDemoScenarios/cart_checkout_success -v

# Run replay determinism test
go test ./internal/harness -run TestDemoScenariosReplay -v
```

## Scenario Format

Each scenario YAML file follows this structure:

```yaml
name: scenario_name
description: "Human-readable description"

specs:
  - path/to/concept.cue
  - path/to/sync.cue

flow_token: "fixed-token-for-determinism"

setup:
  - action: Concept.action
    args:
      key: value

flow:
  - invoke: Concept.action
    args:
      key: value
    expect:
      case: Success
      result:
        field: value

assertions:
  - type: trace_contains
    action: Concept.action
  - type: trace_order
    actions: [First.action, Second.action]
  - type: trace_count
    action: Concept.action
    count: 1
  - type: final_state
    table: table_name
    where:
      key: value
    expect:
      field: expected_value
```

## Deterministic Testing

All demo scenarios use:
- **Fixed flow tokens** (specified in YAML) for reproducible IDs
- **Deterministic logical clocks** (testutil.DeterministicClock)
- **In-memory SQLite database** (isolated per test)

This ensures traces are stable across runs for reliable regression testing.

## Current Limitations

The harness currently bypasses actual engine execution (see harness package docs).
Tests validate:
- Scenario definition format and parsing
- Trace event structure
- Assertion evaluation logic
- Store read/write mechanics

Full engine integration (sync rule execution, action handlers) is tracked in Epic 7.

## Related Documentation

- [Story 7-8: Demo Concept Specs](../../docs/sprint-artifacts/7-8-demo-concept-specs.md)
- [Story 7-9: Demo Sync Rules](../../docs/sprint-artifacts/7-9-demo-sync-rules.md)
- [Story 7-10: Demo Scenarios](../../docs/sprint-artifacts/7-10-demo-scenarios-golden-traces.md)
- [Story 6-1: Scenario Definition Format](../../docs/sprint-artifacts/6-1-scenario-definition-format.md)
