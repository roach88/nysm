# Story 1.4: RFC 8785 Canonical JSON Marshaling

Status: done

## Story

As a **developer building NYSM**,
I want **deterministic JSON serialization following RFC 8785**,
So that **identical data always produces identical bytes for hashing**.

## Acceptance Criteria

1. **`MarshalCanonical(v any) ([]byte, error)` function in `internal/ir/canonical.go`**
   - Works with any IR type or IRValue
   - Returns deterministic bytes suitable for hashing

2. **Object keys sorted by UTF-16 code units** (not UTF-8 bytes) per CP-3
   - Uses `compareKeysRFC8785` from Story 1-2
   - Supplementary characters (U+10000+) handled correctly via surrogate pairs

3. **No floats** - only int64, string, bool, array, object
   - Attempting to marshal a float returns an error
   - This is enforced at the IRValue level (Story 1-2)

4. **No HTML escaping** - `<>&` characters NOT escaped
   - Uses `json.Encoder.SetEscapeHTML(false)`
   - Critical for cross-language canonical hash consistency

5. **Compact output** - no whitespace
   - No pretty-printing, no indentation
   - Single-line output

6. **Unicode NFC normalization at ingestion boundaries**
   - Strings normalized to NFC before serialization
   - Uses `golang.org/x/text/unicode/norm` package

7. **Fuzz tests verify idempotency**
   - Property: `MarshalCanonical(Unmarshal(MarshalCanonical(x))) == MarshalCanonical(x)`

8. **Cross-language fixtures validate correctness**
   - Test fixtures in `testdata/fixtures/rfc8785/`
   - Same input produces same output as reference implementations

## Quick Reference

| Pattern | Key Rule |
|---------|----------|
| **CP-3** | RFC 8785 UTF-16 code unit key ordering |
| **CP-5** | NO floats - only string, int64, bool, array, object |
| **RFC 8785** | JSON Canonicalization Scheme |

## Tasks / Subtasks

- [ ] Task 1: Implement MarshalCanonical function (AC: #1)
  - [ ] 1.1 Create `internal/ir/canonical.go`
  - [ ] 1.2 Implement type dispatch for IRValue types
  - [ ] 1.3 Implement type dispatch for IR structs (ActionSig, etc.)

- [ ] Task 2: Implement sorted key ordering (AC: #2)
  - [ ] 2.1 Reuse `compareKeysRFC8785` from value.go
  - [ ] 2.2 Apply to all map/object serialization

- [ ] Task 3: Implement no HTML escaping (AC: #4)
  - [ ] 3.1 Use custom encoder with `SetEscapeHTML(false)`
  - [ ] 3.2 Test with `<`, `>`, `&` characters

- [ ] Task 4: Implement compact output (AC: #5)
  - [ ] 4.1 No whitespace between tokens
  - [ ] 4.2 No indentation

- [ ] Task 5: Implement NFC normalization (AC: #6)
  - [ ] 5.1 Add `golang.org/x/text/unicode/norm` dependency
  - [ ] 5.2 Normalize all string values before serialization

- [ ] Task 6: Write fuzz tests (AC: #7)
  - [ ] 6.1 Implement fuzz test for idempotency property
  - [ ] 6.2 Test with random IRValue trees

- [ ] Task 7: Create cross-language fixtures (AC: #8)
  - [ ] 7.1 Create `testdata/fixtures/rfc8785/` directory
  - [ ] 7.2 Add fixture files with input/expected output pairs
  - [ ] 7.3 Include UTF-16 ordering edge cases (U+E000 vs U+10000)

## Dev Notes

### MarshalCanonical Implementation

```go
// internal/ir/canonical.go
package ir

import (
    "bytes"
    "encoding/json"
    "fmt"
    "slices"

    "golang.org/x/text/unicode/norm"
)

// MarshalCanonical produces RFC 8785 canonical JSON for hashing.
// CRITICAL: This is the ONLY serialization that should be used for
// content-addressed identity computation.
func MarshalCanonical(v any) ([]byte, error) {
    return marshalCanonical(v)
}

func marshalCanonical(v any) ([]byte, error) {
    switch val := v.(type) {
    case nil:
        return nil, fmt.Errorf("null is forbidden in canonical JSON")
    case IRString:
        return marshalCanonicalString(string(val))
    case IRInt:
        return []byte(fmt.Sprintf("%d", val)), nil
    case IRBool:
        if val {
            return []byte("true"), nil
        }
        return []byte("false"), nil
    case IRArray:
        return marshalCanonicalArray(val)
    case IRObject:
        return marshalCanonicalObject(val)
    case string:
        return marshalCanonicalString(val)
    case int64:
        return []byte(fmt.Sprintf("%d", val)), nil
    case int:
        return []byte(fmt.Sprintf("%d", val)), nil
    case bool:
        if val {
            return []byte("true"), nil
        }
        return []byte("false"), nil
    case []any:
        arr := make(IRArray, len(val))
        for i, elem := range val {
            irElem, err := toIRValue(elem)
            if err != nil {
                return nil, fmt.Errorf("array[%d]: %w", i, err)
            }
            arr[i] = irElem
        }
        return marshalCanonicalArray(arr)
    case map[string]any:
        obj := make(IRObject, len(val))
        for k, elem := range val {
            irElem, err := toIRValue(elem)
            if err != nil {
                return nil, fmt.Errorf("object[%q]: %w", k, err)
            }
            obj[k] = irElem
        }
        return marshalCanonicalObject(obj)
    case float64, float32:
        return nil, fmt.Errorf("floats are forbidden in canonical JSON: %v", val)
    default:
        return nil, fmt.Errorf("unsupported type for canonical JSON: %T", v)
    }
}

func toIRValue(v any) (IRValue, error) {
    switch val := v.(type) {
    case nil:
        return nil, fmt.Errorf("null is forbidden")
    case IRValue:
        return val, nil
    case string:
        return IRString(val), nil
    case int64:
        return IRInt(val), nil
    case int:
        return IRInt(val), nil
    case bool:
        return IRBool(val), nil
    case float64, float32:
        return nil, fmt.Errorf("floats are forbidden")
    case []any:
        arr := make(IRArray, len(val))
        for i, elem := range val {
            irElem, err := toIRValue(elem)
            if err != nil {
                return nil, fmt.Errorf("[%d]: %w", i, err)
            }
            arr[i] = irElem
        }
        return arr, nil
    case map[string]any:
        obj := make(IRObject, len(val))
        for k, elem := range val {
            irElem, err := toIRValue(elem)
            if err != nil {
                return nil, fmt.Errorf("[%q]: %w", k, err)
            }
            obj[k] = irElem
        }
        return obj, nil
    default:
        return nil, fmt.Errorf("unsupported type: %T", v)
    }
}

// marshalCanonicalString produces canonical JSON string with NFC normalization.
// CRITICAL: No HTML escaping - <, >, & are NOT escaped.
func marshalCanonicalString(s string) ([]byte, error) {
    // NFC normalize at serialization boundary
    normalized := norm.NFC.String(s)

    // Use encoder with HTML escaping disabled
    var buf bytes.Buffer
    enc := json.NewEncoder(&buf)
    enc.SetEscapeHTML(false) // CRITICAL: <, >, & must NOT be escaped
    if err := enc.Encode(normalized); err != nil {
        return nil, err
    }

    // json.Encoder adds trailing newline, remove it
    result := buf.Bytes()
    if len(result) > 0 && result[len(result)-1] == '\n' {
        result = result[:len(result)-1]
    }
    return result, nil
}

func marshalCanonicalArray(arr IRArray) ([]byte, error) {
    var buf bytes.Buffer
    buf.WriteByte('[')

    for i, elem := range arr {
        if i > 0 {
            buf.WriteByte(',')
        }
        elemBytes, err := marshalCanonical(elem)
        if err != nil {
            return nil, fmt.Errorf("array[%d]: %w", i, err)
        }
        buf.Write(elemBytes)
    }

    buf.WriteByte(']')
    return buf.Bytes(), nil
}

func marshalCanonicalObject(obj IRObject) ([]byte, error) {
    var buf bytes.Buffer
    buf.WriteByte('{')

    // CRITICAL: RFC 8785 UTF-16 code unit ordering
    keys := obj.SortedKeys()

    for i, k := range keys {
        if i > 0 {
            buf.WriteByte(',')
        }

        // Marshal key (NFC normalized, no HTML escape)
        keyBytes, err := marshalCanonicalString(k)
        if err != nil {
            return nil, fmt.Errorf("key %q: %w", k, err)
        }
        buf.Write(keyBytes)
        buf.WriteByte(':')

        // Marshal value
        valBytes, err := marshalCanonical(obj[k])
        if err != nil {
            return nil, fmt.Errorf("value for key %q: %w", k, err)
        }
        buf.Write(valBytes)
    }

    buf.WriteByte('}')
    return buf.Bytes(), nil
}
```

### No HTML Escaping - Critical Detail

Go's `encoding/json` by default escapes `<`, `>`, `&` to `\u003c`, `\u003e`, `\u0026`.
RFC 8785 does NOT require this escaping. For canonical JSON, we MUST disable it.

```go
// WRONG: Default encoding escapes HTML
json.Marshal("<script>") // Returns: "\u003cscript\u003e"

// CORRECT: Canonical JSON - no HTML escaping
enc := json.NewEncoder(&buf)
enc.SetEscapeHTML(false)
enc.Encode("<script>") // Returns: "<script>"
```

This matters because other languages (Python, JS) don't escape HTML by default,
so cross-language hash consistency requires disabling HTML escaping.

### NFC Normalization

Unicode strings can have multiple representations for the same visual character.
NFC (Canonical Decomposition, followed by Canonical Composition) normalizes to
a single canonical form.

```go
import "golang.org/x/text/unicode/norm"

// "é" can be:
// - U+00E9 (precomposed)
// - U+0065 U+0301 (e + combining acute accent)
// NFC normalizes both to U+00E9

normalized := norm.NFC.String(input)
```

### Cross-Language Fixtures

Create `testdata/fixtures/rfc8785/` with JSON files:

```json
// testdata/fixtures/rfc8785/basic.json
{
  "tests": [
    {
      "name": "empty object",
      "input": {},
      "expected": "{}"
    },
    {
      "name": "sorted keys",
      "input": {"b": 1, "a": 2},
      "expected": "{\"a\":2,\"b\":1}"
    },
    {
      "name": "no html escape",
      "input": {"html": "<script>"},
      "expected": "{\"html\":\"<script>\"}"
    },
    {
      "name": "utf16 ordering",
      "input": {"\uE000": 1, "𐀀": 2},
      "expected": "{\"𐀀\":2,\"\uE000\":1}"
    }
  ]
}
```

### Test Examples

```go
func TestMarshalCanonicalBasic(t *testing.T) {
    tests := []struct {
        name     string
        input    any
        expected string
    }{
        {"string", IRString("hello"), `"hello"`},
        {"int", IRInt(42), "42"},
        {"negative int", IRInt(-100), "-100"},
        {"bool true", IRBool(true), "true"},
        {"bool false", IRBool(false), "false"},
        {"empty array", IRArray{}, "[]"},
        {"empty object", IRObject{}, "{}"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := MarshalCanonical(tt.input)
            require.NoError(t, err)
            assert.Equal(t, tt.expected, string(result))
        })
    }
}

func TestMarshalCanonicalSortedKeys(t *testing.T) {
    obj := IRObject{
        "zebra": IRInt(1),
        "alpha": IRInt(2),
        "beta":  IRInt(3),
    }

    result, err := MarshalCanonical(obj)
    require.NoError(t, err)
    assert.Equal(t, `{"alpha":2,"beta":3,"zebra":1}`, string(result))
}

func TestMarshalCanonicalUTF16Ordering(t *testing.T) {
    // U+E000 vs U+10000 - UTF-16 order differs from UTF-8
    obj := IRObject{
        "\uE000": IRInt(1), // UTF-16: 0xE000
        "𐀀":      IRInt(2), // UTF-16: 0xD800, 0xDC00 (surrogate pair)
    }

    result, err := MarshalCanonical(obj)
    require.NoError(t, err)

    // UTF-16 order: 0xD800 < 0xE000, so 𐀀 comes first
    assert.Equal(t, `{"𐀀":2,"\uE000":1}`, string(result))
}

func TestMarshalCanonicalNoHTMLEscape(t *testing.T) {
    obj := IRObject{
        "html": IRString("<script>alert('xss')</script>"),
        "amp":  IRString("a & b"),
    }

    result, err := MarshalCanonical(obj)
    require.NoError(t, err)

    // MUST NOT escape <, >, &
    assert.Contains(t, string(result), "<script>")
    assert.Contains(t, string(result), "</script>")
    assert.Contains(t, string(result), "a & b")
    assert.NotContains(t, string(result), "\\u003c") // No HTML escape
}

func TestMarshalCanonicalRejectsFloats(t *testing.T) {
    _, err := MarshalCanonical(3.14)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "float")
}

func TestMarshalCanonicalRejectsNull(t *testing.T) {
    _, err := MarshalCanonical(nil)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "null")
}

func TestMarshalCanonicalNFCNormalization(t *testing.T) {
    // "é" as composed (U+00E9) vs decomposed (U+0065 U+0301)
    composed := "caf\u00E9"
    decomposed := "cafe\u0301"

    result1, err := MarshalCanonical(IRString(composed))
    require.NoError(t, err)

    result2, err := MarshalCanonical(IRString(decomposed))
    require.NoError(t, err)

    // Both should produce identical canonical output
    assert.Equal(t, result1, result2, "NFC normalization should make these equal")
}

// Fuzz test for idempotency
func FuzzMarshalCanonicalIdempotent(f *testing.F) {
    f.Add(`{"a":1,"b":"test"}`)
    f.Add(`[1,2,3]`)
    f.Add(`"hello"`)

    f.Fuzz(func(t *testing.T, jsonStr string) {
        // Parse as IRValue
        val, err := UnmarshalIRValue([]byte(jsonStr))
        if err != nil {
            t.Skip() // Invalid JSON or contains floats/null
        }

        // Marshal canonically
        canonical1, err := MarshalCanonical(val)
        if err != nil {
            t.Skip()
        }

        // Unmarshal and marshal again
        val2, err := UnmarshalIRValue(canonical1)
        require.NoError(t, err)

        canonical2, err := MarshalCanonical(val2)
        require.NoError(t, err)

        // Must be identical (idempotency)
        assert.Equal(t, canonical1, canonical2, "canonical marshaling must be idempotent")
    })
}
```

### File List

Files to create/modify:

1. `internal/ir/canonical.go` - MarshalCanonical implementation
2. `internal/ir/canonical_test.go` - Comprehensive tests
3. `testdata/fixtures/rfc8785/basic.json` - Cross-language fixtures
4. `testdata/fixtures/rfc8785/utf16_ordering.json` - UTF-16 edge cases
5. `go.mod` - Add `golang.org/x/text` dependency

### Relationship to Other Stories

- **Story 1-2:** Uses IRValue types and compareKeysRFC8785
- **Story 1-5:** Content-addressed identity uses MarshalCanonical
- **Story 1-3:** ActionSig marshaling can use MarshalCanonical for hashing

### Story Completion Checklist

- [ ] MarshalCanonical function implemented
- [ ] UTF-16 key ordering using compareKeysRFC8785
- [ ] No HTML escaping (SetEscapeHTML(false))
- [ ] Compact output (no whitespace)
- [ ] NFC normalization for strings
- [ ] Floats rejected with error
- [ ] Null rejected with error
- [ ] Fuzz test for idempotency passes
- [ ] Cross-language fixtures validate correctness
- [ ] All tests pass
- [ ] `go vet ./internal/ir/...` passes

### References

- [Source: docs/architecture.md#CP-3] - RFC 8785 UTF-16 key ordering
- [Source: docs/architecture.md#Canonical JSON] - Canonical JSON requirements
- [Source: docs/epics.md#Story 1.4] - Story definition
- [Source: RFC 8785] - JSON Canonicalization Scheme specification

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Validation History

- Initial creation: create-story workflow

### Completion Notes

- This is the MOST CRITICAL story for correctness
- MarshalCanonical is the ONLY serialization for content-addressed hashing
- Three key differences from standard json.Marshal:
  1. UTF-16 key ordering (not UTF-8)
  2. No HTML escaping
  3. NFC normalization
- Cross-language fixtures essential for interoperability validation
