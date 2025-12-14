# Story 3.3: When-Clause Matching

Status: done

## Story

As a **developer defining sync rules**,
I want **when-clauses to match completions by action and event type**,
So that **syncs fire at the right times**.

## Acceptance Criteria

1. **WhenClause struct defined in `internal/ir/clause.go`** (from Story 1.1)
   - Already exists with fields: `Action`, `Event`, `OutputCase`, `Bindings`
   - `OutputCase` is `*string` (nil = match any case, non-nil = match specific)
   - `Event` is `"completed"` or `"invoked"` (this story focuses on "completed")

2. **Matcher implementation in `internal/engine/matcher.go`**
   - Function: `func matches(when WhenClause, comp ir.Completion) bool`
   - Checks: action URI matches, event type matches, output case matches (if specified)
   - Returns true only if ALL conditions satisfied

3. **Binding extraction in `internal/engine/matcher.go`**
   - Function: `func extractBindings(when WhenClause, comp ir.Completion) (ir.IRObject, error)`
   - Extracts fields from `comp.Result` based on `when.Bindings` map
   - Returns error if binding references non-existent field
   - Returns IRObject with bound variables

4. **Output case matching logic**
   - `when.OutputCase == nil` → matches ANY output case (success or error)
   - `when.OutputCase != nil` → matches only if `*when.OutputCase == comp.OutputCase`
   - Case-sensitive exact match

5. **Error handling for missing bindings**
   - If `when.Bindings["var"]` references field not in `comp.Result`, return error
   - Clear error message: `"binding field 'field_name' not found in completion result"`
   - Partial extraction NOT allowed - all-or-nothing

6. **Comprehensive tests in `internal/engine/matcher_test.go`**
   - Test: exact match (action + event + output case)
   - Test: output case matching (nil = any, non-nil = specific)
   - Test: action mismatch returns false
   - Test: event mismatch returns false
   - Test: output case mismatch returns false
   - Test: binding extraction success
   - Test: binding extraction error (missing field)
   - Test: nested field extraction (IRObject values)

## Quick Reference

| Pattern | Key Rule |
|---------|----------|
| **FR-2.2** | Compile when-clause to match completions |
| **MEDIUM-2** | Error matching via output case |
| **CP-5** | Bindings return IRValue types only |

## Tasks / Subtasks

- [ ] Task 1: Implement matcher core logic (AC: #2)
  - [ ] 1.1 Create `internal/engine/matcher.go`
  - [ ] 1.2 Implement `matches(when, comp)` function
  - [ ] 1.3 Check action URI match (`when.Action == comp.InvocationID.ActionURI`)
  - [ ] 1.4 Check event type (hardcode "completed" for completions)
  - [ ] 1.5 Check output case (nil = any, non-nil = exact match)

- [ ] Task 2: Implement binding extraction (AC: #3, #5)
  - [ ] 2.1 Implement `extractBindings(when, comp)` function
  - [ ] 2.2 Iterate over `when.Bindings` map
  - [ ] 2.3 Look up each field in `comp.Result`
  - [ ] 2.4 Return error if field not found
  - [ ] 2.5 Build IRObject with bound variables

- [ ] Task 3: Write comprehensive tests (AC: #6)
  - [ ] 3.1 Create `internal/engine/matcher_test.go`
  - [ ] 3.2 Test exact match (all fields match)
  - [ ] 3.3 Test nil output case (matches any)
  - [ ] 3.4 Test specific output case (success)
  - [ ] 3.5 Test specific output case (error variant)
  - [ ] 3.6 Test action mismatch
  - [ ] 3.7 Test output case mismatch
  - [ ] 3.8 Test binding extraction success
  - [ ] 3.9 Test binding extraction error (missing field)
  - [ ] 3.10 Test empty bindings (valid)

- [ ] Task 4: Verify integration readiness (AC: #4)
  - [ ] 4.1 Verify matches() signature compatible with future engine loop
  - [ ] 4.2 Verify extractBindings() returns IRObject (not map[string]any)
  - [ ] 4.3 Document matcher contract in godoc

## Dev Notes

### Critical Implementation Details

**Matcher Function Signature**
```go
// internal/engine/matcher.go
package engine

import (
    "fmt"

    "github.com/tyler/nysm/internal/ir"
)

// matches checks if a completion matches a when-clause.
// Returns true only if ALL conditions are satisfied:
// 1. Action URI matches
// 2. Event type is "completed" (for completions)
// 3. Output case matches (nil = any, non-nil = exact match)
func matches(when ir.WhenClause, comp ir.Completion) bool {
    // Check action URI match
    // NOTE: We need to get action URI from invocation, not completion
    // For now, assume we have access to invocation via comp.InvocationID lookup
    // This will be refined in Story 3.5 (engine loop integration)

    // For this story, we'll pass action URI separately or extend signature
    // Placeholder: assume comp has ActionURI field (to be resolved)

    // Check event type
    if when.Event != "completed" {
        return false
    }

    // Check output case
    if when.OutputCase != nil {
        if *when.OutputCase != comp.OutputCase {
            return false
        }
    }

    return true
}
```

**IMPORTANT: Action URI Resolution**

The WhenClause has an `Action` field (ActionRef), but Completion only has `InvocationID`. To match action URIs, we need to either:

**Option A:** Extend matches() to take Invocation as well:
```go
func matches(when ir.WhenClause, inv ir.Invocation, comp ir.Completion) bool {
    // Check action URI match
    if when.Action != inv.ActionURI {
        return false
    }

    // Check event type
    if when.Event != "completed" {
        return false
    }

    // Check output case
    if when.OutputCase != nil {
        if *when.OutputCase != comp.OutputCase {
            return false
        }
    }

    return true
}
```

**Option B:** Pass action URI separately:
```go
func matches(when ir.WhenClause, actionURI ir.ActionRef, comp ir.Completion) bool {
    // Check action URI match
    if when.Action != actionURI {
        return false
    }

    // ... rest of logic
}
```

**DECISION:** Use Option A (take both Invocation and Completion) as it's clearer and matches the paper's model where completions reference invocations.

### Binding Extraction Implementation

```go
// extractBindings extracts bound variables from completion result.
// Returns error if any binding references a non-existent field.
// All-or-nothing: partial extraction is not allowed.
func extractBindings(when ir.WhenClause, comp ir.Completion) (ir.IRObject, error) {
    bindings := make(ir.IRObject, len(when.Bindings))

    for boundVar, resultField := range when.Bindings {
        // Look up field in completion result
        value, exists := comp.Result[resultField]
        if !exists {
            return nil, fmt.Errorf("binding field %q not found in completion result", resultField)
        }

        // Store in bindings map
        bindings[boundVar] = value
    }

    return bindings, nil
}
```

**Key Design Decisions:**

1. **All-or-nothing extraction:** If any field is missing, entire extraction fails. No partial results.
2. **Flat field access:** This story only supports top-level fields in `comp.Result`. Nested path syntax (e.g., `"user.name"`) deferred to future stories.
3. **Type preservation:** Bindings preserve IRValue types from result (IRString, IRInt, IRBool, IRArray, IRObject).

### Output Case Matching Examples

**Example 1: Match ANY output case (nil)**
```go
when := ir.WhenClause{
    Action:     "Inventory.reserve",
    Event:      "completed",
    OutputCase: nil, // Match any case
    Bindings:   map[string]string{},
}

comp1 := ir.Completion{
    OutputCase: "Success",
    Result:     ir.IRObject{},
}
// matches(when, inv, comp1) → true

comp2 := ir.Completion{
    OutputCase: "InsufficientStock",
    Result:     ir.IRObject{},
}
// matches(when, inv, comp2) → true (nil matches any)
```

**Example 2: Match specific success case**
```go
successCase := "Success"
when := ir.WhenClause{
    Action:     "Inventory.reserve",
    Event:      "completed",
    OutputCase: &successCase, // Match only Success
    Bindings:   map[string]string{"reservation_id": "reservation_id"},
}

comp1 := ir.Completion{
    OutputCase: "Success",
    Result:     ir.IRObject{"reservation_id": ir.IRString("res-123")},
}
// matches(when, inv, comp1) → true
// extractBindings(when, comp1) → {"reservation_id": IRString("res-123")}

comp2 := ir.Completion{
    OutputCase: "InsufficientStock",
    Result:     ir.IRObject{},
}
// matches(when, inv, comp2) → false (output case mismatch)
```

**Example 3: Match specific error variant**
```go
errorCase := "InsufficientStock"
when := ir.WhenClause{
    Action:     "Inventory.reserve",
    Event:      "completed",
    OutputCase: &errorCase, // Match only this error
    Bindings: map[string]string{
        "item":      "item",
        "requested": "requested",
        "available": "available",
    },
}

comp := ir.Completion{
    OutputCase: "InsufficientStock",
    Result: ir.IRObject{
        "item":      ir.IRString("widget-x"),
        "requested": ir.IRInt(10),
        "available": ir.IRInt(5),
    },
}
// matches(when, inv, comp) → true
// extractBindings(when, comp) → {"item": IRString("widget-x"), "requested": IRInt(10), "available": IRInt(5)}
```

### Test Examples

**Test: Exact Match**
```go
func TestMatches_ExactMatch(t *testing.T) {
    successCase := "Success"
    when := ir.WhenClause{
        Action:     "Cart.checkout",
        Event:      "completed",
        OutputCase: &successCase,
        Bindings:   map[string]string{},
    }

    inv := ir.Invocation{
        ActionURI: "Cart.checkout",
        Args:      ir.IRObject{},
    }

    comp := ir.Completion{
        InvocationID: inv.ID,
        OutputCase:   "Success",
        Result:       ir.IRObject{},
    }

    result := matches(when, inv, comp)
    assert.True(t, result, "should match when all conditions satisfied")
}
```

**Test: Nil Output Case (Match Any)**
```go
func TestMatches_NilOutputCaseMatchesAny(t *testing.T) {
    when := ir.WhenClause{
        Action:     "Cart.checkout",
        Event:      "completed",
        OutputCase: nil, // Match any case
        Bindings:   map[string]string{},
    }

    inv := ir.Invocation{
        ActionURI: "Cart.checkout",
        Args:      ir.IRObject{},
    }

    testCases := []struct {
        name       string
        outputCase string
    }{
        {"success case", "Success"},
        {"error case 1", "InsufficientStock"},
        {"error case 2", "PaymentFailed"},
        {"custom case", "CustomCase"},
    }

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            comp := ir.Completion{
                InvocationID: inv.ID,
                OutputCase:   tc.outputCase,
                Result:       ir.IRObject{},
            }

            result := matches(when, inv, comp)
            assert.True(t, result, "nil output case should match any case")
        })
    }
}
```

**Test: Action Mismatch**
```go
func TestMatches_ActionMismatch(t *testing.T) {
    when := ir.WhenClause{
        Action:     "Cart.checkout",
        Event:      "completed",
        OutputCase: nil,
        Bindings:   map[string]string{},
    }

    inv := ir.Invocation{
        ActionURI: "Inventory.reserve", // Different action
        Args:      ir.IRObject{},
    }

    comp := ir.Completion{
        InvocationID: inv.ID,
        OutputCase:   "Success",
        Result:       ir.IRObject{},
    }

    result := matches(when, inv, comp)
    assert.False(t, result, "should not match when action differs")
}
```

**Test: Output Case Mismatch**
```go
func TestMatches_OutputCaseMismatch(t *testing.T) {
    successCase := "Success"
    when := ir.WhenClause{
        Action:     "Cart.checkout",
        Event:      "completed",
        OutputCase: &successCase, // Match only Success
        Bindings:   map[string]string{},
    }

    inv := ir.Invocation{
        ActionURI: "Cart.checkout",
        Args:      ir.IRObject{},
    }

    comp := ir.Completion{
        InvocationID: inv.ID,
        OutputCase:   "PaymentFailed", // Different case
        Result:       ir.IRObject{},
    }

    result := matches(when, inv, comp)
    assert.False(t, result, "should not match when output case differs")
}
```

**Test: Binding Extraction Success**
```go
func TestExtractBindings_Success(t *testing.T) {
    when := ir.WhenClause{
        Action: "Inventory.reserve",
        Event:  "completed",
        Bindings: map[string]string{
            "res_id":  "reservation_id",
            "item":    "item_name",
            "qty":     "quantity",
        },
    }

    comp := ir.Completion{
        OutputCase: "Success",
        Result: ir.IRObject{
            "reservation_id": ir.IRString("res-123"),
            "item_name":      ir.IRString("widget-x"),
            "quantity":       ir.IRInt(5),
            "extra_field":    ir.IRString("ignored"), // Extra fields OK
        },
    }

    bindings, err := extractBindings(when, comp)
    require.NoError(t, err)

    expected := ir.IRObject{
        "res_id": ir.IRString("res-123"),
        "item":   ir.IRString("widget-x"),
        "qty":    ir.IRInt(5),
    }

    assert.Equal(t, expected, bindings)
}
```

**Test: Binding Extraction Missing Field**
```go
func TestExtractBindings_MissingField(t *testing.T) {
    when := ir.WhenClause{
        Action: "Inventory.reserve",
        Event:  "completed",
        Bindings: map[string]string{
            "res_id":  "reservation_id",
            "missing": "nonexistent_field", // This field doesn't exist
        },
    }

    comp := ir.Completion{
        OutputCase: "Success",
        Result: ir.IRObject{
            "reservation_id": ir.IRString("res-123"),
        },
    }

    bindings, err := extractBindings(when, comp)
    require.Error(t, err)
    assert.Nil(t, bindings, "should return nil on error")
    assert.Contains(t, err.Error(), "nonexistent_field")
    assert.Contains(t, err.Error(), "not found")
}
```

**Test: Empty Bindings (Valid)**
```go
func TestExtractBindings_EmptyBindings(t *testing.T) {
    when := ir.WhenClause{
        Action:   "Cart.checkout",
        Event:    "completed",
        Bindings: map[string]string{}, // Empty bindings
    }

    comp := ir.Completion{
        OutputCase: "Success",
        Result: ir.IRObject{
            "order_id": ir.IRString("order-123"),
        },
    }

    bindings, err := extractBindings(when, comp)
    require.NoError(t, err)

    expected := ir.IRObject{}
    assert.Equal(t, expected, bindings, "empty bindings should return empty IRObject")
}
```

**Test: Nested IRObject Values**
```go
func TestExtractBindings_NestedObjects(t *testing.T) {
    when := ir.WhenClause{
        Action: "Order.process",
        Event:  "completed",
        Bindings: map[string]string{
            "customer": "customer_info", // Binds entire nested object
        },
    }

    comp := ir.Completion{
        OutputCase: "Success",
        Result: ir.IRObject{
            "customer_info": ir.IRObject{
                "id":   ir.IRString("cust-123"),
                "name": ir.IRString("Alice"),
            },
            "order_id": ir.IRString("order-456"),
        },
    }

    bindings, err := extractBindings(when, comp)
    require.NoError(t, err)

    // Verify nested object extracted correctly
    customerObj, ok := bindings["customer"].(ir.IRObject)
    require.True(t, ok, "customer should be IRObject")
    assert.Equal(t, ir.IRString("cust-123"), customerObj["id"])
    assert.Equal(t, ir.IRString("Alice"), customerObj["name"])
}
```

### Event Type Handling

**Current Story Scope: "completed" only**
```go
// matches() for this story only handles "completed" events
func matches(when ir.WhenClause, inv ir.Invocation, comp ir.Completion) bool {
    // Action URI match
    if when.Action != inv.ActionURI {
        return false
    }

    // Event type check (only "completed" for this story)
    if when.Event != "completed" {
        return false // Future: support "invoked" in Story 3.6
    }

    // Output case match
    if when.OutputCase != nil {
        if *when.OutputCase != comp.OutputCase {
            return false
        }
    }

    return true
}
```

**Future Extension (Story 3.6): "invoked" events**
When Story 3.6 adds support for "invoked" events, the signature will change to:
```go
func matchesInvocation(when ir.WhenClause, inv ir.Invocation) bool {
    if when.Action != inv.ActionURI {
        return false
    }

    if when.Event != "invoked" {
        return false
    }

    // No output case matching for invocations
    // Bindings extracted from inv.Args instead of comp.Result

    return true
}
```

### Package Structure

```
internal/engine/
├── matcher.go          # matches() and extractBindings()
└── matcher_test.go     # Comprehensive tests
```

**Exported vs Unexported:**
- `matches()` - unexported (internal to engine package)
- `extractBindings()` - unexported (internal to engine package)
- These will be used by `Engine.processCompletion()` in Story 3.5

**Future Integration:**
Story 3.5 (Engine Loop) will use these functions like:
```go
func (e *Engine) processCompletion(comp ir.Completion) error {
    // Lookup invocation
    inv, err := e.store.GetInvocation(comp.InvocationID)
    if err != nil {
        return err
    }

    // Check all sync rules
    for _, sync := range e.syncRules {
        if matches(sync.When, inv, comp) {
            bindings, err := extractBindings(sync.When, comp)
            if err != nil {
                return err
            }

            // Fire sync rule with bindings...
        }
    }

    return nil
}
```

### File List

Files to create:

1. `internal/engine/matcher.go` - Matcher functions
2. `internal/engine/matcher_test.go` - Comprehensive tests

Files to reference (must exist from previous stories):

1. `internal/ir/clause.go` - WhenClause definition (Story 1.1)
2. `internal/ir/types.go` - Invocation, Completion types (Story 1.1)
3. `internal/ir/value.go` - IRValue types (Story 1.2)

### Story Completion Checklist

- [ ] matches() function implemented
- [ ] extractBindings() function implemented
- [ ] Action URI matching works
- [ ] Event type checking works (hardcoded "completed")
- [ ] Output case matching works (nil = any, non-nil = exact)
- [ ] Binding extraction works for top-level fields
- [ ] Error on missing binding field
- [ ] All-or-nothing extraction (no partial results)
- [ ] Tests for exact match pass
- [ ] Tests for nil output case pass
- [ ] Tests for action/output case mismatch pass
- [ ] Tests for binding extraction pass
- [ ] Tests for missing field error pass
- [ ] `go vet ./internal/engine/...` passes
- [ ] `go test ./internal/engine/...` passes

### References

- [Source: docs/epics.md#Story 3.3] - Story definition
- [Source: docs/prd.md#FR-2.2] - Compile when-clause to match completions
- [Source: docs/architecture.md#MEDIUM-2] - Error matching in when-clause
- [Source: Story 1.1] - WhenClause type definition
- [Source: Story 1.2] - IRValue type system
- [Source: Story 3.2] - SyncRule compilation (prerequisite)

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Validation History

- Initial creation: comprehensive story generation from epics.md and architecture.md

### Completion Notes

- **Matcher is stateless:** Both `matches()` and `extractBindings()` are pure functions with no side effects
- **Action URI resolution:** Matcher takes both Invocation and Completion to resolve action URI from invocation
- **Flat field access only:** This story supports only top-level field bindings. Nested path syntax (e.g., `"user.name"`) deferred to future stories
- **All-or-nothing extraction:** If any binding field is missing, entire extraction fails with clear error
- **Type safety:** Bindings preserve IRValue types from completion result
- **Event type scope:** This story only handles "completed" events. "invoked" events deferred to Story 3.6
- **Output case matching:** Implements MEDIUM-2 pattern for error variant matching
- **No external dependencies:** Matcher has no dependencies on store or other engine components (pure logic)
- **Integration hook:** Story 3.5 (Engine Loop) will use these functions in `processCompletion()`
