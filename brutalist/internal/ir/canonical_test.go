package ir

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarshalCanonicalBasic(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{"string", IRString("hello"), `"hello"`},
		{"empty string", IRString(""), `""`},
		{"int", IRInt(42), "42"},
		{"negative int", IRInt(-100), "-100"},
		{"zero", IRInt(0), "0"},
		{"max int64", IRInt(9223372036854775807), "9223372036854775807"},
		{"min int64", IRInt(-9223372036854775808), "-9223372036854775808"},
		{"bool true", IRBool(true), "true"},
		{"bool false", IRBool(false), "false"},
		{"empty array", IRArray{}, "[]"},
		{"empty object", IRObject{}, "{}"},
		{"array of ints", IRArray{IRInt(1), IRInt(2), IRInt(3)}, "[1,2,3]"},
		{"simple object", IRObject{"a": IRInt(1)}, `{"a":1}`},
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

func TestMarshalCanonicalNestedSortedKeys(t *testing.T) {
	obj := IRObject{
		"z": IRObject{
			"b": IRInt(1),
			"a": IRInt(2),
		},
		"a": IRInt(3),
	}

	result, err := MarshalCanonical(obj)
	require.NoError(t, err)
	assert.Equal(t, `{"a":3,"z":{"a":2,"b":1}}`, string(result))
}

func TestMarshalCanonicalUTF16Ordering(t *testing.T) {
	// U+E000 vs U+10000 - UTF-16 order differs from UTF-8
	// This is THE critical test for RFC 8785 compliance
	obj := IRObject{
		"\uE000": IRInt(1), // UTF-16: 0xE000
		"𐀀":      IRInt(2), // UTF-16: 0xD800, 0xDC00 (surrogate pair)
	}

	result, err := MarshalCanonical(obj)
	require.NoError(t, err)

	// UTF-16 order: 0xD800 < 0xE000, so 𐀀 comes first
	// The key with U+10000 (𐀀) should appear before U+E000 ()
	expected := `{"𐀀":2,"` + "\uE000" + `":1}`
	assert.Equal(t, expected, string(result))
}

func TestMarshalCanonicalNoHTMLEscape(t *testing.T) {
	tests := []struct {
		name     string
		input    IRValue
		expected string
	}{
		{
			name:     "less than",
			input:    IRString("<script>"),
			expected: `"<script>"`,
		},
		{
			name:     "greater than",
			input:    IRString("</script>"),
			expected: `"</script>"`,
		},
		{
			name:     "ampersand",
			input:    IRString("a & b"),
			expected: `"a & b"`,
		},
		{
			name:     "all html chars",
			input:    IRString("<script>alert('xss')</script>"),
			expected: `"<script>alert('xss')</script>"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := MarshalCanonical(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, string(result))

			// Verify NO HTML escaping sequences present
			assert.NotContains(t, string(result), "\\u003c") // <
			assert.NotContains(t, string(result), "\\u003e") // >
			assert.NotContains(t, string(result), "\\u0026") // &
		})
	}
}

func TestMarshalCanonicalHTMLInObject(t *testing.T) {
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
	assert.NotContains(t, string(result), "\\u003c")
	assert.NotContains(t, string(result), "\\u003e")
	assert.NotContains(t, string(result), "\\u0026")
}

func TestMarshalCanonicalRejectsFloats(t *testing.T) {
	tests := []struct {
		name  string
		input any
	}{
		{"float64", float64(3.14)},
		{"float32", float32(3.14)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := MarshalCanonical(tt.input)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "float")
		})
	}
}

func TestMarshalCanonicalRejectsNull(t *testing.T) {
	_, err := MarshalCanonical(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "null")
}

func TestMarshalCanonicalRejectsIRNull(t *testing.T) {
	_, err := MarshalCanonical(IRNull{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "null")
}

func TestMarshalCanonicalNFCNormalization(t *testing.T) {
	// "é" can be represented as:
	// - U+00E9 (precomposed, NFC form)
	// - U+0065 U+0301 (e + combining acute accent, NFD form)
	// NFC normalizes both to U+00E9
	composed := "caf\u00E9"      // café with precomposed é
	decomposed := "cafe\u0301"   // café with e + combining accent

	result1, err := MarshalCanonical(IRString(composed))
	require.NoError(t, err)

	result2, err := MarshalCanonical(IRString(decomposed))
	require.NoError(t, err)

	// Both should produce identical canonical output
	assert.Equal(t, result1, result2, "NFC normalization should make these equal")
}

func TestMarshalCanonicalNFCInObjectKeys(t *testing.T) {
	// Object keys should also be NFC normalized
	composed := "caf\u00E9"
	decomposed := "cafe\u0301"

	obj1 := IRObject{composed: IRInt(1)}
	obj2 := IRObject{decomposed: IRInt(1)}

	result1, err := MarshalCanonical(obj1)
	require.NoError(t, err)

	result2, err := MarshalCanonical(obj2)
	require.NoError(t, err)

	assert.Equal(t, result1, result2, "NFC normalization should make object keys equal")
}

func TestMarshalCanonicalCompactOutput(t *testing.T) {
	obj := IRObject{
		"array": IRArray{IRInt(1), IRInt(2)},
		"bool":  IRBool(true),
		"int":   IRInt(42),
	}

	result, err := MarshalCanonical(obj)
	require.NoError(t, err)

	// No whitespace
	assert.NotContains(t, string(result), " ")
	assert.NotContains(t, string(result), "\n")
	assert.NotContains(t, string(result), "\t")
}

func TestMarshalCanonicalIdempotency(t *testing.T) {
	// Property: MarshalCanonical(Unmarshal(MarshalCanonical(x))) == MarshalCanonical(x)
	testCases := []IRValue{
		IRString("hello"),
		IRInt(42),
		IRBool(true),
		IRArray{IRInt(1), IRString("two"), IRBool(false)},
		IRObject{"a": IRInt(1), "b": IRString("test")},
		IRObject{
			"nested": IRObject{
				"array": IRArray{IRInt(1), IRInt(2)},
			},
			"simple": IRString("value"),
		},
	}

	for _, original := range testCases {
		// Marshal canonically
		canonical1, err := MarshalCanonical(original)
		require.NoError(t, err)

		// Unmarshal
		val, err := UnmarshalIRValue(canonical1)
		require.NoError(t, err)

		// Marshal again
		canonical2, err := MarshalCanonical(val)
		require.NoError(t, err)

		// Must be identical
		assert.Equal(t, canonical1, canonical2, "canonical marshaling must be idempotent")
	}
}

func TestMarshalCanonicalWithGoTypes(t *testing.T) {
	// Test that raw Go types also work
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{"string", "hello", `"hello"`},
		{"int64", int64(42), "42"},
		{"int", 42, "42"},
		{"bool", true, "true"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := MarshalCanonical(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, string(result))
		})
	}
}

func TestMarshalCanonicalWithMapStringAny(t *testing.T) {
	input := map[string]any{
		"b": int64(1),
		"a": "test",
	}

	result, err := MarshalCanonical(input)
	require.NoError(t, err)
	assert.Equal(t, `{"a":"test","b":1}`, string(result))
}

func TestMarshalCanonicalWithSliceAny(t *testing.T) {
	input := []any{int64(1), "two", true}

	result, err := MarshalCanonical(input)
	require.NoError(t, err)
	assert.Equal(t, `[1,"two",true]`, string(result))
}

func TestMarshalCanonicalStringEscaping(t *testing.T) {
	// Standard JSON escapes should still work
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"newline", "a\nb", `"a\nb"`},
		{"tab", "a\tb", `"a\tb"`},
		{"quote", `a"b`, `"a\"b"`},
		{"backslash", `a\b`, `"a\\b"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := MarshalCanonical(IRString(tt.input))
			require.NoError(t, err)
			assert.Equal(t, tt.expected, string(result))
		})
	}
}

func TestMarshalCanonicalU2028U2029NotEscaped(t *testing.T) {
	// RFC 8785: U+2028 (LINE SEPARATOR) and U+2029 (PARAGRAPH SEPARATOR) should NOT be escaped.
	// Only control characters (U+0000-U+001F), backslash, and quote should be escaped.
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "U+2028 LINE SEPARATOR",
			input:    "hello\u2028world",
			expected: "\"hello\u2028world\"",
		},
		{
			name:     "U+2029 PARAGRAPH SEPARATOR",
			input:    "hello\u2029world",
			expected: "\"hello\u2029world\"",
		},
		{
			name:     "both U+2028 and U+2029",
			input:    "a\u2028b\u2029c",
			expected: "\"a\u2028b\u2029c\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := MarshalCanonical(IRString(tt.input))
			require.NoError(t, err)
			assert.Equal(t, tt.expected, string(result))

			// CRITICAL: Must NOT contain escaped forms
			assert.NotContains(t, string(result), `\u2028`, "U+2028 should not be escaped per RFC 8785")
			assert.NotContains(t, string(result), `\u2029`, "U+2029 should not be escaped per RFC 8785")
		})
	}
}

func TestMarshalCanonicalU2028U2029InObject(t *testing.T) {
	// U+2028/U+2029 in object keys and values
	obj := IRObject{
		"key\u2028with\u2029separators": IRString("value\u2028with\u2029separators"),
	}

	result, err := MarshalCanonical(obj)
	require.NoError(t, err)

	// Must NOT contain escaped forms
	assert.NotContains(t, string(result), `\u2028`)
	assert.NotContains(t, string(result), `\u2029`)

	// Must contain literal characters
	assert.Contains(t, string(result), "\u2028")
	assert.Contains(t, string(result), "\u2029")
}

func TestMarshalCanonicalLiteralBackslashU2028(t *testing.T) {
	// Regression test: Strings containing literal backslash followed by "u2028"
	// should NOT be affected by the U+2028 un-escaping logic.
	// The fix uses bytes.ReplaceAll on `\u2028` (the 6-byte escape sequence),
	// NOT on strings that happen to contain "\\u2028" as literal text.
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "literal backslash-u2028 text",
			input:    `the escape sequence is \u2028`,
			expected: `"the escape sequence is \\u2028"`,
		},
		{
			name:     "literal backslash-u2029 text",
			input:    `the escape sequence is \u2029`,
			expected: `"the escape sequence is \\u2029"`,
		},
		{
			name:     "mixed literal and actual",
			input:    "literal \\u2028 and actual \u2028",
			expected: "\"literal \\\\u2028 and actual \u2028\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := MarshalCanonical(IRString(tt.input))
			require.NoError(t, err)
			assert.Equal(t, tt.expected, string(result))
		})
	}
}

// FuzzMarshalCanonicalIdempotent tests the idempotency property via fuzzing
func FuzzMarshalCanonicalIdempotent(f *testing.F) {
	// Seed with various JSON patterns
	f.Add(`{"a":1,"b":"test"}`)
	f.Add(`[1,2,3]`)
	f.Add(`"hello"`)
	f.Add(`42`)
	f.Add(`true`)
	f.Add(`{"nested":{"deep":{"value":123}}}`)

	f.Fuzz(func(t *testing.T, jsonStr string) {
		// Parse as IRValue (skip if invalid or contains floats/null)
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
