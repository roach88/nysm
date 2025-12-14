# Story 6.1: Scenario Definition Format

Status: done

## Story

As a **developer writing conformance tests**,
I want **a YAML format for defining test scenarios**,
So that **I can specify inputs, expected outputs, and assertions**.

## Acceptance Criteria

1. **Scenario struct definition in `internal/harness/scenario.go`**
   ```go
   type Scenario struct {
       Name        string         `yaml:"name"`
       Description string         `yaml:"description"`
       Specs       []string       `yaml:"specs"`         // Paths to CUE spec files
       Setup       []ActionStep   `yaml:"setup"`         // Setup invocations
       Flow        []FlowStep     `yaml:"flow"`          // Main test flow
       Assertions  []Assertion    `yaml:"assertions"`    // Final assertions
       FlowToken   string         `yaml:"flow_token,omitempty"` // Optional fixed token for determinism
   }

   type ActionStep struct {
       Action string                 `yaml:"action"`  // Action URI (e.g., "Inventory.setStock")
       Args   map[string]interface{} `yaml:"args"`    // Action arguments
   }

   type FlowStep struct {
       Invoke string                 `yaml:"invoke"`  // Action URI
       Args   map[string]interface{} `yaml:"args"`    // Action arguments
       Expect *ExpectClause          `yaml:"expect,omitempty"` // Expected completion
   }

   type ExpectClause struct {
       Case   string                 `yaml:"case"`             // Output case name
       Result map[string]interface{} `yaml:"result,omitempty"` // Expected result fields
   }

   type Assertion struct {
       Type   string                 `yaml:"type"`    // Assertion type
       Action string                 `yaml:"action,omitempty"`
       Args   map[string]interface{} `yaml:"args,omitempty"`
       Table  string                 `yaml:"table,omitempty"`
       Where  map[string]interface{} `yaml:"where,omitempty"`
       Expect map[string]interface{} `yaml:"expect,omitempty"`
   }
   ```

2. **LoadScenario function parses YAML files**
   ```go
   func LoadScenario(path string) (*Scenario, error)
   ```
   - Reads YAML file from filesystem
   - Unmarshals into Scenario struct
   - Validates required fields (name, specs)
   - Returns clear errors for malformed YAML

3. **YAML format supports all scenario sections**
   - `name` (required) - Scenario identifier
   - `description` (required) - Human-readable description
   - `specs` (required) - List of CUE spec file paths
   - `setup` (optional) - Setup actions before main flow
   - `flow` (required) - Main test flow with invocations
   - `assertions` (required) - Test assertions
   - `flow_token` (optional) - Fixed flow token for deterministic tests

4. **Setup section for initial state**
   - List of actions to invoke before main flow
   - Each action has `action` (URI) and `args` (map)
   - No expectations - setup actions assumed to succeed

5. **Flow section with invoke and expect**
   - List of invocations in order
   - Each step has `invoke` (action URI), `args`, and optional `expect`
   - `expect.case` matches OutputCase name
   - `expect.result` contains expected field values (subset match)

6. **Assertions section with multiple types**
   - `trace_contains` - Check action appears in trace with args
   - `final_state` - Query table and verify expected values
   - Each assertion type uses relevant fields (action, args, table, where, expect)

## Quick Reference

| Pattern | Key Rule |
|---------|----------|
| **FR-6.1** | Load concept specs and sync rules for test execution |
| **FR-6.2** | Run scenarios with assertions on action traces |

## Tasks / Subtasks

- [ ] Task 1: Create harness package structure (AC: #1)
  - [ ] 1.1 Create `internal/harness/` directory
  - [ ] 1.2 Create `internal/harness/doc.go` with package documentation
  - [ ] 1.3 Create `internal/harness/scenario.go` for types

- [ ] Task 2: Define Scenario struct and related types (AC: #1, #3)
  - [ ] 2.1 Define `Scenario` struct with all fields
  - [ ] 2.2 Define `ActionStep` struct for setup actions
  - [ ] 2.3 Define `FlowStep` struct with invoke/expect
  - [ ] 2.4 Define `ExpectClause` struct for expected completions
  - [ ] 2.5 Define `Assertion` struct with type and fields
  - [ ] 2.6 Add YAML tags to all structs

- [ ] Task 3: Implement LoadScenario function (AC: #2)
  - [ ] 3.1 Create `LoadScenario(path string) (*Scenario, error)` function
  - [ ] 3.2 Read file from filesystem
  - [ ] 3.3 Unmarshal YAML into Scenario struct
  - [ ] 3.4 Validate required fields (name, specs, flow, assertions)
  - [ ] 3.5 Return descriptive errors for missing/invalid fields

- [ ] Task 4: Add YAML validation helpers (AC: #2)
  - [ ] 4.1 Create `validateScenario(s *Scenario) error` function
  - [ ] 4.2 Check name is non-empty
  - [ ] 4.3 Check specs list is non-empty
  - [ ] 4.4 Check flow list is non-empty
  - [ ] 4.5 Check assertions list is non-empty
  - [ ] 4.6 Validate spec paths exist (file check)

- [ ] Task 5: Write comprehensive tests (all AC)
  - [ ] 5.1 Test loading valid scenario YAML
  - [ ] 5.2 Test all required fields parsed correctly
  - [ ] 5.3 Test optional fields (setup, flow_token, expect)
  - [ ] 5.4 Test missing required fields error
  - [ ] 5.5 Test malformed YAML error
  - [ ] 5.6 Test invalid spec paths detected
  - [ ] 5.7 Test assertion types parsed correctly

- [ ] Task 6: Create example scenario fixtures (AC: #3-6)
  - [ ] 6.1 Create `testdata/scenarios/` directory
  - [ ] 6.2 Create `cart_checkout_success.yaml` example
  - [ ] 6.3 Create `cart_checkout_insufficient_stock.yaml` example
  - [ ] 6.4 Examples demonstrate all scenario sections

## Dev Notes

### Scenario Definition

```go
// internal/harness/scenario.go
package harness

import (
    "fmt"
    "os"

    "gopkg.in/yaml.v3"
)

// Scenario defines a conformance test scenario.
// Scenarios validate operational principles by executing a flow of actions
// and asserting on the resulting trace and final state.
type Scenario struct {
    // Name uniquely identifies this scenario.
    Name string `yaml:"name"`

    // Description explains what this scenario validates.
    Description string `yaml:"description"`

    // Specs lists paths to CUE spec files to compile and load.
    // Paths are relative to the scenario file location.
    Specs []string `yaml:"specs"`

    // Setup contains actions to invoke before the main flow.
    // These establish initial state (e.g., setting inventory stock).
    // Setup actions are assumed to succeed.
    Setup []ActionStep `yaml:"setup,omitempty"`

    // Flow contains the main test flow - invocations with expected results.
    // Each step can specify expected output case and result values.
    Flow []FlowStep `yaml:"flow"`

    // Assertions validate the final trace and state.
    // Supported types: trace_contains, trace_order, trace_count, final_state
    Assertions []Assertion `yaml:"assertions"`

    // FlowToken is an optional fixed flow token for deterministic tests.
    // If empty, a random UUIDv7 is generated.
    FlowToken string `yaml:"flow_token,omitempty"`
}

// ActionStep represents a single action invocation.
// Used in Setup sections to establish initial state.
type ActionStep struct {
    // Action is the action URI (e.g., "Inventory.setStock").
    Action string `yaml:"action"`

    // Args contains the action arguments as a map.
    // Values are converted to ir.IRValue types during execution.
    Args map[string]interface{} `yaml:"args"`
}

// FlowStep represents a step in the main test flow.
// Each step invokes an action and optionally validates the completion.
type FlowStep struct {
    // Invoke is the action URI to invoke.
    Invoke string `yaml:"invoke"`

    // Args contains the action arguments.
    Args map[string]interface{} `yaml:"args"`

    // Expect specifies the expected completion result.
    // If nil, no validation is performed (action assumed to succeed).
    Expect *ExpectClause `yaml:"expect,omitempty"`
}

// ExpectClause specifies expected completion behavior.
type ExpectClause struct {
    // Case is the expected OutputCase name (e.g., "Success", "InsufficientStock").
    Case string `yaml:"case"`

    // Result contains expected result field values.
    // This is a subset match - only specified fields are validated.
    // If nil, only the case is validated.
    Result map[string]interface{} `yaml:"result,omitempty"`
}

// Assertion validates trace or final state.
type Assertion struct {
    // Type specifies the assertion type:
    // - "trace_contains": Check action appears in trace with args
    // - "trace_order": Check actions appear in order
    // - "trace_count": Check action appears exactly N times
    // - "final_state": Query table and verify expected values
    Type string `yaml:"type"`

    // Action is the action URI (used by trace_contains, trace_order, trace_count).
    Action string `yaml:"action,omitempty"`

    // Args are the expected action arguments (used by trace_contains).
    // Subset match - only specified fields are validated.
    Args map[string]interface{} `yaml:"args,omitempty"`

    // Table is the state table name (used by final_state).
    Table string `yaml:"table,omitempty"`

    // Where specifies query filters (used by final_state).
    // All fields must match exactly.
    Where map[string]interface{} `yaml:"where,omitempty"`

    // Expect contains expected field values (used by final_state).
    // Subset match - only specified fields are validated.
    Expect map[string]interface{} `yaml:"expect,omitempty"`

    // Count is the expected number of occurrences (used by trace_count).
    Count int `yaml:"count,omitempty"`

    // Actions is the expected action order (used by trace_order).
    Actions []string `yaml:"actions,omitempty"`
}

// LoadScenario reads and parses a scenario YAML file.
// Returns an error if the file doesn't exist, is malformed,
// or is missing required fields.
func LoadScenario(path string) (*Scenario, error) {
    // Read file
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("failed to read scenario file: %w", err)
    }

    // Parse YAML
    var scenario Scenario
    if err := yaml.Unmarshal(data, &scenario); err != nil {
        return nil, fmt.Errorf("failed to parse YAML: %w", err)
    }

    // Validate required fields
    if err := validateScenario(&scenario); err != nil {
        return nil, fmt.Errorf("invalid scenario: %w", err)
    }

    return &scenario, nil
}

// validateScenario checks that required fields are present and valid.
func validateScenario(s *Scenario) error {
    if s.Name == "" {
        return fmt.Errorf("name is required")
    }

    if s.Description == "" {
        return fmt.Errorf("description is required")
    }

    if len(s.Specs) == 0 {
        return fmt.Errorf("specs list is required and must be non-empty")
    }

    if len(s.Flow) == 0 {
        return fmt.Errorf("flow list is required and must be non-empty")
    }

    if len(s.Assertions) == 0 {
        return fmt.Errorf("assertions list is required and must be non-empty")
    }

    // Validate spec paths exist
    for _, specPath := range s.Specs {
        if _, err := os.Stat(specPath); os.IsNotExist(err) {
            return fmt.Errorf("spec file not found: %s", specPath)
        }
    }

    // Validate flow steps
    for i, step := range s.Flow {
        if step.Invoke == "" {
            return fmt.Errorf("flow[%d]: invoke is required", i)
        }
        if step.Args == nil {
            return fmt.Errorf("flow[%d]: args is required (use empty map if no args)", i)
        }
    }

    // Validate assertions
    for i, assertion := range s.Assertions {
        if assertion.Type == "" {
            return fmt.Errorf("assertions[%d]: type is required", i)
        }

        switch assertion.Type {
        case "trace_contains":
            if assertion.Action == "" {
                return fmt.Errorf("assertions[%d]: action is required for trace_contains", i)
            }
        case "trace_order":
            if len(assertion.Actions) == 0 {
                return fmt.Errorf("assertions[%d]: actions list is required for trace_order", i)
            }
        case "trace_count":
            if assertion.Action == "" {
                return fmt.Errorf("assertions[%d]: action is required for trace_count", i)
            }
            if assertion.Count <= 0 {
                return fmt.Errorf("assertions[%d]: count must be positive for trace_count", i)
            }
        case "final_state":
            if assertion.Table == "" {
                return fmt.Errorf("assertions[%d]: table is required for final_state", i)
            }
            if assertion.Expect == nil || len(assertion.Expect) == 0 {
                return fmt.Errorf("assertions[%d]: expect is required for final_state", i)
            }
        default:
            return fmt.Errorf("assertions[%d]: unknown assertion type %q", i, assertion.Type)
        }
    }

    return nil
}
```

### Package Documentation

```go
// internal/harness/doc.go
// Package harness provides conformance testing for NYSM specifications.
//
// The harness loads concept specs and sync rules, executes test scenarios,
// and validates operational principles as executable contract tests.
//
// SCENARIO FORMAT:
//
// Scenarios are defined in YAML files with the following structure:
//
//   name: scenario_name
//   description: "What this scenario validates"
//   specs:
//     - path/to/concept.cue
//     - path/to/sync.cue
//   setup:
//     - action: Concept.setupAction
//       args: { key: value }
//   flow:
//     - invoke: Concept.action
//       args: { key: value }
//       expect:
//         case: Success
//         result: { field: value }
//   assertions:
//     - type: trace_contains
//       action: Concept.action
//       args: { key: value }
//     - type: final_state
//       table: concept_state
//       where: { id: "123" }
//       expect: { status: "completed" }
//
// ASSERTION TYPES:
//
// - trace_contains: Verifies an action appears in the trace with matching args
// - trace_order: Verifies actions appear in specified order
// - trace_count: Verifies an action appears exactly N times
// - final_state: Queries a state table and verifies expected values
//
// All scenarios execute with deterministic clock and flow token generation
// to ensure reproducible test results and golden snapshot comparison.
//
// DETERMINISTIC TESTING:
//
// The harness uses:
// - Fixed flow tokens (from scenario.flow_token or generated deterministically)
// - Deterministic logical clock (testutil.DeterministicClock)
// - In-memory SQLite database (isolated per test)
//
// This ensures identical traces across runs for golden file comparison.
package harness
```

### Example Scenario: Cart Checkout Success

```yaml
# testdata/scenarios/cart_checkout_success.yaml
name: cart_checkout_success
description: "Successful checkout triggers inventory reservation"

specs:
  - specs/cart.concept.cue
  - specs/inventory.concept.cue
  - specs/cart-inventory.sync.cue

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
  # Verify sync rule fired and generated invocation
  - type: trace_contains
    action: Inventory.reserve
    args:
      item_id: "widget"
      quantity: 3

  # Verify final inventory state
  - type: final_state
    table: inventory
    where:
      item_id: "widget"
    expect:
      quantity: 7  # 10 - 3

  # Verify action order
  - type: trace_order
    actions:
      - Cart.addItem
      - Cart.checkout
      - Inventory.reserve

  # Verify single reserve invocation
  - type: trace_count
    action: Inventory.reserve
    count: 1
```

### Example Scenario: Insufficient Stock

```yaml
# testdata/scenarios/cart_checkout_insufficient_stock.yaml
name: cart_checkout_insufficient_stock
description: "Checkout with insufficient stock triggers error handling"

specs:
  - specs/cart.concept.cue
  - specs/inventory.concept.cue
  - specs/cart-inventory.sync.cue

setup:
  - action: Inventory.setStock
    args:
      item_id: "widget"
      quantity: 2

flow:
  - invoke: Cart.addItem
    args:
      item_id: "widget"
      quantity: 3
    expect:
      case: Success

  - invoke: Cart.checkout
    args: {}
    expect:
      case: CheckoutFailed  # Error case propagated from inventory

assertions:
  # Verify reserve was attempted
  - type: trace_contains
    action: Inventory.reserve
    args:
      item_id: "widget"
      quantity: 3

  # Verify reserve failed with error case
  - type: trace_contains
    action: Inventory.reserve.completed
    args:
      output_case: InsufficientStock

  # Verify final inventory unchanged (no partial state)
  - type: final_state
    table: inventory
    where:
      item_id: "widget"
    expect:
      quantity: 2  # Unchanged

  # Verify cart still has items (transaction failed)
  - type: final_state
    table: cart_items
    where:
      item_id: "widget"
    expect:
      quantity: 3
```

### Test Examples

```go
// internal/harness/scenario_test.go
package harness

import (
    "os"
    "path/filepath"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestLoadScenario_ValidFile(t *testing.T) {
    // Create temp scenario file
    dir := t.TempDir()
    scenarioPath := filepath.Join(dir, "test.yaml")

    // Create minimal valid specs for reference
    specsDir := filepath.Join(dir, "specs")
    require.NoError(t, os.Mkdir(specsDir, 0755))
    specPath := filepath.Join(specsDir, "cart.concept.cue")
    require.NoError(t, os.WriteFile(specPath, []byte("concept Cart {}"), 0644))

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
}

func TestLoadScenario_MissingName(t *testing.T) {
    dir := t.TempDir()
    scenarioPath := filepath.Join(dir, "test.yaml")

    content := `
description: "Missing name"
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

    _, err := LoadScenario(scenarioPath)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "name is required")
}

func TestLoadScenario_MissingDescription(t *testing.T) {
    dir := t.TempDir()
    scenarioPath := filepath.Join(dir, "test.yaml")

    content := `
name: test
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

    _, err := LoadScenario(scenarioPath)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "description is required")
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

func TestLoadScenario_WithSetup(t *testing.T) {
    dir := t.TempDir()
    scenarioPath := filepath.Join(dir, "test.yaml")

    specsDir := filepath.Join(dir, "specs")
    require.NoError(t, os.Mkdir(specsDir, 0755))
    specPath := filepath.Join(specsDir, "inventory.concept.cue")
    require.NoError(t, os.WriteFile(specPath, []byte("concept Inventory {}"), 0644))

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

func TestLoadScenario_WithExpectations(t *testing.T) {
    dir := t.TempDir()
    scenarioPath := filepath.Join(dir, "test.yaml")

    specsDir := filepath.Join(dir, "specs")
    require.NoError(t, os.Mkdir(specsDir, 0755))
    specPath := filepath.Join(specsDir, "cart.concept.cue")
    require.NoError(t, os.WriteFile(specPath, []byte("concept Cart {}"), 0644))

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
            wantErr: "count must be positive for trace_count",
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
            name: "unknown_type",
            assertionYAML: `
  - type: unknown_assertion
    action: Cart.addItem
`,
            wantErr: "unknown assertion type",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            dir := t.TempDir()
            scenarioPath := filepath.Join(dir, "test.yaml")

            specsDir := filepath.Join(dir, "specs")
            require.NoError(t, os.Mkdir(specsDir, 0755))
            specPath := filepath.Join(specsDir, "cart.concept.cue")
            require.NoError(t, os.WriteFile(specPath, []byte("concept Cart {}"), 0644))

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

func TestLoadScenario_FixedFlowToken(t *testing.T) {
    dir := t.TempDir()
    scenarioPath := filepath.Join(dir, "test.yaml")

    specsDir := filepath.Join(dir, "specs")
    require.NoError(t, os.Mkdir(specsDir, 0755))
    specPath := filepath.Join(specsDir, "cart.concept.cue")
    require.NoError(t, os.WriteFile(specPath, []byte("concept Cart {}"), 0644))

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
```

### File List

Files to create:

1. `internal/harness/doc.go` - Package documentation
2. `internal/harness/scenario.go` - Scenario struct, LoadScenario function, validation
3. `internal/harness/scenario_test.go` - Comprehensive tests
4. `testdata/scenarios/cart_checkout_success.yaml` - Example scenario
5. `testdata/scenarios/cart_checkout_insufficient_stock.yaml` - Example scenario

Dependencies to add to `go.mod`:
```go
require (
    gopkg.in/yaml.v3 v3.0.1  // YAML parsing
)
```

### Relationship to Other Stories

**Dependencies:**
- Story 1.1 (Project Initialization & IR Type Definitions) - Required for project structure

**Enables:**
- Story 6.2 (Test Execution Engine) - Uses Scenario struct to run tests
- Story 6.3 (Trace Assertions) - Implements assertion validation logic
- Story 6.4 (Final State Assertions) - Implements state query assertions
- Story 6.5 (Operational Principle Validation) - Uses scenarios to validate principles
- Story 6.6 (Golden Trace Snapshots) - Uses scenarios for golden file generation

**Note:** This story defines the data model and loading mechanism. Subsequent stories (6.2-6.6) will implement the execution and assertion logic.

### Story Completion Checklist

- [ ] `internal/harness/` directory created
- [ ] `internal/harness/doc.go` written with package documentation
- [ ] `internal/harness/scenario.go` defines all structs with YAML tags
- [ ] `LoadScenario()` function reads and parses YAML files
- [ ] `validateScenario()` checks all required fields
- [ ] Spec file paths validated (files must exist)
- [ ] All assertion types validated (trace_contains, trace_order, trace_count, final_state)
- [ ] Flow steps validated (invoke and args required)
- [ ] `gopkg.in/yaml.v3` dependency added to go.mod
- [ ] `testdata/scenarios/` directory created
- [ ] `cart_checkout_success.yaml` example created
- [ ] `cart_checkout_insufficient_stock.yaml` example created
- [ ] All tests pass (`go test ./internal/harness/...`)
- [ ] `go build ./...` succeeds
- [ ] `go vet ./internal/harness` passes
- [ ] Test coverage for LoadScenario error cases
- [ ] Test coverage for all assertion type validations
- [ ] Test coverage for optional fields (setup, flow_token, expect)

### References

- [Source: docs/epics.md#Story 6.1] - Acceptance criteria and scenario format
- [Source: docs/prd.md#FR-6.1] - Load concept specs and sync rules requirement
- [Source: docs/prd.md#FR-6.2] - Run scenarios with assertions requirement
- [Source: docs/prd.md#Appendix A] - Canonical demo scenarios
- [Source: docs/architecture.md#Project Structure] - Harness package location
- [Source: docs/architecture.md#Testing Strategy] - Golden file testing approach

## Dev Agent Record

### Agent Model Used

Claude Sonnet 4.5 (claude-sonnet-4-5-20250929)

### Validation History

- Initial creation: 2025-12-12

### Completion Notes

- Scenario struct supports all required sections: name, description, specs, setup, flow, assertions
- YAML format is human-readable and follows standard conventions
- LoadScenario validates all required fields and provides clear error messages
- Assertion types cover trace validation (contains, order, count) and state queries (final_state)
- Setup section enables initial state configuration (e.g., setting inventory stock)
- Flow section supports expected outputs for validation during execution
- Fixed flow tokens enable deterministic testing for golden snapshot comparison
- Spec file path validation ensures referenced files exist before test execution
- Example scenarios demonstrate cart-inventory coordination from PRD Appendix A
- Next stories (6.2-6.6) will implement execution engine and assertion logic using these types
