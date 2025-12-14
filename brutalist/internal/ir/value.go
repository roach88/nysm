package ir

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"unicode/utf16"
)

// IRValue is a sealed interface representing constrained value types.
// Only IRNull, IRString, IRInt, IRBool, IRArray, and IRObject implement this.
// NO IRFloat - floats are forbidden in IR (CP-5, breaks determinism).
type IRValue interface {
	irValue() // Sealed - only these types implement it
}

// IRNull represents a JSON null value in the IR.
// Using an explicit type ensures all IRValues satisfy the sealed interface.
type IRNull struct{}

func (IRNull) irValue() {}

// MarshalJSON implements json.Marshaler for IRNull.
func (IRNull) MarshalJSON() ([]byte, error) {
	return []byte("null"), nil
}

// IRString represents a string value in the IR.
type IRString string

func (IRString) irValue() {}

// IRInt represents an integer value in the IR.
// Always int64, never float64 (CP-5).
type IRInt int64

func (IRInt) irValue() {}

// IRBool represents a boolean value in the IR.
type IRBool bool

func (IRBool) irValue() {}

// IRArray represents an array of IRValue elements.
type IRArray []IRValue

func (IRArray) irValue() {}

// IRObject represents a map of string keys to IRValue elements.
// Use SortedKeys() for deterministic iteration.
type IRObject map[string]IRValue

func (IRObject) irValue() {}

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

// SortedKeys returns keys in RFC 8785 canonical order (UTF-16 code units).
// CRITICAL: Go's sort.Strings uses UTF-8 which produces DIFFERENT order.
func (obj IRObject) SortedKeys() []string {
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	slices.SortFunc(keys, compareKeysRFC8785)
	return keys
}

// compareKeysRFC8785 compares strings using UTF-16 code unit ordering
// as required by RFC 8785 (Canonical JSON).
// CRITICAL: Must use unicode/utf16.Encode for correct surrogate handling.
// Go's default string comparison uses UTF-8 which produces DIFFERENT order.
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

// UnmarshalJSON implements json.Unmarshaler for IRObject.
func (obj *IRObject) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	*obj = make(IRObject, len(raw))
	for k, v := range raw {
		val, err := unmarshalIRValue(v)
		if err != nil {
			return fmt.Errorf("IRObject key %q: %w", k, err)
		}
		(*obj)[k] = val
	}
	return nil
}

// UnmarshalJSON implements json.Unmarshaler for IRArray.
func (arr *IRArray) UnmarshalJSON(data []byte) error {
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	*arr = make(IRArray, len(raw))
	for i, v := range raw {
		val, err := unmarshalIRValue(v)
		if err != nil {
			return fmt.Errorf("IRArray index %d: %w", i, err)
		}
		(*arr)[i] = val
	}
	return nil
}

// unmarshalIRValue decodes a JSON value into the appropriate IRValue type.
// Floats in JSON are rejected (CP-5). This internal version allows null -> IRNull
// for round-tripping existing data. Use UnmarshalIRValue for strict validation.
func unmarshalIRValue(data []byte) (IRValue, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty JSON value")
	}

	switch data[0] {
	case '"':
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return nil, err
		}
		return IRString(s), nil

	case 't', 'f':
		var b bool
		if err := json.Unmarshal(data, &b); err != nil {
			return nil, err
		}
		return IRBool(b), nil

	case 'n':
		// null becomes IRNull (not nil) to satisfy sealed interface
		return IRNull{}, nil

	case '[':
		var arr IRArray
		if err := json.Unmarshal(data, &arr); err != nil {
			return nil, err
		}
		return arr, nil

	case '{':
		var obj IRObject
		if err := json.Unmarshal(data, &obj); err != nil {
			return nil, err
		}
		return obj, nil

	default:
		// Must be a number - try int64 first
		var n json.Number
		if err := json.Unmarshal(data, &n); err != nil {
			return nil, err
		}

		// Try parsing as int64 (CP-5: no floats)
		i, err := n.Int64()
		if err != nil {
			return nil, fmt.Errorf("floats not allowed in IR (CP-5): %s", string(data))
		}
		return IRInt(i), nil
	}
}

// MarshalJSON implements json.Marshaler for IRObject with sorted keys (RFC 8785 ordering).
// NOTE: This is NOT canonical marshaling - may have HTML escaping. Use MarshalCanonical
// (Story 1-4) for content-addressed hashing.
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

// MarshalIRValue marshals an IRValue to JSON bytes.
// Uses type-switch dispatch to handle all IRValue types correctly.
// NOTE: This is NOT canonical marshaling. Use MarshalCanonical (Story 1-4) for hashing.
func MarshalIRValue(v IRValue) ([]byte, error) {
	switch val := v.(type) {
	case IRNull:
		return []byte("null"), nil
	case IRString:
		return json.Marshal(string(val))
	case IRInt:
		return json.Marshal(int64(val))
	case IRBool:
		return json.Marshal(bool(val))
	case IRArray:
		return marshalIRArray(val)
	case IRObject:
		return val.MarshalJSON()
	default:
		return nil, fmt.Errorf("unknown IRValue type: %T", v)
	}
}

// marshalIRArray marshals an IRArray to JSON bytes.
func marshalIRArray(arr IRArray) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('[')

	for i, elem := range arr {
		if i > 0 {
			buf.WriteByte(',')
		}
		elemBytes, err := MarshalIRValue(elem)
		if err != nil {
			return nil, fmt.Errorf("array[%d]: %w", i, err)
		}
		buf.Write(elemBytes)
	}

	buf.WriteByte(']')
	return buf.Bytes(), nil
}

// UnmarshalIRValue deserializes JSON into an IRValue with strict validation.
// CRITICAL: Rejects floats AND null - only string/int/bool/array/object allowed.
// This is the primary API for external JSON parsing.
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

// convertToIRValue recursively converts a Go value to an IRValue.
// Rejects null and floats.
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
		s := string(val)
		if strings.Contains(s, ".") || strings.Contains(s, "e") || strings.Contains(s, "E") {
			return nil, fmt.Errorf("floats are forbidden in IR (CP-5): %s", val)
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
