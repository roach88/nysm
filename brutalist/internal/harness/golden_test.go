package harness

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/roach88/nysm/internal/ir"
)

func TestRunWithGolden_SimpleInvocation(t *testing.T) {
	// Create temp directory for test files
	tmpDir := t.TempDir()

	// Create a minimal spec file
	specContent := `
concept Cart {
	purpose: "Manages shopping cart"
	action addItem: {
		args: {
			item_id: string
			quantity: int
		}
		outputs: [{ case: "Success" }]
	}
}
`
	err := os.WriteFile(filepath.Join(tmpDir, "cart.cue"), []byte(specContent), 0644)
	require.NoError(t, err)

	// Create test scenario with fixed flow token for determinism
	scenario := &Scenario{
		Name:        "simple_invocation",
		Description: "Test simple invocation",
		FlowToken:   "test-flow-token-001",
		Specs:       []string{filepath.Join(tmpDir, "cart.cue")},
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
			{
				Type:   AssertTraceContains,
				Action: "Cart.addItem",
			},
		},
	}

	// Run with golden comparison
	// First run with -update to create golden file:
	//   go test ./internal/harness -run TestRunWithGolden_SimpleInvocation -update
	err = RunWithGolden(t, scenario)
	require.NoError(t, err)
}

func TestRunWithGolden_MultipleSteps(t *testing.T) {
	// Create temp directory for test files
	tmpDir := t.TempDir()

	// Create a minimal spec file
	specContent := `
concept Cart {
	purpose: "Manages shopping cart"
	action addItem: {
		args: {
			item_id: string
			quantity: int
		}
		outputs: [{ case: "Success" }]
	}
	action checkout: {
		args: {}
		outputs: [{ case: "Success" }]
	}
}
`
	err := os.WriteFile(filepath.Join(tmpDir, "cart.cue"), []byte(specContent), 0644)
	require.NoError(t, err)

	// Create test scenario with multiple steps
	scenario := &Scenario{
		Name:        "multiple_steps",
		Description: "Test multiple flow steps",
		FlowToken:   "test-flow-token-002",
		Specs:       []string{filepath.Join(tmpDir, "cart.cue")},
		Flow: []FlowStep{
			{
				Invoke: "Cart.addItem",
				Args: map[string]interface{}{
					"item_id":  "widget",
					"quantity": 3,
				},
			},
			{
				Invoke: "Cart.addItem",
				Args: map[string]interface{}{
					"item_id":  "gadget",
					"quantity": 1,
				},
			},
			{
				Invoke: "Cart.checkout",
				Args:   map[string]interface{}{},
			},
		},
		Assertions: []Assertion{
			{
				Type:   AssertTraceCount,
				Action: "Cart.addItem",
				Count:  2,
			},
		},
	}

	// Run with golden comparison
	err = RunWithGolden(t, scenario)
	require.NoError(t, err)
}

func TestAssertGolden_FromResult(t *testing.T) {
	// Create temp directory for test files
	tmpDir := t.TempDir()

	// Create a minimal spec file
	specContent := `
concept Cart {
	purpose: "Manages shopping cart"
	action addItem: {
		args: {
			item_id: string
			quantity: int
		}
		outputs: [{ case: "Success" }]
	}
}
`
	err := os.WriteFile(filepath.Join(tmpDir, "cart.cue"), []byte(specContent), 0644)
	require.NoError(t, err)

	// Create and run scenario manually
	scenario := &Scenario{
		Name:        "assert_golden_test",
		Description: "Test AssertGolden function",
		FlowToken:   "test-flow-token-003",
		Specs:       []string{filepath.Join(tmpDir, "cart.cue")},
		Flow: []FlowStep{
			{
				Invoke: "Cart.addItem",
				Args: map[string]interface{}{
					"item_id":  "item1",
					"quantity": 5,
				},
			},
		},
		Assertions: []Assertion{
			{
				Type:   AssertTraceContains,
				Action: "Cart.addItem",
			},
		},
	}

	result, err := Run(scenario)
	require.NoError(t, err)

	// Use AssertGolden with the result
	err = AssertGolden(t, "assert_golden_test", result)
	require.NoError(t, err)
}

func TestCanonicalJSONDeterminism(t *testing.T) {
	// Verify that multiple runs produce identical JSON
	// This test doesn't use golden files - it directly compares marshaled output

	// Create trace snapshot
	snapshot := TraceSnapshot{
		ScenarioName: "determinism_test",
		FlowToken:    "fixed-token",
		Trace: []TraceEvent{
			{
				Type:      "invocation",
				ActionURI: "Cart.addItem",
				Args: map[string]interface{}{
					"item_id":  "widget",
					"quantity": 3,
				},
				Seq: 1,
			},
			{
				Type:       "completion",
				OutputCase: "Success",
				Seq:        2,
			},
		},
	}

	// Convert to canonical map and marshal twice
	canonicalMap := snapshot.toCanonicalMap()
	json1, err := ir.MarshalCanonical(canonicalMap)
	require.NoError(t, err)

	json2, err := ir.MarshalCanonical(canonicalMap)
	require.NoError(t, err)

	// Both should be identical
	require.Equal(t, json1, json2, "canonical JSON must be deterministic")
}

func TestTraceSnapshotJSON(t *testing.T) {
	// Test that TraceSnapshot marshals to expected format
	snapshot := TraceSnapshot{
		ScenarioName: "test_scenario",
		FlowToken:    "flow-123",
		Trace: []TraceEvent{
			{
				Type:      "invocation",
				ActionURI: "Cart.addItem",
				Args: map[string]interface{}{
					"item_id": "widget",
				},
				Seq: 1,
			},
		},
	}

	// Convert to canonical map and marshal
	canonicalMap := snapshot.toCanonicalMap()
	jsonBytes, err := ir.MarshalCanonical(canonicalMap)
	require.NoError(t, err)

	// Verify it's valid JSON and contains expected fields
	jsonStr := string(jsonBytes)
	require.Contains(t, jsonStr, `"scenario_name":"test_scenario"`)
	require.Contains(t, jsonStr, `"flow_token":"flow-123"`)
	require.Contains(t, jsonStr, `"trace":[`)
	require.Contains(t, jsonStr, `"action_uri":"Cart.addItem"`)
}

func TestRunWithGolden_WithExpect(t *testing.T) {
	// Test golden comparison with expected outputs
	tmpDir := t.TempDir()

	specContent := `
concept Cart {
	purpose: "Manages shopping cart"
	action addItem: {
		args: {
			item_id: string
			quantity: int
		}
		outputs: [
			{ case: "Success", fields: { new_quantity: int } }
		]
	}
}
`
	err := os.WriteFile(filepath.Join(tmpDir, "cart.cue"), []byte(specContent), 0644)
	require.NoError(t, err)

	scenario := &Scenario{
		Name:        "with_expect",
		Description: "Test with expected output",
		FlowToken:   "test-flow-token-004",
		Specs:       []string{filepath.Join(tmpDir, "cart.cue")},
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
						"new_quantity": 3,
					},
				},
			},
		},
		Assertions: []Assertion{
			{
				Type:   AssertTraceContains,
				Action: "Cart.addItem",
			},
		},
	}

	err = RunWithGolden(t, scenario)
	require.NoError(t, err)
}
