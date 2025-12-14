# Story 1.3: Typed Action Outputs with Error Variants

Status: done

## Story

As a **developer defining concepts**,
I want **actions to have typed output cases including success and error variants**,
So that **sync rules can match on specific outcomes**.

## Acceptance Criteria

1. **ActionSig struct defined in `internal/ir/action.go`**
   ```go
   type ActionSig struct {
       Name     string       `json:"name"`
       Args     []NamedArg   `json:"args"`
       Outputs  []OutputCase `json:"outputs"`
       Requires []string     `json:"requires,omitempty"` // Required permissions (authz)
   }
   ```

2. **NamedArg struct for typed arguments**
   ```go
   type NamedArg struct {
       Name string  `json:"name"`
       Type string  `json:"type"` // "string", "int", "bool", "array", "object"
   }
   ```

3. **OutputCase struct for typed outputs**
   ```go
   type OutputCase struct {
       Case   string              `json:"case"`   // "Success", "InsufficientStock", etc.
       Fields map[string]string   `json:"fields"` // field name -> type
   }
   ```

4. **Validation rules**
   - At least one output case is required (empty outputs is an error)
   - Output case names must be unique within an action
   - The "Success" case is conventionally first but not enforced
   - Field names within an OutputCase must be unique
   - Type strings must be valid: "string", "int", "bool", "array", "object"

5. **Error variants can have their own typed fields**
   - Example: `InsufficientStock` has `available`, `requested` fields
   - Example: `InvalidQuantity` has `message`, `max_allowed` fields

6. **JSON marshaling uses RFC 8785 key ordering**
   - ActionSig, OutputCase marshal with sorted keys for determinism

## Quick Reference

| Pattern | Key Rule |
|---------|----------|
| **CP-5** | Only constrained IRValue types (no floats, no unconstrained any) |
| **FR-1.4** | Typed action outputs with multiple cases |
| **MEDIUM-2** | Error matching in when-clause |

## Tasks / Subtasks

- [ ] Task 1: Define ActionSig struct (AC: #1)
  - [ ] 1.1 Create `internal/ir/action.go`
  - [ ] 1.2 Define ActionSig with Name, Args, Outputs, Requires fields
  - [ ] 1.3 Add JSON tags for serialization

- [ ] Task 2: Define NamedArg struct (AC: #2)
  - [ ] 2.1 Define NamedArg with Name and Type fields
  - [ ] 2.2 Define valid type constants
  - [ ] 2.3 Add type validation helper

- [ ] Task 3: Define OutputCase struct (AC: #3)
  - [ ] 3.1 Define OutputCase with Case and Fields
  - [ ] 3.2 Fields uses map[string]string (field name -> type string)

- [ ] Task 4: Implement validation (AC: #4)
  - [ ] 4.1 Validate at least one output case required
  - [ ] 4.2 Validate unique output case names
  - [ ] 4.3 Validate unique field names within OutputCase
  - [ ] 4.4 Validate type strings are valid

- [ ] Task 5: Implement sorted JSON marshaling (AC: #6)
  - [ ] 5.1 ActionSig.MarshalJSON with sorted keys
  - [ ] 5.2 OutputCase.MarshalJSON with sorted keys
  - [ ] 5.3 Use compareKeysRFC8785 from value.go

- [ ] Task 6: Write comprehensive tests
  - [ ] 6.1 Test valid ActionSig creation
  - [ ] 6.2 Test validation errors (empty outputs, duplicate cases)
  - [ ] 6.3 Test JSON round-trip with sorted keys
  - [ ] 6.4 Test error variant with typed fields

## Dev Notes

### Type System Design

The type strings map to IRValue types from Story 1-2:
- `"string"` → IRString
- `"int"` → IRInt (int64, NO float)
- `"bool"` → IRBool
- `"array"` → IRArray
- `"object"` → IRObject

**IMPORTANT:** There is NO `"float"` type. This enforces CP-5 at the schema level.

### Example ActionSig

```go
// Cart.addItem action signature
addItemAction := ActionSig{
    Name: "addItem",
    Args: []NamedArg{
        {Name: "item_id", Type: "string"},
        {Name: "quantity", Type: "int"},
    },
    Requires: []string{"cart:write"}, // Required permissions for authz
    Outputs: []OutputCase{
        {
            Case: "Success",
            Fields: map[string]string{
                "item_id":      "string",
                "new_quantity": "int",
            },
        },
        {
            Case: "InsufficientStock",
            Fields: map[string]string{
                "available": "int",
                "requested": "int",
            },
        },
        {
            Case: "InvalidQuantity",
            Fields: map[string]string{
                "message":     "string",
                "max_allowed": "int",
            },
        },
    },
}
```

### Validation Implementation

```go
// internal/ir/action.go

// ValidTypes defines the allowed type strings
var ValidTypes = map[string]bool{
    "string": true,
    "int":    true,
    "bool":   true,
    "array":  true,
    "object": true,
    // NO "float" - floats forbidden per CP-5
}

// Validate checks ActionSig against schema rules
func (a *ActionSig) Validate() []ValidationError {
    var errs []ValidationError

    // Rule: At least one output case required
    if len(a.Outputs) == 0 {
        errs = append(errs, ValidationError{
            Field:   "outputs",
            Message: "at least one output case is required",
        })
    }

    // Rule: Unique output case names
    seenCases := make(map[string]bool)
    for i, out := range a.Outputs {
        if seenCases[out.Case] {
            errs = append(errs, ValidationError{
                Field:   fmt.Sprintf("outputs[%d].case", i),
                Message: fmt.Sprintf("duplicate output case name: %q", out.Case),
            })
        }
        seenCases[out.Case] = true

        // Rule: Unique field names within OutputCase
        seenFields := make(map[string]bool)
        for fieldName, fieldType := range out.Fields {
            if seenFields[fieldName] {
                errs = append(errs, ValidationError{
                    Field:   fmt.Sprintf("outputs[%d].fields.%s", i, fieldName),
                    Message: fmt.Sprintf("duplicate field name: %q", fieldName),
                })
            }
            seenFields[fieldName] = true

            // Rule: Valid type strings
            if !ValidTypes[fieldType] {
                errs = append(errs, ValidationError{
                    Field:   fmt.Sprintf("outputs[%d].fields.%s", i, fieldName),
                    Message: fmt.Sprintf("invalid type %q, must be one of: string, int, bool, array, object", fieldType),
                })
            }
        }
    }

    // Validate args types
    for i, arg := range a.Args {
        if !ValidTypes[arg.Type] {
            errs = append(errs, ValidationError{
                Field:   fmt.Sprintf("args[%d].type", i),
                Message: fmt.Sprintf("invalid type %q for arg %q", arg.Type, arg.Name),
            })
        }
    }

    return errs
}
```

### Sorted JSON Marshaling

**NOTE:** This uses the same non-canonical marshaling as Story 1-2. For content-addressed
hashing, use `MarshalCanonical` from Story 1-4.

**IMPORTANT:** Do NOT use `fmt.Sprintf("%q")` for JSON strings - it produces Go string
literals, not JSON escaping. Use `encoding/json` for all string encoding.

```go
// MarshalJSON produces JSON with sorted keys for determinism
func (a ActionSig) MarshalJSON() ([]byte, error) {
    var buf bytes.Buffer
    buf.WriteByte('{')

    // Fixed field order: args, name, outputs, requires (alphabetical)
    buf.WriteString(`"args":`)
    argsBytes, err := json.Marshal(a.Args)
    if err != nil {
        return nil, err
    }
    buf.Write(argsBytes)

    buf.WriteString(`,"name":`)
    nameBytes, err := json.Marshal(a.Name) // Use json.Marshal for proper escaping
    if err != nil {
        return nil, err
    }
    buf.Write(nameBytes)

    buf.WriteString(`,"outputs":`)
    outputsBytes, err := json.Marshal(a.Outputs)
    if err != nil {
        return nil, err
    }
    buf.Write(outputsBytes)

    // Only include requires if non-empty (omitempty behavior)
    if len(a.Requires) > 0 {
        buf.WriteString(`,"requires":`)
        requiresBytes, err := json.Marshal(a.Requires)
        if err != nil {
            return nil, err
        }
        buf.Write(requiresBytes)
    }

    buf.WriteByte('}')
    return buf.Bytes(), nil
}

// MarshalJSON produces JSON with sorted field keys
func (o OutputCase) MarshalJSON() ([]byte, error) {
    var buf bytes.Buffer
    buf.WriteByte('{')

    buf.WriteString(`"case":`)
    caseBytes, err := json.Marshal(o.Case) // Use json.Marshal, NOT fmt.Sprintf("%q")
    if err != nil {
        return nil, err
    }
    buf.Write(caseBytes)

    buf.WriteString(`,"fields":{`)

    // Sort field keys using RFC 8785 ordering
    keys := make([]string, 0, len(o.Fields))
    for k := range o.Fields {
        keys = append(keys, k)
    }
    slices.SortFunc(keys, compareKeysRFC8785)

    for i, k := range keys {
        if i > 0 {
            buf.WriteByte(',')
        }
        // Use json.Marshal for both key and value - proper JSON escaping
        keyBytes, err := json.Marshal(k)
        if err != nil {
            return nil, err
        }
        valBytes, err := json.Marshal(o.Fields[k])
        if err != nil {
            return nil, err
        }
        buf.Write(keyBytes)
        buf.WriteByte(':')
        buf.Write(valBytes)
    }
    buf.WriteString("}}")

    return buf.Bytes(), nil
}
```

### Test Examples

```go
func TestActionSigValidation(t *testing.T) {
    tests := []struct {
        name     string
        action   ActionSig
        wantErrs int
    }{
        {
            name: "valid action",
            action: ActionSig{
                Name: "checkout",
                Args: []NamedArg{{Name: "cart_id", Type: "string"}},
                Outputs: []OutputCase{
                    {Case: "Success", Fields: map[string]string{"order_id": "string"}},
                },
            },
            wantErrs: 0,
        },
        {
            name: "empty outputs",
            action: ActionSig{
                Name:    "invalid",
                Args:    []NamedArg{},
                Outputs: []OutputCase{},
            },
            wantErrs: 1, // "at least one output case required"
        },
        {
            name: "duplicate case names",
            action: ActionSig{
                Name: "bad",
                Outputs: []OutputCase{
                    {Case: "Success", Fields: map[string]string{}},
                    {Case: "Success", Fields: map[string]string{}}, // duplicate
                },
            },
            wantErrs: 1, // "duplicate output case name"
        },
        {
            name: "invalid type",
            action: ActionSig{
                Name: "bad",
                Args: []NamedArg{{Name: "price", Type: "float"}}, // float forbidden!
                Outputs: []OutputCase{
                    {Case: "Success", Fields: map[string]string{}},
                },
            },
            wantErrs: 1, // "invalid type float"
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            errs := tt.action.Validate()
            assert.Len(t, errs, tt.wantErrs)
        })
    }
}

func TestOutputCaseJSONSortedKeys(t *testing.T) {
    out := OutputCase{
        Case: "InsufficientStock",
        Fields: map[string]string{
            "requested": "int",
            "available": "int",
        },
    }

    data, err := json.Marshal(out)
    require.NoError(t, err)

    // Fields should be sorted: available before requested
    expected := `{"case":"InsufficientStock","fields":{"available":"int","requested":"int"}}`
    assert.Equal(t, expected, string(data))
}

func TestErrorVariantWithTypedFields(t *testing.T) {
    // Verify error variants can have rich typed fields
    action := ActionSig{
        Name: "reserve",
        Args: []NamedArg{
            {Name: "item_id", Type: "string"},
            {Name: "quantity", Type: "int"},
        },
        Outputs: []OutputCase{
            {
                Case: "Success",
                Fields: map[string]string{
                    "reservation_id": "string",
                    "expires_at":     "int", // Unix timestamp as int, not float
                },
            },
            {
                Case: "InsufficientStock",
                Fields: map[string]string{
                    "available": "int",
                    "requested": "int",
                    "item_name": "string",
                },
            },
        },
    }

    errs := action.Validate()
    assert.Empty(t, errs, "error variant with typed fields should validate")
}
```

### File List

Files to create/modify:

1. `internal/ir/action.go` - ActionSig, NamedArg, OutputCase structs
2. `internal/ir/action_test.go` - Comprehensive tests

### Relationship to Other Stories

- **Story 1-2:** Uses the type system (IRValue types map to type strings)
- **Story 1-6:** CUE parser will produce ActionSig from concept specs
- **Story 1-7:** Sync rules match on OutputCase.Case for error handling

### Story Completion Checklist

- [ ] ActionSig struct defined with Name, Args, Outputs, Requires
- [ ] NamedArg struct defined with Name, Type
- [ ] OutputCase struct defined with Case, Fields
- [ ] ValidTypes constant excludes "float"
- [ ] Validate() returns all errors (not fail-fast)
- [ ] At least one output case required
- [ ] Unique case names enforced
- [ ] Valid type strings enforced
- [ ] JSON marshaling uses sorted keys
- [ ] All tests pass
- [ ] `go vet ./internal/ir/...` passes

### References

- [Source: docs/architecture.md#CP-5] - Constrained value types, no floats
- [Source: docs/epics.md#Story 1.3] - Story definition
- [Source: docs/prd.md#FR-1.4] - Typed action outputs with multiple cases
- [Source: docs/prd.md#MEDIUM-2] - Error matching in when-clause

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Validation History

- Initial creation: create-story workflow
- 2025-12-12: codex review - fixed MarshalJSON to use json.Marshal instead of fmt.Sprintf("%q") for proper JSON escaping
- 2025-12-12: consensus review (gemini-3-pro-preview) - added Requires []string field for authz hooks to match Story 1-1 definition

### Completion Notes

- Builds on Story 1-2's IRValue type system
- Type strings ("string", "int", "bool", "array", "object") map to IRValue types
- NO "float" type - enforces CP-5 at schema level
- Error variants enable sync rules to match on specific outcomes (MEDIUM-2)
