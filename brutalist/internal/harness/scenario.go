package harness

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

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
	// If empty, defaults to "test-flow-default" for deterministic golden file comparison.
	// Production scenarios should specify an explicit token for traceability.
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

// Assertion type constants.
const (
	AssertTraceContains = "trace_contains"
	AssertTraceOrder    = "trace_order"
	AssertTraceCount    = "trace_count"
	AssertFinalState    = "final_state"
)

// LoadScenario reads and parses a scenario YAML file.
// Returns an error if the file doesn't exist, is malformed,
// contains unknown fields (typos), or is missing required fields.
func LoadScenario(path string) (*Scenario, error) {
	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read scenario file: %w", err)
	}

	// Parse YAML with strict field validation (catches typos like "assertion:" vs "assertions:")
	var scenario Scenario
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true) // Reject unknown fields
	if err := decoder.Decode(&scenario); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Validate required fields
	if err := validateScenario(&scenario); err != nil {
		return nil, fmt.Errorf("invalid scenario: %w", err)
	}

	return &scenario, nil
}

// LoadScenarioWithBasePath reads and parses a scenario YAML file,
// resolving spec paths relative to the provided base path.
// This is useful when scenario files reference specs using relative paths.
func LoadScenarioWithBasePath(path, basePath string) (*Scenario, error) {
	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read scenario file: %w", err)
	}

	// Parse YAML with strict field validation (catches typos)
	var scenario Scenario
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true) // Reject unknown fields
	if err := decoder.Decode(&scenario); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Resolve spec paths relative to base path BEFORE validation
	for i, specPath := range scenario.Specs {
		if !filepath.IsAbs(specPath) && basePath != "" {
			scenario.Specs[i] = filepath.Join(basePath, specPath)
		}
	}

	// Validate required fields (now with resolved paths)
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

	// Validate setup steps (if present)
	for i, step := range s.Setup {
		if step.Action == "" {
			return fmt.Errorf("setup[%d]: action is required", i)
		}
		if step.Args == nil {
			return fmt.Errorf("setup[%d]: args is required (use empty map if no args)", i)
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
		// Validate expect clause if present
		if step.Expect != nil && step.Expect.Case == "" {
			return fmt.Errorf("flow[%d].expect: case is required", i)
		}
	}

	// Validate assertions
	for i, assertion := range s.Assertions {
		if err := validateAssertion(i, &assertion); err != nil {
			return err
		}
	}

	return nil
}

// validateAssertion validates a single assertion based on its type.
func validateAssertion(index int, a *Assertion) error {
	if a.Type == "" {
		return fmt.Errorf("assertions[%d]: type is required", index)
	}

	switch a.Type {
	case AssertTraceContains:
		if a.Action == "" {
			return fmt.Errorf("assertions[%d]: action is required for trace_contains", index)
		}
	case AssertTraceOrder:
		if len(a.Actions) == 0 {
			return fmt.Errorf("assertions[%d]: actions list is required for trace_order", index)
		}
	case AssertTraceCount:
		if a.Action == "" {
			return fmt.Errorf("assertions[%d]: action is required for trace_count", index)
		}
		if a.Count < 0 {
			return fmt.Errorf("assertions[%d]: count must be non-negative for trace_count", index)
		}
	case AssertFinalState:
		if a.Table == "" {
			return fmt.Errorf("assertions[%d]: table is required for final_state", index)
		}
		if a.Expect == nil || len(a.Expect) == 0 {
			return fmt.Errorf("assertions[%d]: expect is required for final_state", index)
		}
	default:
		return fmt.Errorf("assertions[%d]: unknown assertion type %q", index, a.Type)
	}

	return nil
}
