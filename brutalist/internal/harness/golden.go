package harness

import (
	"testing"

	"github.com/sebdah/goldie/v2"

	"github.com/roach88/nysm/internal/ir"
)

// TraceSnapshot captures the complete trace for a scenario execution.
// All fields use canonical JSON serialization for deterministic comparison.
type TraceSnapshot struct {
	ScenarioName string       `json:"scenario_name"`
	FlowToken    string       `json:"flow_token,omitempty"`
	Trace        []TraceEvent `json:"trace"`
}

// toCanonicalMap converts a TraceSnapshot to a map[string]any for canonical JSON serialization.
// This is required because ir.MarshalCanonical only handles IR types and primitives.
func (s *TraceSnapshot) toCanonicalMap() map[string]any {
	// Convert trace events to slice of maps
	traceList := make([]any, len(s.Trace))
	for i, event := range s.Trace {
		eventMap := map[string]any{
			"type": event.Type,
			"seq":  event.Seq,
		}
		if event.ActionURI != "" {
			eventMap["action_uri"] = event.ActionURI
		}
		if event.Args != nil {
			eventMap["args"] = event.Args
		}
		if event.OutputCase != "" {
			eventMap["output_case"] = event.OutputCase
		}
		if event.Result != nil {
			eventMap["result"] = event.Result
		}
		traceList[i] = eventMap
	}

	result := map[string]any{
		"scenario_name": s.ScenarioName,
		"trace":         traceList,
	}
	if s.FlowToken != "" {
		result["flow_token"] = s.FlowToken
	}
	return result
}

// RunWithGolden executes a scenario and compares the trace against a golden file.
// The golden file is stored in testdata/golden/{scenario.Name}.golden
//
// To regenerate golden files, run:
//
//	go test ./internal/harness -update
//
// This function is designed for use in tests to verify that scenario execution
// produces the expected trace output. Golden files serve as the "source of truth"
// for expected trace behavior.
//
// Parameters:
//   - t: testing.T instance for test assertions
//   - scenario: the scenario to execute
//
// Returns error if scenario execution fails.
// Test failure (via goldie) occurs if trace doesn't match golden file.
func RunWithGolden(t *testing.T, scenario *Scenario) error {
	t.Helper()

	// Run the scenario
	result, err := Run(scenario)
	if err != nil {
		return err
	}

	// Build trace snapshot
	snapshot := TraceSnapshot{
		ScenarioName: scenario.Name,
		FlowToken:    scenario.FlowToken,
		Trace:        result.Trace,
	}

	// Convert to canonical map and marshal
	canonicalMap := snapshot.toCanonicalMap()
	traceJSON, err := ir.MarshalCanonical(canonicalMap)
	if err != nil {
		return err
	}

	// Compare with golden file using goldie
	g := goldie.New(t,
		goldie.WithFixtureDir("testdata/golden"),
		goldie.WithNameSuffix(".golden"),
	)
	g.Assert(t, scenario.Name, traceJSON)

	return nil
}

// AssertGolden compares the given result's trace against a golden file.
// This is useful when you've already run a scenario and want to compare
// the result against a golden file without re-running.
//
// Parameters:
//   - t: testing.T instance for test assertions
//   - scenarioName: name used for the golden file (without extension)
//   - result: the result from running a scenario
func AssertGolden(t *testing.T, scenarioName string, result *Result) error {
	t.Helper()

	// Build trace snapshot
	snapshot := TraceSnapshot{
		ScenarioName: scenarioName,
		Trace:        result.Trace,
	}

	// Convert to canonical map and marshal
	canonicalMap := snapshot.toCanonicalMap()
	traceJSON, err := ir.MarshalCanonical(canonicalMap)
	if err != nil {
		return err
	}

	// Compare with golden file using goldie
	g := goldie.New(t,
		goldie.WithFixtureDir("testdata/golden"),
		goldie.WithNameSuffix(".golden"),
	)
	g.Assert(t, scenarioName, traceJSON)

	return nil
}
