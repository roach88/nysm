# Story 1.6: CUE Concept Spec Parser

Status: ready-for-dev

## Story

As a **developer defining concepts**,
I want **to write concept specs in CUE and have them parsed**,
So that **I get the benefits of CUE's type system and validation**.

## Acceptance Criteria

1. **`CompileConcept(cueValue cue.Value) (*ConceptSpec, error)` in `internal/compiler/concept.go`**
   - Takes a CUE value representing a concept definition
   - Returns parsed `ir.ConceptSpec` or error

2. **ConceptSpec IR struct defined**
   ```go
   type ConceptSpec struct {
       Name                 string                 `json:"name"`
       Purpose              string                 `json:"purpose"`
       States               []StateSpec            `json:"states"`
       Actions              []ActionSig            `json:"actions"`
       OperationalPrinciple string                 `json:"operational_principle"`
   }

   type StateSpec struct {
       Name   string            `json:"name"`
       Fields map[string]string `json:"fields"` // field name -> type
   }
   ```

3. **CUE concept spec format parsed**
   ```cue
   concept Cart {
       purpose: "Manages shopping cart state for a user session"

       state CartItem {
           item_id: string
           quantity: int
       }

       action addItem {
           args: {
               item_id: string
               quantity: int
           }
           requires: ["cart:write"]  // Optional authz hooks
           outputs: [{
               case: "Success"
               fields: { item_id: string, new_quantity: int }
           }, {
               case: "InvalidQuantity"
               fields: { message: string }
           }]
       }

       operational_principle: """
           Adding an item increases quantity or creates new entry
           """
   }
   ```

4. **CUE validation errors surfaced with line numbers**
   - Uses CUE's error API to extract source positions
   - Clear error messages for invalid CUE syntax

5. **Missing required fields produce clear error messages**
   - `purpose` is required
   - At least one `action` is required
   - Each action must have outputs

## Quick Reference

| Pattern | Key Rule |
|---------|----------|
| **FR-1.1** | CUE format with purpose, state, actions, operational principles |
| **CP-5** | Type fields use constrained types (no floats) |

## Tasks / Subtasks

- [ ] Task 1: Define ConceptSpec IR struct (AC: #2)
  - [ ] 1.1 Create `internal/ir/concept.go`
  - [ ] 1.2 Define ConceptSpec with all fields
  - [ ] 1.3 Define StateSpec for state definitions

- [ ] Task 2: Create compiler package (AC: #1)
  - [ ] 2.1 Create `internal/compiler/concept.go`
  - [ ] 2.2 Add CUE SDK dependency to go.mod

- [ ] Task 3: Implement concept parsing (AC: #3)
  - [ ] 3.1 Parse concept name from CUE struct
  - [ ] 3.2 Parse purpose field
  - [ ] 3.3 Parse state definitions
  - [ ] 3.4 Parse action definitions (reuse ActionSig from Story 1-3)
  - [ ] 3.5 Parse operational_principle

- [ ] Task 4: Implement error handling (AC: #4, #5)
  - [ ] 4.1 Extract line numbers from CUE errors
  - [ ] 4.2 Generate clear error messages for missing fields
  - [ ] 4.3 Validate required fields (purpose, actions)

- [ ] Task 5: Write comprehensive tests
  - [ ] 5.1 Test valid concept parsing
  - [ ] 5.2 Test missing required fields
  - [ ] 5.3 Test invalid CUE syntax
  - [ ] 5.4 Test multiple states and actions

## Dev Notes

### CUE SDK Usage

```go
// internal/compiler/concept.go
package compiler

import (
    "fmt"

    "cuelang.org/go/cue"
    "cuelang.org/go/cue/cuecontext"
    "cuelang.org/go/cue/errors"
    "cuelang.org/go/cue/token"

    "github.com/your-org/nysm/internal/ir"
)

// CompileConcept parses a CUE value into a ConceptSpec.
// Uses CUE SDK's Go API directly (not CLI subprocess).
func CompileConcept(v cue.Value) (*ir.ConceptSpec, error) {
    if err := v.Err(); err != nil {
        return nil, formatCUEError(err)
    }

    spec := &ir.ConceptSpec{}

    // Parse concept name from struct label
    // The concept name comes from the CUE struct's field name
    // e.g., `concept Cart { ... }` → name is "Cart"
    labels := v.Path().Selectors()
    if len(labels) > 0 {
        spec.Name = labels[len(labels)-1].String()
    }

    // Parse purpose (required)
    purposeVal := v.LookupPath(cue.ParsePath("purpose"))
    if !purposeVal.Exists() {
        return nil, &CompileError{
            Field:   "purpose",
            Message: "purpose is required",
            Pos:     v.Pos(),
        }
    }
    purpose, err := purposeVal.String()
    if err != nil {
        return nil, formatCUEError(err)
    }
    spec.Purpose = purpose

    // Parse states (optional, can be empty)
    spec.States, err = parseStates(v)
    if err != nil {
        return nil, err
    }

    // Parse actions (required, at least one)
    spec.Actions, err = parseActions(v)
    if err != nil {
        return nil, err
    }
    if len(spec.Actions) == 0 {
        return nil, &CompileError{
            Field:   "action",
            Message: "at least one action is required",
            Pos:     v.Pos(),
        }
    }

    // Parse operational_principle (optional)
    opVal := v.LookupPath(cue.ParsePath("operational_principle"))
    if opVal.Exists() {
        op, err := opVal.String()
        if err != nil {
            return nil, formatCUEError(err)
        }
        spec.OperationalPrinciple = op
    }

    return spec, nil
}

func parseStates(v cue.Value) ([]ir.StateSpec, error) {
    var states []ir.StateSpec

    // Look for state definitions
    iter, err := v.LookupPath(cue.ParsePath("state")).Fields()
    if err != nil {
        // state is optional, no error if missing
        return states, nil
    }

    for iter.Next() {
        stateName := iter.Label()
        stateVal := iter.Value()

        state := ir.StateSpec{
            Name:   stateName,
            Fields: make(map[string]string),
        }

        // Parse fields
        fieldIter, err := stateVal.Fields()
        if err != nil {
            return nil, formatCUEError(err)
        }

        for fieldIter.Next() {
            fieldName := fieldIter.Label()
            fieldType, err := extractTypeName(fieldIter.Value())
            if err != nil {
                return nil, err
            }
            state.Fields[fieldName] = fieldType
        }

        states = append(states, state)
    }

    return states, nil
}

func parseActions(v cue.Value) ([]ir.ActionSig, error) {
    var actions []ir.ActionSig

    // Look for action definitions
    iter, err := v.LookupPath(cue.ParsePath("action")).Fields()
    if err != nil {
        return actions, nil
    }

    for iter.Next() {
        actionName := iter.Label()
        actionVal := iter.Value()

        action := ir.ActionSig{
            Name: actionName,
        }

        // Parse args
        argsVal := actionVal.LookupPath(cue.ParsePath("args"))
        if argsVal.Exists() {
            argsIter, err := argsVal.Fields()
            if err != nil {
                return nil, formatCUEError(err)
            }

            for argsIter.Next() {
                argName := argsIter.Label()
                argType, err := extractTypeName(argsIter.Value())
                if err != nil {
                    return nil, err
                }
                action.Args = append(action.Args, ir.NamedArg{
                    Name: argName,
                    Type: argType,
                })
            }
        }

        // Parse requires (optional, for authz hooks)
        requiresVal := actionVal.LookupPath(cue.ParsePath("requires"))
        if requiresVal.Exists() {
            reqIter, err := requiresVal.List()
            if err != nil {
                return nil, formatCUEError(err)
            }
            for reqIter.Next() {
                reqStr, err := reqIter.Value().String()
                if err != nil {
                    return nil, formatCUEError(err)
                }
                action.Requires = append(action.Requires, reqStr)
            }
        }

        // Parse outputs
        outputsVal := actionVal.LookupPath(cue.ParsePath("outputs"))
        if !outputsVal.Exists() {
            return nil, &CompileError{
                Field:   fmt.Sprintf("action.%s.outputs", actionName),
                Message: "action outputs are required",
                Pos:     actionVal.Pos(),
            }
        }

        outputIter, err := outputsVal.List()
        if err != nil {
            return nil, formatCUEError(err)
        }

        for outputIter.Next() {
            outVal := outputIter.Value()

            caseName, err := outVal.LookupPath(cue.ParsePath("case")).String()
            if err != nil {
                return nil, formatCUEError(err)
            }

            output := ir.OutputCase{
                Case:   caseName,
                Fields: make(map[string]string),
            }

            fieldsVal := outVal.LookupPath(cue.ParsePath("fields"))
            if fieldsVal.Exists() {
                fieldsIter, err := fieldsVal.Fields()
                if err != nil {
                    return nil, formatCUEError(err)
                }

                for fieldsIter.Next() {
                    fieldName := fieldsIter.Label()
                    fieldType, err := extractTypeName(fieldsIter.Value())
                    if err != nil {
                        return nil, err
                    }
                    output.Fields[fieldName] = fieldType
                }
            }

            action.Outputs = append(action.Outputs, output)
        }

        actions = append(actions, action)
    }

    return actions, nil
}

// extractTypeName converts CUE type to IR type string
func extractTypeName(v cue.Value) (string, error) {
    // CUE types map to IR types:
    // string → "string"
    // int → "int"
    // bool → "bool"
    // [...] → "array"
    // {...} → "object"
    // float is NOT supported

    switch v.IncompleteKind() {
    case cue.StringKind:
        return "string", nil
    case cue.IntKind:
        return "int", nil
    case cue.BoolKind:
        return "bool", nil
    case cue.ListKind:
        return "array", nil
    case cue.StructKind:
        return "object", nil
    case cue.FloatKind, cue.NumberKind:
        return "", &CompileError{
            Field:   "type",
            Message: "float types are forbidden - use int instead",
            Pos:     v.Pos(),
        }
    default:
        return "", &CompileError{
            Field:   "type",
            Message: fmt.Sprintf("unsupported type kind: %v", v.IncompleteKind()),
            Pos:     v.Pos(),
        }
    }
}

// CompileError represents a compilation error with source position
type CompileError struct {
    Field   string
    Message string
    Pos     token.Pos // Use token.Pos, NOT cue.Pos (which doesn't exist)
}

func (e *CompileError) Error() string {
    if e.Pos.IsValid() {
        return fmt.Sprintf("%s:%d:%d: %s: %s",
            e.Pos.Filename(), e.Pos.Line(), e.Pos.Column(),
            e.Field, e.Message)
    }
    return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// formatCUEError extracts position info from CUE errors
func formatCUEError(err error) error {
    if err == nil {
        return nil
    }

    // CUE errors may contain multiple errors
    errs := errors.Errors(err)
    if len(errs) == 0 {
        return err
    }

    // Return first error with position info
    // errors.Positions returns []token.Pos
    firstErr := errs[0]
    positions := errors.Positions(firstErr)
    if len(positions) > 0 {
        return &CompileError{
            Field:   "cue",
            Message: firstErr.Error(),
            Pos:     positions[0], // token.Pos
        }
    }

    return err
}
```

### ConceptSpec IR Struct

```go
// internal/ir/concept.go
package ir

// ConceptSpec represents a compiled concept definition.
// Concepts define state shapes and action signatures.
type ConceptSpec struct {
    Name                 string      `json:"name"`
    Purpose              string      `json:"purpose"`
    States               []StateSpec `json:"states"`
    Actions              []ActionSig `json:"actions"`
    OperationalPrinciple string      `json:"operational_principle,omitempty"`
}

// StateSpec defines a state shape within a concept.
type StateSpec struct {
    Name   string            `json:"name"`
    Fields map[string]string `json:"fields"` // field name → type string
}
```

### Test Examples

```go
func TestCompileConceptBasic(t *testing.T) {
    ctx := cuecontext.New()
    v := ctx.CompileString(`
        concept Cart {
            purpose: "Manages shopping cart"

            state CartItem {
                item_id: string
                quantity: int
            }

            action addItem {
                args: {
                    item_id: string
                    quantity: int
                }
                outputs: [{
                    case: "Success"
                    fields: { item_id: string, new_quantity: int }
                }]
            }

            operational_principle: "Adding items increases quantity"
        }
    `)

    // Navigate to the concept
    conceptVal := v.LookupPath(cue.ParsePath("concept.Cart"))

    spec, err := CompileConcept(conceptVal)
    require.NoError(t, err)

    assert.Equal(t, "Cart", spec.Name)
    assert.Equal(t, "Manages shopping cart", spec.Purpose)
    assert.Len(t, spec.States, 1)
    assert.Len(t, spec.Actions, 1)
    assert.Equal(t, "addItem", spec.Actions[0].Name)
}

func TestCompileConceptMissingPurpose(t *testing.T) {
    ctx := cuecontext.New()
    v := ctx.CompileString(`
        concept Bad {
            action foo {
                outputs: [{ case: "Success", fields: {} }]
            }
        }
    `)

    conceptVal := v.LookupPath(cue.ParsePath("concept.Bad"))
    _, err := CompileConcept(conceptVal)

    require.Error(t, err)
    assert.Contains(t, err.Error(), "purpose")
    assert.Contains(t, err.Error(), "required")
}

func TestCompileConceptMissingActions(t *testing.T) {
    ctx := cuecontext.New()
    v := ctx.CompileString(`
        concept Empty {
            purpose: "Does nothing"
        }
    `)

    conceptVal := v.LookupPath(cue.ParsePath("concept.Empty"))
    _, err := CompileConcept(conceptVal)

    require.Error(t, err)
    assert.Contains(t, err.Error(), "action")
    assert.Contains(t, err.Error(), "required")
}

func TestCompileConceptRejectsFloat(t *testing.T) {
    ctx := cuecontext.New()
    v := ctx.CompileString(`
        concept Bad {
            purpose: "Has float"

            state Item {
                price: float  // FORBIDDEN
            }

            action buy {
                outputs: [{ case: "Success", fields: {} }]
            }
        }
    `)

    conceptVal := v.LookupPath(cue.ParsePath("concept.Bad"))
    _, err := CompileConcept(conceptVal)

    require.Error(t, err)
    assert.Contains(t, err.Error(), "float")
    assert.Contains(t, err.Error(), "forbidden")
}

func TestCompileConceptMultipleOutputCases(t *testing.T) {
    ctx := cuecontext.New()
    v := ctx.CompileString(`
        concept Inventory {
            purpose: "Tracks stock"

            action reserve {
                args: {
                    item_id: string
                    quantity: int
                }
                outputs: [{
                    case: "Success"
                    fields: { reservation_id: string }
                }, {
                    case: "InsufficientStock"
                    fields: { available: int, requested: int }
                }]
            }
        }
    `)

    conceptVal := v.LookupPath(cue.ParsePath("concept.Inventory"))
    spec, err := CompileConcept(conceptVal)

    require.NoError(t, err)
    require.Len(t, spec.Actions, 1)
    assert.Len(t, spec.Actions[0].Outputs, 2)
    assert.Equal(t, "Success", spec.Actions[0].Outputs[0].Case)
    assert.Equal(t, "InsufficientStock", spec.Actions[0].Outputs[1].Case)
}

func TestCompileConceptErrorWithLineNumber(t *testing.T) {
    ctx := cuecontext.New()
    v := ctx.CompileString(`
        concept Bad {
            purpose: 123  // wrong type
        }
    `)

    conceptVal := v.LookupPath(cue.ParsePath("concept.Bad"))
    _, err := CompileConcept(conceptVal)

    require.Error(t, err)
    // Error should contain line number info
    compileErr, ok := err.(*CompileError)
    if ok && compileErr.Pos.IsValid() {
        assert.Greater(t, compileErr.Pos.Line(), 0, "Should have line number")
    }
}

func TestCompileConceptWithRequires(t *testing.T) {
    ctx := cuecontext.New()
    v := ctx.CompileString(`
        concept Cart {
            purpose: "Manages shopping cart"

            action addItem {
                args: {
                    item_id: string
                    quantity: int
                }
                requires: ["cart:write", "inventory:read"]
                outputs: [{
                    case: "Success"
                    fields: { item_id: string, new_quantity: int }
                }]
            }
        }
    `)

    conceptVal := v.LookupPath(cue.ParsePath("concept.Cart"))
    spec, err := CompileConcept(conceptVal)

    require.NoError(t, err)
    require.Len(t, spec.Actions, 1)
    assert.Equal(t, []string{"cart:write", "inventory:read"}, spec.Actions[0].Requires)
}
```

### File List

Files to create/modify:

1. `internal/ir/concept.go` - ConceptSpec and StateSpec structs
2. `internal/compiler/concept.go` - CompileConcept function
3. `internal/compiler/concept_test.go` - Comprehensive tests
4. `go.mod` - Add CUE SDK dependency: `cuelang.org/go v0.15.1`

### Relationship to Other Stories

- **Story 1-3:** Uses ActionSig struct for action definitions
- **Story 1-7:** Similar pattern for sync rule parsing
- **Story 1-8:** ConceptSpec validated after parsing

### Story Completion Checklist

- [ ] ConceptSpec struct defined in internal/ir/concept.go
- [ ] StateSpec struct defined
- [ ] CompileConcept function implemented
- [ ] Purpose field parsing (required)
- [ ] State definitions parsing
- [ ] Action definitions parsing (uses ActionSig with Requires)
- [ ] Requires field parsing for authz hooks
- [ ] Operational principle parsing
- [ ] Float types rejected with clear error
- [ ] CUE errors include line numbers
- [ ] Missing required fields produce clear errors
- [ ] All tests pass
- [ ] `go vet ./internal/...` passes

### References

- [Source: docs/architecture.md#CUE SDK] - CUE SDK v0.15.1
- [Source: docs/epics.md#Story 1.6] - Story definition
- [Source: docs/prd.md#FR-1.1] - CUE format specification

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Validation History

- Initial creation: create-story workflow
- 2025-12-12: codex review - fixed token.Pos import (was cue.Pos), clarified CUE SDK v0.15.1 API usage
- 2025-12-12: consensus review (gemini-3-pro-preview) - added parsing for ActionSig.Requires field (authz hooks) to match Story 1-1 definition

### Completion Notes

- Uses CUE SDK's Go API directly (not CLI subprocess)
- Float types forbidden at parse time (CP-5)
- Error messages include source file positions when available
- ConceptSpec is the IR representation of a parsed concept
