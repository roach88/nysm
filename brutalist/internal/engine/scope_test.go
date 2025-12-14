package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/roach88/nysm/internal/ir"
)

// Story 3.7: Flow-Scoped Sync Matching tests
// These tests verify scope mode types, validation, and helper functions.

func TestValidateScopeMode_Valid(t *testing.T) {
	validModes := []string{
		"flow",
		"global",
		"keyed",
		"", // Empty defaults to flow
	}

	for _, mode := range validModes {
		t.Run(mode, func(t *testing.T) {
			err := ValidateScopeMode(mode)
			assert.NoError(t, err)
		})
	}
}

func TestValidateScopeMode_Invalid(t *testing.T) {
	invalidModes := []struct {
		mode string
		desc string
	}{
		{"invalid", "unknown mode"},
		{"FLOW", "case-sensitive - uppercase"},
		{"GLOBAL", "case-sensitive - uppercase"},
		{"Flow", "case-sensitive - mixed case"},
		{"keyed(user_id)", "keyed with field syntax not valid as raw mode"},
		{"  flow  ", "whitespace not trimmed"},
		{"local", "made-up mode"},
	}

	for _, tc := range invalidModes {
		t.Run(tc.desc, func(t *testing.T) {
			err := ValidateScopeMode(tc.mode)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid scope mode")
		})
	}
}

func TestDefaultScope_IsFlow(t *testing.T) {
	defaultScope := DefaultScope()

	assert.Equal(t, "flow", defaultScope.Mode)
	assert.Empty(t, defaultScope.Key)
}

func TestNormalizeScope_EmptyDefaultsToFlow(t *testing.T) {
	scope := ir.ScopeSpec{
		Mode: "",
		Key:  "",
	}

	normalized := NormalizeScope(scope)

	assert.Equal(t, "flow", normalized.Mode)
}

func TestNormalizeScope_PreservesExplicitMode(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"flow", "flow"},
		{"global", "global"},
		{"keyed", "keyed"},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			scope := ir.ScopeSpec{
				Mode: tc.input,
			}

			normalized := NormalizeScope(scope)

			assert.Equal(t, tc.expected, normalized.Mode)
		})
	}
}

func TestNormalizeScope_PreservesKey(t *testing.T) {
	scope := ir.ScopeSpec{
		Mode: "keyed",
		Key:  "user_id",
	}

	normalized := NormalizeScope(scope)

	assert.Equal(t, "keyed", normalized.Mode)
	assert.Equal(t, "user_id", normalized.Key)
}

func TestExtractKeyValue_Success(t *testing.T) {
	bindings := ir.IRObject{
		"user_id":   ir.IRString("user-123"),
		"tenant_id": ir.IRString("tenant-456"),
		"count":     ir.IRInt(42),
	}

	testCases := []struct {
		key      string
		expected ir.IRValue
	}{
		{"user_id", ir.IRString("user-123")},
		{"tenant_id", ir.IRString("tenant-456")},
		{"count", ir.IRInt(42)},
	}

	for _, tc := range testCases {
		t.Run(tc.key, func(t *testing.T) {
			value, err := extractKeyValue(bindings, tc.key)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, value)
		})
	}
}

func TestExtractKeyValue_MissingKey(t *testing.T) {
	bindings := ir.IRObject{
		"user_id": ir.IRString("user-123"),
	}

	value, err := extractKeyValue(bindings, "nonexistent_key")
	require.Error(t, err)
	assert.Nil(t, value)
	assert.Contains(t, err.Error(), "nonexistent_key")
	assert.Contains(t, err.Error(), "not found")
}

func TestExtractKeyValue_EmptyKey(t *testing.T) {
	bindings := ir.IRObject{
		"user_id": ir.IRString("user-123"),
	}

	value, err := extractKeyValue(bindings, "")
	require.Error(t, err)
	assert.Nil(t, value)
	assert.Contains(t, err.Error(), "non-empty key field")
}

func TestExtractKeyValue_EmptyBindings(t *testing.T) {
	bindings := ir.IRObject{}

	value, err := extractKeyValue(bindings, "user_id")
	require.Error(t, err)
	assert.Nil(t, value)
	assert.Contains(t, err.Error(), "not found")
}

func TestExtractKeyValue_NilBindings(t *testing.T) {
	var bindings ir.IRObject = nil

	value, err := extractKeyValue(bindings, "user_id")
	require.Error(t, err)
	assert.Nil(t, value)
	assert.Contains(t, err.Error(), "not found")
}

func TestExtractKeyValue_AllIRValueTypes(t *testing.T) {
	bindings := ir.IRObject{
		"string_key": ir.IRString("hello"),
		"int_key":    ir.IRInt(42),
		"bool_key":   ir.IRBool(true),
		"array_key":  ir.IRArray{ir.IRInt(1), ir.IRInt(2)},
		"object_key": ir.IRObject{"nested": ir.IRString("value")},
	}

	testCases := []struct {
		key      string
		expected ir.IRValue
	}{
		{"string_key", ir.IRString("hello")},
		{"int_key", ir.IRInt(42)},
		{"bool_key", ir.IRBool(true)},
		{"array_key", ir.IRArray{ir.IRInt(1), ir.IRInt(2)}},
		{"object_key", ir.IRObject{"nested": ir.IRString("value")}},
	}

	for _, tc := range testCases {
		t.Run(tc.key, func(t *testing.T) {
			value, err := extractKeyValue(bindings, tc.key)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, value)
		})
	}
}

func TestMergeBindings_BothEmpty(t *testing.T) {
	result := mergeBindings(ir.IRObject{}, ir.IRObject{})
	assert.Equal(t, ir.IRObject{}, result)
}

func TestMergeBindings_BothNil(t *testing.T) {
	result := mergeBindings(nil, nil)
	assert.Equal(t, ir.IRObject{}, result)
}

func TestMergeBindings_WhenBindingsOnly(t *testing.T) {
	whenBindings := ir.IRObject{
		"a": ir.IRString("from-when"),
		"b": ir.IRInt(1),
	}

	result := mergeBindings(whenBindings, nil)

	assert.Equal(t, ir.IRString("from-when"), result["a"])
	assert.Equal(t, ir.IRInt(1), result["b"])
	assert.Len(t, result, 2)
}

func TestMergeBindings_WhereBindingsOnly(t *testing.T) {
	whereBindings := ir.IRObject{
		"c": ir.IRString("from-where"),
		"d": ir.IRInt(2),
	}

	result := mergeBindings(nil, whereBindings)

	assert.Equal(t, ir.IRString("from-where"), result["c"])
	assert.Equal(t, ir.IRInt(2), result["d"])
	assert.Len(t, result, 2)
}

func TestMergeBindings_NoConflict(t *testing.T) {
	whenBindings := ir.IRObject{
		"a": ir.IRString("from-when"),
		"b": ir.IRInt(1),
	}
	whereBindings := ir.IRObject{
		"c": ir.IRString("from-where"),
		"d": ir.IRInt(2),
	}

	result := mergeBindings(whenBindings, whereBindings)

	assert.Equal(t, ir.IRString("from-when"), result["a"])
	assert.Equal(t, ir.IRInt(1), result["b"])
	assert.Equal(t, ir.IRString("from-where"), result["c"])
	assert.Equal(t, ir.IRInt(2), result["d"])
	assert.Len(t, result, 4)
}

func TestMergeBindings_WhereOverridesWhen(t *testing.T) {
	whenBindings := ir.IRObject{
		"a": ir.IRString("from-when"),
		"b": ir.IRInt(1), // Will be overridden
	}
	whereBindings := ir.IRObject{
		"b": ir.IRInt(2), // Overrides when-binding
		"c": ir.IRString("from-where"),
	}

	result := mergeBindings(whenBindings, whereBindings)

	assert.Equal(t, ir.IRString("from-when"), result["a"])
	assert.Equal(t, ir.IRInt(2), result["b"], "where-binding should override when-binding")
	assert.Equal(t, ir.IRString("from-where"), result["c"])
	assert.Len(t, result, 3)
}

func TestMergeBindings_DoesNotMutateInputs(t *testing.T) {
	whenBindings := ir.IRObject{
		"a": ir.IRString("original-a"),
	}
	whereBindings := ir.IRObject{
		"b": ir.IRString("original-b"),
	}

	// Make copies of original values
	whenCopy := ir.IRObject{"a": ir.IRString("original-a")}
	whereCopy := ir.IRObject{"b": ir.IRString("original-b")}

	result := mergeBindings(whenBindings, whereBindings)

	// Verify result is correct
	assert.Len(t, result, 2)

	// Verify originals were not mutated
	assert.Equal(t, whenCopy, whenBindings, "whenBindings should not be mutated")
	assert.Equal(t, whereCopy, whereBindings, "whereBindings should not be mutated")
}

func TestScopeModeConstants(t *testing.T) {
	// Verify constant values match expected strings
	assert.Equal(t, ScopeMode("flow"), ScopeModeFlow)
	assert.Equal(t, ScopeMode("global"), ScopeModeGlobal)
	assert.Equal(t, ScopeMode("keyed"), ScopeModeKeyed)
}

// Tests for scope validation in sync rules

func TestSyncRuleWithScope_Flow(t *testing.T) {
	sync := ir.SyncRule{
		ID: "test-sync",
		Scope: ir.ScopeSpec{
			Mode: "flow",
		},
	}

	err := ValidateScopeMode(sync.Scope.Mode)
	assert.NoError(t, err)
}

func TestSyncRuleWithScope_Global(t *testing.T) {
	sync := ir.SyncRule{
		ID: "test-sync",
		Scope: ir.ScopeSpec{
			Mode: "global",
		},
	}

	err := ValidateScopeMode(sync.Scope.Mode)
	assert.NoError(t, err)
}

func TestSyncRuleWithScope_Keyed(t *testing.T) {
	sync := ir.SyncRule{
		ID: "test-sync",
		Scope: ir.ScopeSpec{
			Mode: "keyed",
			Key:  "user_id",
		},
	}

	err := ValidateScopeMode(sync.Scope.Mode)
	assert.NoError(t, err)
	assert.Equal(t, "user_id", sync.Scope.Key)
}

func TestSyncRuleWithScope_EmptyDefaultsToFlow(t *testing.T) {
	sync := ir.SyncRule{
		ID: "test-sync",
		Scope: ir.ScopeSpec{
			Mode: "", // Empty
		},
	}

	normalized := NormalizeScope(sync.Scope)
	assert.Equal(t, "flow", normalized.Mode)
}
