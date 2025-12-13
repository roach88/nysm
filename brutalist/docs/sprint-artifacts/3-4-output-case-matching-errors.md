# Story 3.4: Output Case Matching for Errors

Status: ready-for-dev

## Story

As a **developer handling errors**,
I want **when-clauses to match specific error variants**,
So that **I can build error-handling sync rules**.

## Acceptance Criteria

1. **WhenClause supports OutputCase field in `internal/ir/types.go`**
   ```go
   type WhenClause struct {
       Action     ActionRef         `json:"action"`      // e.g., "Cart.checkout"
       Event      string            `json:"event"`       // "completed" or "invoked"
       OutputCase *string           `json:"output_case"` // nil = match any, "Success" = match specific
       Bindings   map[string]string `json:"bindings"`    // result field → bound variable
   }
   ```

2. **Matcher function checks OutputCase in `internal/engine/matcher.go`**
   ```go
   func (m *Matcher) matches(when WhenClause, comp ir.Completion) bool {
       // Check action URI
       if when.Action != comp.ActionURI {
           return false
       }

       // Check event type (for completions, event must be "completed")
       if when.Event != "completed" {
           return false
       }

       // Check output case
       if when.OutputCase != nil {
           if *when.OutputCase != comp.OutputCase {
               return false // Specific case doesn't match
           }
       }
       // nil OutputCase matches ALL cases (success and errors)

       return true
   }
   ```

3. **case: "Success" only matches Success completions**
   - Given completion with `OutputCase: "Success"`
   - When `WhenClause.OutputCase = ptr("Success")`
   - Then matcher returns true
   - When `WhenClause.OutputCase = ptr("InsufficientStock")`
   - Then matcher returns false

4. **case: "InsufficientStock" only matches InsufficientStock completions**
   - Given completion with `OutputCase: "InsufficientStock"`
   - When `WhenClause.OutputCase = ptr("InsufficientStock")`
   - Then matcher returns true and bindings extracted
   - When `WhenClause.OutputCase = ptr("Success")`
   - Then matcher returns false

5. **case: nil (unspecified) matches ALL outcomes**
   - Given completion with ANY OutputCase
   - When `WhenClause.OutputCase = nil`
   - Then matcher returns true (universal match)
   - This is the default behavior for backward compatibility

6. **Error field extraction works via bindings**
   ```go
   // For error case "InsufficientStock" with fields: available, requested, item
   when := WhenClause{
       Action:     "Inventory.reserve",
       Event:      "completed",
       OutputCase: ptr("InsufficientStock"),
       Bindings: map[string]string{
           "item":      "item",
           "available": "available",
           "requested": "requested",
       },
   }

   comp := ir.Completion{
       ActionURI:  "Inventory.reserve",
       OutputCase: "InsufficientStock",
       Result: ir.IRObject{
           "item":      ir.IRString("product-123"),
           "available": ir.IRInt(3),
           "requested": ir.IRInt(5),
       },
   }

   bindings := extractBindings(when, comp)
   // bindings = {
   //   "item": IRString("product-123"),
   //   "available": IRInt(3),
   //   "requested": IRInt(5),
   // }
   ```

## Quick Reference

| Pattern | Key Rule |
|---------|----------|
| **MEDIUM-2** | Error matching in when-clause |
| **FR-2.2** | Compile when-clause to match on action completions |
| **CP-5** | Error result fields use constrained IRValue types |

## Tasks / Subtasks

- [ ] Task 1: Add OutputCase field to WhenClause (AC: #1)
  - [ ] 1.1 Add `OutputCase *string` field to WhenClause struct
  - [ ] 1.2 Add JSON tag `json:"output_case"`
  - [ ] 1.3 Document nil = match any, non-nil = match specific

- [ ] Task 2: Implement case matching logic (AC: #2, #3, #4, #5)
  - [ ] 2.1 Create `internal/engine/matcher.go`
  - [ ] 2.2 Implement `Matcher.matches(when, comp)` function
  - [ ] 2.3 Check action URI matches
  - [ ] 2.4 Check event type matches
  - [ ] 2.5 Check output case (nil = any, non-nil = specific)

- [ ] Task 3: Implement error field extraction (AC: #6)
  - [ ] 3.1 Implement `extractBindings(when, comp)` in matcher.go
  - [ ] 3.2 Extract fields from comp.Result per when.Bindings map
  - [ ] 3.3 Return IRObject with bound variables

- [ ] Task 4: Write comprehensive tests
  - [ ] 4.1 Test Success-only match (AC: #3)
  - [ ] 4.2 Test error-only match (AC: #4)
  - [ ] 4.3 Test any-match with nil case (AC: #5)
  - [ ] 4.4 Test error field extraction (AC: #6)
  - [ ] 4.5 Test action URI mismatch returns false
  - [ ] 4.6 Test event type mismatch returns false

## Dev Notes

### Critical Implementation Details

**OutputCase Matching Semantics**

The OutputCase field uses a **pointer to string** to distinguish between:
- `nil` - Match ANY outcome (success or error)
- `&"Success"` - Match ONLY success outcomes
- `&"InsufficientStock"` - Match ONLY this specific error variant

This is different from Go's zero-value semantics where `""` (empty string) has meaning.

**Why nil = match all?**

This design choice enables:
1. **Backward compatibility** - Existing syncs without case specification continue to work
2. **Universal handlers** - Some syncs need to fire on ANY completion (logging, metrics)
3. **Explicit opt-in** - Developers must explicitly set case to get filtering

**Error Propagation Without Transactions**

This story implements the paper's "no transactions" integrity model:
```
Success path:
  Inventory.reserve → case: "Success" → Cart.checkout

Error path:
  Inventory.reserve → case: "InsufficientStock" → Cart.checkout.fail
```

By matching specific error variants, the system can:
- Route errors to compensating actions
- Extract error details for user feedback
- Maintain consistency without distributed transactions

### Function Signatures

**Matcher Implementation**

```go
// internal/engine/matcher.go
package engine

import "github.com/tyler/nysm/internal/ir"

// Matcher handles when-clause matching logic
type Matcher struct{}

// NewMatcher creates a new matcher
func NewMatcher() *Matcher {
    return &Matcher{}
}

// Matches checks if a completion matches a when-clause
func (m *Matcher) Matches(when ir.WhenClause, comp ir.Completion) bool {
    // Check action URI
    if when.Action != comp.ActionURI {
        return false
    }

    // Check event type
    // For completions, event must be "completed"
    // (Future: invocations would check for "invoked")
    if when.Event != "completed" {
        return false
    }

    // Check output case
    if when.OutputCase != nil {
        // Specific case match required
        if *when.OutputCase != comp.OutputCase {
            return false
        }
    }
    // nil OutputCase matches ALL cases

    return true
}

// ExtractBindings extracts field values from completion result
// based on the when-clause bindings map.
// Returns IRObject with bound variable names as keys.
func (m *Matcher) ExtractBindings(when ir.WhenClause, comp ir.Completion) ir.IRObject {
    bindings := make(ir.IRObject)

    for boundVar, resultField := range when.Bindings {
        if val, ok := comp.Result[resultField]; ok {
            bindings[boundVar] = val
        }
        // If field missing, skip binding (or error? TBD based on validation story)
    }

    return bindings
}
```

### Helper Function

```go
// Helper for creating *string from string literal in tests
func ptr(s string) *string {
    return &s
}
```

### Test Examples

**Test Success-Only Match**

```go
func TestMatcher_SuccessOnly(t *testing.T) {
    matcher := NewMatcher()

    when := ir.WhenClause{
        Action:     "Cart.checkout",
        Event:      "completed",
        OutputCase: ptr("Success"),
    }

    // Success completion should match
    successComp := ir.Completion{
        ActionURI:  "Cart.checkout",
        OutputCase: "Success",
        Result:     ir.IRObject{"order_id": ir.IRString("order-123")},
    }
    assert.True(t, matcher.Matches(when, successComp))

    // Error completion should NOT match
    errorComp := ir.Completion{
        ActionURI:  "Cart.checkout",
        OutputCase: "PaymentFailed",
        Result:     ir.IRObject{"reason": ir.IRString("card_declined")},
    }
    assert.False(t, matcher.Matches(when, errorComp))
}
```

**Test Error-Only Match**

```go
func TestMatcher_ErrorOnly(t *testing.T) {
    matcher := NewMatcher()

    when := ir.WhenClause{
        Action:     "Inventory.reserve",
        Event:      "completed",
        OutputCase: ptr("InsufficientStock"),
    }

    // InsufficientStock completion should match
    errorComp := ir.Completion{
        ActionURI:  "Inventory.reserve",
        OutputCase: "InsufficientStock",
        Result: ir.IRObject{
            "item":      ir.IRString("product-123"),
            "available": ir.IRInt(3),
            "requested": ir.IRInt(5),
        },
    }
    assert.True(t, matcher.Matches(when, errorComp))

    // Success completion should NOT match
    successComp := ir.Completion{
        ActionURI:  "Inventory.reserve",
        OutputCase: "Success",
        Result:     ir.IRObject{"reservation_id": ir.IRString("res-456")},
    }
    assert.False(t, matcher.Matches(when, successComp))

    // Different error should NOT match
    otherErrorComp := ir.Completion{
        ActionURI:  "Inventory.reserve",
        OutputCase: "InvalidQuantity",
        Result:     ir.IRObject{"message": ir.IRString("quantity must be positive")},
    }
    assert.False(t, matcher.Matches(when, otherErrorComp))
}
```

**Test Any-Match (nil case)**

```go
func TestMatcher_AnyMatch(t *testing.T) {
    matcher := NewMatcher()

    when := ir.WhenClause{
        Action:     "Cart.addItem",
        Event:      "completed",
        OutputCase: nil, // Match ANY outcome
    }

    // Success should match
    successComp := ir.Completion{
        ActionURI:  "Cart.addItem",
        OutputCase: "Success",
        Result:     ir.IRObject{"item_added": ir.IRBool(true)},
    }
    assert.True(t, matcher.Matches(when, successComp))

    // Error should ALSO match
    errorComp := ir.Completion{
        ActionURI:  "Cart.addItem",
        OutputCase: "ItemUnavailable",
        Result:     ir.IRObject{"reason": ir.IRString("out_of_stock")},
    }
    assert.True(t, matcher.Matches(when, errorComp))

    // Any other error should match
    anotherErrorComp := ir.Completion{
        ActionURI:  "Cart.addItem",
        OutputCase: "DuplicateItem",
        Result:     ir.IRObject{"message": ir.IRString("item already in cart")},
    }
    assert.True(t, matcher.Matches(when, anotherErrorComp))
}
```

**Test Error Field Extraction**

```go
func TestMatcher_ErrorFieldExtraction(t *testing.T) {
    matcher := NewMatcher()

    when := ir.WhenClause{
        Action:     "Inventory.reserve",
        Event:      "completed",
        OutputCase: ptr("InsufficientStock"),
        Bindings: map[string]string{
            "item":      "item",
            "available": "available",
            "requested": "requested",
        },
    }

    comp := ir.Completion{
        ActionURI:  "Inventory.reserve",
        OutputCase: "InsufficientStock",
        Result: ir.IRObject{
            "item":      ir.IRString("product-123"),
            "available": ir.IRInt(3),
            "requested": ir.IRInt(5),
        },
    }

    // First verify it matches
    assert.True(t, matcher.Matches(when, comp))

    // Then extract bindings
    bindings := matcher.ExtractBindings(when, comp)

    // Verify extracted values
    assert.Equal(t, ir.IRString("product-123"), bindings["item"])
    assert.Equal(t, ir.IRInt(3), bindings["available"])
    assert.Equal(t, ir.IRInt(5), bindings["requested"])
}
```

**Test Action URI Mismatch**

```go
func TestMatcher_ActionMismatch(t *testing.T) {
    matcher := NewMatcher()

    when := ir.WhenClause{
        Action:     "Cart.checkout",
        Event:      "completed",
        OutputCase: ptr("Success"),
    }

    comp := ir.Completion{
        ActionURI:  "Inventory.reserve", // Different action
        OutputCase: "Success",
        Result:     ir.IRObject{},
    }

    assert.False(t, matcher.Matches(when, comp))
}
```

**Test Event Type Mismatch**

```go
func TestMatcher_EventTypeMismatch(t *testing.T) {
    matcher := NewMatcher()

    when := ir.WhenClause{
        Action:     "Cart.checkout",
        Event:      "invoked", // Looking for invocations
        OutputCase: nil,
    }

    // This is a completion (event = "completed")
    comp := ir.Completion{
        ActionURI:  "Cart.checkout",
        OutputCase: "Success",
        Result:     ir.IRObject{},
    }

    // Should not match because event types differ
    // Note: In practice, you'd match invocations against invocations,
    // but this test verifies the event type check works
    assert.False(t, matcher.Matches(when, comp))
}
```

**Test Missing Bindings Fields**

```go
func TestMatcher_MissingBindingField(t *testing.T) {
    matcher := NewMatcher()

    when := ir.WhenClause{
        Action:     "Inventory.reserve",
        Event:      "completed",
        OutputCase: ptr("InsufficientStock"),
        Bindings: map[string]string{
            "item":      "item",
            "available": "available",
            "requested": "requested",
            "extra":     "nonexistent_field", // Field not in result
        },
    }

    comp := ir.Completion{
        ActionURI:  "Inventory.reserve",
        OutputCase: "InsufficientStock",
        Result: ir.IRObject{
            "item":      ir.IRString("product-123"),
            "available": ir.IRInt(3),
            "requested": ir.IRInt(5),
            // "nonexistent_field" is missing
        },
    }

    bindings := matcher.ExtractBindings(when, comp)

    // Verify present fields extracted
    assert.Equal(t, ir.IRString("product-123"), bindings["item"])
    assert.Equal(t, ir.IRInt(3), bindings["available"])
    assert.Equal(t, ir.IRInt(5), bindings["requested"])

    // Verify missing field NOT in bindings
    _, exists := bindings["extra"]
    assert.False(t, exists, "missing result field should not appear in bindings")
}
```

### Example Sync Rules

**Success Handler**

```cue
sync "on-successful-reserve" {
    when: Inventory.reserve.completed {
        case: "Success"
        bind: { reservation_id: result.reservation_id }
    }
    where: Cart.items[item_id == bound.item_id]
    then: Cart.markReserved(
        item_id: bound.item_id,
        reservation_id: bound.reservation_id
    )
}
```

**Error Handler**

```cue
sync "handle-insufficient-stock" {
    when: Inventory.reserve.completed {
        case: "InsufficientStock"
        bind: {
            item: result.item,
            available: result.available,
            requested: result.requested
        }
    }
    where: Cart.items[item_id == bound.item]
    then: Cart.checkout.fail(
        reason: "insufficient_stock",
        item: bound.item,
        details: "Requested " + bound.requested + " but only " + bound.available + " available"
    )
}
```

**Universal Handler (any outcome)**

```cue
sync "log-all-completions" {
    when: *.*.completed {
        // No case specified = match all outcomes
        bind: { action: action_uri, outcome: output_case }
    }
    where: true  // No additional filtering
    then: Logger.record(
        action: bound.action,
        outcome: bound.outcome,
        timestamp: clock.now()
    )
}
```

### Integration with Sync Engine

**Engine Loop with Case Matching**

```go
// internal/engine/engine.go

func (e *Engine) ProcessCompletion(ctx context.Context, comp ir.Completion) error {
    // Write completion to store
    if err := e.store.WriteCompletion(ctx, comp); err != nil {
        return fmt.Errorf("write completion: %w", err)
    }

    // Find matching sync rules
    for _, sync := range e.syncs {
        if e.matcher.Matches(sync.When, comp) {
            // Extract bindings from when-clause
            whenBindings := e.matcher.ExtractBindings(sync.When, comp)

            // Execute where-clause to get additional bindings
            whereBindings, err := e.executeWhereClause(ctx, sync, comp.FlowToken, whenBindings)
            if err != nil {
                return fmt.Errorf("execute where-clause for sync %s: %w", sync.ID, err)
            }

            // Fire sync for each binding set
            for _, bindings := range whereBindings {
                if err := e.fireSyncRule(ctx, sync, comp, bindings); err != nil {
                    return fmt.Errorf("fire sync %s: %w", sync.ID, err)
                }
            }
        }
    }

    return nil
}
```

### File List

Files to create:

1. `internal/engine/matcher.go` - Matcher struct and matching logic
2. `internal/engine/matcher_test.go` - Comprehensive tests

Files to modify:

1. `internal/ir/types.go` - Add OutputCase field to WhenClause

Files to reference (must exist from previous stories):

1. `internal/ir/types.go` - WhenClause, Completion, IRObject types
2. `internal/ir/value.go` - IRValue types (IRString, IRInt, etc.)
3. Story 1.3 - OutputCase definition in ActionSig

### Story Completion Checklist

- [ ] WhenClause.OutputCase field added (*string type)
- [ ] Matcher.Matches function implemented
- [ ] Action URI check works
- [ ] Event type check works
- [ ] OutputCase check works (nil = any, non-nil = specific)
- [ ] Matcher.ExtractBindings function implemented
- [ ] Test: Success-only match passes
- [ ] Test: Error-only match passes
- [ ] Test: Any-match (nil case) passes
- [ ] Test: Error field extraction passes
- [ ] Test: Action URI mismatch returns false
- [ ] Test: Event type mismatch returns false
- [ ] Test: Missing binding fields handled gracefully
- [ ] `go vet ./internal/engine/...` passes
- [ ] `go test ./internal/engine/...` passes

### References

- [Source: docs/architecture.md#MEDIUM-2] - Error matching in when-clause
- [Source: docs/prd.md#FR-2.2] - Compile when-clause to match on completions
- [Source: docs/epics.md#Story 3.3] - When-clause matching foundation
- [Source: docs/epics.md#Story 3.4] - Story definition
- [Source: Story 1.3] - Typed action outputs with error variants

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Validation History

- Initial creation: comprehensive story generation

### Completion Notes

- OutputCase uses *string to distinguish nil (any) from specific cases
- This is the key mechanism for "no transactions" error propagation
- Error completions have the same structure as success completions
- Bindings extraction works identically for success and error fields
- nil case enables universal handlers (logging, metrics, auditing)
- Specific case matching enables error-specific compensation logic
- Missing binding fields are silently skipped (not an error)
- Future validation story may add strict mode for binding extraction
