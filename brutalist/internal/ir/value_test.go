package ir

import (
	"encoding/json"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIRValueSealed(t *testing.T) {
	// Verify all types implement IRValue (compile-time check via assignment)
	var _ IRValue = IRNull{}
	var _ IRValue = IRString("test")
	var _ IRValue = IRInt(42)
	var _ IRValue = IRBool(true)
	var _ IRValue = IRArray{IRString("a"), IRInt(1)}
	var _ IRValue = IRObject{"key": IRString("value")}
}

func TestIRObjectSortedKeys(t *testing.T) {
	obj := IRObject{
		"zebra":  IRString("z"),
		"apple":  IRString("a"),
		"banana": IRString("b"),
	}

	keys := obj.SortedKeys()

	assert.Equal(t, []string{"apple", "banana", "zebra"}, keys)
}

func TestIRObjectSortedKeysRFC8785Order(t *testing.T) {
	// RFC 8785 uses UTF-16 code unit ordering
	// For ASCII, this is the same as lexicographic, but we test edge cases
	obj := IRObject{
		"a":  IRInt(1),
		"A":  IRInt(2),
		"aa": IRInt(3),
		"aA": IRInt(4),
		"Aa": IRInt(5),
		"AA": IRInt(6),
	}

	keys := obj.SortedKeys()

	// UTF-16 order: uppercase before lowercase for same position
	// A (65) < Aa (65,97) < AA (65,65) < a (97) < aA (97,65) < aa (97,97)
	// Wait, that's not right. Let's verify the actual order.
	// 'A' = 65, 'a' = 97
	// So "A" < "AA" < "Aa" < "a" < "aA" < "aa"
	expected := []string{"A", "AA", "Aa", "a", "aA", "aa"}
	assert.Equal(t, expected, keys)
}

func TestIRObjectEmpty(t *testing.T) {
	obj := IRObject{}
	keys := obj.SortedKeys()
	assert.Empty(t, keys)
}

func TestIRArrayNested(t *testing.T) {
	arr := IRArray{
		IRString("outer"),
		IRArray{
			IRInt(1),
			IRInt(2),
			IRObject{"nested": IRBool(true)},
		},
	}

	// Just verify we can create nested structures
	assert.Len(t, arr, 2)

	inner, ok := arr[1].(IRArray)
	assert.True(t, ok)
	assert.Len(t, inner, 3)
}

func TestIRObjectNested(t *testing.T) {
	obj := IRObject{
		"level1": IRObject{
			"level2": IRObject{
				"value": IRInt(42),
			},
		},
	}

	level1 := obj["level1"].(IRObject)
	level2 := level1["level2"].(IRObject)
	value := level2["value"].(IRInt)

	assert.Equal(t, IRInt(42), value)
}

func TestNoIRFloatExists(t *testing.T) {
	// This test documents that IRFloat does not exist (CP-5)
	// The test passes by not having IRFloat to reference
	// If someone adds IRFloat, this comment should trigger a review

	// Verify int64 is used for numbers
	var num IRInt = 9223372036854775807 // max int64
	assert.Equal(t, IRInt(9223372036854775807), num)
}

func TestCompareKeysRFC8785(t *testing.T) {
	tests := []struct {
		a, b     string
		expected int
	}{
		{"a", "b", -1},
		{"b", "a", 1},
		{"a", "a", 0},
		{"aa", "a", 1},
		{"a", "aa", -1},
		{"A", "a", -32}, // 65 - 97
		{"", "", 0},
		{"", "a", -1},
	}

	for _, tt := range tests {
		t.Run(tt.a+"_vs_"+tt.b, func(t *testing.T) {
			result := compareKeysRFC8785(tt.a, tt.b)
			if tt.expected < 0 {
				assert.Less(t, result, 0)
			} else if tt.expected > 0 {
				assert.Greater(t, result, 0)
			} else {
				assert.Equal(t, 0, result)
			}
		})
	}
}

func TestIRNullMarshaling(t *testing.T) {
	// Test IRNull marshals to "null"
	data, err := json.Marshal(IRNull{})
	require.NoError(t, err)
	assert.Equal(t, "null", string(data))
}

func TestIRNullInObject(t *testing.T) {
	// Test IRNull in an object round-trips correctly
	obj := IRObject{
		"present": IRString("value"),
		"missing": IRNull{},
	}

	data, err := json.Marshal(obj)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"missing":null`)

	var decoded IRObject
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	// Verify IRNull is returned, not nil
	val := decoded["missing"]
	_, isNull := val.(IRNull)
	assert.True(t, isNull, "expected IRNull, got %T", val)
}

func TestIRNullInArray(t *testing.T) {
	// Test IRNull in an array round-trips correctly
	arr := IRArray{IRString("a"), IRNull{}, IRInt(1)}

	data, err := json.Marshal(arr)
	require.NoError(t, err)
	assert.Equal(t, `["a",null,1]`, string(data))

	var decoded IRArray
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	require.Len(t, decoded, 3)
	_, isNull := decoded[1].(IRNull)
	assert.True(t, isNull, "expected IRNull at index 1, got %T", decoded[1])
}

// ============================================================================
// Story 1-2: Comprehensive Tests
// ============================================================================

// TestUnmarshalRejectsFloats verifies that UnmarshalIRValue rejects floats (CP-5).
func TestUnmarshalRejectsFloats(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"simple float", `3.14`},
		{"scientific notation", `1e10`},
		{"scientific notation uppercase", `1E10`},
		{"negative float", `-2.5`},
		{"nested float in object", `{"value": 1.5}`},
		{"array with float", `[1, 2.0, 3]`},
		{"deeply nested float", `{"a": {"b": [1.5]}}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := UnmarshalIRValue([]byte(tt.input))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "float")
		})
	}
}

// TestUnmarshalRejectsNull verifies that UnmarshalIRValue rejects null values.
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

// TestSortedKeysUTF16Order tests the critical UTF-8 vs UTF-16 ordering difference.
// This is the canonical test that proves correct RFC 8785 implementation.
func TestSortedKeysUTF16Order(t *testing.T) {
	// CRITICAL TEST: U+E000 (Private Use Area) vs U+10000 (Linear B Syllable B008)
	//
	// U+E000 ("") - UTF-8: [0xEE, 0x80, 0x80], UTF-16: [0xE000]
	// U+10000 ("𐀀") - UTF-8: [0xF0, 0x90, 0x80, 0x80], UTF-16: [0xD800, 0xDC00]
	//
	// UTF-8 byte comparison: 0xEE < 0xF0, so "" < "𐀀"
	// UTF-16 code unit: 0xD800 < 0xE000, so "𐀀" < ""
	obj := IRObject{
		"\uE000": IRInt(1), // U+E000 (Private Use Area)
		"𐀀":      IRInt(2), // U+10000 (Linear B) - surrogate pair 0xD800, 0xDC00
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

// TestSortedKeysBasicCases tests common sorting scenarios.
func TestSortedKeysBasicCases(t *testing.T) {
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
				"a": IRInt(1),
				"":  IRInt(2),
			},
			expected: []string{"", "a"},
		},
		{
			name: "numbers as strings - lexicographic",
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

// TestMarshalIRValueRoundTrip tests MarshalIRValue and UnmarshalIRValue round-trip.
func TestMarshalIRValueRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		value IRValue
	}{
		{"string", IRString("hello")},
		{"empty string", IRString("")},
		{"int", IRInt(42)},
		{"negative int", IRInt(-100)},
		{"max int64", IRInt(9223372036854775807)},
		{"min int64", IRInt(-9223372036854775808)},
		{"bool true", IRBool(true)},
		{"bool false", IRBool(false)},
		{"empty array", IRArray{}},
		{"array of ints", IRArray{IRInt(1), IRInt(2), IRInt(3)}},
		{"empty object", IRObject{}},
		{"simple object", IRObject{"key": IRString("value")}},
		{"nested", IRObject{
			"array":  IRArray{IRInt(1), IRObject{"nested": IRBool(true)}},
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

// TestMarshalIRObjectKeyOrder verifies MarshalJSON produces sorted keys.
func TestMarshalIRObjectKeyOrder(t *testing.T) {
	obj := IRObject{
		"zebra": IRString("z"),
		"apple": IRString("a"),
		"mango": IRString("m"),
	}

	data, err := json.Marshal(obj)
	require.NoError(t, err)

	// Keys should appear in sorted order: apple, mango, zebra
	expected := `{"apple":"a","mango":"m","zebra":"z"}`
	assert.Equal(t, expected, string(data))
}

// TestHelperConstructors tests the ergonomic constructor functions.
func TestHelperConstructors(t *testing.T) {
	// Test NewIRString
	s := NewIRString("hello")
	assert.Equal(t, IRString("hello"), s)

	// Test NewIRInt
	n := NewIRInt(42)
	assert.Equal(t, IRInt(42), n)

	// Test NewIRBool
	b := NewIRBool(true)
	assert.Equal(t, IRBool(true), b)

	// Test NewIRArray
	arr := NewIRArray(IRString("a"), IRInt(1), IRBool(false))
	assert.Equal(t, IRArray{IRString("a"), IRInt(1), IRBool(false)}, arr)

	// Test NewIRObjectFromMap
	m := map[string]IRValue{"key": IRString("value")}
	obj := NewIRObjectFromMap(m)
	assert.Equal(t, IRObject{"key": IRString("value")}, obj)

	// Test NewIRObjectFromPairs
	obj2 := NewIRObjectFromPairs(
		IRPair{"name", IRString("test")},
		IRPair{"count", IRInt(5)},
	)
	assert.Equal(t, IRString("test"), obj2["name"])
	assert.Equal(t, IRInt(5), obj2["count"])

	// Test O helper
	obj3 := NewIRObjectFromPairs(
		O("name", NewIRString("cart")),
		O("count", NewIRInt(5)),
	)
	assert.Equal(t, IRString("cart"), obj3["name"])
	assert.Equal(t, IRInt(5), obj3["count"])
}

// TestEmptyValuesMarshaling tests edge cases with empty values.
func TestEmptyValuesMarshaling(t *testing.T) {
	tests := []struct {
		name     string
		value    IRValue
		expected string
	}{
		{"empty string", IRString(""), `""`},
		{"empty array", IRArray{}, `[]`},
		{"empty object", IRObject{}, `{}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := MarshalIRValue(tt.value)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, string(data))
		})
	}
}

// TestDeepNesting tests deeply nested structures.
func TestDeepNesting(t *testing.T) {
	deep := IRObject{
		"level1": IRObject{
			"level2": IRObject{
				"level3": IRArray{
					IRObject{
						"level4": IRInt(42),
					},
				},
			},
		},
	}

	data, err := MarshalIRValue(deep)
	require.NoError(t, err)

	result, err := UnmarshalIRValue(data)
	require.NoError(t, err)

	assert.Equal(t, deep, result)
}

// TestUnmarshalValidJSON tests that valid JSON without floats/nulls parses correctly.
func TestUnmarshalValidJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected IRValue
	}{
		{"string", `"hello"`, IRString("hello")},
		{"integer", `42`, IRInt(42)},
		{"negative integer", `-100`, IRInt(-100)},
		{"bool true", `true`, IRBool(true)},
		{"bool false", `false`, IRBool(false)},
		{"simple array", `[1,2,3]`, IRArray{IRInt(1), IRInt(2), IRInt(3)}},
		{"simple object", `{"a":1}`, IRObject{"a": IRInt(1)}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := UnmarshalIRValue([]byte(tt.input))
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
