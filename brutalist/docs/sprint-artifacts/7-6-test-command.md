# Story 7.6: Test Command

Status: done

## Story

As a **developer running tests**,
I want **a test command that runs the conformance harness**,
So that **I can validate my specs against scenarios**.

## Acceptance Criteria

1. **nysm test command in `internal/cli/test.go`**
   ```go
   // TestCmd represents the test command
   var TestCmd = &cobra.Command{
       Use:   "test [specs-dir] [scenarios-dir]",
       Short: "Run conformance test scenarios",
       Long: `Run conformance test scenarios against compiled specs.

   The test command loads concept specs and sync rules from specs-dir,
   then executes all scenarios from scenarios-dir using the conformance harness.

   Exit codes:
     0 - All tests passed
     1 - One or more tests failed
     2 - Command error (invalid args, file not found, etc.)`,
       Args: cobra.ExactArgs(2),
       RunE: runTest,
   }

   func runTest(cmd *cobra.Command, args []string) error {
       specsDir := args[0]
       scenariosDir := args[1]

       // Load and compile specs
       // Discover scenarios
       // Run harness.Run() for each scenario
       // Report results
       // Return exit code
   }
   ```
   - Takes specs-dir and scenarios-dir as positional arguments
   - Returns appropriate exit code (0 = pass, 1 = fail, 2 = error)

2. **--update flag for golden file regeneration in `internal/cli/test.go`**
   ```go
   var (
       updateGolden bool
       filterPattern string
   )

   func init() {
       TestCmd.Flags().BoolVar(&updateGolden, "update", false,
           "Regenerate golden files instead of comparing")
       TestCmd.Flags().StringVar(&filterPattern, "filter", "",
           "Run only scenarios matching pattern (glob)")
   }
   ```
   - `--update` flag regenerates golden files (passes to harness)
   - Default false (compare mode)

3. **--filter flag for scenario subset in `internal/cli/test.go`**
   ```go
   func discoverScenarios(scenariosDir string, pattern string) ([]string, error) {
       // Find all *.yaml files in scenariosDir
       allScenarios, err := filepath.Glob(filepath.Join(scenariosDir, "*.yaml"))
       if err != nil {
           return nil, err
       }

       // If no filter, return all
       if pattern == "" {
           return allScenarios, nil
       }

       // Filter by pattern
       var filtered []string
       for _, scenario := range allScenarios {
           matched, err := filepath.Match(pattern, filepath.Base(scenario))
           if err != nil {
               return nil, fmt.Errorf("invalid pattern %q: %w", pattern, err)
           }
           if matched {
               filtered = append(filtered, scenario)
           }
       }

       return filtered, nil
   }
   ```
   - `--filter` flag accepts glob pattern
   - Matches against scenario file names
   - Empty filter runs all scenarios

4. **Exit code based on pass/fail in `internal/cli/test.go`**
   ```go
   func runTest(cmd *cobra.Command, args []string) error {
       // ... load specs and scenarios ...

       passedCount := 0
       failedCount := 0

       for _, scenarioPath := range scenarioPaths {
           result, err := harness.Run(scenario)
           if err != nil {
               return fmt.Errorf("scenario %s: execution error: %w",
                   scenarioPath, err)
           }

           if result.Pass {
               passedCount++
           } else {
               failedCount++
           }
       }

       // Print summary
       fmt.Fprintf(cmd.OutOrStdout(), "\n%d/%d passed, %d failed\n",
           passedCount, passedCount+failedCount, failedCount)

       // Exit with appropriate code
       if failedCount > 0 {
           os.Exit(1)  // One or more tests failed
       }

       return nil  // All tests passed (exit 0)
   }
   ```
   - Exit 0 if all tests pass
   - Exit 1 if one or more tests fail
   - Exit 2 if command error (cobra default)

5. **Progress and summary output in `internal/cli/test.go`**
   ```go
   func runTest(cmd *cobra.Command, args []string) error {
       // ... setup ...

       out := cmd.OutOrStdout()

       // Header
       fmt.Fprintf(out, "Running %d scenarios...\n\n", len(scenarioPaths))

       var results []testResult
       passedCount := 0
       failedCount := 0

       for _, scenarioPath := range scenarioPaths {
           scenarioName := filepath.Base(scenarioPath)
           scenarioName = strings.TrimSuffix(scenarioName, filepath.Ext(scenarioName))

           // Load scenario
           scenario, err := harness.LoadScenario(scenarioPath)
           if err != nil {
               return fmt.Errorf("failed to load %s: %w", scenarioPath, err)
           }

           // Run scenario
           start := time.Now()
           result, err := harness.Run(scenario)
           duration := time.Since(start)

           if err != nil {
               return fmt.Errorf("scenario %s: execution error: %w",
                   scenarioName, err)
           }

           // Print result line
           if result.Pass {
               fmt.Fprintf(out, "✓ %s (%.2fs)\n", scenarioName, duration.Seconds())
               passedCount++
           } else {
               fmt.Fprintf(out, "✗ %s (%.2fs)\n", scenarioName, duration.Seconds())
               for _, errMsg := range result.Errors {
                   fmt.Fprintf(out, "    %s\n", errMsg)
               }
               failedCount++
           }

           results = append(results, testResult{
               Name:     scenarioName,
               Pass:     result.Pass,
               Duration: duration,
               Errors:   result.Errors,
           })
       }

       // Summary
       fmt.Fprintf(out, "\n%d/%d passed, %d failed\n",
           passedCount, passedCount+failedCount, failedCount)

       // Exit code
       if failedCount > 0 {
           os.Exit(1)
       }

       return nil
   }

   type testResult struct {
       Name     string
       Pass     bool
       Duration time.Duration
       Errors   []string
   }
   ```
   - Shows progress during execution (one line per scenario)
   - Shows duration for each scenario
   - Shows errors inline for failed tests
   - Shows summary at end

6. **JSON output format with --format=json in `internal/cli/test.go`**
   ```go
   var (
       updateGolden  bool
       filterPattern string
       outputFormat  string
   )

   func init() {
       TestCmd.Flags().BoolVar(&updateGolden, "update", false,
           "Regenerate golden files instead of comparing")
       TestCmd.Flags().StringVar(&filterPattern, "filter", "",
           "Run only scenarios matching pattern (glob)")
       TestCmd.Flags().StringVar(&outputFormat, "format", "text",
           "Output format: text or json")
   }

   type TestOutput struct {
       Status   string        `json:"status"`   // "ok" or "error"
       Passed   int           `json:"passed"`
       Failed   int           `json:"failed"`
       Total    int           `json:"total"`
       Scenarios []ScenarioResult `json:"scenarios"`
   }

   type ScenarioResult struct {
       Name     string   `json:"name"`
       Pass     bool     `json:"pass"`
       Duration float64  `json:"duration_seconds"`
       Errors   []string `json:"errors,omitempty"`
   }

   func outputResults(cmd *cobra.Command, results []testResult,
                     passedCount, failedCount int) error {
       if outputFormat == "json" {
           return outputJSON(cmd.OutOrStdout(), results, passedCount, failedCount)
       }
       return outputText(cmd.OutOrStdout(), results, passedCount, failedCount)
   }

   func outputJSON(w io.Writer, results []testResult,
                  passedCount, failedCount int) error {
       scenarios := make([]ScenarioResult, len(results))
       for i, r := range results {
           scenarios[i] = ScenarioResult{
               Name:     r.Name,
               Pass:     r.Pass,
               Duration: r.Duration.Seconds(),
               Errors:   r.Errors,
           }
       }

       output := TestOutput{
           Status:    "ok",
           Passed:    passedCount,
           Failed:    failedCount,
           Total:     passedCount + failedCount,
           Scenarios: scenarios,
       }

       if failedCount > 0 {
           output.Status = "error"
       }

       enc := json.NewEncoder(w)
       enc.SetIndent("", "  ")
       return enc.Encode(output)
   }
   ```
   - `--format=json` outputs structured JSON
   - Includes all scenario results
   - Includes pass/fail counts and status

## Quick Reference

| Pattern | Key Rule |
|---------|----------|
| **CLI flags** | kebab-case: --update, --filter, --format |
| **Exit codes** | 0 = pass, 1 = fail, 2 = error |
| **FR-6.2** | Run scenarios with assertions on action traces |
| **NFR-4.1** | Actionable error messages |
| **Story 6.2** | harness.Run() provides Result with Pass, Trace, Errors |

## Tasks / Subtasks

- [ ] Task 1: Create test command structure (AC: #1)
  - [ ] 1.1 Create `internal/cli/test.go` with TestCmd
  - [ ] 1.2 Add command to root.go
  - [ ] 1.3 Implement basic runTest() function signature
  - [ ] 1.4 Add positional arguments validation (specs-dir, scenarios-dir)
  - [ ] 1.5 Add command documentation (Use, Short, Long)

- [ ] Task 2: Add CLI flags (AC: #2, #3, #6)
  - [ ] 2.1 Add --update flag for golden file regeneration
  - [ ] 2.2 Add --filter flag for scenario pattern matching
  - [ ] 2.3 Add --format flag for output format (text/json)
  - [ ] 2.4 Document all flags in help text
  - [ ] 2.5 Write tests for flag parsing

- [ ] Task 3: Implement scenario discovery (AC: #3)
  - [ ] 3.1 Implement discoverScenarios() function
  - [ ] 3.2 Find all *.yaml files in scenarios-dir
  - [ ] 3.3 Filter by --filter pattern if provided
  - [ ] 3.4 Return error for invalid pattern
  - [ ] 3.5 Write tests for scenario discovery (all, filtered, invalid pattern)

- [ ] Task 4: Implement spec loading and compilation (AC: #1)
  - [ ] 4.1 Load CUE specs from specs-dir
  - [ ] 4.2 Compile specs to IR (use compiler package)
  - [ ] 4.3 Validate compiled IR
  - [ ] 4.4 Return error with file:line for invalid specs
  - [ ] 4.5 Write tests for spec loading (valid, invalid)

- [ ] Task 5: Implement scenario execution loop (AC: #1, #4, #5)
  - [ ] 5.1 Loop over discovered scenarios
  - [ ] 5.2 Load each scenario (harness.LoadScenario)
  - [ ] 5.3 Run each scenario (harness.Run)
  - [ ] 5.4 Track pass/fail counts
  - [ ] 5.5 Collect results for reporting
  - [ ] 5.6 Write tests for execution loop

- [ ] Task 6: Implement text output format (AC: #5)
  - [ ] 6.1 Print header with scenario count
  - [ ] 6.2 Print progress line per scenario (✓/✗, name, duration)
  - [ ] 6.3 Print errors inline for failed scenarios
  - [ ] 6.4 Print summary at end (passed/failed counts)
  - [ ] 6.5 Write tests for text output

- [ ] Task 7: Implement JSON output format (AC: #6)
  - [ ] 7.1 Define TestOutput and ScenarioResult structs
  - [ ] 7.2 Implement outputJSON() function
  - [ ] 7.3 Marshal results to JSON with indentation
  - [ ] 7.4 Write tests for JSON output

- [ ] Task 8: Implement exit code logic (AC: #4)
  - [ ] 8.1 Exit 0 if all tests pass
  - [ ] 8.2 Exit 1 if one or more tests fail
  - [ ] 8.3 Return error (exit 2) for command errors
  - [ ] 8.4 Write tests for exit codes

- [ ] Task 9: Integration tests (AC: all)
  - [ ] 9.1 Test nysm test with all scenarios passing
  - [ ] 9.2 Test nysm test with one scenario failing
  - [ ] 9.3 Test --filter flag with pattern matching
  - [ ] 9.4 Test --format=json output
  - [ ] 9.5 Test --update flag for golden regeneration
  - [ ] 9.6 Test invalid specs-dir
  - [ ] 9.7 Test invalid scenarios-dir
  - [ ] 9.8 Test invalid --filter pattern

## Dev Notes

### Command Architecture

```
┌─────────────────────────────────────────────────┐
│ nysm test ./specs ./testdata/scenarios          │
│                                                 │
│ Flags:                                          │
│   --update        Regenerate golden files       │
│   --filter GLOB   Run matching scenarios        │
│   --format json   JSON output (default: text)   │
└─────────────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────┐
│ 1. Load and compile specs from specs-dir       │
│    - Load CUE files                             │
│    - Compile to IR                              │
│    - Validate IR                                │
│    - Return error if invalid                    │
└─────────────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────┐
│ 2. Discover scenarios from scenarios-dir       │
│    - Find all *.yaml files                      │
│    - Filter by --filter pattern if provided     │
│    - Return error if no scenarios found         │
└─────────────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────┐
│ 3. Run each scenario                            │
│    For each scenario:                           │
│      - Load scenario YAML                       │
│      - Run harness.Run(scenario)                │
│      - Track pass/fail                          │
│      - Collect errors                           │
│      - Measure duration                         │
└─────────────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────┐
│ 4. Output results                               │
│    Text format (default):                       │
│      Running 5 scenarios...                     │
│      ✓ cart_checkout_success (0.12s)            │
│      ✓ cart_add_existing_item (0.08s)           │
│      ✗ inventory_negative_quantity (0.03s)      │
│          assertion failed: quantity >= 0        │
│      4/5 passed, 1 failed                       │
│                                                 │
│    JSON format (--format=json):                 │
│      {                                          │
│        "status": "error",                       │
│        "passed": 4,                             │
│        "failed": 1,                             │
│        "total": 5,                              │
│        "scenarios": [...]                       │
│      }                                          │
└─────────────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────┐
│ 5. Exit with code                               │
│    0 - All tests passed                         │
│    1 - One or more tests failed                 │
│    2 - Command error (invalid args, etc.)       │
└─────────────────────────────────────────────────┘
```

### Exit Code Semantics

**Exit 0 (Success):**
- All scenarios discovered and executed
- All scenarios passed (Result.Pass = true)
- No errors during execution

**Exit 1 (Test Failure):**
- One or more scenarios failed (Result.Pass = false)
- Scenarios executed successfully, but assertions failed
- This is the normal test failure case

**Exit 2 (Command Error):**
- Invalid arguments (missing specs-dir or scenarios-dir)
- Specs-dir not found or not readable
- Scenarios-dir not found or not readable
- Invalid --filter pattern
- Compilation errors in specs
- Scenario loading errors
- This is a configuration/setup error, not a test failure

**CI Integration:**
```bash
# In CI, test for exit code 0
nysm test ./specs ./testdata/scenarios
if [ $? -ne 0 ]; then
    echo "Tests failed"
    exit 1
fi

# With JSON output for parsing
nysm test ./specs ./testdata/scenarios --format=json > results.json
if [ $? -ne 0 ]; then
    cat results.json | jq '.scenarios[] | select(.pass == false)'
    exit 1
fi
```

### Filter Pattern Examples

```bash
# Run all scenarios
nysm test ./specs ./testdata/scenarios

# Run only checkout scenarios
nysm test ./specs ./testdata/scenarios --filter "checkout*"

# Run only success scenarios
nysm test ./specs ./testdata/scenarios --filter "*_success.yaml"

# Run only cart scenarios
nysm test ./specs ./testdata/scenarios --filter "cart_*"

# Invalid pattern (exit 2)
nysm test ./specs ./testdata/scenarios --filter "["
```

### Golden File Workflow

**Without --update (default):**
```bash
# Compare mode - compare trace to existing golden file
nysm test ./specs ./testdata/scenarios

# If golden file doesn't match, test fails:
# ✗ cart_checkout_success (0.12s)
#     golden file mismatch: testdata/golden/cart_checkout_success.golden
#     expected: [trace A]
#     got:      [trace B]
```

**With --update:**
```bash
# Regenerate mode - write new golden files
nysm test ./specs ./testdata/scenarios --update

# Output:
# Running 5 scenarios...
# ✓ cart_checkout_success (0.12s) [golden updated]
# ✓ cart_add_existing_item (0.08s) [golden updated]
# ...
# 5/5 passed, 0 failed
# 5 golden files updated
```

### Scenario Discovery Implementation

```go
// internal/cli/test.go

// discoverScenarios finds all scenario files matching the filter pattern.
//
// Pattern matching uses filepath.Match glob syntax:
//   - * matches any sequence of non-separator characters
//   - ? matches any single non-separator character
//   - [range] matches character range
//
// Examples:
//   ""             matches all *.yaml files
//   "cart_*"       matches cart_checkout.yaml, cart_add_item.yaml
//   "*_success"    matches checkout_success.yaml, reserve_success.yaml
func discoverScenarios(scenariosDir string, pattern string) ([]string, error) {
    // Find all *.yaml files
    allScenarios, err := filepath.Glob(filepath.Join(scenariosDir, "*.yaml"))
    if err != nil {
        return nil, fmt.Errorf("failed to glob scenarios: %w", err)
    }

    if len(allScenarios) == 0 {
        return nil, fmt.Errorf("no scenario files found in %s", scenariosDir)
    }

    // If no filter, return all
    if pattern == "" {
        return allScenarios, nil
    }

    // Filter by pattern
    var filtered []string
    for _, scenarioPath := range allScenarios {
        scenarioName := filepath.Base(scenarioPath)
        matched, err := filepath.Match(pattern, scenarioName)
        if err != nil {
            return nil, fmt.Errorf("invalid filter pattern %q: %w", pattern, err)
        }
        if matched {
            filtered = append(filtered, scenarioPath)
        }
    }

    if len(filtered) == 0 {
        return nil, fmt.Errorf("no scenarios matched filter %q", pattern)
    }

    return filtered, nil
}
```

### Output Format Implementation

```go
// internal/cli/test.go

// outputText prints test results in human-readable format.
func outputText(w io.Writer, results []testResult, passedCount, failedCount int) {
    for _, r := range results {
        if r.Pass {
            fmt.Fprintf(w, "✓ %s (%.2fs)\n", r.Name, r.Duration.Seconds())
        } else {
            fmt.Fprintf(w, "✗ %s (%.2fs)\n", r.Name, r.Duration.Seconds())
            for _, errMsg := range r.Errors {
                fmt.Fprintf(w, "    %s\n", errMsg)
            }
        }
    }

    fmt.Fprintf(w, "\n%d/%d passed, %d failed\n",
        passedCount, passedCount+failedCount, failedCount)
}

// outputJSON prints test results in JSON format.
func outputJSON(w io.Writer, results []testResult, passedCount, failedCount int) error {
    scenarios := make([]ScenarioResult, len(results))
    for i, r := range results {
        scenarios[i] = ScenarioResult{
            Name:     r.Name,
            Pass:     r.Pass,
            Duration: r.Duration.Seconds(),
            Errors:   r.Errors,
        }
    }

    output := TestOutput{
        Status:    "ok",
        Passed:    passedCount,
        Failed:    failedCount,
        Total:     passedCount + failedCount,
        Scenarios: scenarios,
    }

    if failedCount > 0 {
        output.Status = "error"
    }

    enc := json.NewEncoder(w)
    enc.SetIndent("", "  ")
    return enc.Encode(output)
}
```

## Test Examples

### Example 1: All scenarios passing

```go
// internal/cli/test_test.go
package cli

import (
    "bytes"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestTestCmd_AllPass(t *testing.T) {
    // Setup test fixtures
    specsDir := "testdata/fixtures/specs"
    scenariosDir := "testdata/fixtures/scenarios/all_pass"

    // Run command
    cmd := TestCmd
    cmd.SetArgs([]string{specsDir, scenariosDir})

    var out bytes.Buffer
    cmd.SetOut(&out)

    err := cmd.Execute()
    require.NoError(t, err)

    // Verify output
    output := out.String()
    assert.Contains(t, output, "Running 3 scenarios...")
    assert.Contains(t, output, "✓ scenario_1")
    assert.Contains(t, output, "✓ scenario_2")
    assert.Contains(t, output, "✓ scenario_3")
    assert.Contains(t, output, "3/3 passed, 0 failed")
}
```

### Example 2: One scenario failing

```go
func TestTestCmd_OneFail(t *testing.T) {
    // Setup test fixtures
    specsDir := "testdata/fixtures/specs"
    scenariosDir := "testdata/fixtures/scenarios/one_fail"

    // Run command
    cmd := TestCmd
    cmd.SetArgs([]string{specsDir, scenariosDir})

    var out bytes.Buffer
    cmd.SetOut(&out)

    err := cmd.Execute()
    // Command should succeed (exit 0) but test should fail (exit 1)
    // In test, we can't check os.Exit, so we check the output
    require.NoError(t, err) // runTest returns nil, but calls os.Exit(1)

    // Verify output
    output := out.String()
    assert.Contains(t, output, "Running 3 scenarios...")
    assert.Contains(t, output, "✓ scenario_1")
    assert.Contains(t, output, "✗ scenario_2")
    assert.Contains(t, output, "assertion failed")
    assert.Contains(t, output, "✓ scenario_3")
    assert.Contains(t, output, "2/3 passed, 1 failed")
}
```

### Example 3: Filter by pattern

```go
func TestTestCmd_Filter(t *testing.T) {
    // Setup test fixtures
    specsDir := "testdata/fixtures/specs"
    scenariosDir := "testdata/fixtures/scenarios/mixed"

    // Run command with filter
    cmd := TestCmd
    cmd.SetArgs([]string{specsDir, scenariosDir, "--filter", "cart_*"})

    var out bytes.Buffer
    cmd.SetOut(&out)

    err := cmd.Execute()
    require.NoError(t, err)

    // Verify output
    output := out.String()
    assert.Contains(t, output, "Running 2 scenarios...")
    assert.Contains(t, output, "✓ cart_checkout")
    assert.Contains(t, output, "✓ cart_add_item")
    assert.NotContains(t, output, "inventory_reserve")
    assert.Contains(t, output, "2/2 passed, 0 failed")
}
```

### Example 4: JSON output

```go
func TestTestCmd_JSONOutput(t *testing.T) {
    // Setup test fixtures
    specsDir := "testdata/fixtures/specs"
    scenariosDir := "testdata/fixtures/scenarios/all_pass"

    // Run command with JSON output
    cmd := TestCmd
    cmd.SetArgs([]string{specsDir, scenariosDir, "--format", "json"})

    var out bytes.Buffer
    cmd.SetOut(&out)

    err := cmd.Execute()
    require.NoError(t, err)

    // Verify JSON output
    var result TestOutput
    err = json.Unmarshal(out.Bytes(), &result)
    require.NoError(t, err)

    assert.Equal(t, "ok", result.Status)
    assert.Equal(t, 3, result.Passed)
    assert.Equal(t, 0, result.Failed)
    assert.Equal(t, 3, result.Total)
    assert.Len(t, result.Scenarios, 3)

    // Verify each scenario
    for _, scenario := range result.Scenarios {
        assert.True(t, scenario.Pass)
        assert.Greater(t, scenario.Duration, 0.0)
        assert.Empty(t, scenario.Errors)
    }
}
```

### Example 5: Invalid specs-dir

```go
func TestTestCmd_InvalidSpecsDir(t *testing.T) {
    // Run command with non-existent specs-dir
    cmd := TestCmd
    cmd.SetArgs([]string{"/nonexistent/specs", "testdata/scenarios"})

    var out bytes.Buffer
    cmd.SetOut(&out)

    err := cmd.Execute()
    require.Error(t, err)
    assert.Contains(t, err.Error(), "specs-dir not found")
}
```

### Example 6: Invalid filter pattern

```go
func TestTestCmd_InvalidFilterPattern(t *testing.T) {
    // Setup test fixtures
    specsDir := "testdata/fixtures/specs"
    scenariosDir := "testdata/fixtures/scenarios/all_pass"

    // Run command with invalid filter pattern
    cmd := TestCmd
    cmd.SetArgs([]string{specsDir, scenariosDir, "--filter", "["})

    var out bytes.Buffer
    cmd.SetOut(&out)

    err := cmd.Execute()
    require.Error(t, err)
    assert.Contains(t, err.Error(), "invalid filter pattern")
}
```

## File List

Files to create:

1. `internal/cli/test.go` - TestCmd with runTest(), discoverScenarios(), outputText(), outputJSON()
2. `internal/cli/test_test.go` - Tests for all scenarios (pass, fail, filter, JSON, errors)
3. `testdata/fixtures/scenarios/all_pass/` - Test scenarios that all pass
4. `testdata/fixtures/scenarios/one_fail/` - Test scenarios with one failure
5. `testdata/fixtures/scenarios/mixed/` - Test scenarios with multiple types

## Relationship to Other Stories

**Dependencies:**
- Story 1.1 (Project Initialization & IR Type Definitions) - Required for ir.* types
- Story 1.6 (CUE Concept Spec Parser) - Required for spec compilation
- Story 1.7 (CUE Sync Rule Parser) - Required for sync rule compilation
- Story 6.1 (Scenario Definition Format) - Required for scenario loading
- Story 6.2 (Test Execution Engine) - Required for harness.Run()
- Story 6.6 (Golden Trace Snapshots) - Required for --update flag functionality
- Story 7.1 (CLI Framework Setup) - Required for cobra commands

**Enables:**
- Story 7.10 (Demo Scenarios and Golden Traces) - Uses nysm test to run demo
- CI/CD pipeline - Exit codes enable automated testing
- Developer workflow - Run tests during development

**Partial Dependencies:**
- Story 7.2 (Compile Command) - Can share spec loading/compilation logic

## Story Completion Checklist

- [ ] `internal/cli/test.go` created with TestCmd
- [ ] TestCmd added to root command
- [ ] runTest() function implemented with spec loading and compilation
- [ ] discoverScenarios() function implemented with glob matching
- [ ] --update flag for golden file regeneration
- [ ] --filter flag for scenario pattern matching
- [ ] --format flag for output format (text/json)
- [ ] Exit code logic (0 = pass, 1 = fail, 2 = error)
- [ ] Text output format (progress, summary, errors)
- [ ] JSON output format (structured results)
- [ ] All tests pass (`go test ./internal/cli/...`)
- [ ] `go build ./cmd/nysm` succeeds
- [ ] `go vet ./internal/cli` passes
- [ ] Test: nysm test with all scenarios passing (exit 0)
- [ ] Test: nysm test with one scenario failing (exit 1)
- [ ] Test: --filter flag with pattern matching
- [ ] Test: --format=json output
- [ ] Test: --update flag for golden regeneration
- [ ] Test: Invalid specs-dir (exit 2)
- [ ] Test: Invalid scenarios-dir (exit 2)
- [ ] Test: Invalid --filter pattern (exit 2)
- [ ] Test: No scenarios found (exit 2)
- [ ] Manual test: Run nysm test on demo scenarios

## References

- [Source: docs/architecture.md#CLI commands] - CLI structure and patterns
- [Source: docs/architecture.md#CLI flags] - Flag naming conventions (kebab-case)
- [Source: docs/architecture.md#CLI output structure] - Output format patterns
- [Source: docs/epics.md#Story 7.6] - Story definition and acceptance criteria
- [Source: docs/prd.md#FR-6.2] - Run scenarios with assertions on action traces
- [Source: docs/prd.md#NFR-4.1] - Actionable error messages
- [Source: docs/sprint-artifacts/6-2-test-execution-engine.md] - harness.Run() signature and Result struct
- [Source: docs/sprint-artifacts/6-6-golden-trace-snapshots.md] - Golden file comparison and --update flag

## Dev Agent Record

### Agent Model Used

Claude Sonnet 4.5 (claude-sonnet-4-5-20250929)

### Validation History

- Initial creation: 2025-12-12

### Completion Notes

- nysm test command is the primary way developers run conformance tests
- Exit codes (0/1/2) enable CI/CD integration
- --filter flag enables selective test execution (TDD workflow)
- --update flag regenerates golden files (after intentional changes)
- --format=json enables programmatic result parsing
- Text output is human-readable with progress and summary
- JSON output is machine-readable for CI/CD parsing
- Scenario discovery uses glob matching (filepath.Match)
- Spec loading and compilation shared with compile command (future refactor)
- Error messages are actionable (file paths, line numbers, assertion details)
- Progress output shows duration per scenario (performance tracking)
- Summary shows pass/fail counts (quick assessment)
- Exit 2 for command errors (invalid args, missing files) vs exit 1 for test failures
- Integration with harness package (Story 6.2) for scenario execution
- Supports golden trace snapshots (Story 6.6) via --update flag
