# Story 1.8: IR Schema Validation

Status: done

## Story

As a **developer compiling specs**,
I want **comprehensive validation of compiled IR against schema rules**,
So that **invalid specs are caught at compile time, not runtime**.

## Acceptance Criteria

1. **`Validate(ir any) []ValidationError` in `internal/compiler/validate.go`**
   - Works with ConceptSpec and SyncRule
   - Returns all errors (not fail-fast)
   - Empty slice means valid

2. **ValidationError struct with clear information**
   ```go
   type ValidationError struct {
       Field   string `json:"field"`
       Message string `json:"message"`
       Code    string `json:"code"`    // E1XX format
       Line    int    `json:"line,omitempty"`
   }
   ```

3. **ConceptSpec validation rules**
   - `purpose` is non-empty
   - At least one `action` defined
   - Each action has at least one output case
   - State field types are valid IRValue types (string, int, bool, array, object)
   - No duplicate action or state names
   - No float types anywhere

4. **SyncRule validation rules**
   - `when` references a valid action format (Concept.action)
   - `when.event_type` is one of: "completed" or "invoked"
   - `scope` is one of: "flow", "global", or "keyed"
   - If scope is "keyed", the `key` field must be non-empty
   - `where` clause has valid `from` field if present
   - `then` references a valid action format
   - Bound variables in `then.args` are defined in `when.bindings` or `where.bindings`

5. **Error codes follow Architecture pattern**
   - E100-E199 for validation errors
   - Each error type has unique code for programmatic handling

## Quick Reference

| Pattern | Key Rule |
|---------|----------|
| **FR-1.2** | Validate against canonical IR schema |
| **E1XX** | Validation error codes |

## Tasks / Subtasks

- [ ] Task 1: Define ValidationError struct (AC: #2)
  - [ ] 1.1 Create `internal/compiler/validate.go`
  - [ ] 1.2 Define error codes E100-E199
  - [ ] 1.3 Implement Error() method

- [ ] Task 2: Implement ConceptSpec validation (AC: #3)
  - [ ] 2.1 Validate purpose non-empty (E101)
  - [ ] 2.2 Validate at least one action (E102)
  - [ ] 2.3 Validate action outputs non-empty (E103)
  - [ ] 2.4 Validate state field types (E104)
  - [ ] 2.5 Validate no duplicate names (E105)
  - [ ] 2.6 Validate no float types (E106)

- [ ] Task 3: Implement SyncRule validation (AC: #4)
  - [ ] 3.1 Validate when action reference format (E110)
  - [ ] 3.2 Validate scope mode (E111)
  - [ ] 3.3 Validate keyed scope has non-empty key (E111)
  - [ ] 3.4 Validate where clause if present (E112)
  - [ ] 3.5 Validate then action reference format (E113)
  - [ ] 3.6 Validate bound variables defined (E114)
  - [ ] 3.7 Validate event type is "completed" or "invoked" (E116)

- [ ] Task 4: Implement cross-validation helpers
  - [ ] 4.1 Action reference format validator
  - [ ] 4.2 Bound variable collector
  - [ ] 4.3 Type string validator

- [ ] Task 5: Write comprehensive tests
  - [ ] 5.1 Test valid specs pass
  - [ ] 5.2 Test each ConceptSpec rule
  - [ ] 5.3 Test each SyncRule rule
  - [ ] 5.4 Test error codes correct
  - [ ] 5.5 Test multiple errors collected

## Dev Notes

### Error Codes

```go
// Validation error codes (E100-E199)
const (
    // ConceptSpec errors (E100-E109)
    ErrConceptPurposeEmpty       = "E101" // purpose is required
    ErrConceptNoActions          = "E102" // at least one action required
    ErrActionNoOutputs           = "E103" // action must have outputs
    ErrInvalidFieldType          = "E104" // invalid type string
    ErrDuplicateName             = "E105" // duplicate action/state name
    ErrFloatTypeForbidden        = "E106" // float types not allowed

    // SyncRule errors (E110-E119)
    ErrInvalidActionRef          = "E110" // invalid action reference format
    ErrInvalidScopeMode          = "E111" // invalid scope mode or missing keyed key
    ErrInvalidWhereClause        = "E112" // invalid where clause
    ErrInvalidThenClause         = "E113" // invalid then clause
    ErrUndefinedBoundVariable    = "E114" // bound variable not defined
    ErrMissingSyncClause         = "E115" // missing required clause
    ErrInvalidEventType          = "E116" // invalid event type (must be "completed" or "invoked")
)
```

### Validation Implementation

```go
// internal/compiler/validate.go
package compiler

import (
    "fmt"
    "regexp"
    "strings"

    "github.com/your-org/nysm/internal/ir"
)

// ValidationError represents a schema validation error.
type ValidationError struct {
    Field   string `json:"field"`
    Message string `json:"message"`
    Code    string `json:"code"`
    Line    int    `json:"line,omitempty"`
}

func (e ValidationError) Error() string {
    if e.Line > 0 {
        return fmt.Sprintf("[%s] line %d: %s: %s", e.Code, e.Line, e.Field, e.Message)
    }
    return fmt.Sprintf("[%s] %s: %s", e.Code, e.Field, e.Message)
}

// Validate validates compiled IR against schema rules.
// Returns all errors found (does not fail-fast).
func Validate(v any) []ValidationError {
    switch ir := v.(type) {
    case *ir.ConceptSpec:
        return validateConceptSpec(ir)
    case ir.ConceptSpec:
        return validateConceptSpec(&ir)
    case *ir.SyncRule:
        return validateSyncRule(ir)
    case ir.SyncRule:
        return validateSyncRule(&ir)
    default:
        return []ValidationError{{
            Field:   "type",
            Message: fmt.Sprintf("unsupported IR type: %T", v),
            Code:    "E100",
        }}
    }
}

// ValidateConceptSpec validates a concept specification.
func validateConceptSpec(spec *ir.ConceptSpec) []ValidationError {
    var errs []ValidationError

    // E101: purpose is required
    if strings.TrimSpace(spec.Purpose) == "" {
        errs = append(errs, ValidationError{
            Field:   "purpose",
            Message: "purpose is required and must be non-empty",
            Code:    ErrConceptPurposeEmpty,
        })
    }

    // E102: at least one action required
    if len(spec.Actions) == 0 {
        errs = append(errs, ValidationError{
            Field:   "actions",
            Message: "at least one action is required",
            Code:    ErrConceptNoActions,
        })
    }

    // Track names for duplicate detection
    actionNames := make(map[string]bool)
    stateNames := make(map[string]bool)

    // Validate actions
    for i, action := range spec.Actions {
        // E105: duplicate action name
        if actionNames[action.Name] {
            errs = append(errs, ValidationError{
                Field:   fmt.Sprintf("actions[%d].name", i),
                Message: fmt.Sprintf("duplicate action name: %q", action.Name),
                Code:    ErrDuplicateName,
            })
        }
        actionNames[action.Name] = true

        // E103: action must have outputs
        if len(action.Outputs) == 0 {
            errs = append(errs, ValidationError{
                Field:   fmt.Sprintf("actions[%d].outputs", i),
                Message: fmt.Sprintf("action %q must have at least one output case", action.Name),
                Code:    ErrActionNoOutputs,
            })
        }

        // Validate arg types
        for j, arg := range action.Args {
            if !isValidType(arg.Type) {
                errs = append(errs, ValidationError{
                    Field:   fmt.Sprintf("actions[%d].args[%d].type", i, j),
                    Message: fmt.Sprintf("invalid type %q for arg %q", arg.Type, arg.Name),
                    Code:    ErrInvalidFieldType,
                })
            }
            // E106: float forbidden
            if arg.Type == "float" || arg.Type == "float64" || arg.Type == "number" {
                errs = append(errs, ValidationError{
                    Field:   fmt.Sprintf("actions[%d].args[%d].type", i, j),
                    Message: fmt.Sprintf("float type forbidden for arg %q, use int instead", arg.Name),
                    Code:    ErrFloatTypeForbidden,
                })
            }
        }

        // Validate output field types
        for j, out := range action.Outputs {
            for fieldName, fieldType := range out.Fields {
                if !isValidType(fieldType) {
                    errs = append(errs, ValidationError{
                        Field:   fmt.Sprintf("actions[%d].outputs[%d].fields.%s", i, j, fieldName),
                        Message: fmt.Sprintf("invalid type %q for field %q", fieldType, fieldName),
                        Code:    ErrInvalidFieldType,
                    })
                }
                if fieldType == "float" || fieldType == "float64" || fieldType == "number" {
                    errs = append(errs, ValidationError{
                        Field:   fmt.Sprintf("actions[%d].outputs[%d].fields.%s", i, j, fieldName),
                        Message: fmt.Sprintf("float type forbidden for field %q", fieldName),
                        Code:    ErrFloatTypeForbidden,
                    })
                }
            }
        }
    }

    // Validate states
    for i, state := range spec.States {
        // E105: duplicate state name
        if stateNames[state.Name] {
            errs = append(errs, ValidationError{
                Field:   fmt.Sprintf("states[%d].name", i),
                Message: fmt.Sprintf("duplicate state name: %q", state.Name),
                Code:    ErrDuplicateName,
            })
        }
        stateNames[state.Name] = true

        // Validate field types
        for fieldName, fieldType := range state.Fields {
            if !isValidType(fieldType) {
                errs = append(errs, ValidationError{
                    Field:   fmt.Sprintf("states[%d].fields.%s", i, fieldName),
                    Message: fmt.Sprintf("invalid type %q for field %q", fieldType, fieldName),
                    Code:    ErrInvalidFieldType,
                })
            }
            if fieldType == "float" || fieldType == "float64" || fieldType == "number" {
                errs = append(errs, ValidationError{
                    Field:   fmt.Sprintf("states[%d].fields.%s", i, fieldName),
                    Message: fmt.Sprintf("float type forbidden for field %q", fieldName),
                    Code:    ErrFloatTypeForbidden,
                })
            }
        }
    }

    return errs
}

// validateSyncRule validates a sync rule specification.
func validateSyncRule(rule *ir.SyncRule) []ValidationError {
    var errs []ValidationError

    // E111: validate scope mode
    if !isValidScopeMode(rule.Scope.Mode) {
        errs = append(errs, ValidationError{
            Field:   "scope",
            Message: fmt.Sprintf("invalid scope mode %q, must be 'flow', 'global', or 'keyed'", rule.Scope.Mode),
            Code:    ErrInvalidScopeMode,
        })
    }

    // E111: keyed scope must have non-empty key
    if rule.Scope.Mode == "keyed" && strings.TrimSpace(rule.Scope.Key) == "" {
        errs = append(errs, ValidationError{
            Field:   "scope.key",
            Message: "keyed scope requires a non-empty key field",
            Code:    ErrInvalidScopeMode,
        })
    }

    // E110: validate when action reference
    if !isValidActionRef(rule.When.ActionRef) {
        errs = append(errs, ValidationError{
            Field:   "when.action_ref",
            Message: fmt.Sprintf("invalid action reference %q, expected format 'Concept.action'", rule.When.ActionRef),
            Code:    ErrInvalidActionRef,
        })
    }

    // E116: validate event type
    if rule.When.EventType != "completed" && rule.When.EventType != "invoked" {
        errs = append(errs, ValidationError{
            Field:   "when.event_type",
            Message: fmt.Sprintf("invalid event type %q, must be 'completed' or 'invoked'", rule.When.EventType),
            Code:    ErrInvalidEventType,
        })
    }

    // E113: validate then action reference
    if !isValidActionRef(rule.Then.ActionRef) {
        errs = append(errs, ValidationError{
            Field:   "then.action_ref",
            Message: fmt.Sprintf("invalid action reference %q, expected format 'Concept.action'", rule.Then.ActionRef),
            Code:    ErrInvalidActionRef,
        })
    }

    // E112: validate where clause if present
    if rule.Where != nil {
        if strings.TrimSpace(rule.Where.Source) == "" {
            errs = append(errs, ValidationError{
                Field:   "where.source",
                Message: "where clause requires non-empty 'from' source",
                Code:    ErrInvalidWhereClause,
            })
        }
    }

    // E114: validate bound variables in then.args are defined
    definedVars := collectBoundVariables(rule)
    for argName, argExpr := range rule.Then.Args {
        usedVars := extractBoundVariableRefs(argExpr)
        for _, usedVar := range usedVars {
            if !definedVars[usedVar] {
                errs = append(errs, ValidationError{
                    Field:   fmt.Sprintf("then.args.%s", argName),
                    Message: fmt.Sprintf("undefined bound variable %q in expression %q", usedVar, argExpr),
                    Code:    ErrUndefinedBoundVariable,
                })
            }
        }
    }

    return errs
}

// isValidType checks if a type string is valid
func isValidType(t string) bool {
    validTypes := map[string]bool{
        "string": true,
        "int":    true,
        "bool":   true,
        "array":  true,
        "object": true,
    }
    return validTypes[t]
}

// isValidScopeMode checks if a scope mode is valid
func isValidScopeMode(mode string) bool {
    return mode == "flow" || mode == "global" || mode == "keyed"
}

// actionRefPattern matches "Concept.action" format
var actionRefPattern = regexp.MustCompile(`^[A-Z][a-zA-Z0-9]*\.[a-z][a-zA-Z0-9]*$`)

// isValidActionRef checks if an action reference has valid format
func isValidActionRef(ref string) bool {
    return actionRefPattern.MatchString(ref)
}

// collectBoundVariables returns all variable names defined in when and where bindings
func collectBoundVariables(rule *ir.SyncRule) map[string]bool {
    vars := make(map[string]bool)

    for varName := range rule.When.Bindings {
        vars[varName] = true
    }

    if rule.Where != nil {
        for varName := range rule.Where.Bindings {
            vars[varName] = true
        }
    }

    return vars
}

// boundVarPattern matches "bound.variable_name" references
var boundVarPattern = regexp.MustCompile(`bound\.([a-zA-Z_][a-zA-Z0-9_]*)`)

// extractBoundVariableRefs extracts bound variable names from an expression
func extractBoundVariableRefs(expr string) []string {
    matches := boundVarPattern.FindAllStringSubmatch(expr, -1)
    vars := make([]string, 0, len(matches))
    for _, match := range matches {
        if len(match) > 1 {
            vars = append(vars, match[1])
        }
    }
    return vars
}
```

### Test Examples

```go
func TestValidateConceptSpecValid(t *testing.T) {
    spec := &ir.ConceptSpec{
        Name:    "Cart",
        Purpose: "Manages shopping cart",
        Actions: []ir.ActionSig{
            {
                Name: "addItem",
                Args: []ir.NamedArg{{Name: "item_id", Type: "string"}},
                Outputs: []ir.OutputCase{
                    {Case: "Success", Fields: map[string]string{"id": "string"}},
                },
            },
        },
    }

    errs := Validate(spec)
    assert.Empty(t, errs, "valid spec should have no errors")
}

func TestValidateConceptSpecMissingPurpose(t *testing.T) {
    spec := &ir.ConceptSpec{
        Name:    "Bad",
        Purpose: "",  // Missing
        Actions: []ir.ActionSig{
            {
                Name:    "foo",
                Outputs: []ir.OutputCase{{Case: "Success"}},
            },
        },
    }

    errs := Validate(spec)
    require.Len(t, errs, 1)
    assert.Equal(t, ErrConceptPurposeEmpty, errs[0].Code)
    assert.Contains(t, errs[0].Message, "purpose")
}

func TestValidateConceptSpecNoActions(t *testing.T) {
    spec := &ir.ConceptSpec{
        Name:    "Empty",
        Purpose: "Does nothing",
        Actions: []ir.ActionSig{}, // No actions
    }

    errs := Validate(spec)
    require.Len(t, errs, 1)
    assert.Equal(t, ErrConceptNoActions, errs[0].Code)
}

func TestValidateConceptSpecFloatForbidden(t *testing.T) {
    spec := &ir.ConceptSpec{
        Name:    "Bad",
        Purpose: "Has float",
        Actions: []ir.ActionSig{
            {
                Name: "buy",
                Args: []ir.NamedArg{{Name: "price", Type: "float"}}, // Forbidden!
                Outputs: []ir.OutputCase{{Case: "Success"}},
            },
        },
    }

    errs := Validate(spec)
    require.Len(t, errs, 2) // Invalid type + float forbidden

    codes := make(map[string]bool)
    for _, e := range errs {
        codes[e.Code] = true
    }
    assert.True(t, codes[ErrFloatTypeForbidden] || codes[ErrInvalidFieldType])
}

func TestValidateConceptSpecDuplicateAction(t *testing.T) {
    spec := &ir.ConceptSpec{
        Name:    "Dup",
        Purpose: "Has duplicates",
        Actions: []ir.ActionSig{
            {Name: "foo", Outputs: []ir.OutputCase{{Case: "Success"}}},
            {Name: "foo", Outputs: []ir.OutputCase{{Case: "Success"}}}, // Duplicate
        },
    }

    errs := Validate(spec)
    require.Len(t, errs, 1)
    assert.Equal(t, ErrDuplicateName, errs[0].Code)
}

func TestValidateSyncRuleValid(t *testing.T) {
    rule := &ir.SyncRule{
        ID:    "test",
        Scope: ir.ScopeSpec{Mode: "flow"},
        When: ir.WhenClause{
            ActionRef: "Cart.checkout",
            EventType: "completed",
            Bindings:  map[string]string{"cart_id": "result.cart_id"},
        },
        Then: ir.ThenClause{
            ActionRef: "Inventory.reserve",
            Args:      map[string]string{"id": "bound.cart_id"},
        },
    }

    errs := Validate(rule)
    assert.Empty(t, errs, "valid rule should have no errors")
}

func TestValidateSyncRuleInvalidScope(t *testing.T) {
    rule := &ir.SyncRule{
        ID:    "bad",
        Scope: ir.ScopeSpec{Mode: "invalid"},
        When:  ir.WhenClause{ActionRef: "Cart.checkout", EventType: "completed"},
        Then:  ir.ThenClause{ActionRef: "Inventory.reserve"},
    }

    errs := Validate(rule)
    require.Len(t, errs, 1)
    assert.Equal(t, ErrInvalidScopeMode, errs[0].Code)
}

func TestValidateSyncRuleKeyedScopeMissingKey(t *testing.T) {
    rule := &ir.SyncRule{
        ID:    "bad",
        Scope: ir.ScopeSpec{Mode: "keyed", Key: ""}, // Missing key!
        When:  ir.WhenClause{ActionRef: "Cart.checkout", EventType: "completed"},
        Then:  ir.ThenClause{ActionRef: "Inventory.reserve"},
    }

    errs := Validate(rule)
    require.Len(t, errs, 1)
    assert.Equal(t, ErrInvalidScopeMode, errs[0].Code)
    assert.Contains(t, errs[0].Message, "key")
}

func TestValidateSyncRuleInvalidEventType(t *testing.T) {
    rule := &ir.SyncRule{
        ID:    "bad",
        Scope: ir.ScopeSpec{Mode: "flow"},
        When:  ir.WhenClause{ActionRef: "Cart.checkout", EventType: "started"}, // Invalid!
        Then:  ir.ThenClause{ActionRef: "Inventory.reserve"},
    }

    errs := Validate(rule)
    require.Len(t, errs, 1)
    assert.Equal(t, ErrInvalidEventType, errs[0].Code)
}

func TestValidateSyncRuleInvalidActionRef(t *testing.T) {
    rule := &ir.SyncRule{
        ID:    "bad",
        Scope: ir.ScopeSpec{Mode: "flow"},
        When:  ir.WhenClause{ActionRef: "invalid-format", EventType: "completed"}, // Wrong format!
        Then:  ir.ThenClause{ActionRef: "Inventory.reserve"},
    }

    errs := Validate(rule)
    require.Len(t, errs, 1)
    assert.Equal(t, ErrInvalidActionRef, errs[0].Code)
}

func TestValidateSyncRuleUndefinedBoundVar(t *testing.T) {
    rule := &ir.SyncRule{
        ID:    "bad",
        Scope: ir.ScopeSpec{Mode: "flow"},
        When: ir.WhenClause{
            ActionRef: "Cart.checkout",
            EventType: "completed",
            Bindings:  map[string]string{"cart_id": "result.cart_id"},
        },
        Then: ir.ThenClause{
            ActionRef: "Inventory.reserve",
            Args:      map[string]string{"id": "bound.undefined_var"}, // Not defined!
        },
    }

    errs := Validate(rule)
    require.Len(t, errs, 1)
    assert.Equal(t, ErrUndefinedBoundVariable, errs[0].Code)
    assert.Contains(t, errs[0].Message, "undefined_var")
}

func TestValidateCollectsAllErrors(t *testing.T) {
    spec := &ir.ConceptSpec{
        Name:    "",  // Missing name is OK, but purpose...
        Purpose: "",  // E101
        Actions: []ir.ActionSig{
            {
                Name:    "foo",
                Args:    []ir.NamedArg{{Name: "x", Type: "float"}}, // E104 + E106
                Outputs: []ir.OutputCase{}, // E103
            },
            {
                Name:    "foo", // E105 duplicate
                Outputs: []ir.OutputCase{{Case: "Success"}},
            },
        },
    }

    errs := Validate(spec)
    assert.GreaterOrEqual(t, len(errs), 4, "should collect multiple errors")
}
```

### File List

Files to create/modify:

1. `internal/compiler/validate.go` - Validation implementation
2. `internal/compiler/validate_test.go` - Comprehensive tests
3. `internal/compiler/errors.go` - Error code constants (optional, can be in validate.go)

### Relationship to Other Stories

- **Story 1-3:** ActionSig validated here
- **Story 1-6:** ConceptSpec from parser validated here
- **Story 1-7:** SyncRule from parser validated here

### Story Completion Checklist

- [ ] ValidationError struct defined with Code, Field, Message, Line
- [ ] Error codes E100-E199 defined
- [ ] Validate() function dispatches to correct validator
- [ ] ConceptSpec validation: purpose, actions, outputs, types, duplicates, no floats
- [ ] SyncRule validation: scope, action refs, bound variables
- [ ] All errors collected (not fail-fast)
- [ ] Error codes match Architecture pattern
- [ ] All tests pass
- [ ] `go vet ./internal/...` passes

### References

- [Source: docs/architecture.md#Error Codes] - E1XX validation errors
- [Source: docs/epics.md#Story 1.8] - Story definition
- [Source: docs/prd.md#FR-1.2] - Validate against canonical IR schema

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Validation History

- Initial creation: create-story workflow
- 2025-12-12: codex review - added event type validation (E116), added keyed scope key validation, updated tests to include EventType field

### Completion Notes

- Validation is non-fail-fast: collects ALL errors
- Error codes enable programmatic error handling
- Bound variable validation prevents runtime errors in sync execution
- Float type rejection enforced at validation layer (defense in depth)
