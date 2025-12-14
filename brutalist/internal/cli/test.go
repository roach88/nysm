package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/roach88/nysm/internal/harness"
	"github.com/roach88/nysm/internal/ir"
)

// TestOptions holds flags for the test command.
type TestOptions struct {
	*RootOptions
	Update bool   // regenerate golden files
	Filter string // scenario filter (glob pattern)
}

// ScenarioResult holds the result of a single scenario execution.
type ScenarioResult struct {
	Name   string   `json:"name"`
	Pass   bool     `json:"pass"`
	Errors []string `json:"errors,omitempty"`
}

// TestResult holds the overall test result.
type TestResult struct {
	Scenarios []ScenarioResult `json:"scenarios"`
	Passed    int              `json:"passed"`
	Failed    int              `json:"failed"`
	Total     int              `json:"total"`
}

// NewTestCommand creates the test command.
func NewTestCommand(rootOpts *RootOptions) *cobra.Command {
	opts := &TestOptions{RootOptions: rootOpts}

	cmd := &cobra.Command{
		Use:   "test <specs-dir> <scenarios-dir>",
		Short: "Run conformance harness",
		Long: `Run conformance tests using the harness framework.

Executes scenario files against the specs, validating trace output
and final state assertions. Supports golden file comparison.

Exit codes:
  0 - All scenarios passed
  1 - One or more scenarios failed
  2 - Command error (invalid paths, etc.)

Examples:
  nysm test ./specs ./scenarios
  nysm test ./specs ./scenarios --filter "cart-*"
  nysm test ./specs ./scenarios --update
  nysm test ./specs ./scenarios --format json`,
		Args:          cobra.ExactArgs(2),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTests(opts, args[0], args[1], cmd)
		},
	}

	cmd.Flags().BoolVar(&opts.Update, "update", false, "regenerate golden files")
	cmd.Flags().StringVar(&opts.Filter, "filter", "", "filter scenarios by glob pattern")

	return cmd
}

func runTests(opts *TestOptions, specsDir, scenariosDir string, cmd *cobra.Command) error {
	// Validate directories
	if _, err := os.Stat(specsDir); os.IsNotExist(err) {
		return NewExitError(ExitCommandError, fmt.Sprintf("specs directory not found: %s", specsDir))
	}
	if _, err := os.Stat(scenariosDir); os.IsNotExist(err) {
		return NewExitError(ExitCommandError, fmt.Sprintf("scenarios directory not found: %s", scenariosDir))
	}

	// Find scenario files
	scenarioFiles, err := findScenarioFiles(scenariosDir, opts.Filter)
	if err != nil {
		return fmt.Errorf("failed to find scenarios: %w", err)
	}

	if len(scenarioFiles) == 0 {
		if opts.Format == "json" {
			return outputTestJSON(cmd, TestResult{
				Scenarios: []ScenarioResult{},
				Total:     0,
			})
		}
		fmt.Fprintln(cmd.OutOrStdout(), "No scenarios found.")
		return nil
	}

	// Run scenarios
	result := TestResult{
		Scenarios: make([]ScenarioResult, 0, len(scenarioFiles)),
		Total:     len(scenarioFiles),
	}

	for _, scenarioFile := range scenarioFiles {
		scenResult := runScenario(scenarioFile, specsDir, opts, cmd)
		result.Scenarios = append(result.Scenarios, scenResult)

		if scenResult.Pass {
			result.Passed++
		} else {
			result.Failed++
		}
	}

	// Output results
	if opts.Format == "json" {
		return outputTestJSON(cmd, result)
	}

	return outputTestText(cmd, result)
}

// findScenarioFiles finds all YAML scenario files in a directory.
func findScenarioFiles(dir string, filter string) ([]string, error) {
	var files []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Only process .yaml and .yml files
		ext := filepath.Ext(path)
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}

		// Apply filter if specified
		if filter != "" {
			base := filepath.Base(path)
			name := strings.TrimSuffix(base, ext)
			matched, err := filepath.Match(filter, name)
			if err != nil {
				return fmt.Errorf("invalid filter pattern: %w", err)
			}
			if !matched {
				return nil
			}
		}

		files = append(files, path)
		return nil
	})

	return files, err
}

// runScenario executes a single scenario and returns the result.
func runScenario(scenarioFile string, specsDir string, opts *TestOptions, cmd *cobra.Command) ScenarioResult {
	w := cmd.OutOrStdout()

	// Load scenario with specs dir as base path for relative spec references
	scenario, err := harness.LoadScenarioWithBasePath(scenarioFile, specsDir)
	if err != nil {
		if opts.Format != "json" {
			fmt.Fprintf(w, "✗ %s\n", filepath.Base(scenarioFile))
			fmt.Fprintf(w, "  Load error: %v\n", err)
		}
		return ScenarioResult{
			Name:   filepath.Base(scenarioFile),
			Pass:   false,
			Errors: []string{fmt.Sprintf("failed to load scenario: %v", err)},
		}
	}

	// Run scenario
	result, err := harness.Run(scenario)
	if err != nil {
		if opts.Format != "json" {
			fmt.Fprintf(w, "✗ %s\n", scenario.Name)
			fmt.Fprintf(w, "  Execution error: %v\n", err)
		}
		return ScenarioResult{
			Name:   scenario.Name,
			Pass:   false,
			Errors: []string{fmt.Sprintf("execution failed: %v", err)},
		}
	}

	// Handle golden file comparison
	if opts.Update {
		// Update golden file
		if err := updateGoldenFile(scenario, result, scenarioFile); err != nil {
			if opts.Format != "json" {
				fmt.Fprintf(w, "✗ %s\n", scenario.Name)
				fmt.Fprintf(w, "  Golden update error: %v\n", err)
			}
			return ScenarioResult{
				Name:   scenario.Name,
				Pass:   false,
				Errors: []string{fmt.Sprintf("failed to update golden file: %v", err)},
			}
		}
		if opts.Format != "json" {
			fmt.Fprintf(w, "✓ %s (golden updated)\n", scenario.Name)
		}
		return ScenarioResult{
			Name: scenario.Name,
			Pass: true,
		}
	}

	// Compare against golden file
	goldenPath := goldenFilePath(scenarioFile)
	if _, err := os.Stat(goldenPath); os.IsNotExist(err) {
		// No golden file - use assertion-based validation only
		if result.Pass {
			if opts.Format != "json" {
				fmt.Fprintf(w, "✓ %s\n", scenario.Name)
			}
			return ScenarioResult{
				Name: scenario.Name,
				Pass: true,
			}
		}

		if opts.Format != "json" {
			fmt.Fprintf(w, "✗ %s\n", scenario.Name)
			for _, e := range result.Errors {
				fmt.Fprintf(w, "  %s\n", e)
			}
		}
		return ScenarioResult{
			Name:   scenario.Name,
			Pass:   false,
			Errors: result.Errors,
		}
	}

	// Compare with golden file
	match, err := compareWithGolden(scenario, result, goldenPath)
	if err != nil {
		if opts.Format != "json" {
			fmt.Fprintf(w, "✗ %s\n", scenario.Name)
			fmt.Fprintf(w, "  Golden comparison error: %v\n", err)
		}
		return ScenarioResult{
			Name:   scenario.Name,
			Pass:   false,
			Errors: []string{fmt.Sprintf("golden comparison failed: %v", err)},
		}
	}

	if !match {
		if opts.Format != "json" {
			fmt.Fprintf(w, "✗ %s\n", scenario.Name)
			fmt.Fprintln(w, "  Golden file mismatch (run with --update to regenerate)")
		}
		return ScenarioResult{
			Name:   scenario.Name,
			Pass:   false,
			Errors: []string{"trace does not match golden file"},
		}
	}

	// Both assertions and golden match
	if result.Pass {
		if opts.Format != "json" {
			fmt.Fprintf(w, "✓ %s\n", scenario.Name)
		}
		return ScenarioResult{
			Name: scenario.Name,
			Pass: true,
		}
	}

	if opts.Format != "json" {
		fmt.Fprintf(w, "✗ %s\n", scenario.Name)
		for _, e := range result.Errors {
			fmt.Fprintf(w, "  %s\n", e)
		}
	}
	return ScenarioResult{
		Name:   scenario.Name,
		Pass:   false,
		Errors: result.Errors,
	}
}

// goldenFilePath returns the path to the golden file for a scenario.
func goldenFilePath(scenarioFile string) string {
	dir := filepath.Dir(scenarioFile)
	base := filepath.Base(scenarioFile)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	return filepath.Join(dir, "golden", name+".golden")
}

// updateGoldenFile writes the current trace as the golden file.
func updateGoldenFile(scenario *harness.Scenario, result *harness.Result, scenarioFile string) error {
	goldenPath := goldenFilePath(scenarioFile)

	// Ensure golden directory exists
	goldenDir := filepath.Dir(goldenPath)
	if err := os.MkdirAll(goldenDir, 0755); err != nil {
		return fmt.Errorf("failed to create golden directory: %w", err)
	}

	// Build trace snapshot
	snapshot := map[string]any{
		"scenario_name": scenario.Name,
		"trace":         convertTraceToCanonical(result.Trace),
	}
	if scenario.FlowToken != "" {
		snapshot["flow_token"] = scenario.FlowToken
	}

	// Marshal to canonical JSON
	data, err := ir.MarshalCanonical(snapshot)
	if err != nil {
		return fmt.Errorf("failed to marshal trace: %w", err)
	}

	// Write golden file
	if err := os.WriteFile(goldenPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write golden file: %w", err)
	}

	return nil
}

// compareWithGolden compares the result trace against the golden file.
func compareWithGolden(scenario *harness.Scenario, result *harness.Result, goldenPath string) (bool, error) {
	// Read golden file
	goldenData, err := os.ReadFile(goldenPath)
	if err != nil {
		return false, fmt.Errorf("failed to read golden file: %w", err)
	}

	// Build current trace snapshot
	snapshot := map[string]any{
		"scenario_name": scenario.Name,
		"trace":         convertTraceToCanonical(result.Trace),
	}
	if scenario.FlowToken != "" {
		snapshot["flow_token"] = scenario.FlowToken
	}

	// Marshal current trace to canonical JSON
	currentData, err := ir.MarshalCanonical(snapshot)
	if err != nil {
		return false, fmt.Errorf("failed to marshal current trace: %w", err)
	}

	// Compare bytes
	return string(goldenData) == string(currentData), nil
}

// convertTraceToCanonical converts trace events to a format suitable for canonical JSON.
func convertTraceToCanonical(trace []harness.TraceEvent) []any {
	result := make([]any, len(trace))
	for i, event := range trace {
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
		result[i] = eventMap
	}
	return result
}

// outputTestJSON outputs the test result as JSON.
func outputTestJSON(cmd *cobra.Command, result TestResult) error {
	status := "ok"
	if result.Failed > 0 {
		status = "error"
	}

	response := CLIResponse{
		Status: status,
		Data:   result,
	}

	if result.Failed > 0 {
		response.Error = &CLIError{
			Code:    "E_TEST_FAILED",
			Message: fmt.Sprintf("%d scenario(s) failed", result.Failed),
		}
	}

	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(response); err != nil {
		return err
	}

	if result.Failed > 0 {
		// Test failures = exit code 1
		return NewExitError(ExitFailure, fmt.Sprintf("%d scenario(s) failed", result.Failed))
	}
	return nil
}

// outputTestText outputs the test result as text.
func outputTestText(cmd *cobra.Command, result TestResult) error {
	w := cmd.OutOrStdout()

	fmt.Fprintln(w)
	fmt.Fprintf(w, "Test Summary: %d passed, %d failed, %d total\n", result.Passed, result.Failed, result.Total)

	if result.Failed > 0 {
		// Test failures = exit code 1
		return NewExitError(ExitFailure, fmt.Sprintf("%d scenario(s) failed", result.Failed))
	}

	fmt.Fprintln(w, "✓ All scenarios passed")
	return nil
}
