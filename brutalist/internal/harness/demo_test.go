package harness

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// projectRoot returns the project root directory.
// This is needed because tests run from the package directory,
// but scenario spec paths are relative to project root.
func projectRoot() string {
	// From internal/harness/, go up two levels to project root
	root, _ := filepath.Abs("../..")
	return root
}

// TestDemoScenarios validates the canonical demo scenarios from PRD Appendix A.
// These scenarios serve as:
// 1. End-to-end validation of the NYSM MVP
// 2. Reference implementation examples
// 3. Regression test fixtures
//
// NOTE: Currently the harness bypasses actual engine execution (see package docs).
// These tests validate scenario format, assertion logic, and trace generation.
// Full engine integration is deferred to Epic 7 enhancements.
func TestDemoScenarios(t *testing.T) {
	tests := []struct {
		name         string
		scenarioPath string
	}{
		{
			name:         "cart_checkout_success",
			scenarioPath: "../../testdata/scenarios/cart_checkout_success.yaml",
		},
		{
			name:         "cart_checkout_insufficient_stock",
			scenarioPath: "../../testdata/scenarios/cart_checkout_insufficient_stock.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Load scenario from YAML file with base path for spec resolution
			absPath, err := filepath.Abs(tt.scenarioPath)
			require.NoError(t, err, "failed to get absolute path")

			// Use LoadScenarioWithBasePath to resolve spec paths relative to project root
			scenario, err := LoadScenarioWithBasePath(absPath, projectRoot())
			require.NoError(t, err, "failed to load scenario from %s", tt.scenarioPath)

			// Verify scenario metadata
			assert.Equal(t, tt.name, scenario.Name, "scenario name mismatch")
			assert.NotEmpty(t, scenario.Description, "scenario should have description")
			assert.NotEmpty(t, scenario.FlowToken, "scenario should have flow_token")

			// Run scenario
			result, err := Run(scenario)
			require.NoError(t, err, "scenario execution failed")
			require.NotNil(t, result, "result should not be nil")

			// Verify scenario passed (harness generates expected completions)
			assert.True(t, result.Pass, "scenario should pass: errors=%v", result.Errors)
			assert.Empty(t, result.Errors, "scenario should have no errors")

			// Verify trace was generated
			assert.NotEmpty(t, result.Trace, "trace should not be empty")

			t.Logf("Scenario %s: %d trace events", tt.name, len(result.Trace))
		})
	}
}

// TestDemoScenariosReplay validates deterministic replay.
// Running the same scenario twice should produce identical traces.
func TestDemoScenariosReplay(t *testing.T) {
	scenarioPath := "../../testdata/scenarios/cart_checkout_success.yaml"
	absPath, err := filepath.Abs(scenarioPath)
	require.NoError(t, err)

	scenario, err := LoadScenarioWithBasePath(absPath, projectRoot())
	require.NoError(t, err)

	// First run
	result1, err := Run(scenario)
	require.NoError(t, err)
	require.True(t, result1.Pass)

	// Second run with identical setup
	result2, err := Run(scenario)
	require.NoError(t, err)
	require.True(t, result2.Pass)

	// Both runs should produce identical trace lengths
	require.Equal(t, len(result1.Trace), len(result2.Trace),
		"replay should produce same number of trace events")

	// Trace events should be identical
	for i := range result1.Trace {
		assert.Equal(t, result1.Trace[i].Type, result2.Trace[i].Type,
			"trace[%d].Type mismatch", i)
		assert.Equal(t, result1.Trace[i].ActionURI, result2.Trace[i].ActionURI,
			"trace[%d].ActionURI mismatch", i)
		assert.Equal(t, result1.Trace[i].OutputCase, result2.Trace[i].OutputCase,
			"trace[%d].OutputCase mismatch", i)
		assert.Equal(t, result1.Trace[i].Seq, result2.Trace[i].Seq,
			"trace[%d].Seq mismatch", i)
	}

	t.Log("Deterministic replay verified: identical traces produced")
}

// TestDemoScenarioTraceOrder validates that trace events are in expected order.
func TestDemoScenarioTraceOrder(t *testing.T) {
	scenarioPath := "../../testdata/scenarios/cart_checkout_success.yaml"
	absPath, err := filepath.Abs(scenarioPath)
	require.NoError(t, err)

	scenario, err := LoadScenarioWithBasePath(absPath, projectRoot())
	require.NoError(t, err)

	result, err := Run(scenario)
	require.NoError(t, err)

	// Verify seq values are strictly increasing
	for i := 1; i < len(result.Trace); i++ {
		assert.Greater(t, result.Trace[i].Seq, result.Trace[i-1].Seq,
			"seq values should be strictly increasing: trace[%d].Seq=%d <= trace[%d].Seq=%d",
			i, result.Trace[i].Seq, i-1, result.Trace[i-1].Seq)
	}

	// Verify alternating invocation/completion pattern
	for i := 0; i < len(result.Trace); i += 2 {
		if i < len(result.Trace) {
			assert.Equal(t, "invocation", result.Trace[i].Type,
				"even trace indices should be invocations")
		}
		if i+1 < len(result.Trace) {
			assert.Equal(t, "completion", result.Trace[i+1].Type,
				"odd trace indices should be completions")
		}
	}
}

// TestDemoScenarioActions validates that expected actions appear in trace.
func TestDemoScenarioActions(t *testing.T) {
	tests := []struct {
		name            string
		scenarioPath    string
		expectedActions []string
	}{
		{
			name:         "success_scenario_actions",
			scenarioPath: "../../testdata/scenarios/cart_checkout_success.yaml",
			expectedActions: []string{
				"Inventory.setStock", // Setup
				"Cart.addItem",       // Flow
				"Cart.checkout",      // Flow
			},
		},
		{
			name:         "error_scenario_actions",
			scenarioPath: "../../testdata/scenarios/cart_checkout_insufficient_stock.yaml",
			expectedActions: []string{
				"Inventory.setStock", // Setup
				"Cart.addItem",       // Flow
				"Cart.checkout",      // Flow
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			absPath, err := filepath.Abs(tt.scenarioPath)
			require.NoError(t, err)

			scenario, err := LoadScenarioWithBasePath(absPath, projectRoot())
			require.NoError(t, err)

			result, err := Run(scenario)
			require.NoError(t, err)

			// Collect all invocation actions from trace
			var traceActions []string
			for _, event := range result.Trace {
				if event.Type == "invocation" && event.ActionURI != "" {
					traceActions = append(traceActions, event.ActionURI)
				}
			}

			// Verify expected actions appear in order
			expectedIdx := 0
			for _, action := range traceActions {
				if expectedIdx < len(tt.expectedActions) && action == tt.expectedActions[expectedIdx] {
					expectedIdx++
				}
			}

			assert.Equal(t, len(tt.expectedActions), expectedIdx,
				"not all expected actions found in trace: expected %v, got %v",
				tt.expectedActions, traceActions)
		})
	}
}

// TestDemoScenarioOutputCases validates expected output cases.
func TestDemoScenarioOutputCases(t *testing.T) {
	tests := []struct {
		name           string
		scenarioPath   string
		checkoutCase   string
	}{
		{
			name:         "success_has_success_case",
			scenarioPath: "../../testdata/scenarios/cart_checkout_success.yaml",
			checkoutCase: "Success",
		},
		{
			name:         "error_has_checkout_failed_case",
			scenarioPath: "../../testdata/scenarios/cart_checkout_insufficient_stock.yaml",
			checkoutCase: "CheckoutFailed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			absPath, err := filepath.Abs(tt.scenarioPath)
			require.NoError(t, err)

			scenario, err := LoadScenarioWithBasePath(absPath, projectRoot())
			require.NoError(t, err)

			result, err := Run(scenario)
			require.NoError(t, err)

			// Find Cart.checkout completion
			var checkoutCompletion *TraceEvent
			for i, event := range result.Trace {
				if event.Type == "completion" && i > 0 {
					prevEvent := result.Trace[i-1]
					if prevEvent.Type == "invocation" && prevEvent.ActionURI == "Cart.checkout" {
						checkoutCompletion = &result.Trace[i]
						break
					}
				}
			}

			require.NotNil(t, checkoutCompletion,
				"Cart.checkout completion should be in trace")
			assert.Equal(t, tt.checkoutCase, checkoutCompletion.OutputCase,
				"Cart.checkout output case mismatch")
		})
	}
}
