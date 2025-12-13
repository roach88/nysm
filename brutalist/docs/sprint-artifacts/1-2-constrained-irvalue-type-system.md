# Story 1.2: Constrained IRValue Type System

Status: ready-for-dev

## Story

As a **developer building NYSM**,
I want **a constrained value type system that forbids floats**,
So that **all IR values are deterministically serializable**.

## Acceptance Criteria

1. **IRValue sealed interface defined in `internal/ir/value.go`**
   - Sealed interface with unexported `irValue()` marker method
   - ONLY these types implement IRValue: IRString, IRInt, IRBool, IRArray, IRObject
   - NO IRFloat - floats are FORBIDDEN in IR (breaks determinism)

2. **Type implementations are correct**
   ```go
   type IRString string      // Wraps string
   type IRInt int64          // Wraps int64 (NO float64)
   type IRBool bool          // Wraps bool
   type IRArray []IRValue    // Recursive slice of IRValue
   type IRObject map[string]IRValue  // Recursive map of IRValue
   ```

3. **IRObject deterministic iteration via SortedKeys()**
   - Uses RFC 8785 UTF-16 code unit ordering (NOT Go's default UTF-8)
   - Helper: `func (obj IRObject) SortedKeys() []string`
   - All map iteration in NYSM MUST use this helper

4. **JSON marshaling/unmarshaling for IRValue types**
   - Custom `MarshalJSON`/`UnmarshalJSON` for type-safe round-tripping
   - IRObject marshals with sorted keys (RFC 8785 ordering)
   - Unmarshaling rejects floats AND null - only string/int/bool/array/object allowed
   - **NOTE:** This is NOT canonical marshaling (may have HTML escaping, etc.)
   - Story 1.4 will implement `MarshalCanonical` for content-addressed hashing

5. **Helper constructors for ergonomic IRValue creation**
   - `NewIRString(s string) IRString`
   - `NewIRInt(n int64) IRInt`
   - `NewIRBool(b bool) IRBool`
   - `NewIRArray(vals ...IRValue) IRArray`
   - `NewIRObjectFromMap(m map[string]IRValue) IRObject` (from existing map)
   - `IRPair{Key string, Value IRValue}` struct for typed construction

6. **Comprehensive test coverage in `internal/ir/value_test.go`**
   - Sealed interface test (only allowed types implement)
   - JSON round-trip tests for all types
   - Float rejection test (JSON with floats must error)
   - Deterministic ordering test (sorted keys)
   - Nested structure tests (arrays of objects, objects of arrays)

## Quick Reference

| Pattern | Key Rule |
|---------|----------|
| **CP-3** | RFC 8785 UTF-16 code unit key ordering (not UTF-8 bytes) |
| **CP-5** | NO floats in IRValue - only string, int64, bool, array, object |

## Tasks / Subtasks

- [ ] Task 1: Implement IRValue sealed interface (AC: #1)
  - [ ] 1.1 Define `IRValue` interface with unexported `irValue()` marker
  - [ ] 1.2 Add package-level documentation explaining sealed pattern

- [ ] Task 2: Implement concrete IRValue types (AC: #2)
  - [ ] 2.1 Implement `IRString` with `irValue()` method
  - [ ] 2.2 Implement `IRInt` with `irValue()` method
  - [ ] 2.3 Implement `IRBool` with `irValue()` method
  - [ ] 2.4 Implement `IRArray` with `irValue()` method
  - [ ] 2.5 Implement `IRObject` with `irValue()` method

- [ ] Task 3: Implement RFC 8785 UTF-16 key ordering (AC: #3)
  - [ ] 3.1 Implement `compareKeysRFC8785(a, b string) int` comparator
  - [ ] 3.2 Implement `func (obj IRObject) SortedKeys() []string`
  - [ ] 3.3 Add tests verifying UTF-16 ordering differs from UTF-8

- [ ] Task 4: Implement JSON marshaling (AC: #4)
  - [ ] 4.1 Implement `MarshalJSON()` for IRObject with sorted keys
  - [ ] 4.2 Implement `MarshalJSON()` for IRArray (delegates to MarshalIRValue for elements)
  - [ ] 4.3 Implement helper `MarshalIRValue(v IRValue) ([]byte, error)` - type-switch dispatcher
  - [ ] 4.4 Implement helper `UnmarshalIRValue(data []byte) (IRValue, error)` - float rejection entry point

  **Note:** `MarshalIRValue` and `UnmarshalIRValue` are the primary API for IRValue JSON handling.
  They provide type-safe marshaling with proper dispatch and float/null rejection that standard
  `json.Marshal`/`json.Unmarshal` cannot enforce.

  **IMPORTANT:** This marshaling is NOT canonical - it uses Go's default `encoding/json` which
  may HTML-escape characters like `<`, `>`, `&`. This is fine for debugging and logging, but
  MUST NOT be used for content-addressed hashing. Story 1.4 will implement `MarshalCanonical`
  with `SetEscapeHTML(false)` and strict RFC 8785 compliance for hash computation.

- [ ] Task 5: Implement helper constructors (AC: #5)
  - [ ] 5.1 Implement `NewIRString`, `NewIRInt`, `NewIRBool`
  - [ ] 5.2 Implement `NewIRArray`
  - [ ] 5.3 Implement `NewIRObject` with key/value pairs

- [ ] Task 6: Write comprehensive tests (AC: #6)
  - [ ] 6.1 Test sealed interface (type assertion tests)
  - [ ] 6.2 Test JSON round-trip for all types
  - [ ] 6.3 Test float rejection in JSON unmarshaling
  - [ ] 6.4 Test deterministic ordering with UTF-16 edge cases
  - [ ] 6.5 Test nested structures (deep nesting)
  - [ ] 6.6 Test empty values (empty string, empty array, empty object)

## Dev Notes

### Critical Pattern Details

**CP-3: RFC 8785 UTF-16 Key Ordering**

This is one of the most critical patterns. Go's `sort.Strings()` uses UTF-8 byte ordering which produces DIFFERENT results than RFC 8785's UTF-16 code unit ordering.

```go
// CRITICAL: RFC 8785 uses UTF-16 code units, NOT UTF-8 bytes
// Go's sort.Strings uses UTF-8 which produces DIFFERENT order
//
// Key insight: For supplementary characters (U+10000 and above), Go runes
// are single values, but UTF-16 uses surrogate pairs. We must compare
// the full UTF-16 code unit sequences, not rune-by-rune.
//
// Example where UTF-8 and UTF-16 differ:
// Consider "𐀀" (U+10000) vs "a{":
// - UTF-8:  "𐀀" = [0xF0, 0x90, 0x80, 0x80], "a{" = [0x61, 0x7B]
//           First byte 0xF0 > 0x61, so UTF-8 says "𐀀" > "a{"
// - UTF-16: "𐀀" = [0xD800, 0xDC00], "a{" = [0x0061, 0x007B]
//           First unit 0xD800 > 0x0061, so UTF-16 also says "𐀀" > "a{"
//           BUT the surrogate 0xD800 has different comparison semantics

import (
    "unicode/utf16"
)

// compareKeysRFC8785 compares two strings according to RFC 8785,
// which requires lexicographical comparison of UTF-16 code units.
// CRITICAL: Must use unicode/utf16.Encode for correct surrogate handling.
func compareKeysRFC8785(a, b string) int {
    // Convert entire strings to UTF-16 code units
    a16 := utf16.Encode([]rune(a))
    b16 := utf16.Encode([]rune(b))

    // Compare code unit by code unit
    minLen := len(a16)
    if len(b16) < minLen {
        minLen = len(b16)
    }

    for i := 0; i < minLen; i++ {
        if a16[i] != b16[i] {
            if a16[i] < b16[i] {
                return -1
            }
            return 1
        }
    }

    // If all compared units are equal, shorter string comes first
    if len(a16) < len(b16) {
        return -1
    }
    if len(a16) > len(b16) {
        return 1
    }
    return 0
}

// WRONG: Default Go string comparison
sort.Strings(keys) // ❌ UTF-8 byte order differs from UTF-16
```

**CP-5: Constrained Value Types (No Floats)**

```go
// internal/ir/value.go - Sealed interface pattern

// IRValue represents a constrained value type in the NYSM IR.
// This is a sealed interface - only types defined in this package implement it.
//
// The constraint forbids floats because floating-point numbers cannot be
// deterministically serialized across languages/platforms (IEEE 754 edge cases,
// formatting differences, etc.). Use IRInt for numbers.
type IRValue interface {
    irValue() // Sealed - only these types implement it
}

type IRString string
func (IRString) irValue() {}

type IRInt int64
func (IRInt) irValue() {}

type IRBool bool
func (IRBool) irValue() {}

type IRArray []IRValue
func (IRArray) irValue() {}

type IRObject map[string]IRValue
func (IRObject) irValue() {}

// NO IRFloat - floats are FORBIDDEN in IR (breaks determinism)

// SortedKeys returns object keys in RFC 8785 (UTF-16 code unit) order.
// REQUIRED: All iteration over IRObject MUST use this method.
func (obj IRObject) SortedKeys() []string {
    keys := make([]string, 0, len(obj))
    for k := range obj {
        keys = append(keys, k)
    }
    slices.SortFunc(keys, compareKeysRFC8785)
    return keys
}
```

### JSON Marshaling Implementation

**IRObject MarshalJSON with sorted keys:**
```go
func (obj IRObject) MarshalJSON() ([]byte, error) {
    var buf bytes.Buffer
    buf.WriteByte('{')

    keys := obj.SortedKeys() // RFC 8785 ordering
    for i, k := range keys {
        if i > 0 {
            buf.WriteByte(',')
        }
        // Marshal key
        keyBytes, err := json.Marshal(k)
        if err != nil {
            return nil, fmt.Errorf("marshal key %q: %w", k, err)
        }
        buf.Write(keyBytes)
        buf.WriteByte(':')

        // Marshal value
        valBytes, err := MarshalIRValue(obj[k])
        if err != nil {
            return nil, fmt.Errorf("marshal value for key %q: %w", k, err)
        }
        buf.Write(valBytes)
    }

    buf.WriteByte('}')
    return buf.Bytes(), nil
}
```

**Float Rejection in UnmarshalJSON:**
```go
// UnmarshalIRValue deserializes JSON into an IRValue.
// CRITICAL: Rejects floats - any non-integer number causes an error.
func UnmarshalIRValue(data []byte) (IRValue, error) {
    // Use json.Decoder with UseNumber() to detect floats
    dec := json.NewDecoder(bytes.NewReader(data))
    dec.UseNumber()

    var raw any
    if err := dec.Decode(&raw); err != nil {
        return nil, err
    }

    return convertToIRValue(raw)
}

func convertToIRValue(v any) (IRValue, error) {
    switch val := v.(type) {
    case nil:
        // CRITICAL: JSON null is REJECTED - only IRString/IRInt/IRBool/IRArray/IRObject allowed
        return nil, fmt.Errorf("null is forbidden in IR: only string, int, bool, array, object allowed")
    case bool:
        return IRBool(val), nil
    case string:
        return IRString(val), nil
    case json.Number:
        // CRITICAL: Check if this is a float
        if strings.Contains(string(val), ".") || strings.Contains(string(val), "e") || strings.Contains(string(val), "E") {
            return nil, fmt.Errorf("floats are forbidden in IR: %s", val)
        }
        n, err := val.Int64()
        if err != nil {
            return nil, fmt.Errorf("number out of int64 range: %s", val)
        }
        return IRInt(n), nil
    case []any:
        arr := make(IRArray, len(val))
        for i, elem := range val {
            irElem, err := convertToIRValue(elem)
            if err != nil {
                return nil, fmt.Errorf("array[%d]: %w", i, err)
            }
            arr[i] = irElem
        }
        return arr, nil
    case map[string]any:
        obj := make(IRObject, len(val))
        for k, elem := range val {
            irElem, err := convertToIRValue(elem)
            if err != nil {
                return nil, fmt.Errorf("object[%q]: %w", k, err)
            }
            obj[k] = irElem
        }
        return obj, nil
    default:
        return nil, fmt.Errorf("unsupported type: %T", v)
    }
}
```

### Helper Constructors

```go
// NewIRString creates an IRString value.
func NewIRString(s string) IRString {
    return IRString(s)
}

// NewIRInt creates an IRInt value.
func NewIRInt(n int64) IRInt {
    return IRInt(n)
}

// NewIRBool creates an IRBool value.
func NewIRBool(b bool) IRBool {
    return IRBool(b)
}

// NewIRArray creates an IRArray from values.
func NewIRArray(vals ...IRValue) IRArray {
    return IRArray(vals)
}

// IRPair represents a key-value pair for typed IRObject construction.
// This provides compile-time type safety - floats cannot be passed.
type IRPair struct {
    Key   string
    Value IRValue
}

// NewIRObjectFromMap creates an IRObject from an existing map.
// Preferred for programmatic construction.
func NewIRObjectFromMap(m map[string]IRValue) IRObject {
    return IRObject(m)
}

// NewIRObjectFromPairs creates an IRObject from typed key-value pairs.
// Provides compile-time type safety - cannot pass floats.
// Example: NewIRObjectFromPairs(IRPair{"name", NewIRString("cart")}, IRPair{"count", NewIRInt(5)})
func NewIRObjectFromPairs(pairs ...IRPair) IRObject {
    obj := make(IRObject, len(pairs))
    for _, p := range pairs {
        obj[p.Key] = p.Value
    }
    return obj
}

// O is a shorthand for IRPair for ergonomic construction.
// Example: NewIRObjectFromPairs(O("name", NewIRString("cart")), O("count", NewIRInt(5)))
func O(key string, value IRValue) IRPair {
    return IRPair{Key: key, Value: value}
}
```

### Test Examples

```go
// Test sealed interface - only allowed types should work
func TestIRValueSealed(t *testing.T) {
    // These should all satisfy IRValue
    var _ IRValue = IRString("test")
    var _ IRValue = IRInt(42)
    var _ IRValue = IRBool(true)
    var _ IRValue = IRArray{IRString("a")}
    var _ IRValue = IRObject{"key": IRInt(1)}

    // Compile-time verification that irValue() is unexported
    // (Cannot be called from outside package)
}

// Test float rejection
func TestUnmarshalRejectsFloats(t *testing.T) {
    tests := []struct {
        name  string
        input string
    }{
        {"simple float", `3.14`},
        {"scientific notation", `1e10`},
        {"negative float", `-2.5`},
        {"nested float", `{"value": 1.5}`},
        {"array with float", `[1, 2.0, 3]`},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := UnmarshalIRValue([]byte(tt.input))
            require.Error(t, err)
            assert.Contains(t, err.Error(), "float")
        })
    }
}

// Test null rejection - null is NOT allowed in IRValue
func TestUnmarshalRejectsNull(t *testing.T) {
    tests := []struct {
        name  string
        input string
    }{
        {"top-level null", `null`},
        {"nested null in object", `{"key": null}`},
        {"null in array", `[1, null, 2]`},
        {"deeply nested null", `{"a": {"b": [null]}}`},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := UnmarshalIRValue([]byte(tt.input))
            require.Error(t, err)
            assert.Contains(t, err.Error(), "null")
        })
    }
}

// Test RFC 8785 UTF-16 ordering with REAL UTF-8 vs UTF-16 divergence
func TestSortedKeysUTF16Order(t *testing.T) {
    // CRITICAL TEST: This case MUST expose the difference between UTF-8 and UTF-16 sorting.
    //
    // Key insight: U+E000 (Private Use Area) vs U+10000 (Linear B Syllable B008)
    //
    // U+E000 ("") - UTF-8: [0xEE, 0x80, 0x80], UTF-16: [0xE000]
    // U+10000 ("𐀀") - UTF-8: [0xF0, 0x90, 0x80, 0x80], UTF-16: [0xD800, 0xDC00]
    //
    // UTF-8 byte comparison:
    //   First bytes: 0xEE < 0xF0, so UTF-8 says "" < "𐀀"
    //   Order: ["", "𐀀"]
    //
    // UTF-16 code unit comparison (RFC 8785):
    //   First units: 0xE000 > 0xD800, so UTF-16 says "" > "𐀀"
    //   Order: ["𐀀", ""]  ← DIFFERENT!
    //
    // This is the canonical test case that proves correct RFC 8785 implementation.
    obj := IRObject{
        "\uE000": IRInt(1), // U+E000 (Private Use Area)
        "𐀀":      IRInt(2), // U+10000 (Linear B Syllable B008) - surrogate pair 0xD800, 0xDC00
    }

    // RFC 8785 UTF-16 order: surrogate high (0xD800) < BMP high (0xE000)
    expectedRFC8785Order := []string{"𐀀", "\uE000"}

    keys := obj.SortedKeys()
    assert.Equal(t, expectedRFC8785Order, keys, "RFC 8785 UTF-16 ordering must be used")

    // Verify determinism - same order every time
    for i := 0; i < 100; i++ {
        assert.Equal(t, keys, obj.SortedKeys(), "ordering must be deterministic")
    }

    // CRITICAL: Prove that Go's default sort.Strings produces WRONG order
    wrongOrderKeys := []string{"\uE000", "𐀀"}
    sort.Strings(wrongOrderKeys)
    expectedUTF8Order := []string{"\uE000", "𐀀"} // UTF-8: 0xEE < 0xF0
    assert.Equal(t, expectedUTF8Order, wrongOrderKeys, "UTF-8 sort produces different order")
    assert.NotEqual(t, expectedRFC8785Order, wrongOrderKeys, "UTF-8 and UTF-16 orders MUST differ for this test")
}

// Additional test with characters that definitely differ between UTF-8 and UTF-16
func TestSortedKeysUTF16vsUTF8Divergence(t *testing.T) {
    // These test fixtures should be placed in testdata/fixtures/rfc8785/
    // for cross-language validation. The key insight is that supplementary
    // characters (> U+FFFF) are represented as surrogate pairs in UTF-16.

    tests := []struct {
        name     string
        input    map[string]IRValue
        expected []string
    }{
        {
            name: "basic latin",
            input: map[string]IRValue{
                "b": IRInt(1),
                "a": IRInt(2),
                "c": IRInt(3),
            },
            expected: []string{"a", "b", "c"},
        },
        {
            name: "empty string first",
            input: map[string]IRValue{
                "a":  IRInt(1),
                "":   IRInt(2),
            },
            expected: []string{"", "a"},
        },
        {
            name: "numbers as strings",
            input: map[string]IRValue{
                "10": IRInt(1),
                "2":  IRInt(2),
                "1":  IRInt(3),
            },
            expected: []string{"1", "10", "2"}, // Lexicographic, not numeric
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            obj := IRObject(tt.input)
            assert.Equal(t, tt.expected, obj.SortedKeys())
        })
    }
}

// Test JSON round-trip
func TestJSONRoundTrip(t *testing.T) {
    tests := []struct {
        name  string
        value IRValue
    }{
        {"string", IRString("hello")},
        {"int", IRInt(42)},
        {"negative int", IRInt(-100)},
        {"bool true", IRBool(true)},
        {"bool false", IRBool(false)},
        {"empty array", IRArray{}},
        {"array of ints", IRArray{IRInt(1), IRInt(2), IRInt(3)}},
        {"empty object", IRObject{}},
        {"simple object", IRObject{"key": IRString("value")}},
        {"nested", IRObject{
            "array": IRArray{IRInt(1), IRObject{"nested": IRBool(true)}},
            "string": IRString("test"),
        }},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            data, err := MarshalIRValue(tt.value)
            require.NoError(t, err)

            result, err := UnmarshalIRValue(data)
            require.NoError(t, err)

            assert.Equal(t, tt.value, result)
        })
    }
}
```

### File List

Files to create/modify:

1. `internal/ir/value.go` - IRValue sealed interface and implementations
2. `internal/ir/value_test.go` - Comprehensive tests

### Relationship to Story 1-1

Story 1-1 defined the placeholder for IRValue in `internal/ir/value.go`. This story fully implements:
- The sealed interface pattern
- All five concrete types
- RFC 8785 UTF-16 key ordering
- JSON marshaling with float rejection
- Helper constructors

The `Args` and `Result` fields on `Invocation` and `Completion` (from story 1-1) use `IRObject`, which depends on this complete implementation.

### Story Completion Checklist

- [ ] IRValue sealed interface defined with irValue() marker
- [ ] All 5 concrete types implemented (IRString, IRInt, IRBool, IRArray, IRObject)
- [ ] NO IRFloat type anywhere
- [ ] SortedKeys() uses RFC 8785 UTF-16 ordering
- [ ] MarshalJSON produces sorted keys
- [ ] UnmarshalJSON rejects floats with clear error
- [ ] Helper constructors work correctly
- [ ] All tests pass
- [ ] `go vet ./internal/ir/...` passes
- [ ] No float64 usage in value.go

### References

- [Source: docs/architecture.md#CP-3] - RFC 8785 UTF-16 key ordering
- [Source: docs/architecture.md#CP-5] - Constrained value types, no floats
- [Source: docs/architecture.md#Internal/ir/] - Package structure
- [Source: docs/epics.md#Story 1.2] - Story definition
- [Source: docs/prd.md#FR-1.4] - Typed action outputs with multiple cases
- [Source: RFC 8785] - JSON Canonicalization Scheme

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Validation History

- Initial creation: create-story workflow
- Validation 1: Gemini 2.5 Pro via Zen codereview
  - HIGH: Fixed `compareKeysRFC8785` implementation to use `unicode/utf16` package
  - MEDIUM: Improved test cases to better expose UTF-8 vs UTF-16 ordering differences
  - LOW: Added clarification about MarshalIRValue/UnmarshalIRValue helper purpose
- Validation 2: OpenAI Codex via Zen clink (codereviewer role)
  - CRITICAL: Added REAL UTF-8 vs UTF-16 divergence test (U+E000 vs U+10000)
  - CRITICAL: Changed null handling from allowing nil to rejecting with error
  - HIGH: Replaced `NewIRObject(...any)` with typed `NewIRObjectFromPairs(pairs ...IRPair)`
  - HIGH: Clarified marshaling is NOT canonical (Story 1.4 adds canonical)
  - MEDIUM: Updated architecture.md CP-3 snippet to use correct `unicode/utf16` implementation
  - LOW: Added null rejection tests

### Completion Notes

- Builds directly on Story 1-1's IR type definitions
- Critical for deterministic serialization across the entire system
- RFC 8785 UTF-16 ordering is non-obvious but essential for cross-language compatibility
- Float rejection prevents a major source of non-determinism
