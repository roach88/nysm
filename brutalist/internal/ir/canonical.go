package ir

import (
	"bytes"
	"encoding/json"
	"fmt"

	"golang.org/x/text/unicode/norm"
)

// MarshalCanonical produces RFC 8785 canonical JSON for hashing.
// CRITICAL: This is the ONLY serialization that should be used for
// content-addressed identity computation.
//
// Key differences from standard json.Marshal:
// 1. Object keys sorted by UTF-16 code units (not UTF-8 bytes)
// 2. No HTML escaping (< > & are NOT escaped)
// 3. Strings are NFC normalized
// 4. No floats (returns error)
// 5. No null (returns error)
func MarshalCanonical(v any) ([]byte, error) {
	return marshalCanonical(v)
}

func marshalCanonical(v any) ([]byte, error) {
	switch val := v.(type) {
	case nil:
		return nil, fmt.Errorf("null is forbidden in canonical JSON")
	case IRNull:
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

// toIRValue converts a Go value to an IRValue.
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
// CRITICAL: RFC 8785 compliance:
// - No HTML escaping (<, >, & are NOT escaped)
// - U+2028 (LINE SEPARATOR) and U+2029 (PARAGRAPH SEPARATOR) are NOT escaped
// - Only control characters (U+0000-U+001F), backslash, and quote are escaped
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

	// RFC 8785: U+2028 and U+2029 should NOT be escaped.
	// Go's json.Encoder escapes them for JavaScript compatibility, but this
	// violates RFC 8785 canonical JSON. We must unescape them.
	//
	// CRITICAL: We must NOT replace \u2028 when it's part of \\u2028 (escaped backslash).
	// The json encoder produces:
	// - \u2028  for actual U+2028 character (should be unescaped)
	// - \\u2028 for literal backslash + "u2028" text (should stay escaped)
	result = unescapeU2028U2029(result)

	return result, nil
}

// unescapeU2028U2029 converts \u2028 and \u2029 escape sequences to literal characters
// per RFC 8785, but preserves \\u2028/\\u2029 (escaped backslash followed by u2028/u2029).
func unescapeU2028U2029(data []byte) []byte {
	// Fast path: if no \u202 sequences, return unchanged
	if !bytes.Contains(data, []byte(`\u202`)) {
		return data
	}

	var result []byte
	i := 0
	for i < len(data) {
		// Look for \u2028 or \u2029 sequences
		if i+6 <= len(data) && data[i] == '\\' && data[i+1] == 'u' && data[i+2] == '2' && data[i+3] == '0' && data[i+4] == '2' {
			if data[i+5] == '8' || data[i+5] == '9' {
				// Check if preceded by backslash (making it \\u202x)
				// In JSON, \\ represents an escaped backslash
				backslashCount := 0
				for j := i - 1; j >= 0 && (result == nil && data[j] == '\\' || result != nil && j >= len(data)-len(result)-1); j-- {
					if j < len(data) && data[j] == '\\' {
						backslashCount++
					} else {
						break
					}
				}
				// Count backslashes from the result we've built so far
				if result != nil {
					for j := len(result) - 1; j >= 0 && result[j] == '\\'; j-- {
						backslashCount++
					}
				}

				// If odd number of backslashes precede us, this \u202x is real and should be unescaped
				// If even number (including 0), the backslashes are paired and this is escaped (\\u202x)
				// Wait, let's think about this more carefully...
				//
				// Actually, let's use a simpler approach: count backslashes immediately before this position
				// If we haven't started result yet, count from data; otherwise count from result
				actualBackslashes := 0
				if result == nil {
					for j := i - 1; j >= 0 && data[j] == '\\'; j-- {
						actualBackslashes++
					}
				} else {
					for j := len(result) - 1; j >= 0 && result[j] == '\\'; j-- {
						actualBackslashes++
					}
				}

				// If even number of backslashes (including 0), this is an actual \u202x escape to unescape
				// If odd number, the last backslash is escaping this one (\\u202x should stay)
				if actualBackslashes%2 == 0 {
					// Unescape: replace \u2028/\u2029 with literal character
					if result == nil {
						result = make([]byte, 0, len(data))
						result = append(result, data[:i]...)
					}
					if data[i+5] == '8' {
						result = append(result, "\u2028"...)
					} else {
						result = append(result, "\u2029"...)
					}
					i += 6
					continue
				}
			}
		}

		if result != nil {
			result = append(result, data[i])
		}
		i++
	}

	if result == nil {
		return data
	}
	return result
}

// marshalCanonicalArray marshals an array to canonical JSON.
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

// marshalCanonicalObject marshals an object to canonical JSON with RFC 8785 key ordering.
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
