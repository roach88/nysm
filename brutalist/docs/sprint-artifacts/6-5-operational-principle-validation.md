# Story 6.5: Operational Principle Validation

Status: done

## Story

As a **developer defining concepts**,
I want **operational principles validated as executable tests**,
So that **documentation can't rot**.

## Acceptance Criteria

1. **`ExtractScenarios(principle OperationalPrincipleSpec) ([]string, error)` in `internal/harness/principle.go`**
   - Takes an operational principle spec
   - Returns list of scenario file paths or error
   - Supports explicit scenario file references (primary approach)
   - Natural language parsing deferred (future/optional)

2. **OperationalPrincipleSpec supports file references**
   ```go
   // Structured format with explicit scenario reference
   type OperationalPrincipleSpec struct {
       Description string `json:"description"`
       Scenario    string `json:"scenario"` // Path to scenario file
   }

   // CUE format:
   operational_principle: {
       description: "Adding existing item increases quantity"
       scenario: "testdata/scenarios/cart_add_existing.yaml"
   }

   // Simple string format (legacy):
   operational_principle: """
       When a user adds an item that already exists in the cart,
       the quantity is increased rather than creating a duplicate entry.
       """
   ```

3. **Validation ensures referenced scenarios exist**
   - Check that scenario file exists at compile time
   - Clear error if scenario file missing
   - Support relative paths from spec directory

4. **Integration with harness.Run**
   ```go
   func (h *Harness) ValidateOperationalPrinciples(
       specs []ir.ConceptSpec,
   ) (*ValidationResult, error) {
       // For each spec with operational_principle:
       // 1. Extract scenario file paths
       // 2. Load scenarios
       // 3. Run scenarios
       // 4. Report results
   }
   ```

5. **Multiple operational principles per concept**
   ```go
   type ConceptSpec struct {
       Name                  string                        `json:"name"`
       Purpose               string                        `json:"purpose"`
       States                []StateSpec                   `json:"states"`
       Actions               []ActionSig                   `json:"actions"`
       OperationalPrinciples []OperationalPrincipleSpec    `json:"operational_principles"`
   }
   ```

## Quick Reference

| Pattern | Key Rule |
|---------|----------|
| **FR-6.3** | Validate operational principles as executable tests |
| **Primary Approach** | Explicit scenario file references |
| **Future Approach** | Natural language parsing (AI-assisted) |

## Tasks / Subtasks

- [ ] Task 1: Define OperationalPrincipleSpec types (AC: #2, #5)
  - [ ] 1.1 Update ConceptSpec in internal/ir/concept.go
  - [ ] 1.2 Define OperationalPrincipleSpec struct
  - [ ] 1.3 Support both structured and string formats

- [ ] Task 2: Implement scenario extraction (AC: #1)
  - [ ] 2.1 Create internal/harness/principle.go
  - [ ] 2.2 Implement ExtractScenarios function
  - [ ] 2.3 Handle structured format with scenario field
  - [ ] 2.4 Handle legacy string format (no scenarios)

- [ ] Task 3: Implement file validation (AC: #3)
  - [ ] 3.1 Check scenario file existence
  - [ ] 3.2 Resolve relative paths from spec directory
  - [ ] 3.3 Generate clear error for missing files

- [ ] Task 4: Integrate with harness (AC: #4)
  - [ ] 4.1 Implement ValidateOperationalPrinciples
  - [ ] 4.2 Load scenarios from extracted paths
  - [ ] 4.3 Run scenarios via harness.Run
  - [ ] 4.4 Collect and report results

- [ ] Task 5: Update compiler to parse principles (AC: #5)
  - [ ] 5.1 Update CUE concept parser
  - [ ] 5.2 Support array of operational_principles
  - [ ] 5.3 Parse both structured and string formats

- [ ] Task 6: Write comprehensive tests
  - [ ] 6.1 Test scenario file reference extraction
  - [ ] 6.2 Test missing scenario file error
  - [ ] 6.3 Test multiple principles per concept
  - [ ] 6.4 Test relative path resolution
  - [ ] 6.5 Test harness integration

## Dev Notes

### ExtractScenarios Implementation

```go
// internal/harness/principle.go
package harness

import (
    "fmt"
    "os"
    "path/filepath"

    "github.com/your-org/nysm/internal/ir"
)

// ExtractScenarios extracts scenario file paths from an operational principle.
// Supports explicit scenario file references (primary).
// Natural language parsing deferred to future (would be AI-assisted).
func ExtractScenarios(principle ir.OperationalPrincipleSpec, specDir string) ([]string, error) {
    // Primary approach: Explicit scenario file reference
    if principle.Scenario != "" {
        scenarioPath := principle.Scenario

        // Resolve relative paths from spec directory
        if !filepath.IsAbs(scenarioPath) {
            scenarioPath = filepath.Join(specDir, scenarioPath)
        }

        // Validate that scenario file exists
        if _, err := os.Stat(scenarioPath); os.IsNotExist(err) {
            return nil, &ScenarioNotFoundError{
                Principle:    principle.Description,
                ScenarioPath: principle.Scenario,
                ResolvedPath: scenarioPath,
            }
        }

        return []string{scenarioPath}, nil
    }

    // Legacy string format with no scenario reference
    // Return empty list (no scenarios to run)
    return []string{}, nil
}

// ScenarioNotFoundError is returned when a referenced scenario file doesn't exist.
type ScenarioNotFoundError struct {
    Principle    string
    ScenarioPath string
    ResolvedPath string
}

func (e *ScenarioNotFoundError) Error() string {
    return fmt.Sprintf(
        "operational principle %q references scenario file %q which does not exist (resolved to: %s)",
        e.Principle,
        e.ScenarioPath,
        e.ResolvedPath,
    )
}
```

### ValidateOperationalPrinciples Implementation

```go
// internal/harness/harness.go
package harness

import (
    "context"
    "fmt"

    "github.com/your-org/nysm/internal/ir"
)

// ValidationResult contains results from validating operational principles.
type ValidationResult struct {
    TotalPrinciples int
    TotalScenarios  int
    Passed          int
    Failed          int
    Failures        []PrincipleFailure
}

// PrincipleFailure represents a failed operational principle validation.
type PrincipleFailure struct {
    ConceptName string
    Principle   string
    ScenarioPath string
    Error       string
}

// ValidateOperationalPrinciples validates all operational principles in the given specs.
// Returns a summary of results.
func (h *Harness) ValidateOperationalPrinciples(
    ctx context.Context,
    specs []ir.ConceptSpec,
    specDir string,
) (*ValidationResult, error) {
    result := &ValidationResult{}

    for _, spec := range specs {
        for _, principle := range spec.OperationalPrinciples {
            result.TotalPrinciples++

            // Extract scenario file paths
            scenarioPaths, err := ExtractScenarios(principle, specDir)
            if err != nil {
                result.Failed++
                result.Failures = append(result.Failures, PrincipleFailure{
                    ConceptName:  spec.Name,
                    Principle:    principle.Description,
                    ScenarioPath: principle.Scenario,
                    Error:        err.Error(),
                })
                continue
            }

            // Run each scenario
            for _, scenarioPath := range scenarioPaths {
                result.TotalScenarios++

                // Load scenario
                scenario, err := LoadScenario(scenarioPath)
                if err != nil {
                    result.Failed++
                    result.Failures = append(result.Failures, PrincipleFailure{
                        ConceptName:  spec.Name,
                        Principle:    principle.Description,
                        ScenarioPath: scenarioPath,
                        Error:        fmt.Sprintf("failed to load scenario: %v", err),
                    })
                    continue
                }

                // Run scenario
                _, err = h.Run(ctx, scenario)
                if err != nil {
                    result.Failed++
                    result.Failures = append(result.Failures, PrincipleFailure{
                        ConceptName:  spec.Name,
                        Principle:    principle.Description,
                        ScenarioPath: scenarioPath,
                        Error:        fmt.Sprintf("scenario failed: %v", err),
                    })
                    continue
                }

                result.Passed++
            }
        }
    }

    return result, nil
}
```

### ConceptSpec Update

```go
// internal/ir/concept.go
package ir

// ConceptSpec represents a compiled concept definition.
// Concepts define state shapes, action signatures, and operational principles.
type ConceptSpec struct {
    Name                  string                     `json:"name"`
    Purpose               string                     `json:"purpose"`
    States                []StateSpec                `json:"states"`
    Actions               []ActionSig                `json:"actions"`
    OperationalPrinciples []OperationalPrincipleSpec `json:"operational_principles,omitempty"`
}

// OperationalPrincipleSpec defines an operational principle and its test scenario.
type OperationalPrincipleSpec struct {
    Description string `json:"description"`           // Human-readable description
    Scenario    string `json:"scenario,omitempty"`    // Path to scenario file
}
```

### Compiler Update for Parsing

```go
// internal/compiler/concept.go
func CompileConcept(v cue.Value) (*ir.ConceptSpec, error) {
    // ... existing parsing ...

    // Parse operational_principles (optional, can be array or single value)
    opVal := v.LookupPath(cue.ParsePath("operational_principles"))
    if !opVal.Exists() {
        // Try singular form for backward compatibility
        opVal = v.LookupPath(cue.ParsePath("operational_principle"))
    }

    if opVal.Exists() {
        spec.OperationalPrinciples, err = parseOperationalPrinciples(opVal)
        if err != nil {
            return nil, err
        }
    }

    return spec, nil
}

func parseOperationalPrinciples(v cue.Value) ([]ir.OperationalPrincipleSpec, error) {
    var principles []ir.OperationalPrincipleSpec

    // Check if it's a list
    listIter, err := v.List()
    if err == nil {
        // Array of operational principles
        for listIter.Next() {
            principle, err := parseOperationalPrinciple(listIter.Value())
            if err != nil {
                return nil, err
            }
            principles = append(principles, principle)
        }
        return principles, nil
    }

    // Single operational principle
    principle, err := parseOperationalPrinciple(v)
    if err != nil {
        return nil, err
    }
    principles = append(principles, principle)

    return principles, nil
}

func parseOperationalPrinciple(v cue.Value) (ir.OperationalPrincipleSpec, error) {
    var principle ir.OperationalPrincipleSpec

    // Check if it's a structured object
    descVal := v.LookupPath(cue.ParsePath("description"))
    if descVal.Exists() {
        // Structured format with description and scenario
        desc, err := descVal.String()
        if err != nil {
            return principle, formatCUEError(err)
        }
        principle.Description = desc

        scenarioVal := v.LookupPath(cue.ParsePath("scenario"))
        if scenarioVal.Exists() {
            scenario, err := scenarioVal.String()
            if err != nil {
                return principle, formatCUEError(err)
            }
            principle.Scenario = scenario
        }

        return principle, nil
    }

    // Legacy string format
    str, err := v.String()
    if err != nil {
        return principle, formatCUEError(err)
    }
    principle.Description = str

    return principle, nil
}
```

### Test Examples

```go
// internal/harness/principle_test.go
package harness

import (
    "os"
    "path/filepath"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "github.com/your-org/nysm/internal/ir"
)

func TestExtractScenariosExplicitReference(t *testing.T) {
    // Create temp scenario file
    tmpDir := t.TempDir()
    scenarioPath := filepath.Join(tmpDir, "test_scenario.yaml")
    err := os.WriteFile(scenarioPath, []byte("name: test\n"), 0644)
    require.NoError(t, err)

    principle := ir.OperationalPrincipleSpec{
        Description: "Test principle",
        Scenario:    "test_scenario.yaml",
    }

    scenarios, err := ExtractScenarios(principle, tmpDir)
    require.NoError(t, err)

    assert.Len(t, scenarios, 1)
    assert.Equal(t, scenarioPath, scenarios[0])
}

func TestExtractScenariosMissingFile(t *testing.T) {
    tmpDir := t.TempDir()

    principle := ir.OperationalPrincipleSpec{
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
}

func TestExtractScenariosLegacyString(t *testing.T) {
    tmpDir := t.TempDir()

    // Legacy string format with no scenario reference
    principle := ir.OperationalPrincipleSpec{
        Description: "When a user adds an item, the quantity increases",
        Scenario:    "", // No scenario reference
    }

    scenarios, err := ExtractScenarios(principle, tmpDir)
    require.NoError(t, err)

    // Should return empty list (no scenarios to run)
    assert.Empty(t, scenarios)
}

func TestExtractScenariosAbsolutePath(t *testing.T) {
    tmpDir := t.TempDir()
    scenarioPath := filepath.Join(tmpDir, "test_scenario.yaml")
    err := os.WriteFile(scenarioPath, []byte("name: test\n"), 0644)
    require.NoError(t, err)

    principle := ir.OperationalPrincipleSpec{
        Description: "Test principle",
        Scenario:    scenarioPath, // Absolute path
    }

    scenarios, err := ExtractScenarios(principle, "/some/other/dir")
    require.NoError(t, err)

    assert.Len(t, scenarios, 1)
    assert.Equal(t, scenarioPath, scenarios[0])
}

func TestValidateOperationalPrinciplesAllPass(t *testing.T) {
    tmpDir := t.TempDir()

    // Create scenario file
    scenarioPath := filepath.Join(tmpDir, "cart_add.yaml")
    scenarioContent := `
name: cart_add_success
description: "Add item to cart"

specs:
  - specs/cart.concept.cue

flow:
  - invoke: Cart.addItem
    args: { item_id: "widget", quantity: 1 }
    expect:
      case: Success
`
    err := os.WriteFile(scenarioPath, []byte(scenarioContent), 0644)
    require.NoError(t, err)

    specs := []ir.ConceptSpec{
        {
            Name:    "Cart",
            Purpose: "Manages cart",
            OperationalPrinciples: []ir.OperationalPrincipleSpec{
                {
                    Description: "Adding items increases quantity",
                    Scenario:    "cart_add.yaml",
                },
            },
        },
    }

    harness := NewHarness(/* ... */)
    result, err := harness.ValidateOperationalPrinciples(context.Background(), specs, tmpDir)

    require.NoError(t, err)
    assert.Equal(t, 1, result.TotalPrinciples)
    assert.Equal(t, 1, result.TotalScenarios)
    assert.Equal(t, 1, result.Passed)
    assert.Equal(t, 0, result.Failed)
    assert.Empty(t, result.Failures)
}

func TestValidateOperationalPrinciplesMissingScenario(t *testing.T) {
    tmpDir := t.TempDir()

    specs := []ir.ConceptSpec{
        {
            Name:    "Cart",
            Purpose: "Manages cart",
            OperationalPrinciples: []ir.OperationalPrincipleSpec{
                {
                    Description: "Adding items increases quantity",
                    Scenario:    "missing.yaml",
                },
            },
        },
    }

    harness := NewHarness(/* ... */)
    result, err := harness.ValidateOperationalPrinciples(context.Background(), specs, tmpDir)

    require.NoError(t, err)
    assert.Equal(t, 1, result.TotalPrinciples)
    assert.Equal(t, 0, result.TotalScenarios)
    assert.Equal(t, 0, result.Passed)
    assert.Equal(t, 1, result.Failed)
    assert.Len(t, result.Failures, 1)
    assert.Contains(t, result.Failures[0].Error, "does not exist")
}

func TestValidateOperationalPrinciplesMultiple(t *testing.T) {
    tmpDir := t.TempDir()

    // Create two scenario files
    scenario1 := filepath.Join(tmpDir, "add_item.yaml")
    scenario2 := filepath.Join(tmpDir, "remove_item.yaml")

    err := os.WriteFile(scenario1, []byte("name: add_item\n"), 0644)
    require.NoError(t, err)
    err = os.WriteFile(scenario2, []byte("name: remove_item\n"), 0644)
    require.NoError(t, err)

    specs := []ir.ConceptSpec{
        {
            Name:    "Cart",
            Purpose: "Manages cart",
            OperationalPrinciples: []ir.OperationalPrincipleSpec{
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

    harness := NewHarness(/* ... */)
    result, err := harness.ValidateOperationalPrinciples(context.Background(), specs, tmpDir)

    require.NoError(t, err)
    assert.Equal(t, 2, result.TotalPrinciples)
    assert.Equal(t, 2, result.TotalScenarios)
}

func TestCompileConceptWithOperationalPrinciples(t *testing.T) {
    ctx := cuecontext.New()
    v := ctx.CompileString(`
        concept Cart {
            purpose: "Manages shopping cart"

            action addItem {
                args: { item_id: string, quantity: int }
                outputs: [{ case: "Success", fields: {} }]
            }

            operational_principles: [
                {
                    description: "Adding items increases quantity"
                    scenario: "testdata/scenarios/cart_add.yaml"
                },
                {
                    description: "Removing items decreases quantity"
                    scenario: "testdata/scenarios/cart_remove.yaml"
                }
            ]
        }
    `)

    conceptVal := v.LookupPath(cue.ParsePath("concept.Cart"))
    spec, err := CompileConcept(conceptVal)

    require.NoError(t, err)
    assert.Len(t, spec.OperationalPrinciples, 2)
    assert.Equal(t, "Adding items increases quantity", spec.OperationalPrinciples[0].Description)
    assert.Equal(t, "testdata/scenarios/cart_add.yaml", spec.OperationalPrinciples[0].Scenario)
}

func TestCompileConceptWithLegacyOperationalPrinciple(t *testing.T) {
    ctx := cuecontext.New()
    v := ctx.CompileString(`
        concept Cart {
            purpose: "Manages shopping cart"

            action addItem {
                args: { item_id: string, quantity: int }
                outputs: [{ case: "Success", fields: {} }]
            }

            operational_principle: "Adding an item increases quantity"
        }
    `)

    conceptVal := v.LookupPath(cue.ParsePath("concept.Cart"))
    spec, err := CompileConcept(conceptVal)

    require.NoError(t, err)
    assert.Len(t, spec.OperationalPrinciples, 1)
    assert.Equal(t, "Adding an item increases quantity", spec.OperationalPrinciples[0].Description)
    assert.Equal(t, "", spec.OperationalPrinciples[0].Scenario)
}
```

### CUE Format Examples

```cue
// Explicit scenario references (primary approach)
concept Cart {
    purpose: "Manages shopping cart state"

    action addItem {
        args: { item_id: string, quantity: int }
        outputs: [{ case: "Success", fields: {} }]
    }

    // Multiple operational principles
    operational_principles: [
        {
            description: "Adding an item that exists increases quantity"
            scenario: "testdata/scenarios/cart_add_existing.yaml"
        },
        {
            description: "Adding a new item creates an entry"
            scenario: "testdata/scenarios/cart_add_new.yaml"
        },
        {
            description: "Checkout with empty cart fails"
            scenario: "testdata/scenarios/cart_checkout_empty.yaml"
        }
    ]
}

// Single operational principle
concept Inventory {
    purpose: "Tracks inventory stock"

    action reserve {
        args: { item_id: string, quantity: int }
        outputs: [
            { case: "Success", fields: { reservation_id: string } },
            { case: "InsufficientStock", fields: { available: int } }
        ]
    }

    operational_principles: [
        {
            description: "Reserving stock decreases available quantity"
            scenario: "testdata/scenarios/inventory_reserve_success.yaml"
        }
    ]
}

// Legacy string format (backward compatibility, no scenarios executed)
concept Web {
    purpose: "Handles web requests"

    action request {
        args: { url: string, method: string }
        outputs: [{ case: "Success", fields: {} }]
    }

    operational_principle: """
        Every request produces a response.
        Timeouts are surfaced as error cases.
        """
}
```

### File List

Files to create/modify:

1. `internal/ir/concept.go` - Add OperationalPrinciples field and OperationalPrincipleSpec struct
2. `internal/harness/principle.go` - ExtractScenarios function
3. `internal/harness/harness.go` - ValidateOperationalPrinciples function
4. `internal/harness/principle_test.go` - ExtractScenarios tests
5. `internal/harness/harness_test.go` - ValidateOperationalPrinciples tests
6. `internal/compiler/concept.go` - Parse operational_principles from CUE
7. `internal/compiler/concept_test.go` - Test operational_principles parsing

### Relationship to Other Stories

- **Story 1.6:** Extends ConceptSpec with OperationalPrinciples field
- **Story 6.1:** Uses Scenario loading infrastructure
- **Story 6.2:** Uses harness.Run for scenario execution
- **Story 6.3:** Builds on trace assertions
- **Story 6.4:** Integrates with golden snapshot comparison

### Story Completion Checklist

- [ ] OperationalPrincipleSpec struct defined
- [ ] ConceptSpec updated with OperationalPrinciples array
- [ ] ExtractScenarios implemented for explicit file references
- [ ] Scenario file existence validation
- [ ] ValidateOperationalPrinciples harness integration
- [ ] Compiler parses operational_principles from CUE
- [ ] Support for both array and single principle
- [ ] Support for both structured and legacy string formats
- [ ] Missing scenario file errors clearly
- [ ] Relative path resolution from spec directory
- [ ] Multiple principles validated correctly
- [ ] All tests pass
- [ ] `go vet ./internal/harness` passes
- [ ] `go vet ./internal/compiler` passes

### References

- [Source: docs/architecture.md#Harness] - Conformance harness architecture
- [Source: docs/epics.md#Story 6.5] - Story definition
- [Source: docs/prd.md#FR-6.3] - Validate operational principles as tests
- [Source: docs/sprint-artifacts/1-6-cue-concept-spec-parser.md] - ConceptSpec parsing

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Validation History

- Initial creation: 2025-12-12

### Completion Notes

- Primary approach: Explicit scenario file references
- Natural language parsing deferred (future/optional, would be AI-assisted)
- Supports both legacy string format (no scenarios) and structured format
- Multiple operational principles per concept supported
- Validation at compile time ensures scenario files exist
- Integration with harness.Run for scenario execution
- Clear error messages when scenario files missing
