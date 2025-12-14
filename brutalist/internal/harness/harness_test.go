package harness

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRun_MinimalScenario(t *testing.T) {
	scenario := &Scenario{
		Name:        "minimal",
		Description: "Minimal test scenario",
		Specs:       []string{}, // Empty specs for test
		FlowToken:   "test-flow-minimal",
		Setup:       []ActionStep{},
		Flow: []FlowStep{
			{
				Invoke: "Test.action",
				Args:   map[string]interface{}{},
			},
		},
		Assertions: []Assertion{
			{Type: "trace_contains", Action: "Test.action"},
		},
	}

	result, err := Run(scenario)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Should pass (stub always succeeds)
	assert.True(t, result.Pass)
	assert.Empty(t, result.Errors)

	// Should have 2 trace events (invocation + completion)
	assert.Len(t, result.Trace, 2)
	assert.Equal(t, "invocation", result.Trace[0].Type)
	assert.Equal(t, "completion", result.Trace[1].Type)
}

func TestRun_WithSetup(t *testing.T) {
	scenario := &Scenario{
		Name:        "with_setup",
		Description: "Test scenario with setup steps",
		Specs:       []string{},
		FlowToken:   "test-flow-setup",
		Setup: []ActionStep{
			{
				Action: "Inventory.setStock",
				Args: map[string]interface{}{
					"item_id":  "widget",
					"quantity": 10,
				},
			},
		},
		Flow: []FlowStep{
			{
				Invoke: "Cart.addItem",
				Args: map[string]interface{}{
					"item_id":  "widget",
					"quantity": 3,
				},
			},
		},
		Assertions: []Assertion{
			{Type: "trace_contains", Action: "Cart.addItem"},
		},
	}

	result, err := Run(scenario)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Should pass
	assert.True(t, result.Pass)

	// Should have 4 trace events (setup inv + setup comp + flow inv + flow comp)
	assert.Len(t, result.Trace, 4)

	// Setup events come first
	assert.Equal(t, "invocation", result.Trace[0].Type)
	assert.Equal(t, "Inventory.setStock", result.Trace[0].ActionURI)
	assert.Equal(t, "completion", result.Trace[1].Type)
	assert.Equal(t, "Success", result.Trace[1].OutputCase)

	// Flow events follow
	assert.Equal(t, "invocation", result.Trace[2].Type)
	assert.Equal(t, "Cart.addItem", result.Trace[2].ActionURI)
	assert.Equal(t, "completion", result.Trace[3].Type)
}

func TestRun_WithExpectClause(t *testing.T) {
	scenario := &Scenario{
		Name:        "with_expect",
		Description: "Test scenario with expect clauses",
		Specs:       []string{},
		FlowToken:   "test-flow-expect",
		Flow: []FlowStep{
			{
				Invoke: "Cart.addItem",
				Args: map[string]interface{}{
					"item_id":  "widget",
					"quantity": 3,
				},
				Expect: &ExpectClause{
					Case: "Success",
					Result: map[string]interface{}{
						"item_id":      "widget",
						"new_quantity": 3,
					},
				},
			},
		},
		Assertions: []Assertion{
			{Type: "trace_contains", Action: "Cart.addItem"},
		},
	}

	result, err := Run(scenario)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Should pass (stub generates expected completion)
	assert.True(t, result.Pass)
	assert.Empty(t, result.Errors)

	// Completion should have expected output case
	assert.Equal(t, "Success", result.Trace[1].OutputCase)
}

func TestRun_WithErrorExpect(t *testing.T) {
	scenario := &Scenario{
		Name:        "error_case",
		Description: "Test scenario expecting error case",
		Specs:       []string{},
		FlowToken:   "test-flow-error",
		Flow: []FlowStep{
			{
				Invoke: "Cart.checkout",
				Args:   map[string]interface{}{},
				Expect: &ExpectClause{
					Case: "InsufficientStock",
				},
			},
		},
		Assertions: []Assertion{
			{Type: "trace_contains", Action: "Cart.checkout"},
		},
	}

	result, err := Run(scenario)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Should pass (stub generates expected completion)
	assert.True(t, result.Pass)

	// Completion should have error case
	assert.Equal(t, "InsufficientStock", result.Trace[1].OutputCase)
}

func TestRun_Deterministic(t *testing.T) {
	scenario := &Scenario{
		Name:        "determinism",
		Description: "Test deterministic execution",
		Specs:       []string{},
		FlowToken:   "test-flow-determinism",
		Flow: []FlowStep{
			{
				Invoke: "Test.action1",
				Args:   map[string]interface{}{"key": "value1"},
			},
			{
				Invoke: "Test.action2",
				Args:   map[string]interface{}{"key": "value2"},
			},
		},
		Assertions: []Assertion{
			{Type: "trace_contains", Action: "Test.action1"},
		},
	}

	// Run scenario twice
	result1, err := Run(scenario)
	require.NoError(t, err)

	result2, err := Run(scenario)
	require.NoError(t, err)

	// Both should pass
	assert.True(t, result1.Pass)
	assert.True(t, result2.Pass)

	// Traces should have same length
	require.Equal(t, len(result1.Trace), len(result2.Trace))

	// Seq values should be identical
	for i := range result1.Trace {
		assert.Equal(t, result1.Trace[i].Seq, result2.Trace[i].Seq,
			"seq mismatch at trace index %d", i)
		assert.Equal(t, result1.Trace[i].Type, result2.Trace[i].Type,
			"type mismatch at trace index %d", i)
	}
}

func TestRun_FreshDatabasePerTest(t *testing.T) {
	// Run first scenario that modifies state
	scenario1 := &Scenario{
		Name:        "scenario1",
		Description: "First scenario",
		Specs:       []string{},
		FlowToken:   "test-flow-1",
		Setup: []ActionStep{
			{
				Action: "State.set",
				Args:   map[string]interface{}{"value": "from_scenario_1"},
			},
		},
		Flow: []FlowStep{
			{
				Invoke: "State.read",
				Args:   map[string]interface{}{},
			},
		},
		Assertions: []Assertion{
			{Type: "trace_contains", Action: "State.read"},
		},
	}

	result1, err := Run(scenario1)
	require.NoError(t, err)
	assert.True(t, result1.Pass)

	// Run second scenario - should have fresh database
	scenario2 := &Scenario{
		Name:        "scenario2",
		Description: "Second scenario",
		Specs:       []string{},
		FlowToken:   "test-flow-2",
		Flow: []FlowStep{
			{
				Invoke: "State.read",
				Args:   map[string]interface{}{},
			},
		},
		Assertions: []Assertion{
			{Type: "trace_contains", Action: "State.read"},
		},
	}

	result2, err := Run(scenario2)
	require.NoError(t, err)
	assert.True(t, result2.Pass)

	// Both scenarios should have passed independently
	// (no state pollution between tests)
}

func TestRun_TraceOrder(t *testing.T) {
	scenario := &Scenario{
		Name:        "trace_order",
		Description: "Test trace event ordering",
		Specs:       []string{},
		FlowToken:   "test-flow-order",
		Setup: []ActionStep{
			{Action: "Setup.step1", Args: map[string]interface{}{}},
			{Action: "Setup.step2", Args: map[string]interface{}{}},
		},
		Flow: []FlowStep{
			{Invoke: "Flow.step1", Args: map[string]interface{}{}},
			{Invoke: "Flow.step2", Args: map[string]interface{}{}},
		},
		Assertions: []Assertion{
			{Type: "trace_contains", Action: "Flow.step1"},
		},
	}

	result, err := Run(scenario)
	require.NoError(t, err)

	// Should have 8 events total (2 setup * 2 + 2 flow * 2)
	require.Len(t, result.Trace, 8)

	// Verify order: setup1 inv/comp, setup2 inv/comp, flow1 inv/comp, flow2 inv/comp
	expectedActions := []string{
		"Setup.step1", "", // inv, comp
		"Setup.step2", "",
		"Flow.step1", "",
		"Flow.step2", "",
	}

	for i, expected := range expectedActions {
		if expected != "" {
			assert.Equal(t, expected, result.Trace[i].ActionURI, "trace[%d]", i)
		}
	}

	// Verify seq values are strictly increasing
	for i := 1; i < len(result.Trace); i++ {
		assert.Greater(t, result.Trace[i].Seq, result.Trace[i-1].Seq,
			"seq should increase: trace[%d].Seq=%d > trace[%d].Seq=%d",
			i, result.Trace[i].Seq, i-1, result.Trace[i-1].Seq)
	}
}

func TestRun_VariousArgTypes(t *testing.T) {
	scenario := &Scenario{
		Name:        "arg_types",
		Description: "Test various argument types",
		Specs:       []string{},
		FlowToken:   "test-flow-args",
		Flow: []FlowStep{
			{
				Invoke: "Test.action",
				Args: map[string]interface{}{
					"string_val": "hello",
					"int_val":    42,
					"bool_val":   true,
					"array_val":  []interface{}{"a", "b", "c"},
					"object_val": map[string]interface{}{
						"nested": "value",
					},
				},
			},
		},
		Assertions: []Assertion{
			{Type: "trace_contains", Action: "Test.action"},
		},
	}

	result, err := Run(scenario)
	require.NoError(t, err)
	assert.True(t, result.Pass)

	// Verify args were captured in trace
	args := result.Trace[0].Args.(map[string]interface{})
	assert.Equal(t, "hello", args["string_val"])
	assert.Equal(t, 42, args["int_val"])
	assert.Equal(t, true, args["bool_val"])
}

func TestRun_FloatsForbidden(t *testing.T) {
	// Floats are forbidden in IR (CP-5)
	scenario := &Scenario{
		Name:        "float_forbidden",
		Description: "Test that floats are rejected",
		Specs:       []string{},
		FlowToken:   "test-flow-float",
		Flow: []FlowStep{
			{
				Invoke: "Test.action",
				Args: map[string]interface{}{
					"float_val": 3.14,
				},
			},
		},
		Assertions: []Assertion{
			{Type: "trace_contains", Action: "Test.action"},
		},
	}

	_, err := Run(scenario)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "floats are forbidden")
}

func TestRun_NullsForbidden(t *testing.T) {
	// Null values are forbidden in IR (canonical JSON does not support null)
	scenario := &Scenario{
		Name:        "null_forbidden",
		Description: "Test that null values are rejected early",
		Specs:       []string{},
		FlowToken:   "test-flow-null",
		Flow: []FlowStep{
			{
				Invoke: "Test.action",
				Args: map[string]interface{}{
					"null_val": nil, // YAML null/~
				},
			},
		},
		Assertions: []Assertion{
			{Type: "trace_contains", Action: "Test.action"},
		},
	}

	_, err := Run(scenario)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "null values are forbidden")
}

func TestConvertArgsToIRObject(t *testing.T) {
	tests := []struct {
		name    string
		args    map[string]interface{}
		wantErr bool
	}{
		{
			name: "string",
			args: map[string]interface{}{"key": "value"},
		},
		{
			name: "int",
			args: map[string]interface{}{"key": 42},
		},
		{
			name:    "float_forbidden",
			args:    map[string]interface{}{"key": 3.14},
			wantErr: true, // Floats are forbidden in IR (CP-5)
		},
		{
			name: "bool",
			args: map[string]interface{}{"key": true},
		},
		{
			name:    "nil_forbidden",
			args:    map[string]interface{}{"key": nil},
			wantErr: true, // Null values forbidden in IR (canonical JSON)
		},
		{
			name: "array",
			args: map[string]interface{}{"key": []interface{}{"a", "b"}},
		},
		{
			name: "nested_object",
			args: map[string]interface{}{
				"outer": map[string]interface{}{
					"inner": "value",
				},
			},
		},
		{
			name: "empty",
			args: map[string]interface{}{},
		},
		{
			name: "nil_map",
			args: nil,
		},
		{
			name: "integer_as_float64",
			args: map[string]interface{}{"key": float64(42)}, // Integer-like floats are allowed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := convertArgsToIRObject(tt.args)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestResult_AddError(t *testing.T) {
	result := NewResult()
	assert.True(t, result.Pass)
	assert.Empty(t, result.Errors)

	result.AddError("first error")
	assert.False(t, result.Pass)
	assert.Len(t, result.Errors, 1)
	assert.Equal(t, "first error", result.Errors[0])

	result.AddError("second error")
	assert.Len(t, result.Errors, 2)
}

func TestResult_AddTrace(t *testing.T) {
	result := NewResult()
	assert.Empty(t, result.Trace)

	result.AddInvocationTrace("Test.action", map[string]interface{}{"key": "val"}, 1)
	assert.Len(t, result.Trace, 1)
	assert.Equal(t, "invocation", result.Trace[0].Type)
	assert.Equal(t, "Test.action", result.Trace[0].ActionURI)
	assert.Equal(t, int64(1), result.Trace[0].Seq)

	result.AddCompletionTrace("Success", nil, 2)
	assert.Len(t, result.Trace, 2)
	assert.Equal(t, "completion", result.Trace[1].Type)
	assert.Equal(t, "Success", result.Trace[1].OutputCase)
	assert.Equal(t, int64(2), result.Trace[1].Seq)
}

// Integration tests for assertions through Run()

func TestRun_TraceContainsAssertion_Pass(t *testing.T) {
	scenario := &Scenario{
		Name:        "trace_contains_pass",
		Description: "Test trace_contains assertion passing",
		Specs:       []string{},
		FlowToken:   "test-flow-trace-contains",
		Flow: []FlowStep{
			{
				Invoke: "Cart.addItem",
				Args:   map[string]interface{}{"item_id": "widget", "quantity": 3},
			},
		},
		Assertions: []Assertion{
			{Type: AssertTraceContains, Action: "Cart.addItem", Args: map[string]interface{}{"item_id": "widget"}},
		},
	}

	result, err := Run(scenario)
	require.NoError(t, err)
	assert.True(t, result.Pass)
	assert.Empty(t, result.Errors)
}

func TestRun_TraceContainsAssertion_Fail(t *testing.T) {
	scenario := &Scenario{
		Name:        "trace_contains_fail",
		Description: "Test trace_contains assertion failing",
		Specs:       []string{},
		FlowToken:   "test-flow-trace-contains-fail",
		Flow: []FlowStep{
			{
				Invoke: "Cart.addItem",
				Args:   map[string]interface{}{"item_id": "widget"},
			},
		},
		Assertions: []Assertion{
			{Type: AssertTraceContains, Action: "Cart.checkout"}, // Not in trace
		},
	}

	result, err := Run(scenario)
	require.NoError(t, err)
	assert.False(t, result.Pass)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0], "Cart.checkout")
}

func TestRun_TraceOrderAssertion_Pass(t *testing.T) {
	scenario := &Scenario{
		Name:        "trace_order_pass",
		Description: "Test trace_order assertion passing",
		Specs:       []string{},
		FlowToken:   "test-flow-trace-order",
		Flow: []FlowStep{
			{Invoke: "Inventory.reserve", Args: map[string]interface{}{}},
			{Invoke: "Payment.charge", Args: map[string]interface{}{}},
			{Invoke: "Order.create", Args: map[string]interface{}{}},
		},
		Assertions: []Assertion{
			{Type: AssertTraceOrder, Actions: []string{"Inventory.reserve", "Payment.charge", "Order.create"}},
		},
	}

	result, err := Run(scenario)
	require.NoError(t, err)
	assert.True(t, result.Pass)
	assert.Empty(t, result.Errors)
}

func TestRun_TraceOrderAssertion_Fail(t *testing.T) {
	scenario := &Scenario{
		Name:        "trace_order_fail",
		Description: "Test trace_order assertion failing",
		Specs:       []string{},
		FlowToken:   "test-flow-trace-order-fail",
		Flow: []FlowStep{
			{Invoke: "Payment.charge", Args: map[string]interface{}{}}, // Payment before inventory
			{Invoke: "Inventory.reserve", Args: map[string]interface{}{}},
		},
		Assertions: []Assertion{
			{Type: AssertTraceOrder, Actions: []string{"Inventory.reserve", "Payment.charge"}}, // Expected opposite order
		},
	}

	result, err := Run(scenario)
	require.NoError(t, err)
	assert.False(t, result.Pass)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0], "should be before")
}

func TestRun_TraceCountAssertion_Pass(t *testing.T) {
	scenario := &Scenario{
		Name:        "trace_count_pass",
		Description: "Test trace_count assertion passing",
		Specs:       []string{},
		FlowToken:   "test-flow-trace-count",
		Flow: []FlowStep{
			{Invoke: "Notification.send", Args: map[string]interface{}{}},
			{Invoke: "Notification.send", Args: map[string]interface{}{}},
			{Invoke: "Notification.send", Args: map[string]interface{}{}},
		},
		Assertions: []Assertion{
			{Type: AssertTraceCount, Action: "Notification.send", Count: 3},
		},
	}

	result, err := Run(scenario)
	require.NoError(t, err)
	assert.True(t, result.Pass)
	assert.Empty(t, result.Errors)
}

func TestRun_TraceCountAssertion_Fail(t *testing.T) {
	scenario := &Scenario{
		Name:        "trace_count_fail",
		Description: "Test trace_count assertion failing",
		Specs:       []string{},
		FlowToken:   "test-flow-trace-count-fail",
		Flow: []FlowStep{
			{Invoke: "Notification.send", Args: map[string]interface{}{}},
		},
		Assertions: []Assertion{
			{Type: AssertTraceCount, Action: "Notification.send", Count: 3}, // Expected 3, got 1
		},
	}

	result, err := Run(scenario)
	require.NoError(t, err)
	assert.False(t, result.Pass)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0], "3 occurrences")
}

func TestRun_MultipleAssertions(t *testing.T) {
	scenario := &Scenario{
		Name:        "multiple_assertions",
		Description: "Test multiple assertions together",
		Specs:       []string{},
		FlowToken:   "test-flow-multi-assert",
		Setup: []ActionStep{
			{Action: "Inventory.setStock", Args: map[string]interface{}{"item_id": "widget", "quantity": 100}},
		},
		Flow: []FlowStep{
			{Invoke: "Cart.addItem", Args: map[string]interface{}{"item_id": "widget", "quantity": 5}},
			{Invoke: "Cart.checkout", Args: map[string]interface{}{}},
		},
		Assertions: []Assertion{
			{Type: AssertTraceContains, Action: "Inventory.setStock"},
			{Type: AssertTraceContains, Action: "Cart.addItem", Args: map[string]interface{}{"item_id": "widget"}},
			{Type: AssertTraceContains, Action: "Cart.checkout"},
			{Type: AssertTraceOrder, Actions: []string{"Inventory.setStock", "Cart.addItem", "Cart.checkout"}},
			{Type: AssertTraceCount, Action: "Cart.addItem", Count: 1},
			{Type: AssertTraceCount, Action: "Cart.checkout", Count: 1},
		},
	}

	result, err := Run(scenario)
	require.NoError(t, err)
	assert.True(t, result.Pass)
	assert.Empty(t, result.Errors)
}
