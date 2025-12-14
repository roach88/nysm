package harness

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/roach88/nysm/internal/ir"
)

// ScenarioNotFoundError is returned when a referenced scenario file doesn't exist.
type ScenarioNotFoundError struct {
	Principle    string
	ScenarioPath string
	ResolvedPath string
}

// Error implements the error interface.
func (e *ScenarioNotFoundError) Error() string {
	return fmt.Sprintf(
		"operational principle %q references scenario file %q which does not exist (resolved to: %s)",
		e.Principle,
		e.ScenarioPath,
		e.ResolvedPath,
	)
}

// ExtractScenarios extracts scenario file paths from an operational principle.
// Supports explicit scenario file references (primary approach).
// Natural language parsing is deferred to future (would be AI-assisted).
//
// If the principle has a Scenario field, it is resolved relative to specDir
// and validated to exist. If no Scenario field is present, an empty slice
// is returned (legacy string format with no scenarios to run).
func ExtractScenarios(principle ir.OperationalPrinciple, specDir string) ([]string, error) {
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

// ValidationResult contains results from validating operational principles.
type ValidationResult struct {
	TotalPrinciples int                `json:"total_principles"`
	TotalScenarios  int                `json:"total_scenarios"`
	Passed          int                `json:"passed"`
	Failed          int                `json:"failed"`
	Skipped         int                `json:"skipped"` // Principles without scenarios
	Failures        []PrincipleFailure `json:"failures,omitempty"`
}

// PrincipleFailure represents a failed operational principle validation.
type PrincipleFailure struct {
	ConceptName  string `json:"concept_name"`
	Principle    string `json:"principle"`
	ScenarioPath string `json:"scenario_path"`
	Error        string `json:"error"`
}

// ValidateOperationalPrinciples validates all operational principles in the given specs.
// Returns a summary of results.
//
// For each concept spec with operational_principles:
// 1. Extract scenario file paths
// 2. Load scenarios
// 3. Run scenarios via harness.Run
// 4. Collect and report results
func ValidateOperationalPrinciples(
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

			// If no scenarios referenced, skip (legacy string format)
			if len(scenarioPaths) == 0 {
				result.Skipped++
				continue
			}

			// Run each scenario
			for _, scenarioPath := range scenarioPaths {
				result.TotalScenarios++

				// Load scenario with base path for relative spec resolution
				scenarioDir := filepath.Dir(scenarioPath)
				scenario, err := LoadScenarioWithBasePath(scenarioPath, scenarioDir)
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
				runResult, err := Run(scenario)
				if err != nil {
					result.Failed++
					result.Failures = append(result.Failures, PrincipleFailure{
						ConceptName:  spec.Name,
						Principle:    principle.Description,
						ScenarioPath: scenarioPath,
						Error:        fmt.Sprintf("scenario execution failed: %v", err),
					})
					continue
				}

				// Check if scenario passed
				if !runResult.Pass {
					result.Failed++
					result.Failures = append(result.Failures, PrincipleFailure{
						ConceptName:  spec.Name,
						Principle:    principle.Description,
						ScenarioPath: scenarioPath,
						Error:        fmt.Sprintf("scenario assertions failed: %v", runResult.Errors),
					})
					continue
				}

				result.Passed++
			}
		}
	}

	return result, nil
}
