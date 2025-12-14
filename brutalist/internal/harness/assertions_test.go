package harness

import (
	"context"
	"testing"

	"github.com/roach88/nysm/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAssertTraceContains_Found(t *testing.T) {
	trace := []TraceEvent{
		{Type: "invocation", ActionURI: "Cart.addItem", Args: map[string]interface{}{"item_id": "widget", "quantity": 3}, Seq: 1},
		{Type: "completion", OutputCase: "Success", Seq: 2},
	}

	assertion := Assertion{
		Type:   AssertTraceContains,
		Action: "Cart.addItem",
		Args:   map[string]interface{}{"item_id": "widget"},
	}

	err := assertTraceContains(trace, assertion)
	assert.NoError(t, err)
}

func TestAssertTraceContains_NotFound(t *testing.T) {
	trace := []TraceEvent{
		{Type: "invocation", ActionURI: "Cart.addItem", Args: map[string]interface{}{"item_id": "widget"}, Seq: 1},
		{Type: "completion", OutputCase: "Success", Seq: 2},
	}

	assertion := Assertion{
		Type:   AssertTraceContains,
		Action: "Cart.checkout", // Different action
	}

	err := assertTraceContains(trace, assertion)
	require.Error(t, err)

	assertErr, ok := err.(*AssertionError)
	require.True(t, ok)
	assert.Equal(t, "trace_contains", assertErr.Type)
	assert.Contains(t, assertErr.Expected, "Cart.checkout")
	assert.Equal(t, "not found in trace", assertErr.Actual)
}

func TestAssertTraceContains_WrongArgs(t *testing.T) {
	trace := []TraceEvent{
		{Type: "invocation", ActionURI: "Cart.addItem", Args: map[string]interface{}{"item_id": "widget"}, Seq: 1},
		{Type: "completion", OutputCase: "Success", Seq: 2},
	}

	assertion := Assertion{
		Type:   AssertTraceContains,
		Action: "Cart.addItem",
		Args:   map[string]interface{}{"item_id": "gadget"}, // Wrong value
	}

	err := assertTraceContains(trace, assertion)
	require.Error(t, err)
}

func TestAssertTraceContains_SubsetMatch(t *testing.T) {
	// Actual has more args than expected - should still match
	trace := []TraceEvent{
		{Type: "invocation", ActionURI: "Cart.addItem", Args: map[string]interface{}{
			"item_id":  "widget",
			"quantity": 3,
			"metadata": map[string]interface{}{"source": "web"},
		}, Seq: 1},
	}

	assertion := Assertion{
		Type:   AssertTraceContains,
		Action: "Cart.addItem",
		Args:   map[string]interface{}{"item_id": "widget"}, // Only checking item_id
	}

	err := assertTraceContains(trace, assertion)
	assert.NoError(t, err)
}

func TestAssertTraceContains_NoArgsRequired(t *testing.T) {
	trace := []TraceEvent{
		{Type: "invocation", ActionURI: "Cart.checkout", Args: map[string]interface{}{"cart_id": "abc"}, Seq: 1},
	}

	assertion := Assertion{
		Type:   AssertTraceContains,
		Action: "Cart.checkout",
		// No Args specified - should match any invocation of this action
	}

	err := assertTraceContains(trace, assertion)
	assert.NoError(t, err)
}

func TestAssertTraceOrder_Correct(t *testing.T) {
	trace := []TraceEvent{
		{Type: "invocation", ActionURI: "Inventory.reserve", Seq: 1},
		{Type: "completion", OutputCase: "Success", Seq: 2},
		{Type: "invocation", ActionURI: "Payment.charge", Seq: 3},
		{Type: "completion", OutputCase: "Success", Seq: 4},
		{Type: "invocation", ActionURI: "Order.create", Seq: 5},
		{Type: "completion", OutputCase: "Success", Seq: 6},
	}

	assertion := Assertion{
		Type:    AssertTraceOrder,
		Actions: []string{"Inventory.reserve", "Payment.charge", "Order.create"},
	}

	err := assertTraceOrder(trace, assertion)
	assert.NoError(t, err)
}

func TestAssertTraceOrder_WrongOrder(t *testing.T) {
	trace := []TraceEvent{
		{Type: "invocation", ActionURI: "Payment.charge", Seq: 1}, // Payment before inventory
		{Type: "completion", OutputCase: "Success", Seq: 2},
		{Type: "invocation", ActionURI: "Inventory.reserve", Seq: 3},
		{Type: "completion", OutputCase: "Success", Seq: 4},
	}

	assertion := Assertion{
		Type:    AssertTraceOrder,
		Actions: []string{"Inventory.reserve", "Payment.charge"}, // Expected: inventory first
	}

	err := assertTraceOrder(trace, assertion)
	require.Error(t, err)

	assertErr, ok := err.(*AssertionError)
	require.True(t, ok)
	assert.Equal(t, "trace_order", assertErr.Type)
	assert.Contains(t, assertErr.Actual, "should be before")
}

func TestAssertTraceOrder_MissingAction(t *testing.T) {
	trace := []TraceEvent{
		{Type: "invocation", ActionURI: "Inventory.reserve", Seq: 1},
		{Type: "completion", OutputCase: "Success", Seq: 2},
	}

	assertion := Assertion{
		Type:    AssertTraceOrder,
		Actions: []string{"Inventory.reserve", "Payment.charge"}, // Payment not in trace
	}

	err := assertTraceOrder(trace, assertion)
	require.Error(t, err)

	assertErr, ok := err.(*AssertionError)
	require.True(t, ok)
	assert.Contains(t, assertErr.Actual, "missing action")
	assert.Contains(t, assertErr.Actual, "Payment.charge")
}

func TestAssertTraceOrder_InterveningActionsAllowed(t *testing.T) {
	// Actions don't need to be consecutive
	trace := []TraceEvent{
		{Type: "invocation", ActionURI: "Inventory.reserve", Seq: 1},
		{Type: "completion", OutputCase: "Success", Seq: 2},
		{Type: "invocation", ActionURI: "Log.info", Seq: 3}, // Intervening action
		{Type: "completion", OutputCase: "Success", Seq: 4},
		{Type: "invocation", ActionURI: "Payment.charge", Seq: 5},
		{Type: "completion", OutputCase: "Success", Seq: 6},
	}

	assertion := Assertion{
		Type:    AssertTraceOrder,
		Actions: []string{"Inventory.reserve", "Payment.charge"},
	}

	err := assertTraceOrder(trace, assertion)
	assert.NoError(t, err)
}

func TestAssertTraceCount_Exact(t *testing.T) {
	trace := []TraceEvent{
		{Type: "invocation", ActionURI: "Notification.send", Seq: 1},
		{Type: "completion", OutputCase: "Success", Seq: 2},
		{Type: "invocation", ActionURI: "Notification.send", Seq: 3},
		{Type: "completion", OutputCase: "Success", Seq: 4},
		{Type: "invocation", ActionURI: "Notification.send", Seq: 5},
		{Type: "completion", OutputCase: "Success", Seq: 6},
	}

	assertion := Assertion{
		Type:   AssertTraceCount,
		Action: "Notification.send",
		Count:  3,
	}

	err := assertTraceCount(trace, assertion)
	assert.NoError(t, err)
}

func TestAssertTraceCount_TooFew(t *testing.T) {
	trace := []TraceEvent{
		{Type: "invocation", ActionURI: "Notification.send", Seq: 1},
		{Type: "completion", OutputCase: "Success", Seq: 2},
	}

	assertion := Assertion{
		Type:   AssertTraceCount,
		Action: "Notification.send",
		Count:  3, // Expected 3, got 1
	}

	err := assertTraceCount(trace, assertion)
	require.Error(t, err)

	assertErr, ok := err.(*AssertionError)
	require.True(t, ok)
	assert.Equal(t, "trace_count", assertErr.Type)
	assert.Contains(t, assertErr.Expected, "3 occurrences")
	assert.Contains(t, assertErr.Actual, "1 occurrences")
}

func TestAssertTraceCount_TooMany(t *testing.T) {
	trace := []TraceEvent{
		{Type: "invocation", ActionURI: "Notification.send", Seq: 1},
		{Type: "completion", OutputCase: "Success", Seq: 2},
		{Type: "invocation", ActionURI: "Notification.send", Seq: 3},
		{Type: "completion", OutputCase: "Success", Seq: 4},
	}

	assertion := Assertion{
		Type:   AssertTraceCount,
		Action: "Notification.send",
		Count:  1, // Expected 1, got 2
	}

	err := assertTraceCount(trace, assertion)
	require.Error(t, err)

	assertErr, ok := err.(*AssertionError)
	require.True(t, ok)
	assert.Contains(t, assertErr.Actual, "2 occurrences")
}

func TestAssertTraceCount_Zero(t *testing.T) {
	trace := []TraceEvent{
		{Type: "invocation", ActionURI: "Cart.addItem", Seq: 1},
		{Type: "completion", OutputCase: "Success", Seq: 2},
	}

	// Assert that an action does NOT appear
	assertion := Assertion{
		Type:   AssertTraceCount,
		Action: "Error.log",
		Count:  0,
	}

	err := assertTraceCount(trace, assertion)
	assert.NoError(t, err)
}

func TestMatchArgs_SubsetSemantics(t *testing.T) {
	tests := []struct {
		name     string
		actual   interface{}
		expected map[string]interface{}
		want     bool
	}{
		{
			name:     "exact_match",
			actual:   map[string]interface{}{"key": "value"},
			expected: map[string]interface{}{"key": "value"},
			want:     true,
		},
		{
			name:     "subset_match",
			actual:   map[string]interface{}{"key1": "value1", "key2": "value2"},
			expected: map[string]interface{}{"key1": "value1"},
			want:     true,
		},
		{
			name:     "missing_key",
			actual:   map[string]interface{}{"key1": "value1"},
			expected: map[string]interface{}{"key1": "value1", "key2": "value2"},
			want:     false,
		},
		{
			name:     "value_mismatch",
			actual:   map[string]interface{}{"key": "actual"},
			expected: map[string]interface{}{"key": "expected"},
			want:     false,
		},
		{
			name:     "empty_expected",
			actual:   map[string]interface{}{"key": "value"},
			expected: map[string]interface{}{},
			want:     true,
		},
		{
			name:     "nil_expected",
			actual:   map[string]interface{}{"key": "value"},
			expected: nil,
			want:     true,
		},
		{
			name:     "nested_match",
			actual:   map[string]interface{}{"outer": map[string]interface{}{"inner": "value"}},
			expected: map[string]interface{}{"outer": map[string]interface{}{"inner": "value"}},
			want:     true,
		},
		{
			name:     "non_map_actual",
			actual:   "not a map",
			expected: map[string]interface{}{"key": "value"},
			want:     false,
		},
		{
			name:     "int_match",
			actual:   map[string]interface{}{"count": 42},
			expected: map[string]interface{}{"count": 42},
			want:     true,
		},
		{
			name:     "bool_match",
			actual:   map[string]interface{}{"enabled": true},
			expected: map[string]interface{}{"enabled": true},
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchArgs(tt.actual, tt.expected)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestValuesEqual(t *testing.T) {
	tests := []struct {
		name     string
		actual   interface{}
		expected interface{}
		want     bool
	}{
		{"both_nil", nil, nil, true},
		{"actual_nil", nil, "value", false},
		{"expected_nil", "value", nil, false},
		{"strings_equal", "hello", "hello", true},
		{"strings_different", "hello", "world", false},
		{"ints_equal", 42, 42, true},
		{"ints_different", 42, 43, false},
		{"bools_equal", true, true, true},
		{"bools_different", true, false, false},
		{"arrays_equal", []interface{}{"a", "b"}, []interface{}{"a", "b"}, true},
		{"arrays_different", []interface{}{"a", "b"}, []interface{}{"a", "c"}, false},
		{"maps_equal",
			map[string]interface{}{"key": "value"},
			map[string]interface{}{"key": "value"},
			true},
		{"maps_different",
			map[string]interface{}{"key": "value1"},
			map[string]interface{}{"key": "value2"},
			false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := valuesEqual(tt.actual, tt.expected)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestEvaluateAssertions_AllPass(t *testing.T) {
	result := &Result{
		Trace: []TraceEvent{
			{Type: "invocation", ActionURI: "Cart.addItem", Args: map[string]interface{}{"item_id": "widget"}, Seq: 1},
			{Type: "completion", OutputCase: "Success", Seq: 2},
			{Type: "invocation", ActionURI: "Cart.checkout", Seq: 3},
			{Type: "completion", OutputCase: "Success", Seq: 4},
		},
	}

	assertions := []Assertion{
		{Type: AssertTraceContains, Action: "Cart.addItem"},
		{Type: AssertTraceContains, Action: "Cart.checkout"},
		{Type: AssertTraceOrder, Actions: []string{"Cart.addItem", "Cart.checkout"}},
		{Type: AssertTraceCount, Action: "Cart.addItem", Count: 1},
	}

	errors := EvaluateAssertions(result, assertions, nil)
	assert.Empty(t, errors)
}

func TestEvaluateAssertions_SomeFail(t *testing.T) {
	result := &Result{
		Trace: []TraceEvent{
			{Type: "invocation", ActionURI: "Cart.addItem", Seq: 1},
			{Type: "completion", OutputCase: "Success", Seq: 2},
		},
	}

	assertions := []Assertion{
		{Type: AssertTraceContains, Action: "Cart.addItem"},  // Should pass
		{Type: AssertTraceContains, Action: "Cart.checkout"}, // Should fail - not in trace
		{Type: AssertTraceCount, Action: "Cart.addItem", Count: 3}, // Should fail - count is 1, not 3
	}

	errors := EvaluateAssertions(result, assertions, nil)
	require.Len(t, errors, 2)
	assert.Contains(t, errors[0], "Cart.checkout")
	assert.Contains(t, errors[1], "3 occurrences")
}

func TestEvaluateAssertions_UnknownType(t *testing.T) {
	result := &Result{
		Trace: []TraceEvent{},
	}

	assertions := []Assertion{
		{Type: "unknown_assertion_type"},
	}

	errors := EvaluateAssertions(result, assertions, nil)
	require.Len(t, errors, 1)
	assert.Contains(t, errors[0], "unknown assertion type")
}

func TestAssertionError_ErrorFormat(t *testing.T) {
	trace := []TraceEvent{
		{Type: "invocation", ActionURI: "Cart.addItem", Args: map[string]interface{}{"item": "widget"}, Seq: 1},
		{Type: "completion", OutputCase: "Success", Seq: 2},
	}

	err := &AssertionError{
		Type:     "trace_contains",
		Expected: "action Cart.checkout with args map[]",
		Actual:   "not found in trace",
		Trace:    trace,
	}

	errorStr := err.Error()
	assert.Contains(t, errorStr, "Assertion failed: trace_contains")
	assert.Contains(t, errorStr, "Expected: action Cart.checkout")
	assert.Contains(t, errorStr, "Actual: not found in trace")
	assert.Contains(t, errorStr, "Full trace:")
	assert.Contains(t, errorStr, "Cart.addItem")
}

// Final State Assertion Tests

func TestBuildWhereClause_Empty(t *testing.T) {
	sql, args, err := buildWhereClause(nil)
	require.NoError(t, err)
	assert.Equal(t, "", sql)
	assert.Nil(t, args)
}

func TestBuildWhereClause_SingleKey(t *testing.T) {
	where := map[string]interface{}{
		"item_id": "widget",
	}
	sql, args, err := buildWhereClause(where)
	require.NoError(t, err)
	assert.Equal(t, "item_id = ?", sql)
	assert.Equal(t, []interface{}{"widget"}, args)
}

func TestBuildWhereClause_MultipleKeys_SortedDeterministic(t *testing.T) {
	where := map[string]interface{}{
		"status":  "active",
		"item_id": "widget",
	}
	sql, args, err := buildWhereClause(where)
	require.NoError(t, err)
	// Keys should be sorted alphabetically
	assert.Equal(t, "item_id = ? AND status = ?", sql)
	assert.Equal(t, []interface{}{"widget", "active"}, args)
}

func TestBuildWhereClause_NoInterpolation(t *testing.T) {
	// Ensure values are NEVER interpolated into SQL string (HIGH-3)
	where := map[string]interface{}{
		"item_id": "widget'; DROP TABLE users; --",
	}
	sql, args, err := buildWhereClause(where)
	require.NoError(t, err)
	// SQL should NOT contain the malicious value
	assert.NotContains(t, sql, "widget")
	assert.NotContains(t, sql, "DROP TABLE")
	// Value should only be in args
	assert.Contains(t, args, "widget'; DROP TABLE users; --")
}

func TestBuildWhereClause_InvalidColumnName(t *testing.T) {
	// Ensure invalid column names are rejected
	tests := []struct {
		name   string
		column string
	}{
		{"sql_injection", "item_id; DROP TABLE users; --"},
		{"starts_with_number", "1column"},
		{"contains_space", "item id"},
		{"contains_hyphen", "item-id"},
		{"contains_semicolon", "item;id"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			where := map[string]interface{}{
				tt.column: "value",
			}
			_, _, err := buildWhereClause(where)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid column name")
		})
	}
}

func TestAssertFinalState_InvalidTableName(t *testing.T) {
	// Ensure invalid table names are rejected
	ctx := context.Background()
	assertion := Assertion{
		Type:  AssertFinalState,
		Table: "users; DROP TABLE users; --",
	}

	err := assertFinalState(ctx, nil, assertion)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid table name")
}

func TestToSQLValue_Types(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		{"string", "hello", "hello"},
		{"int", 42, 42},
		{"int64", int64(42), int64(42)},
		{"bool_true", true, true},
		{"bool_false", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toSQLValue(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatWhereClause_Empty(t *testing.T) {
	result := formatWhereClause(nil)
	assert.Equal(t, "(no conditions)", result)
}

func TestFormatWhereClause_Multiple(t *testing.T) {
	where := map[string]interface{}{
		"status":  "active",
		"item_id": "widget",
	}
	result := formatWhereClause(where)
	// Keys should be sorted
	assert.Equal(t, "item_id=widget AND status=active", result)
}

func TestStateValuesEqual_Strings(t *testing.T) {
	assert.True(t, stateValuesEqual("hello", "hello"))
	assert.False(t, stateValuesEqual("hello", "world"))
	assert.False(t, stateValuesEqual("hello", 42))
}

func TestStateValuesEqual_Integers(t *testing.T) {
	assert.True(t, stateValuesEqual(int64(42), int64(42)))
	assert.True(t, stateValuesEqual(42, int64(42)))     // int to int64
	assert.False(t, stateValuesEqual(int64(42), int64(43)))
	assert.False(t, stateValuesEqual(int64(42), "42"))
}

func TestStateValuesEqual_Booleans(t *testing.T) {
	assert.True(t, stateValuesEqual(true, true))
	assert.True(t, stateValuesEqual(false, false))
	assert.False(t, stateValuesEqual(true, false))
	// SQLite stores bools as integers
	assert.True(t, stateValuesEqual(true, int64(1)))
	assert.True(t, stateValuesEqual(false, int64(0)))
}

func TestStateValuesEqual_Nil(t *testing.T) {
	assert.True(t, stateValuesEqual(nil, nil))
	assert.False(t, stateValuesEqual(nil, "value"))
	assert.False(t, stateValuesEqual("value", nil))
}

// Integration tests for assertFinalState with real database

func setupTestStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	return st
}

func createTestTable(t *testing.T, st *store.Store) {
	t.Helper()
	_, err := st.DB().Exec(`
		CREATE TABLE test_items (
			item_id TEXT PRIMARY KEY,
			quantity INTEGER,
			status TEXT,
			active INTEGER
		)
	`)
	require.NoError(t, err)
}

func TestAssertFinalState_RowFound_Pass(t *testing.T) {
	st := setupTestStore(t)
	createTestTable(t, st)

	// Insert test row
	_, err := st.DB().Exec(`INSERT INTO test_items (item_id, quantity, status) VALUES (?, ?, ?)`,
		"widget", 10, "available")
	require.NoError(t, err)

	assertion := Assertion{
		Type:  AssertFinalState,
		Table: "test_items",
		Where: map[string]interface{}{"item_id": "widget"},
		Expect: map[string]interface{}{
			"quantity": int64(10),
			"status":   "available",
		},
	}

	err = assertFinalState(context.Background(), st, assertion)
	assert.NoError(t, err)
}

func TestAssertFinalState_RowNotFound_Fail(t *testing.T) {
	st := setupTestStore(t)
	createTestTable(t, st)

	// No row inserted

	assertion := Assertion{
		Type:   AssertFinalState,
		Table:  "test_items",
		Where:  map[string]interface{}{"item_id": "widget"},
		Expect: map[string]interface{}{"quantity": int64(10)},
	}

	err := assertFinalState(context.Background(), st, assertion)
	require.Error(t, err)

	assertErr, ok := err.(*AssertionError)
	require.True(t, ok)
	assert.Equal(t, "final_state", assertErr.Type)
	assert.Contains(t, assertErr.Actual, "row not found")
}

func TestAssertFinalState_ValueMismatch_Fail(t *testing.T) {
	st := setupTestStore(t)
	createTestTable(t, st)

	// Insert row with quantity=5
	_, err := st.DB().Exec(`INSERT INTO test_items (item_id, quantity) VALUES (?, ?)`,
		"widget", 5)
	require.NoError(t, err)

	assertion := Assertion{
		Type:   AssertFinalState,
		Table:  "test_items",
		Where:  map[string]interface{}{"item_id": "widget"},
		Expect: map[string]interface{}{"quantity": int64(10)}, // Expected 10, actual 5
	}

	err = assertFinalState(context.Background(), st, assertion)
	require.Error(t, err)

	assertErr, ok := err.(*AssertionError)
	require.True(t, ok)
	assert.Contains(t, assertErr.Expected, "quantity")
	assert.Contains(t, assertErr.Expected, "10")
	assert.Contains(t, assertErr.Actual, "5")
}

func TestAssertFinalState_SubsetMatch_ExtraColumnsIgnored(t *testing.T) {
	st := setupTestStore(t)
	createTestTable(t, st)

	// Insert row with multiple columns
	_, err := st.DB().Exec(`INSERT INTO test_items (item_id, quantity, status, active) VALUES (?, ?, ?, ?)`,
		"widget", 10, "available", 1)
	require.NoError(t, err)

	// Only check quantity, ignore status and active
	assertion := Assertion{
		Type:   AssertFinalState,
		Table:  "test_items",
		Where:  map[string]interface{}{"item_id": "widget"},
		Expect: map[string]interface{}{"quantity": int64(10)},
	}

	err = assertFinalState(context.Background(), st, assertion)
	assert.NoError(t, err) // Should pass - extra columns ignored
}

func TestAssertFinalState_MissingColumn_Fail(t *testing.T) {
	st := setupTestStore(t)
	createTestTable(t, st)

	// Insert row
	_, err := st.DB().Exec(`INSERT INTO test_items (item_id, quantity) VALUES (?, ?)`,
		"widget", 10)
	require.NoError(t, err)

	// Try to assert on a column that doesn't exist
	assertion := Assertion{
		Type:   AssertFinalState,
		Table:  "test_items",
		Where:  map[string]interface{}{"item_id": "widget"},
		Expect: map[string]interface{}{"nonexistent_column": "value"},
	}

	err = assertFinalState(context.Background(), st, assertion)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent_column")
}

func TestAssertFinalState_TypeMismatch_Fail(t *testing.T) {
	st := setupTestStore(t)
	createTestTable(t, st)

	// Insert row with integer quantity
	_, err := st.DB().Exec(`INSERT INTO test_items (item_id, quantity) VALUES (?, ?)`,
		"widget", 10)
	require.NoError(t, err)

	// Expect quantity as string (type mismatch)
	assertion := Assertion{
		Type:   AssertFinalState,
		Table:  "test_items",
		Where:  map[string]interface{}{"item_id": "widget"},
		Expect: map[string]interface{}{"quantity": "10"}, // String, not int
	}

	err = assertFinalState(context.Background(), st, assertion)
	require.Error(t, err)
	// Error should show type information
	assertErr, ok := err.(*AssertionError)
	require.True(t, ok)
	assert.Contains(t, assertErr.Expected, "type")
	assert.Contains(t, assertErr.Actual, "type")
}

func TestAssertFinalState_TableNotFound_Fail(t *testing.T) {
	st := setupTestStore(t)
	// Don't create the table

	assertion := Assertion{
		Type:   AssertFinalState,
		Table:  "nonexistent_table",
		Where:  map[string]interface{}{"id": "1"},
		Expect: map[string]interface{}{"value": "test"},
	}

	err := assertFinalState(context.Background(), st, assertion)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent_table")
}

func TestAssertFinalState_EmptyExpect_Pass(t *testing.T) {
	st := setupTestStore(t)
	createTestTable(t, st)

	// Insert row
	_, err := st.DB().Exec(`INSERT INTO test_items (item_id, quantity) VALUES (?, ?)`,
		"widget", 10)
	require.NoError(t, err)

	// Empty Expect - just check row exists
	assertion := Assertion{
		Type:   AssertFinalState,
		Table:  "test_items",
		Where:  map[string]interface{}{"item_id": "widget"},
		Expect: map[string]interface{}{},
	}

	err = assertFinalState(context.Background(), st, assertion)
	assert.NoError(t, err) // Should pass - row exists, no specific values to check
}

func TestAssertFinalState_MultipleWhereConditions(t *testing.T) {
	st := setupTestStore(t)
	createTestTable(t, st)

	// Insert multiple rows
	_, err := st.DB().Exec(`INSERT INTO test_items (item_id, quantity, status) VALUES (?, ?, ?)`,
		"widget", 10, "available")
	require.NoError(t, err)
	_, err = st.DB().Exec(`INSERT INTO test_items (item_id, quantity, status) VALUES (?, ?, ?)`,
		"gadget", 5, "available")
	require.NoError(t, err)

	// Query with multiple conditions
	assertion := Assertion{
		Type:  AssertFinalState,
		Table: "test_items",
		Where: map[string]interface{}{
			"item_id": "widget",
			"status":  "available",
		},
		Expect: map[string]interface{}{"quantity": int64(10)},
	}

	err = assertFinalState(context.Background(), st, assertion)
	assert.NoError(t, err)
}

func TestEvaluateAssertions_FinalStateWithoutContext_Fail(t *testing.T) {
	result := &Result{Trace: []TraceEvent{}}

	assertions := []Assertion{
		{Type: AssertFinalState, Table: "test", Expect: map[string]interface{}{"col": "val"}},
	}

	// Pass nil context - should fail
	errors := EvaluateAssertions(result, assertions, nil)
	require.Len(t, errors, 1)
	assert.Contains(t, errors[0], "requires database context")
}

func TestEvaluateAssertions_FinalStateWithContext_Pass(t *testing.T) {
	st := setupTestStore(t)
	createTestTable(t, st)

	// Insert row
	_, err := st.DB().Exec(`INSERT INTO test_items (item_id, quantity) VALUES (?, ?)`,
		"widget", 10)
	require.NoError(t, err)

	result := &Result{Trace: []TraceEvent{}}

	assertions := []Assertion{
		{Type: AssertFinalState, Table: "test_items", Where: map[string]interface{}{"item_id": "widget"}, Expect: map[string]interface{}{"quantity": int64(10)}},
	}

	actx := &AssertionContext{
		Store: st,
		Ctx:   context.Background(),
	}

	errors := EvaluateAssertions(result, assertions, actx)
	assert.Empty(t, errors)
}
