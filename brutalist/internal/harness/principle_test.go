package harness

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/roach88/nysm/internal/ir"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractScenarios_ExplicitReference(t *testing.T) {
	// Create temp scenario file
	tmpDir := t.TempDir()
	scenarioPath := filepath.Join(tmpDir, "test_scenario.yaml")
	err := os.WriteFile(scenarioPath, []byte("name: test\n"), 0644)
	require.NoError(t, err)

	principle := ir.OperationalPrinciple{
		Description: "Test principle",
		Scenario:    "test_scenario.yaml",
	}

	scenarios, err := ExtractScenarios(principle, tmpDir)
	require.NoError(t, err)

	assert.Len(t, scenarios, 1)
	assert.Equal(t, scenarioPath, scenarios[0])
}

func TestExtractScenarios_MissingFile(t *testing.T) {
	tmpDir := t.TempDir()

	principle := ir.OperationalPrinciple{
		Description: "Test principle",
		Scenario:    "missing_scenario.yaml",
	}

	scenarios, err := ExtractScenarios(principle, tmpDir)

	require.Error(t, err)
	assert.Nil(t, scenarios)

	// Check error type and message
	scenarioErr, ok := err.(*ScenarioNotFoundError)
	require.True(t, ok, "Expected ScenarioNotFoundError")
	assert.Equal(t, "Test principle", scenarioErr.Principle)
	assert.Equal(t, "missing_scenario.yaml", scenarioErr.ScenarioPath)
	assert.Contains(t, scenarioErr.ResolvedPath, "missing_scenario.yaml")
}

func TestExtractScenarios_LegacyString(t *testing.T) {
	tmpDir := t.TempDir()

	// Legacy string format with no scenario reference
	principle := ir.OperationalPrinciple{
		Description: "When a user adds an item, the quantity increases",
		Scenario:    "", // No scenario reference
	}

	scenarios, err := ExtractScenarios(principle, tmpDir)
	require.NoError(t, err)

	// Should return empty list (no scenarios to run)
	assert.Empty(t, scenarios)
}

func TestExtractScenarios_AbsolutePath(t *testing.T) {
	tmpDir := t.TempDir()
	scenarioPath := filepath.Join(tmpDir, "test_scenario.yaml")
	err := os.WriteFile(scenarioPath, []byte("name: test\n"), 0644)
	require.NoError(t, err)

	principle := ir.OperationalPrinciple{
		Description: "Test principle",
		Scenario:    scenarioPath, // Absolute path
	}

	// specDir should be ignored when scenario path is absolute
	scenarios, err := ExtractScenarios(principle, "/some/other/dir")
	require.NoError(t, err)

	assert.Len(t, scenarios, 1)
	assert.Equal(t, scenarioPath, scenarios[0])
}

func TestExtractScenarios_RelativePath(t *testing.T) {
	tmpDir := t.TempDir()

	// Create nested directory structure
	nestedDir := filepath.Join(tmpDir, "scenarios")
	err := os.MkdirAll(nestedDir, 0755)
	require.NoError(t, err)

	scenarioPath := filepath.Join(nestedDir, "test_scenario.yaml")
	err = os.WriteFile(scenarioPath, []byte("name: test\n"), 0644)
	require.NoError(t, err)

	principle := ir.OperationalPrinciple{
		Description: "Test principle",
		Scenario:    "scenarios/test_scenario.yaml", // Relative path
	}

	scenarios, err := ExtractScenarios(principle, tmpDir)
	require.NoError(t, err)

	assert.Len(t, scenarios, 1)
	assert.Equal(t, scenarioPath, scenarios[0])
}

func TestScenarioNotFoundError_ErrorMessage(t *testing.T) {
	err := &ScenarioNotFoundError{
		Principle:    "Test principle",
		ScenarioPath: "missing.yaml",
		ResolvedPath: "/full/path/to/missing.yaml",
	}

	errStr := err.Error()
	assert.Contains(t, errStr, "Test principle")
	assert.Contains(t, errStr, "missing.yaml")
	assert.Contains(t, errStr, "/full/path/to/missing.yaml")
	assert.Contains(t, errStr, "does not exist")
}

func TestValidateOperationalPrinciples_NoSpecs(t *testing.T) {
	result, err := ValidateOperationalPrinciples(context.Background(), nil, "")
	require.NoError(t, err)

	assert.Equal(t, 0, result.TotalPrinciples)
	assert.Equal(t, 0, result.TotalScenarios)
	assert.Equal(t, 0, result.Passed)
	assert.Equal(t, 0, result.Failed)
	assert.Equal(t, 0, result.Skipped)
}

func TestValidateOperationalPrinciples_LegacySkipped(t *testing.T) {
	specs := []ir.ConceptSpec{
		{
			Name:    "Cart",
			Purpose: "Manages cart",
			OperationalPrinciples: []ir.OperationalPrinciple{
				{
					Description: "Legacy principle without scenario",
					Scenario:    "", // No scenario
				},
			},
		},
	}

	result, err := ValidateOperationalPrinciples(context.Background(), specs, t.TempDir())
	require.NoError(t, err)

	assert.Equal(t, 1, result.TotalPrinciples)
	assert.Equal(t, 0, result.TotalScenarios)
	assert.Equal(t, 0, result.Passed)
	assert.Equal(t, 0, result.Failed)
	assert.Equal(t, 1, result.Skipped) // Legacy format skipped
}

func TestValidateOperationalPrinciples_MissingScenario(t *testing.T) {
	tmpDir := t.TempDir()

	specs := []ir.ConceptSpec{
		{
			Name:    "Cart",
			Purpose: "Manages cart",
			OperationalPrinciples: []ir.OperationalPrinciple{
				{
					Description: "Adding items increases quantity",
					Scenario:    "missing.yaml",
				},
			},
		},
	}

	result, err := ValidateOperationalPrinciples(context.Background(), specs, tmpDir)
	require.NoError(t, err)

	assert.Equal(t, 1, result.TotalPrinciples)
	assert.Equal(t, 0, result.TotalScenarios)
	assert.Equal(t, 0, result.Passed)
	assert.Equal(t, 1, result.Failed)
	assert.Len(t, result.Failures, 1)
	assert.Contains(t, result.Failures[0].Error, "does not exist")
}

func TestValidateOperationalPrinciples_MultiplePrinciples(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a minimal spec file (required by scenario loader)
	specContent := `
concept Cart {
	purpose: "Manages shopping cart"
	state: {
		items: {}
	}
	action addItem: {
		args: {
			item_id: string
			quantity: int
		}
		outputs: [{ case: "Success" }]
	}
	action removeItem: {
		args: {
			item_id: string
		}
		outputs: [{ case: "Success" }]
	}
}
`
	err := os.WriteFile(filepath.Join(tmpDir, "cart.cue"), []byte(specContent), 0644)
	require.NoError(t, err)

	// Create scenario files
	scenario1Content := `
name: add_item
description: "Add item test"
specs:
  - cart.cue
flow_token: "test-flow-1"
flow:
  - invoke: Cart.addItem
    args: { item_id: "widget", quantity: 1 }
assertions:
  - type: trace_contains
    action: Cart.addItem
`
	scenario2Content := `
name: remove_item
description: "Remove item test"
specs:
  - cart.cue
flow_token: "test-flow-2"
flow:
  - invoke: Cart.removeItem
    args: { item_id: "widget" }
assertions:
  - type: trace_contains
    action: Cart.removeItem
`
	err = os.WriteFile(filepath.Join(tmpDir, "add_item.yaml"), []byte(scenario1Content), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "remove_item.yaml"), []byte(scenario2Content), 0644)
	require.NoError(t, err)

	specs := []ir.ConceptSpec{
		{
			Name:    "Cart",
			Purpose: "Manages cart",
			OperationalPrinciples: []ir.OperationalPrinciple{
				{
					Description: "Adding items increases quantity",
					Scenario:    "add_item.yaml",
				},
				{
					Description: "Removing items decreases quantity",
					Scenario:    "remove_item.yaml",
				},
			},
		},
	}

	result, err := ValidateOperationalPrinciples(context.Background(), specs, tmpDir)
	require.NoError(t, err)

	assert.Equal(t, 2, result.TotalPrinciples)
	assert.Equal(t, 2, result.TotalScenarios)
	assert.Equal(t, 2, result.Passed)
	assert.Equal(t, 0, result.Failed)
}

func TestValidateOperationalPrinciples_MixedResults(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a minimal spec file (required by scenario loader)
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

	// Create one valid scenario file
	scenarioContent := `
name: add_item
description: "Add item test"
specs:
  - cart.cue
flow_token: "test-flow"
flow:
  - invoke: Cart.addItem
    args: { item_id: "widget", quantity: 1 }
assertions:
  - type: trace_contains
    action: Cart.addItem
`
	err = os.WriteFile(filepath.Join(tmpDir, "add_item.yaml"), []byte(scenarioContent), 0644)
	require.NoError(t, err)

	specs := []ir.ConceptSpec{
		{
			Name:    "Cart",
			Purpose: "Manages cart",
			OperationalPrinciples: []ir.OperationalPrinciple{
				{
					Description: "Adding items (has scenario)",
					Scenario:    "add_item.yaml",
				},
				{
					Description: "Legacy principle (no scenario)",
					Scenario:    "", // Legacy - will be skipped
				},
				{
					Description: "Missing scenario file",
					Scenario:    "missing.yaml", // Will fail
				},
			},
		},
	}

	result, err := ValidateOperationalPrinciples(context.Background(), specs, tmpDir)
	require.NoError(t, err)

	assert.Equal(t, 3, result.TotalPrinciples)
	assert.Equal(t, 1, result.TotalScenarios)
	assert.Equal(t, 1, result.Passed)
	assert.Equal(t, 1, result.Failed)
	assert.Equal(t, 1, result.Skipped)
}

func TestValidateOperationalPrinciples_MultipleSpecs(t *testing.T) {
	tmpDir := t.TempDir()

	// Create spec files
	cartSpecContent := `
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
	inventorySpecContent := `
concept Inventory {
	purpose: "Manages inventory"
	action reserve: {
		args: {
			item_id: string
			quantity: int
		}
		outputs: [{ case: "Success" }]
	}
}
`
	err := os.WriteFile(filepath.Join(tmpDir, "cart.cue"), []byte(cartSpecContent), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "inventory.cue"), []byte(inventorySpecContent), 0644)
	require.NoError(t, err)

	// Create scenario files
	cartScenarioContent := `
name: cart_test
description: "Cart test"
specs:
  - cart.cue
flow_token: "test-flow-cart"
flow:
  - invoke: Cart.addItem
    args: { item_id: "widget", quantity: 1 }
assertions:
  - type: trace_contains
    action: Cart.addItem
`
	inventoryScenarioContent := `
name: inventory_test
description: "Inventory test"
specs:
  - inventory.cue
flow_token: "test-flow-inventory"
flow:
  - invoke: Inventory.reserve
    args: { item_id: "widget", quantity: 1 }
assertions:
  - type: trace_contains
    action: Inventory.reserve
`
	err = os.WriteFile(filepath.Join(tmpDir, "cart_test.yaml"), []byte(cartScenarioContent), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "inventory_test.yaml"), []byte(inventoryScenarioContent), 0644)
	require.NoError(t, err)

	specs := []ir.ConceptSpec{
		{
			Name:    "Cart",
			Purpose: "Manages cart",
			OperationalPrinciples: []ir.OperationalPrinciple{
				{
					Description: "Cart principle",
					Scenario:    "cart_test.yaml",
				},
			},
		},
		{
			Name:    "Inventory",
			Purpose: "Manages inventory",
			OperationalPrinciples: []ir.OperationalPrinciple{
				{
					Description: "Inventory principle",
					Scenario:    "inventory_test.yaml",
				},
			},
		},
	}

	result, err := ValidateOperationalPrinciples(context.Background(), specs, tmpDir)
	require.NoError(t, err)

	assert.Equal(t, 2, result.TotalPrinciples)
	assert.Equal(t, 2, result.TotalScenarios)
	assert.Equal(t, 2, result.Passed)
	assert.Equal(t, 0, result.Failed)
}

func TestPrincipleFailure_Fields(t *testing.T) {
	failure := PrincipleFailure{
		ConceptName:  "Cart",
		Principle:    "Adding items increases quantity",
		ScenarioPath: "add_item.yaml",
		Error:        "scenario not found",
	}

	assert.Equal(t, "Cart", failure.ConceptName)
	assert.Equal(t, "Adding items increases quantity", failure.Principle)
	assert.Equal(t, "add_item.yaml", failure.ScenarioPath)
	assert.Equal(t, "scenario not found", failure.Error)
}
