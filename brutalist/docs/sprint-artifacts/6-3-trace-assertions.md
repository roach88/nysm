# Story 6.3: Trace Assertions

Status: done

## Story

As a **developer verifying behavior**,
I want **to assert on the action trace**,
So that **I can verify sync rules fired correctly**.

## Acceptance Criteria

1. **TraceContains assertion type**
   ```go
   type TraceContains struct {
       Action string      // Action URI to find (e.g., "nysm://myapp/action/Inventory/reserve")
       Args   ir.IRObject // Expected args (subset match - can be partial)
   }
   ```
   - Searches entire trace for matching invocation
   - Args match is flexible (subset check - specified args must match, extra args ignored)
   - Returns clear error if action not found
   - Error message includes full trace for debugging

2. **TraceOrder assertion type**
   ```go
   type TraceOrder struct {
       Actions []string  // Action URIs that must appear in this order
   }
   ```
   - Verifies actions appear in specified sequence
   - Actions don't need to be consecutive (intervening actions allowed)
   - Returns clear error showing actual order vs expected order
   - Error message includes positions of found actions

3. **TraceCount assertion type**
   ```go
   type TraceCount struct {
       Action string  // Action URI to count
       Count  int     // Expected number of occurrences
   }
   ```
   - Counts exact occurrences of action in trace
   - Returns error if count doesn't match
   - Error message shows expected vs actual count

4. **assertTraceContains implementation**
   ```go
   func (h *Harness) assertTraceContains(trace []ir.Event, assertion TraceContains) error {
       for _, event := range trace {
           if inv, ok := event.(ir.Invocation); ok {
               if inv.ActionURI == assertion.Action && h.matchArgs(inv.Args, assertion.Args) {
                   return nil  // Found matching invocation
               }
           }
       }
       return &AssertionError{
           Type:     "trace_contains",
           Expected: fmt.Sprintf("action %s with args %v", assertion.Action, assertion.Args),
           Actual:   "not found in trace",
           Trace:    trace,  // Include full trace in error for debugging
       }
   }
   ```

5. **Flexible arg matching (subset check)**
   ```go
   func (h *Harness) matchArgs(actual, expected ir.IRObject) bool {
       // For each key in expected, check it exists in actual with same value
       for key, expectedVal := range expected {
           actualVal, exists := actual[key]
           if !exists {
               return false  // Required key missing
           }
           if !reflect.DeepEqual(actualVal, expectedVal) {
               return false  // Value mismatch
           }
       }
       // Extra keys in actual are ignored (subset match)
       return true
   }
   ```

6. **assertTraceOrder implementation**
   ```go
   func (h *Harness) assertTraceOrder(trace []ir.Event, assertion TraceOrder) error {
       positions := make(map[string]int)  // action -> first position in trace

       for i, event := range trace {
           if inv, ok := event.(ir.Invocation); ok {
               for _, expectedAction := range assertion.Actions {
                   if inv.ActionURI == expectedAction && positions[expectedAction] == 0 {
                       positions[expectedAction] = i + 1  // 1-indexed for human readability
                   }
               }
           }
       }

       // Check all actions found
       for _, action := range assertion.Actions {
           if positions[action] == 0 {
               return &AssertionError{
                   Type:     "trace_order",
                   Expected: fmt.Sprintf("all actions present: %v", assertion.Actions),
                   Actual:   fmt.Sprintf("missing action: %s", action),
                   Trace:    trace,
               }
           }
       }

       // Check order
       for i := 1; i < len(assertion.Actions); i++ {
           prev := assertion.Actions[i-1]
           curr := assertion.Actions[i]
           if positions[prev] >= positions[curr] {
               return &AssertionError{
                   Type:     "trace_order",
                   Expected: fmt.Sprintf("actions in order: %v", assertion.Actions),
                   Actual:   fmt.Sprintf("%s (pos %d) should be before %s (pos %d)",
                       prev, positions[prev], curr, positions[curr]),
                   Trace:    trace,
               }
           }
       }

       return nil
   }
   ```

7. **assertTraceCount implementation**
   ```go
   func (h *Harness) assertTraceCount(trace []ir.Event, assertion TraceCount) error {
       count := 0
       for _, event := range trace {
           if inv, ok := event.(ir.Invocation); ok {
               if inv.ActionURI == assertion.Action {
                   count++
               }
           }
       }

       if count != assertion.Count {
           return &AssertionError{
               Type:     "trace_count",
               Expected: fmt.Sprintf("%d occurrences of %s", assertion.Count, assertion.Action),
               Actual:   fmt.Sprintf("%d occurrences", count),
               Trace:    trace,
           }
       }

       return nil
   }
   ```

8. **AssertionError type for clear error messages**
   ```go
   type AssertionError struct {
       Type     string      // "trace_contains", "trace_order", "trace_count"
       Expected string      // Human-readable expected value
       Actual   string      // Human-readable actual value
       Trace    []ir.Event  // Full trace for debugging
   }

   func (e *AssertionError) Error() string {
       var buf strings.Builder
       fmt.Fprintf(&buf, "Assertion failed: %s\n", e.Type)
       fmt.Fprintf(&buf, "  Expected: %s\n", e.Expected)
       fmt.Fprintf(&buf, "  Actual: %s\n", e.Actual)
       fmt.Fprintf(&buf, "\nFull trace:\n")
       for i, event := range e.Trace {
           if inv, ok := event.(ir.Invocation); ok {
               fmt.Fprintf(&buf, "  [%d] %s %v\n", i+1, inv.ActionURI, inv.Args)
           }
       }
       return buf.String()
   }
   ```

## Quick Reference

| Pattern | Key Rule |
|---------|----------|
| **FR-6.2** | Run scenarios with assertions on action traces |
| **NFR-4.1** | Actionable error messages with clear context |
| **Subset Match** | Expected args must match, extra args ignored (flexible matching) |
| **Trace Format** | `[]ir.Event` where Event can be Invocation or Completion |

## Tasks / Subtasks

- [ ] Task 1: Define assertion types (AC: #1, #2, #3)
  - [ ] 1.1 Add TraceContains struct to `internal/harness/assertions.go`
  - [ ] 1.2 Add TraceOrder struct
  - [ ] 1.3 Add TraceCount struct
  - [ ] 1.4 Add AssertionError type with Error() method
  - [ ] 1.5 Verify all types follow Go conventions

- [ ] Task 2: Implement TraceContains assertion (AC: #1, #4, #5)
  - [ ] 2.1 Add assertTraceContains function to `internal/harness/assertions.go`
  - [ ] 2.2 Implement matchArgs helper with subset semantics
  - [ ] 2.3 Add test: action found with matching args
  - [ ] 2.4 Add test: action found with subset args (extra args ignored)
  - [ ] 2.5 Add test: action not found returns clear error
  - [ ] 2.6 Add test: action found but args mismatch
  - [ ] 2.7 Verify error message includes full trace

- [ ] Task 3: Implement TraceOrder assertion (AC: #2, #6)
  - [ ] 3.1 Add assertTraceOrder function
  - [ ] 3.2 Implement position tracking logic
  - [ ] 3.3 Add test: actions in correct order
  - [ ] 3.4 Add test: actions out of order returns error
  - [ ] 3.5 Add test: missing action returns error
  - [ ] 3.6 Add test: non-consecutive actions (intervening actions allowed)
  - [ ] 3.7 Verify error shows expected vs actual positions

- [ ] Task 4: Implement TraceCount assertion (AC: #3, #7)
  - [ ] 4.1 Add assertTraceCount function
  - [ ] 4.2 Implement counting logic
  - [ ] 4.3 Add test: exact count match
  - [ ] 4.4 Add test: count too low returns error
  - [ ] 4.5 Add test: count too high returns error
  - [ ] 4.6 Add test: zero count (action not present)
  - [ ] 4.7 Verify error shows expected vs actual count

- [ ] Task 5: Integrate with harness execution (AC: all)
  - [ ] 5.1 Add assertion evaluation to scenario runner
  - [ ] 5.2 Support multiple assertions per scenario
  - [ ] 5.3 Add test: scenario with multiple trace assertions
  - [ ] 5.4 Add test: mixed assertion types (contains + order + count)
  - [ ] 5.5 Verify all assertions evaluated even if one fails
  - [ ] 5.6 Add assertion result reporting (pass/fail with details)

- [ ] Task 6: Error message quality (AC: #8)
  - [ ] 6.1 Verify AssertionError.Error() produces human-readable output
  - [ ] 6.2 Add test: error message includes trace with positions
  - [ ] 6.3 Add test: error message shows expected vs actual clearly
  - [ ] 6.4 Verify error messages are actionable (developer can fix issue)
  - [ ] 6.5 Test error formatting with large traces (readability)

- [ ] Task 7: Verify and test (AC: all)
  - [ ] 7.1 Integration test: full scenario with trace assertions
  - [ ] 7.2 Integration test: cart checkout flow with trace validation
  - [ ] 7.3 Integration test: sync rule firing order verification
  - [ ] 7.4 Performance test: large traces (1000+ events) handled efficiently
  - [ ] 7.5 Verify all tests pass with `go test ./internal/harness/...`

## Dev Notes

### Critical Pattern Details

**FR-6.2: Assertions on Action Traces**

Trace assertions are the primary mechanism for verifying that sync rules fire correctly. They answer questions like:
- Did the expected invocation happen? (trace_contains)
- Did actions fire in the correct order? (trace_order)
- Did an action fire the expected number of times? (trace_count)

These assertions are essential for operational principle validation (Story 6.5) and conformance testing.

**Subset Arg Matching Philosophy:**

The flexible subset matching for args is intentional and important:

```go
// Expected args (in test):
{"item_id": "widget"}

// Actual invocation args:
{"item_id": "widget", "quantity": 3, "user_id": "usr-123"}

// Result: MATCH ✓
```

**Why subset matching?**
1. Tests focus on what matters (item_id) without brittle coupling to implementation details
2. Sync rules may add context fields (user_id, timestamps) that tests don't care about
3. Makes tests resilient to non-breaking changes (new optional args)

**When to use exact matching:**
If you need to verify that NO extra args are present, use `trace_contains` with all args specified, or create a custom assertion type.

### Trace Structure

**Event Types:**

The trace is a slice of `ir.Event` which can be either:
- `ir.Invocation` - Action was invoked
- `ir.Completion` - Action completed with result

For Story 6.3, we focus on `ir.Invocation` events. Completion assertions are a future enhancement.

**Trace Format Example:**

```go
trace := []ir.Event{
    ir.Invocation{
        ID:        "inv-001",
        FlowToken: "flow-abc",
        ActionURI: "nysm://myapp/action/Cart/addItem",
        Args: ir.IRObject{
            "item_id": ir.IRString("widget"),
            "quantity": ir.IRInt(3),
        },
        Seq: 1,
    },
    ir.Completion{
        ID:           "comp-001",
        InvocationID: "inv-001",
        OutputCase:   "Success",
        Result: ir.IRObject{
            "new_quantity": ir.IRInt(3),
        },
        Seq: 2,
    },
    ir.Invocation{
        ID:        "inv-002",
        FlowToken: "flow-abc",
        ActionURI: "nysm://myapp/action/Cart/checkout",
        Args:      ir.IRObject{},
        Seq:       3,
    },
    // ... more events
}
```

### Implementation Details

**TraceContains Algorithm:**

```go
func (h *Harness) assertTraceContains(trace []ir.Event, assertion TraceContains) error {
    // Linear scan through trace
    for _, event := range trace {
        // Type switch to handle Event interface
        if inv, ok := event.(ir.Invocation); ok {
            // 1. Check action URI matches
            if inv.ActionURI != assertion.Action {
                continue
            }

            // 2. Check args match (subset semantics)
            if h.matchArgs(inv.Args, assertion.Args) {
                return nil  // Success - found matching invocation
            }
        }
        // Completions ignored in this assertion
    }

    // Not found - return detailed error
    return &AssertionError{
        Type:     "trace_contains",
        Expected: fmt.Sprintf("action %s with args %v", assertion.Action, assertion.Args),
        Actual:   "not found in trace",
        Trace:    trace,
    }
}
```

**matchArgs Subset Semantics:**

```go
func (h *Harness) matchArgs(actual, expected ir.IRObject) bool {
    // For each expected key-value pair
    for key, expectedVal := range expected {
        // 1. Key must exist in actual
        actualVal, exists := actual[key]
        if !exists {
            return false  // Required key missing
        }

        // 2. Values must match exactly
        if !reflect.DeepEqual(actualVal, expectedVal) {
            return false  // Value mismatch
        }
    }

    // Extra keys in actual are OK (subset match)
    return true
}
```

**Why reflect.DeepEqual for IRValue comparison:**

```go
// IRValue types are interfaces, so == doesn't work
val1 := ir.IRString("widget")
val2 := ir.IRString("widget")

val1 == val2  // ❌ Compares pointers/interfaces, not values

reflect.DeepEqual(val1, val2)  // ✓ Compares actual string values
```

**TraceOrder Algorithm:**

```go
func (h *Harness) assertTraceOrder(trace []ir.Event, assertion TraceOrder) error {
    // Step 1: Find first position of each expected action
    positions := make(map[string]int)

    for i, event := range trace {
        if inv, ok := event.(ir.Invocation); ok {
            for _, expectedAction := range assertion.Actions {
                if inv.ActionURI == expectedAction && positions[expectedAction] == 0 {
                    positions[expectedAction] = i + 1  // 1-indexed for readability
                }
            }
        }
    }

    // Step 2: Verify all actions found
    for _, action := range assertion.Actions {
        if positions[action] == 0 {
            return &AssertionError{
                Type:     "trace_order",
                Expected: fmt.Sprintf("all actions present: %v", assertion.Actions),
                Actual:   fmt.Sprintf("missing action: %s", action),
                Trace:    trace,
            }
        }
    }

    // Step 3: Verify order
    for i := 1; i < len(assertion.Actions); i++ {
        prev := assertion.Actions[i-1]
        curr := assertion.Actions[i]

        if positions[prev] >= positions[curr] {
            return &AssertionError{
                Type:     "trace_order",
                Expected: fmt.Sprintf("actions in order: %v", assertion.Actions),
                Actual:   fmt.Sprintf("%s (pos %d) should be before %s (pos %d)",
                    prev, positions[prev], curr, positions[curr]),
                Trace:    trace,
            }
        }
    }

    return nil
}
```

**TraceCount Algorithm:**

```go
func (h *Harness) assertTraceCount(trace []ir.Event, assertion TraceCount) error {
    count := 0

    // Count invocations matching action URI
    for _, event := range trace {
        if inv, ok := event.(ir.Invocation); ok {
            if inv.ActionURI == assertion.Action {
                count++
            }
        }
    }

    // Check exact count match
    if count != assertion.Count {
        return &AssertionError{
            Type:     "trace_count",
            Expected: fmt.Sprintf("%d occurrences of %s", assertion.Count, assertion.Action),
            Actual:   fmt.Sprintf("%d occurrences", count),
            Trace:    trace,
        }
    }

    return nil
}
```

### Error Message Design

**AssertionError implements error interface:**

```go
type AssertionError struct {
    Type     string      // Assertion type for categorization
    Expected string      // Human-readable expected outcome
    Actual   string      // Human-readable actual outcome
    Trace    []ir.Event  // Full trace for debugging context
}

func (e *AssertionError) Error() string {
    var buf strings.Builder

    // Header with assertion type
    fmt.Fprintf(&buf, "Assertion failed: %s\n", e.Type)

    // Expected vs Actual (most important info)
    fmt.Fprintf(&buf, "  Expected: %s\n", e.Expected)
    fmt.Fprintf(&buf, "  Actual: %s\n", e.Actual)

    // Full trace for context
    fmt.Fprintf(&buf, "\nFull trace:\n")
    for i, event := range e.Trace {
        if inv, ok := event.(ir.Invocation); ok {
            fmt.Fprintf(&buf, "  [%d] %s %v\n", i+1, inv.ActionURI, inv.Args)
        }
    }

    return buf.String()
}
```

**Example error output:**

```
Assertion failed: trace_contains
  Expected: action nysm://myapp/action/Inventory/reserve with args map[item_id:widget]
  Actual: not found in trace

Full trace:
  [1] nysm://myapp/action/Cart/addItem map[item_id:widget quantity:3]
  [2] nysm://myapp/action/Cart/checkout map[]
  [3] nysm://myapp/action/Web/respond map[status:200]
```

This output gives the developer:
1. What assertion failed
2. What was expected
3. What actually happened
4. Full trace to understand why (shows all invocations)

### Testing Strategy

**Unit Tests (internal/harness/assertions_test.go):**

```go
func TestTraceContains_Found(t *testing.T) {
    trace := []ir.Event{
        ir.Invocation{
            ActionURI: "nysm://test/Cart/addItem",
            Args: ir.IRObject{
                "item_id":  ir.IRString("widget"),
                "quantity": ir.IRInt(3),
            },
        },
    }

    assertion := TraceContains{
        Action: "nysm://test/Cart/addItem",
        Args:   ir.IRObject{"item_id": ir.IRString("widget")},
    }

    h := &Harness{}
    err := h.assertTraceContains(trace, assertion)

    assert.NoError(t, err)
}

func TestTraceContains_NotFound(t *testing.T) {
    trace := []ir.Event{
        ir.Invocation{
            ActionURI: "nysm://test/Cart/addItem",
            Args:      ir.IRObject{"item_id": ir.IRString("widget")},
        },
    }

    assertion := TraceContains{
        Action: "nysm://test/Inventory/reserve",  // Different action
        Args:   ir.IRObject{"item_id": ir.IRString("widget")},
    }

    h := &Harness{}
    err := h.assertTraceContains(trace, assertion)

    assert.Error(t, err)
    assert.Contains(t, err.Error(), "not found in trace")
    assert.Contains(t, err.Error(), "Full trace:")  // Verify trace included
}

func TestTraceContains_SubsetMatch(t *testing.T) {
    trace := []ir.Event{
        ir.Invocation{
            ActionURI: "nysm://test/Cart/addItem",
            Args: ir.IRObject{
                "item_id":  ir.IRString("widget"),
                "quantity": ir.IRInt(3),
                "user_id":  ir.IRString("usr-123"),  // Extra arg
            },
        },
    }

    assertion := TraceContains{
        Action: "nysm://test/Cart/addItem",
        Args:   ir.IRObject{"item_id": ir.IRString("widget")},  // Subset
    }

    h := &Harness{}
    err := h.assertTraceContains(trace, assertion)

    assert.NoError(t, err)  // Should match despite extra args
}

func TestTraceOrder_CorrectOrder(t *testing.T) {
    trace := []ir.Event{
        ir.Invocation{ActionURI: "nysm://test/Cart/addItem"},
        ir.Invocation{ActionURI: "nysm://test/Cart/checkout"},
        ir.Invocation{ActionURI: "nysm://test/Inventory/reserve"},
    }

    assertion := TraceOrder{
        Actions: []string{
            "nysm://test/Cart/addItem",
            "nysm://test/Cart/checkout",
            "nysm://test/Inventory/reserve",
        },
    }

    h := &Harness{}
    err := h.assertTraceOrder(trace, assertion)

    assert.NoError(t, err)
}

func TestTraceOrder_WrongOrder(t *testing.T) {
    trace := []ir.Event{
        ir.Invocation{ActionURI: "nysm://test/Cart/checkout"},
        ir.Invocation{ActionURI: "nysm://test/Cart/addItem"},  // Wrong order
    }

    assertion := TraceOrder{
        Actions: []string{
            "nysm://test/Cart/addItem",
            "nysm://test/Cart/checkout",
        },
    }

    h := &Harness{}
    err := h.assertTraceOrder(trace, assertion)

    assert.Error(t, err)
    assert.Contains(t, err.Error(), "should be before")
    assert.Contains(t, err.Error(), "pos")  // Verify positions shown
}

func TestTraceCount_ExactMatch(t *testing.T) {
    trace := []ir.Event{
        ir.Invocation{ActionURI: "nysm://test/Inventory/reserve"},
        ir.Invocation{ActionURI: "nysm://test/Inventory/reserve"},
        ir.Invocation{ActionURI: "nysm://test/Inventory/reserve"},
    }

    assertion := TraceCount{
        Action: "nysm://test/Inventory/reserve",
        Count:  3,
    }

    h := &Harness{}
    err := h.assertTraceCount(trace, assertion)

    assert.NoError(t, err)
}

func TestTraceCount_Mismatch(t *testing.T) {
    trace := []ir.Event{
        ir.Invocation{ActionURI: "nysm://test/Inventory/reserve"},
        ir.Invocation{ActionURI: "nysm://test/Inventory/reserve"},
    }

    assertion := TraceCount{
        Action: "nysm://test/Inventory/reserve",
        Count:  3,  // Expected 3, got 2
    }

    h := &Harness{}
    err := h.assertTraceCount(trace, assertion)

    assert.Error(t, err)
    assert.Contains(t, err.Error(), "3 occurrences")  // Expected
    assert.Contains(t, err.Error(), "2 occurrences")  // Actual
}
```

**Integration Tests (internal/harness/integration_test.go):**

```go
func TestScenario_CartCheckoutTraceAssertions(t *testing.T) {
    // Full end-to-end scenario testing trace assertions
    scenario := &Scenario{
        Name: "cart_checkout_trace_validation",
        Setup: []Step{
            {Action: "Inventory.setStock", Args: map[string]any{"item_id": "widget", "quantity": 10}},
        },
        Flow: []Step{
            {Action: "Cart.addItem", Args: map[string]any{"item_id": "widget", "quantity": 3}},
            {Action: "Cart.checkout", Args: map[string]any{}},
        },
        Assertions: []Assertion{
            // Verify reserve was triggered
            TraceContains{
                Action: "nysm://myapp/action/Inventory/reserve",
                Args:   ir.IRObject{"item_id": ir.IRString("widget"), "quantity": ir.IRInt(3)},
            },
            // Verify correct order
            TraceOrder{
                Actions: []string{
                    "nysm://myapp/action/Cart/addItem",
                    "nysm://myapp/action/Cart/checkout",
                    "nysm://myapp/action/Inventory/reserve",
                },
            },
            // Verify reserve fired once
            TraceCount{
                Action: "nysm://myapp/action/Inventory/reserve",
                Count:  1,
            },
        },
    }

    harness := NewHarness(t)
    result, err := harness.Run(scenario)

    require.NoError(t, err)
    assert.True(t, result.Passed)
}

func TestScenario_MultiBindingTraceCount(t *testing.T) {
    // Cart with 3 items should trigger 3 reserve invocations
    scenario := &Scenario{
        Name: "multi_item_cart_checkout",
        Setup: []Step{
            {Action: "Inventory.setStock", Args: map[string]any{"item_id": "widget", "quantity": 10}},
            {Action: "Inventory.setStock", Args: map[string]any{"item_id": "gadget", "quantity": 5}},
            {Action: "Inventory.setStock", Args: map[string]any{"item_id": "thing", "quantity": 20}},
        },
        Flow: []Step{
            {Action: "Cart.addItem", Args: map[string]any{"item_id": "widget", "quantity": 2}},
            {Action: "Cart.addItem", Args: map[string]any{"item_id": "gadget", "quantity": 1}},
            {Action: "Cart.addItem", Args: map[string]any{"item_id": "thing", "quantity": 5}},
            {Action: "Cart.checkout", Args: map[string]any{}},
        },
        Assertions: []Assertion{
            // Verify 3 reserve invocations (one per cart item)
            TraceCount{
                Action: "nysm://myapp/action/Inventory/reserve",
                Count:  3,
            },
        },
    }

    harness := NewHarness(t)
    result, err := harness.Run(scenario)

    require.NoError(t, err)
    assert.True(t, result.Passed)
}
```

### Scenario YAML Examples

**Example 1: Basic trace_contains**

```yaml
# testdata/scenarios/cart_checkout_success.yaml
name: cart_checkout_success
description: "Successful checkout triggers inventory reservation"

specs:
  - specs/cart.concept.cue
  - specs/inventory.concept.cue
  - specs/cart-inventory.sync.cue

setup:
  - action: Inventory.setStock
    args: { item_id: "widget", quantity: 10 }

flow:
  - invoke: Cart.addItem
    args: { item_id: "widget", quantity: 3 }
    expect:
      case: Success

  - invoke: Cart.checkout
    args: {}
    expect:
      case: Success

assertions:
  - type: trace_contains
    action: nysm://myapp/action/Inventory/reserve
    args: { item_id: "widget", quantity: 3 }
```

**Example 2: Combined assertions**

```yaml
# testdata/scenarios/cart_multi_item_checkout.yaml
name: cart_multi_item_checkout
description: "Multiple cart items trigger multiple reserve invocations"

specs:
  - specs/cart.concept.cue
  - specs/inventory.concept.cue
  - specs/cart-inventory.sync.cue

setup:
  - action: Inventory.setStock
    args: { item_id: "widget", quantity: 10 }
  - action: Inventory.setStock
    args: { item_id: "gadget", quantity: 5 }

flow:
  - invoke: Cart.addItem
    args: { item_id: "widget", quantity: 2 }
  - invoke: Cart.addItem
    args: { item_id: "gadget", quantity: 1 }
  - invoke: Cart.checkout
    args: {}

assertions:
  # Verify both items reserved
  - type: trace_contains
    action: nysm://myapp/action/Inventory/reserve
    args: { item_id: "widget" }

  - type: trace_contains
    action: nysm://myapp/action/Inventory/reserve
    args: { item_id: "gadget" }

  # Verify correct order
  - type: trace_order
    actions:
      - nysm://myapp/action/Cart/addItem
      - nysm://myapp/action/Cart/checkout
      - nysm://myapp/action/Inventory/reserve

  # Verify exactly 2 reserve invocations
  - type: trace_count
    action: nysm://myapp/action/Inventory/reserve
    count: 2
```

**Example 3: Missing action error**

```yaml
# testdata/scenarios/sync_rule_not_firing.yaml
name: sync_rule_not_firing
description: "Test case demonstrating clear error when sync rule doesn't fire"

specs:
  - specs/cart.concept.cue
  - specs/inventory.concept.cue
  - specs/cart-inventory.sync.cue

setup:
  - action: Inventory.setStock
    args: { item_id: "widget", quantity: 10 }

flow:
  - invoke: Cart.addItem
    args: { item_id: "widget", quantity: 3 }
  # NOTE: Not calling checkout, so reserve should NOT fire

assertions:
  # This should FAIL with clear error message
  - type: trace_contains
    action: nysm://myapp/action/Inventory/reserve
    args: { item_id: "widget" }
```

**Expected error output:**

```
Assertion failed: trace_contains
  Expected: action nysm://myapp/action/Inventory/reserve with args map[item_id:widget]
  Actual: not found in trace

Full trace:
  [1] nysm://myapp/action/Inventory/setStock map[item_id:widget quantity:10]
  [2] nysm://myapp/action/Cart/addItem map[item_id:widget quantity:3]
```

### File List

Files to create/modify:

1. `internal/harness/assertions.go` - Assertion types and implementations (new/modify)
2. `internal/harness/assertions_test.go` - Unit tests for assertions (new file)
3. `internal/harness/harness.go` - Integrate assertions into scenario runner (modify)
4. `internal/harness/integration_test.go` - Full scenario tests (modify)
5. `testdata/scenarios/cart_checkout_success.yaml` - Demo scenario with trace assertions (modify)
6. `testdata/scenarios/cart_multi_item_checkout.yaml` - Multi-binding scenario (new file)
7. `testdata/scenarios/sync_rule_not_firing.yaml` - Negative test scenario (new file)

### Relationship to Other Stories

**Dependencies (must complete before this):**
- Story 6.1: Scenario Definition Format - Need scenario loading infrastructure
- Story 6.2: Test Execution Engine - Need harness runner that executes scenarios

**Enables (unblock after this):**
- Story 6.4: Final State Assertions - Can add more assertion types
- Story 6.5: Operational Principle Validation - Uses trace assertions to validate principles
- Story 6.6: Golden Trace Snapshots - Trace format established for golden files

**Related Stories:**
- Story 3.3: When-Clause Matching - Trace assertions verify when-clause fired
- Story 3.7: Flow-Scoped Sync Matching - Trace assertions verify correct flow scoping
- Story 4.4: Binding Set Execution - Trace count verifies multi-binding syncs

### Story Completion Checklist

- [ ] TraceContains type and implementation
- [ ] TraceOrder type and implementation
- [ ] TraceCount type and implementation
- [ ] AssertionError type with Error() method
- [ ] matchArgs helper with subset semantics
- [ ] Assertion evaluation integrated into harness
- [ ] All unit tests pass (assertions_test.go)
- [ ] All integration tests pass (full scenarios)
- [ ] Error messages are clear and actionable
- [ ] Demo scenarios updated with trace assertions
- [ ] `go build ./...` succeeds
- [ ] `go vet ./...` passes
- [ ] `go test ./internal/harness/...` passes

### References

- [Source: docs/prd.md#FR-6.2] - Run scenarios with assertions on action traces
- [Source: docs/architecture.md#NFR-4.1] - Actionable error messages
- [Source: docs/epics.md#Story 6.3] - Story definition
- [Source: docs/epics.md#Story 6.2] - Test execution engine (prerequisite)
- [Source: docs/epics.md#Story 6.5] - Operational principle validation (depends on this)

## Dev Agent Record

### Agent Model Used

_To be filled during implementation_

### Validation History

_To be filled during implementation_

### Completion Notes

_To be filled during implementation_
