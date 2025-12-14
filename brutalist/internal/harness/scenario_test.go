package harness

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestSpec creates a minimal CUE spec file for testing.
func createTestSpec(t *testing.T, dir, name string) string {
	t.Helper()
	specsDir := filepath.Join(dir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatal(err)
	}
	specPath := filepath.Join(specsDir, name)
	if err := os.WriteFile(specPath, []byte("// placeholder concept"), 0644); err != nil {
		t.Fatal(err)
	}
	return specPath
}

func TestLoadScenario_ValidFile(t *testing.T) {
	dir := t.TempDir()
	specPath := createTestSpec(t, dir, "cart.concept.cue")
	scenarioPath := filepath.Join(dir, "test.yaml")

	content := `
name: test_scenario
description: "Test scenario for validation"
specs:
  - ` + specPath + `
flow:
  - invoke: Cart.addItem
    args:
      item_id: "widget"
      quantity: 1
assertions:
  - type: trace_contains
    action: Cart.addItem
`
	require.NoError(t, os.WriteFile(scenarioPath, []byte(content), 0644))

	scenario, err := LoadScenario(scenarioPath)
	require.NoError(t, err)

	assert.Equal(t, "test_scenario", scenario.Name)
	assert.Equal(t, "Test scenario for validation", scenario.Description)
	assert.Len(t, scenario.Specs, 1)
	assert.Len(t, scenario.Flow, 1)
	assert.Len(t, scenario.Assertions, 1)
	assert.Equal(t, "Cart.addItem", scenario.Flow[0].Invoke)
	assert.Equal(t, "widget", scenario.Flow[0].Args["item_id"])
}

func TestLoadScenario_MissingFile(t *testing.T) {
	_, err := LoadScenario("/nonexistent/scenario.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read scenario file")
}

func TestLoadScenario_MissingName(t *testing.T) {
	dir := t.TempDir()
	specPath := createTestSpec(t, dir, "cart.concept.cue")
	scenarioPath := filepath.Join(dir, "test.yaml")

	content := `
description: "Missing name"
specs:
  - ` + specPath + `
flow:
  - invoke: Cart.addItem
    args: {}
assertions:
  - type: trace_contains
    action: Cart.addItem
`
	require.NoError(t, os.WriteFile(scenarioPath, []byte(content), 0644))

	_, err := LoadScenario(scenarioPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}

func TestLoadScenario_MissingDescription(t *testing.T) {
	dir := t.TempDir()
	specPath := createTestSpec(t, dir, "cart.concept.cue")
	scenarioPath := filepath.Join(dir, "test.yaml")

	content := `
name: test
specs:
  - ` + specPath + `
flow:
  - invoke: Cart.addItem
    args: {}
assertions:
  - type: trace_contains
    action: Cart.addItem
`
	require.NoError(t, os.WriteFile(scenarioPath, []byte(content), 0644))

	_, err := LoadScenario(scenarioPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "description is required")
}

func TestLoadScenario_MissingSpecs(t *testing.T) {
	dir := t.TempDir()
	scenarioPath := filepath.Join(dir, "test.yaml")

	content := `
name: test
description: "Test"
specs: []
flow:
  - invoke: Cart.addItem
    args: {}
assertions:
  - type: trace_contains
    action: Cart.addItem
`
	require.NoError(t, os.WriteFile(scenarioPath, []byte(content), 0644))

	_, err := LoadScenario(scenarioPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "specs list is required")
}

func TestLoadScenario_MissingFlow(t *testing.T) {
	dir := t.TempDir()
	specPath := createTestSpec(t, dir, "cart.concept.cue")
	scenarioPath := filepath.Join(dir, "test.yaml")

	content := `
name: test
description: "Test"
specs:
  - ` + specPath + `
flow: []
assertions:
  - type: trace_contains
    action: Cart.addItem
`
	require.NoError(t, os.WriteFile(scenarioPath, []byte(content), 0644))

	_, err := LoadScenario(scenarioPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "flow list is required")
}

func TestLoadScenario_MissingAssertions(t *testing.T) {
	dir := t.TempDir()
	specPath := createTestSpec(t, dir, "cart.concept.cue")
	scenarioPath := filepath.Join(dir, "test.yaml")

	content := `
name: test
description: "Test"
specs:
  - ` + specPath + `
flow:
  - invoke: Cart.addItem
    args: {}
assertions: []
`
	require.NoError(t, os.WriteFile(scenarioPath, []byte(content), 0644))

	_, err := LoadScenario(scenarioPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "assertions list is required")
}

func TestLoadScenario_InvalidSpecPath(t *testing.T) {
	dir := t.TempDir()
	scenarioPath := filepath.Join(dir, "test.yaml")

	content := `
name: test
description: "Test"
specs:
  - /nonexistent/spec.cue
flow:
  - invoke: Cart.addItem
    args: {}
assertions:
  - type: trace_contains
    action: Cart.addItem
`
	require.NoError(t, os.WriteFile(scenarioPath, []byte(content), 0644))

	_, err := LoadScenario(scenarioPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "spec file not found")
}

func TestLoadScenario_MalformedYAML(t *testing.T) {
	dir := t.TempDir()
	scenarioPath := filepath.Join(dir, "test.yaml")

	content := `
name: test
description: "Test"
specs:
  - invalid yaml structure
  unclosed: [bracket
`
	require.NoError(t, os.WriteFile(scenarioPath, []byte(content), 0644))

	_, err := LoadScenario(scenarioPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse YAML")
}

func TestLoadScenario_FlowMissingInvoke(t *testing.T) {
	dir := t.TempDir()
	specPath := createTestSpec(t, dir, "cart.concept.cue")
	scenarioPath := filepath.Join(dir, "test.yaml")

	content := `
name: test
description: "Test"
specs:
  - ` + specPath + `
flow:
  - args: {}
assertions:
  - type: trace_contains
    action: Cart.addItem
`
	require.NoError(t, os.WriteFile(scenarioPath, []byte(content), 0644))

	_, err := LoadScenario(scenarioPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "flow[0]: invoke is required")
}

func TestLoadScenario_FlowMissingArgs(t *testing.T) {
	dir := t.TempDir()
	specPath := createTestSpec(t, dir, "cart.concept.cue")
	scenarioPath := filepath.Join(dir, "test.yaml")

	content := `
name: test
description: "Test"
specs:
  - ` + specPath + `
flow:
  - invoke: Cart.addItem
assertions:
  - type: trace_contains
    action: Cart.addItem
`
	require.NoError(t, os.WriteFile(scenarioPath, []byte(content), 0644))

	_, err := LoadScenario(scenarioPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "flow[0]: args is required")
}

func TestLoadScenario_WithSetup(t *testing.T) {
	dir := t.TempDir()
	specPath := createTestSpec(t, dir, "inventory.concept.cue")
	scenarioPath := filepath.Join(dir, "test.yaml")

	content := `
name: test
description: "Test with setup"
specs:
  - ` + specPath + `
setup:
  - action: Inventory.setStock
    args:
      item_id: "widget"
      quantity: 10
flow:
  - invoke: Inventory.reserve
    args:
      item_id: "widget"
      quantity: 3
assertions:
  - type: trace_contains
    action: Inventory.reserve
`
	require.NoError(t, os.WriteFile(scenarioPath, []byte(content), 0644))

	scenario, err := LoadScenario(scenarioPath)
	require.NoError(t, err)

	assert.Len(t, scenario.Setup, 1)
	assert.Equal(t, "Inventory.setStock", scenario.Setup[0].Action)
	assert.Equal(t, "widget", scenario.Setup[0].Args["item_id"])
	assert.Equal(t, 10, scenario.Setup[0].Args["quantity"])
}

func TestLoadScenario_SetupMissingAction(t *testing.T) {
	dir := t.TempDir()
	specPath := createTestSpec(t, dir, "cart.concept.cue")
	scenarioPath := filepath.Join(dir, "test.yaml")

	content := `
name: test
description: "Test"
specs:
  - ` + specPath + `
setup:
  - args:
      item_id: "widget"
flow:
  - invoke: Cart.addItem
    args: {}
assertions:
  - type: trace_contains
    action: Cart.addItem
`
	require.NoError(t, os.WriteFile(scenarioPath, []byte(content), 0644))

	_, err := LoadScenario(scenarioPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "setup[0]: action is required")
}

func TestLoadScenario_SetupMissingArgs(t *testing.T) {
	dir := t.TempDir()
	specPath := createTestSpec(t, dir, "cart.concept.cue")
	scenarioPath := filepath.Join(dir, "test.yaml")

	content := `
name: test
description: "Test"
specs:
  - ` + specPath + `
setup:
  - action: Inventory.setStock
flow:
  - invoke: Cart.addItem
    args: {}
assertions:
  - type: trace_contains
    action: Cart.addItem
`
	require.NoError(t, os.WriteFile(scenarioPath, []byte(content), 0644))

	_, err := LoadScenario(scenarioPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "setup[0]: args is required")
}

func TestLoadScenario_WithExpectations(t *testing.T) {
	dir := t.TempDir()
	specPath := createTestSpec(t, dir, "cart.concept.cue")
	scenarioPath := filepath.Join(dir, "test.yaml")

	content := `
name: test
description: "Test with expectations"
specs:
  - ` + specPath + `
flow:
  - invoke: Cart.addItem
    args:
      item_id: "widget"
      quantity: 1
    expect:
      case: Success
      result:
        new_quantity: 1
assertions:
  - type: trace_contains
    action: Cart.addItem
`
	require.NoError(t, os.WriteFile(scenarioPath, []byte(content), 0644))

	scenario, err := LoadScenario(scenarioPath)
	require.NoError(t, err)

	require.NotNil(t, scenario.Flow[0].Expect)
	assert.Equal(t, "Success", scenario.Flow[0].Expect.Case)
	assert.Equal(t, 1, scenario.Flow[0].Expect.Result["new_quantity"])
}

func TestLoadScenario_ExpectMissingCase(t *testing.T) {
	dir := t.TempDir()
	specPath := createTestSpec(t, dir, "cart.concept.cue")
	scenarioPath := filepath.Join(dir, "test.yaml")

	content := `
name: test
description: "Test"
specs:
  - ` + specPath + `
flow:
  - invoke: Cart.addItem
    args: {}
    expect:
      result:
        new_quantity: 1
assertions:
  - type: trace_contains
    action: Cart.addItem
`
	require.NoError(t, os.WriteFile(scenarioPath, []byte(content), 0644))

	_, err := LoadScenario(scenarioPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "flow[0].expect: case is required")
}

func TestLoadScenario_FixedFlowToken(t *testing.T) {
	dir := t.TempDir()
	specPath := createTestSpec(t, dir, "cart.concept.cue")
	scenarioPath := filepath.Join(dir, "test.yaml")

	content := `
name: test
description: "Test with fixed flow token"
flow_token: "01234567-89ab-cdef-0123-456789abcdef"
specs:
  - ` + specPath + `
flow:
  - invoke: Cart.addItem
    args: {}
assertions:
  - type: trace_contains
    action: Cart.addItem
`
	require.NoError(t, os.WriteFile(scenarioPath, []byte(content), 0644))

	scenario, err := LoadScenario(scenarioPath)
	require.NoError(t, err)
	assert.Equal(t, "01234567-89ab-cdef-0123-456789abcdef", scenario.FlowToken)
}

func TestLoadScenario_AssertionTypes(t *testing.T) {
	tests := []struct {
		name          string
		assertionYAML string
		wantErr       string
	}{
		{
			name: "trace_contains_valid",
			assertionYAML: `
  - type: trace_contains
    action: Cart.addItem
    args:
      item_id: "widget"
`,
			wantErr: "",
		},
		{
			name: "trace_contains_missing_action",
			assertionYAML: `
  - type: trace_contains
    args:
      item_id: "widget"
`,
			wantErr: "action is required for trace_contains",
		},
		{
			name: "trace_order_valid",
			assertionYAML: `
  - type: trace_order
    actions:
      - Cart.addItem
      - Cart.checkout
`,
			wantErr: "",
		},
		{
			name: "trace_order_missing_actions",
			assertionYAML: `
  - type: trace_order
`,
			wantErr: "actions list is required for trace_order",
		},
		{
			name: "trace_count_valid",
			assertionYAML: `
  - type: trace_count
    action: Cart.addItem
    count: 2
`,
			wantErr: "",
		},
		{
			name: "trace_count_missing_count",
			assertionYAML: `
  - type: trace_count
    action: Cart.addItem
`,
			// Missing count defaults to 0, which is now valid (assert action does not appear)
			wantErr: "",
		},
		{
			name: "trace_count_zero_count",
			assertionYAML: `
  - type: trace_count
    action: Cart.addItem
    count: 0
`,
			// Count of 0 is now valid (useful for "does not happen" assertions)
			wantErr: "",
		},
		{
			name: "trace_count_negative_count",
			assertionYAML: `
  - type: trace_count
    action: Cart.addItem
    count: -1
`,
			wantErr: "count must be non-negative for trace_count",
		},
		{
			name: "trace_count_missing_action",
			assertionYAML: `
  - type: trace_count
    count: 2
`,
			wantErr: "action is required for trace_count",
		},
		{
			name: "final_state_valid",
			assertionYAML: `
  - type: final_state
    table: cart_items
    where:
      item_id: "widget"
    expect:
      quantity: 1
`,
			wantErr: "",
		},
		{
			name: "final_state_missing_table",
			assertionYAML: `
  - type: final_state
    expect:
      quantity: 1
`,
			wantErr: "table is required for final_state",
		},
		{
			name: "final_state_missing_expect",
			assertionYAML: `
  - type: final_state
    table: cart_items
    where:
      item_id: "widget"
`,
			wantErr: "expect is required for final_state",
		},
		{
			name: "final_state_empty_expect",
			assertionYAML: `
  - type: final_state
    table: cart_items
    expect: {}
`,
			wantErr: "expect is required for final_state",
		},
		{
			name: "unknown_type",
			assertionYAML: `
  - type: unknown_assertion
    action: Cart.addItem
`,
			wantErr: "unknown assertion type",
		},
		{
			name: "missing_type",
			assertionYAML: `
  - action: Cart.addItem
`,
			wantErr: "type is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			specPath := createTestSpec(t, dir, "cart.concept.cue")
			scenarioPath := filepath.Join(dir, "test.yaml")

			content := `
name: test
description: "Test"
specs:
  - ` + specPath + `
flow:
  - invoke: Cart.addItem
    args: {}
assertions:
` + tt.assertionYAML

			require.NoError(t, os.WriteFile(scenarioPath, []byte(content), 0644))

			_, err := LoadScenario(scenarioPath)

			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestLoadScenario_MultipleSpecs(t *testing.T) {
	dir := t.TempDir()
	specPath1 := createTestSpec(t, dir, "cart.concept.cue")
	specPath2 := createTestSpec(t, dir, "inventory.concept.cue")
	specPath3 := createTestSpec(t, dir, "cart-inventory.sync.cue")
	scenarioPath := filepath.Join(dir, "test.yaml")

	content := `
name: test
description: "Test with multiple specs"
specs:
  - ` + specPath1 + `
  - ` + specPath2 + `
  - ` + specPath3 + `
flow:
  - invoke: Cart.addItem
    args: {}
assertions:
  - type: trace_contains
    action: Cart.addItem
`
	require.NoError(t, os.WriteFile(scenarioPath, []byte(content), 0644))

	scenario, err := LoadScenario(scenarioPath)
	require.NoError(t, err)
	assert.Len(t, scenario.Specs, 3)
}

func TestLoadScenario_ComplexScenario(t *testing.T) {
	dir := t.TempDir()
	specPath1 := createTestSpec(t, dir, "cart.concept.cue")
	specPath2 := createTestSpec(t, dir, "inventory.concept.cue")
	scenarioPath := filepath.Join(dir, "test.yaml")

	content := `
name: cart_checkout_success
description: "Successful checkout triggers inventory reservation"
flow_token: "test-flow-token-123"
specs:
  - ` + specPath1 + `
  - ` + specPath2 + `
setup:
  - action: Inventory.setStock
    args:
      item_id: "widget"
      quantity: 10
flow:
  - invoke: Cart.addItem
    args:
      item_id: "widget"
      quantity: 3
    expect:
      case: Success
      result:
        item_id: "widget"
        new_quantity: 3
  - invoke: Cart.checkout
    args: {}
    expect:
      case: Success
assertions:
  - type: trace_contains
    action: Inventory.reserve
    args:
      item_id: "widget"
      quantity: 3
  - type: final_state
    table: inventory
    where:
      item_id: "widget"
    expect:
      quantity: 7
  - type: trace_order
    actions:
      - Cart.addItem
      - Cart.checkout
      - Inventory.reserve
  - type: trace_count
    action: Inventory.reserve
    count: 1
`
	require.NoError(t, os.WriteFile(scenarioPath, []byte(content), 0644))

	scenario, err := LoadScenario(scenarioPath)
	require.NoError(t, err)

	assert.Equal(t, "cart_checkout_success", scenario.Name)
	assert.Equal(t, "Successful checkout triggers inventory reservation", scenario.Description)
	assert.Equal(t, "test-flow-token-123", scenario.FlowToken)
	assert.Len(t, scenario.Specs, 2)
	assert.Len(t, scenario.Setup, 1)
	assert.Len(t, scenario.Flow, 2)
	assert.Len(t, scenario.Assertions, 4)

	// Validate setup
	assert.Equal(t, "Inventory.setStock", scenario.Setup[0].Action)
	assert.Equal(t, "widget", scenario.Setup[0].Args["item_id"])
	assert.Equal(t, 10, scenario.Setup[0].Args["quantity"])

	// Validate flow
	assert.Equal(t, "Cart.addItem", scenario.Flow[0].Invoke)
	assert.NotNil(t, scenario.Flow[0].Expect)
	assert.Equal(t, "Success", scenario.Flow[0].Expect.Case)
	assert.Equal(t, "Cart.checkout", scenario.Flow[1].Invoke)

	// Validate assertions
	assert.Equal(t, "trace_contains", scenario.Assertions[0].Type)
	assert.Equal(t, "Inventory.reserve", scenario.Assertions[0].Action)
	assert.Equal(t, "final_state", scenario.Assertions[1].Type)
	assert.Equal(t, "inventory", scenario.Assertions[1].Table)
	assert.Equal(t, "trace_order", scenario.Assertions[2].Type)
	assert.Len(t, scenario.Assertions[2].Actions, 3)
	assert.Equal(t, "trace_count", scenario.Assertions[3].Type)
	assert.Equal(t, 1, scenario.Assertions[3].Count)
}

func TestLoadScenario_EmptyArgs(t *testing.T) {
	dir := t.TempDir()
	specPath := createTestSpec(t, dir, "cart.concept.cue")
	scenarioPath := filepath.Join(dir, "test.yaml")

	content := `
name: test
description: "Test with empty args"
specs:
  - ` + specPath + `
flow:
  - invoke: Cart.checkout
    args: {}
assertions:
  - type: trace_contains
    action: Cart.checkout
`
	require.NoError(t, os.WriteFile(scenarioPath, []byte(content), 0644))

	scenario, err := LoadScenario(scenarioPath)
	require.NoError(t, err)

	assert.NotNil(t, scenario.Flow[0].Args)
	assert.Len(t, scenario.Flow[0].Args, 0)
}

func TestLoadScenario_NumericAndBooleanArgs(t *testing.T) {
	dir := t.TempDir()
	specPath := createTestSpec(t, dir, "cart.concept.cue")
	scenarioPath := filepath.Join(dir, "test.yaml")

	content := `
name: test
description: "Test with various arg types"
specs:
  - ` + specPath + `
flow:
  - invoke: Cart.addItem
    args:
      item_id: "widget"
      quantity: 5
      is_gift: true
      discount: 0.15
assertions:
  - type: trace_contains
    action: Cart.addItem
`
	require.NoError(t, os.WriteFile(scenarioPath, []byte(content), 0644))

	scenario, err := LoadScenario(scenarioPath)
	require.NoError(t, err)

	args := scenario.Flow[0].Args
	assert.Equal(t, "widget", args["item_id"])
	assert.Equal(t, 5, args["quantity"])
	assert.Equal(t, true, args["is_gift"])
	assert.Equal(t, 0.15, args["discount"])
}

func TestLoadScenarioWithBasePath(t *testing.T) {
	dir := t.TempDir()
	createTestSpec(t, dir, "cart.concept.cue")
	scenarioPath := filepath.Join(dir, "test.yaml")

	// Use relative path in scenario
	content := `
name: test
description: "Test with relative spec path"
specs:
  - specs/cart.concept.cue
flow:
  - invoke: Cart.addItem
    args: {}
assertions:
  - type: trace_contains
    action: Cart.addItem
`
	require.NoError(t, os.WriteFile(scenarioPath, []byte(content), 0644))

	// Load without base path - should fail because spec path is relative
	_, err := LoadScenario(scenarioPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "spec file not found")

	// Now load with base path
	scenario, err := LoadScenarioWithBasePath(scenarioPath, dir)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "specs/cart.concept.cue"), scenario.Specs[0])
}

// Note: Path handling now uses filepath.IsAbs and filepath.Join from stdlib
// which provides proper cross-platform support. These tests verify the behavior
// of LoadScenarioWithBasePath with the stdlib functions.

func TestLoadScenarioWithBasePath_AbsoluteSpecPath(t *testing.T) {
	// Create temp directory with a spec file
	dir := t.TempDir()
	specDir := filepath.Join(dir, "absolute")
	require.NoError(t, os.MkdirAll(specDir, 0755))

	specContent := `concept Cart { purpose: "Test" action addItem: { args: {}, outputs: [{ case: "Success" }] } }`
	specPath := filepath.Join(specDir, "cart.cue")
	require.NoError(t, os.WriteFile(specPath, []byte(specContent), 0644))

	// Create scenario with absolute spec path
	scenarioContent := fmt.Sprintf(`
name: test
description: Test absolute paths
specs:
  - %s
flow:
  - invoke: Cart.addItem
    args: {}
assertions:
  - type: trace_contains
    action: Cart.addItem
`, specPath)

	scenarioPath := filepath.Join(dir, "test.yaml")
	require.NoError(t, os.WriteFile(scenarioPath, []byte(scenarioContent), 0644))

	// Load with base path - absolute paths should NOT be joined
	scenario, err := LoadScenarioWithBasePath(scenarioPath, "/some/other/base")
	require.NoError(t, err)
	assert.Equal(t, specPath, scenario.Specs[0]) // Should remain absolute
}

func TestLoadScenario_UnknownFieldsRejected(t *testing.T) {
	// YAML files with typos (unknown fields) should be rejected
	dir := t.TempDir()
	specPath := createTestSpec(t, dir, "cart.cue")

	tests := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{
			name: "typo_assertion_singular",
			yaml: fmt.Sprintf(`
name: test
description: Test typo
specs: [%s]
flow:
  - invoke: Cart.addItem
    args: {}
assertion:
  - type: trace_contains
    action: Cart.addItem
assertions:
  - type: trace_contains
    action: Cart.addItem
`, specPath),
			wantErr: "field assertion not found",
		},
		{
			name: "typo_in_flow_step",
			yaml: fmt.Sprintf(`
name: test
description: Test typo
specs: [%s]
flow:
  - invok: Cart.addItem
    args: {}
assertions:
  - type: trace_contains
    action: Cart.addItem
`, specPath),
			wantErr: "field invok not found",
		},
		{
			name: "unknown_top_level_field",
			yaml: fmt.Sprintf(`
name: test
description: Test typo
specs: [%s]
unknown_field: value
flow:
  - invoke: Cart.addItem
    args: {}
assertions:
  - type: trace_contains
    action: Cart.addItem
`, specPath),
			wantErr: "field unknown_field not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scenarioPath := filepath.Join(dir, tt.name+".yaml")
			require.NoError(t, os.WriteFile(scenarioPath, []byte(tt.yaml), 0644))

			_, err := LoadScenario(scenarioPath)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestLoadScenario_TraceCountZeroAllowed(t *testing.T) {
	// trace_count: 0 should be valid (assert action does NOT appear)
	dir := t.TempDir()
	specPath := createTestSpec(t, dir, "cart.cue")

	scenarioContent := fmt.Sprintf(`
name: test_zero_count
description: Test trace_count with zero
specs: [%s]
flow:
  - invoke: Cart.addItem
    args: {}
assertions:
  - type: trace_count
    action: Cart.checkout
    count: 0
`, specPath)

	scenarioPath := filepath.Join(dir, "zero_count.yaml")
	require.NoError(t, os.WriteFile(scenarioPath, []byte(scenarioContent), 0644))

	scenario, err := LoadScenario(scenarioPath)
	require.NoError(t, err)
	assert.Equal(t, 0, scenario.Assertions[0].Count)
}

func TestLoadScenario_TraceCountNegativeRejected(t *testing.T) {
	// trace_count: -1 should be invalid
	dir := t.TempDir()
	specPath := createTestSpec(t, dir, "cart.cue")

	scenarioContent := fmt.Sprintf(`
name: test_negative_count
description: Test trace_count with negative
specs: [%s]
flow:
  - invoke: Cart.addItem
    args: {}
assertions:
  - type: trace_count
    action: Cart.checkout
    count: -1
`, specPath)

	scenarioPath := filepath.Join(dir, "negative_count.yaml")
	require.NoError(t, os.WriteFile(scenarioPath, []byte(scenarioContent), 0644))

	_, err := LoadScenario(scenarioPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "count must be non-negative")
}

func TestAssertionConstants(t *testing.T) {
	assert.Equal(t, "trace_contains", AssertTraceContains)
	assert.Equal(t, "trace_order", AssertTraceOrder)
	assert.Equal(t, "trace_count", AssertTraceCount)
	assert.Equal(t, "final_state", AssertFinalState)
}

// TestLoadExampleScenarios validates the example scenario files in testdata/scenarios.
// These serve as documentation and regression tests.
func TestLoadExampleScenarios(t *testing.T) {
	// Get project root (two levels up from this test)
	projectRoot := "../../"

	tests := []struct {
		name           string
		scenarioFile   string
		wantName       string
		wantSetupCount int
		wantFlowCount  int
		wantAssertions int
	}{
		{
			name:           "cart_checkout_success",
			scenarioFile:   "testdata/scenarios/cart_checkout_success.yaml",
			wantName:       "cart_checkout_success",
			wantSetupCount: 1,
			wantFlowCount:  2,
			wantAssertions: 5, // trace_contains x3, trace_order, trace_count
		},
		{
			name:           "cart_checkout_insufficient_stock",
			scenarioFile:   "testdata/scenarios/cart_checkout_insufficient_stock.yaml",
			wantName:       "cart_checkout_insufficient_stock",
			wantSetupCount: 1,
			wantFlowCount:  2,
			wantAssertions: 5, // trace_contains x3, trace_order, trace_count
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scenarioPath := filepath.Join(projectRoot, tt.scenarioFile)
			scenario, err := LoadScenarioWithBasePath(scenarioPath, projectRoot)
			require.NoError(t, err, "Failed to load example scenario %s", tt.scenarioFile)

			assert.Equal(t, tt.wantName, scenario.Name)
			assert.Len(t, scenario.Setup, tt.wantSetupCount)
			assert.Len(t, scenario.Flow, tt.wantFlowCount)
			assert.Len(t, scenario.Assertions, tt.wantAssertions)
		})
	}
}
