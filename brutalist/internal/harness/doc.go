// Package harness provides conformance testing for NYSM specifications.
//
// The harness loads concept specs and sync rules, executes test scenarios,
// and validates operational principles as executable contract tests.
//
// # Scenario Format
//
// Scenarios are defined in YAML files with the following structure:
//
//	name: scenario_name
//	description: "What this scenario validates"
//	specs:
//	  - path/to/concept.cue
//	  - path/to/sync.cue
//	setup:
//	  - action: Concept.setupAction
//	    args: { key: value }
//	flow:
//	  - invoke: Concept.action
//	    args: { key: value }
//	    expect:
//	      case: Success
//	      result: { field: value }
//	assertions:
//	  - type: trace_contains
//	    action: Concept.action
//	    args: { key: value }
//	  - type: final_state
//	    table: concept_state
//	    where: { id: "123" }
//	    expect: { status: "completed" }
//
// # Assertion Types
//
// The following assertion types are supported:
//
//   - trace_contains: Verifies an action appears in the trace with matching args
//   - trace_order: Verifies actions appear in specified order
//   - trace_count: Verifies an action appears exactly N times
//   - final_state: Queries a state table and verifies expected values
//
// # Deterministic Testing
//
// All scenarios execute with deterministic clock and flow token generation
// to ensure reproducible test results and golden snapshot comparison.
//
// The harness uses:
//   - Fixed flow tokens (from scenario.flow_token or generated deterministically)
//   - Deterministic logical clock (testutil.DeterministicClock)
//   - In-memory SQLite database (isolated per test)
//
// This ensures identical traces across runs for golden file comparison.
//
// # Usage
//
// Load a scenario:
//
//	scenario, err := harness.LoadScenario("testdata/scenarios/checkout.yaml")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// Execute with the harness (implemented in Story 6.2):
//
//	result := harness.Run(scenario)
//	if !result.Pass {
//	    for _, err := range result.Errors {
//	        log.Println(err)
//	    }
//	}
package harness
