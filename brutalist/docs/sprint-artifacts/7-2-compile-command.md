# Story 7.2: Compile Command

Status: ready-for-dev

## Story

As a **developer building NYSM apps**,
I want **a compile command that produces canonical IR**,
So that **I can see what my specs compile to**.

## Acceptance Criteria

1. **`nysm compile ./specs` command exists**
   - Takes a directory path containing CUE spec files
   - Compiles all concept and sync specs to canonical IR
   - Returns exit code 0 on success, 1 on errors

2. **Human-readable output by default**
   ```bash
   $ nysm compile ./specs
   ✓ Compiled 3 concepts, 2 syncs

   Concepts:
     Cart: 3 actions, 1 operational principle
     Inventory: 2 actions, 1 operational principle
     Web: 2 actions

   Syncs:
     cart-inventory: Cart.checkout → Inventory.reserve
     inventory-response: Inventory.reserve → Web.respond
   ```

3. **JSON output with `--format json` flag**
   ```bash
   $ nysm compile ./specs --format json
   {
     "status": "ok",
     "data": {
       "concepts": [...],
       "syncs": [...]
     }
   }
   ```

4. **Validation errors reported with file:line references**
   ```bash
   $ nysm compile ./specs-with-errors
   ✗ Compilation failed

   specs/cart.concept.cue:15:3
     E101: Missing required field: purpose

   specs/cart-inventory.sync.cue:8:5
     E102: Unknown action reference: Cart.invalid
   ```

5. **`--output <file>` flag writes IR to file**
   ```bash
   $ nysm compile ./specs --output compiled.json
   ✓ Compiled 3 concepts, 2 syncs
   Wrote canonical IR to compiled.json
   ```

6. **Summary statistics displayed**
   - Count of concepts compiled
   - Count of syncs compiled
   - Total actions across all concepts
   - Total operational principles

## Quick Reference

| Pattern | Key Rule |
|---------|----------|
| **CLI Flags** | kebab-case (--format, --output) |
| **Error Codes** | E001-E099 compilation, E100-E199 validation |
| **Output Format** | Human by default, JSON with --format json |
| **Exit Codes** | 0 = success, 1 = errors |

## Tasks / Subtasks

- [ ] Task 1: Create CLI command structure (AC: #1)
  - [ ] 1.1 Create `internal/cli/compile.go`
  - [ ] 1.2 Define `compileCmd` using Cobra
  - [ ] 1.3 Register command with root CLI
  - [ ] 1.4 Add `--format` flag (json|text, default: text)
  - [ ] 1.5 Add `--output` flag (optional file path)

- [ ] Task 2: Implement spec discovery (AC: #1)
  - [ ] 2.1 Walk directory to find .cue files
  - [ ] 2.2 Categorize files as concept vs sync specs
  - [ ] 2.3 Load CUE instances using load.Instances

- [ ] Task 3: Implement compilation orchestration (AC: #1, #6)
  - [ ] 3.1 Compile all concept specs using compiler.CompileConcept
  - [ ] 3.2 Compile all sync specs using compiler.CompileSync
  - [ ] 3.3 Collect compilation errors
  - [ ] 3.4 Generate summary statistics
  - [ ] 3.5 Build compiled IR structure

- [ ] Task 4: Implement human-readable output (AC: #2)
  - [ ] 4.1 Format success output with checkmark
  - [ ] 4.2 Display concept summary (name, action count, OP count)
  - [ ] 4.3 Display sync summary (name, when → then flow)
  - [ ] 4.4 Use color for success/error (optional, graceful if no TTY)

- [ ] Task 5: Implement JSON output (AC: #3)
  - [ ] 5.1 Define CLIResponse struct with status, data, error
  - [ ] 5.2 Marshal compiled IR to canonical JSON
  - [ ] 5.3 Write JSON to stdout

- [ ] Task 6: Implement error reporting (AC: #4)
  - [ ] 6.1 Format CompileError with file:line:col
  - [ ] 6.2 Assign error codes (E001-E199)
  - [ ] 6.3 Collect all errors before reporting (not fail-fast)
  - [ ] 6.4 Display errors with context

- [ ] Task 7: Implement file output (AC: #5)
  - [ ] 7.1 Check if --output flag provided
  - [ ] 7.2 Write canonical IR to file
  - [ ] 7.3 Confirm file write in output

- [ ] Task 8: Write comprehensive tests
  - [ ] 8.1 Test valid spec compilation
  - [ ] 8.2 Test --format json output
  - [ ] 8.3 Test --output file writing
  - [ ] 8.4 Test error reporting with invalid specs
  - [ ] 8.5 Test multiple concept and sync files
  - [ ] 8.6 Test empty directory handling

## Dev Notes

### Command Implementation

```go
// internal/cli/compile.go
package cli

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"

    "cuelang.org/go/cue/cuecontext"
    "cuelang.org/go/cue/load"
    "github.com/spf13/cobra"

    "github.com/tyler/nysm/internal/compiler"
    "github.com/tyler/nysm/internal/ir"
)

var (
    compileFormat string
    compileOutput string
)

var compileCmd = &cobra.Command{
    Use:   "compile <specs-dir>",
    Short: "Compile CUE specs to canonical IR",
    Long: `Compile concept and sync specs from CUE format to canonical JSON IR.

By default, displays human-readable summary. Use --format json for machine-readable output.`,
    Args: cobra.ExactArgs(1),
    RunE: runCompile,
}

func init() {
    compileCmd.Flags().StringVar(&compileFormat, "format", "text", "Output format (text|json)")
    compileCmd.Flags().StringVar(&compileOutput, "output", "", "Write compiled IR to file")
}

type CompilationResult struct {
    Concepts []ir.ConceptSpec `json:"concepts"`
    Syncs    []ir.SyncRule    `json:"syncs"`
}

func runCompile(cmd *cobra.Command, args []string) error {
    specsDir := args[0]

    // Verify directory exists
    if _, err := os.Stat(specsDir); os.IsNotExist(err) {
        return fmt.Errorf("specs directory not found: %s", specsDir)
    }

    // Load CUE instances
    ctx := cuecontext.New()
    instances := load.Instances([]string{specsDir}, &load.Config{
        Dir: specsDir,
    })

    if len(instances) == 0 {
        return fmt.Errorf("no CUE files found in %s", specsDir)
    }

    // Check for load errors
    for _, inst := range instances {
        if inst.Err != nil {
            return fmt.Errorf("loading CUE files: %w", inst.Err)
        }
    }

    // Build value from instances
    value := ctx.BuildInstance(instances[0])
    if err := value.Err(); err != nil {
        return formatCUEError(err)
    }

    // Compile concepts and syncs
    result, compileErrors := compileAll(value)

    if len(compileErrors) > 0 {
        // Format and display all errors
        if compileFormat == "json" {
            return outputJSONError(compileErrors)
        }
        return outputTextErrors(compileErrors)
    }

    // Output results
    if compileFormat == "json" {
        return outputJSON(result)
    }

    return outputText(result)
}

func compileAll(value cue.Value) (*CompilationResult, []error) {
    result := &CompilationResult{
        Concepts: []ir.ConceptSpec{},
        Syncs:    []ir.SyncRule{},
    }
    var errors []error

    // Compile concepts
    conceptsIter, err := value.LookupPath(cue.ParsePath("concept")).Fields()
    if err == nil {
        for conceptsIter.Next() {
            conceptVal := conceptsIter.Value()
            spec, err := compiler.CompileConcept(conceptVal)
            if err != nil {
                errors = append(errors, err)
                continue
            }
            result.Concepts = append(result.Concepts, *spec)
        }
    }

    // Compile syncs
    syncsIter, err := value.LookupPath(cue.ParsePath("sync")).Fields()
    if err == nil {
        for syncsIter.Next() {
            syncVal := syncsIter.Value()
            syncRule, err := compiler.CompileSync(syncVal)
            if err != nil {
                errors = append(errors, err)
                continue
            }
            result.Syncs = append(result.Syncs, *syncRule)
        }
    }

    return result, errors
}

func outputText(result *CompilationResult) error {
    fmt.Printf("✓ Compiled %d concepts, %d syncs\n\n",
        len(result.Concepts), len(result.Syncs))

    if len(result.Concepts) > 0 {
        fmt.Println("Concepts:")
        for _, concept := range result.Concepts {
            opCount := 0
            if concept.OperationalPrinciple != "" {
                opCount = 1
            }
            fmt.Printf("  %s: %d actions, %d operational principle\n",
                concept.Name, len(concept.Actions), opCount)
        }
        fmt.Println()
    }

    if len(result.Syncs) > 0 {
        fmt.Println("Syncs:")
        for _, sync := range result.Syncs {
            whenAction := formatActionRef(sync.When.Action)
            thenAction := formatActionRef(sync.Then.Action)
            fmt.Printf("  %s: %s → %s\n", sync.ID, whenAction, thenAction)
        }
        fmt.Println()
    }

    // Write to file if --output specified
    if compileOutput != "" {
        if err := writeIRToFile(result, compileOutput); err != nil {
            return err
        }
        fmt.Printf("Wrote canonical IR to %s\n", compileOutput)
    }

    return nil
}

func outputJSON(result *CompilationResult) error {
    response := CLIResponse{
        Status: "ok",
        Data:   result,
    }

    encoder := json.NewEncoder(os.Stdout)
    encoder.SetIndent("", "  ")
    return encoder.Encode(response)
}

func outputTextErrors(errors []error) error {
    fmt.Println("✗ Compilation failed\n")

    for _, err := range errors {
        if compileErr, ok := err.(*compiler.CompileError); ok {
            if compileErr.Pos.IsValid() {
                fmt.Printf("%s:%d:%d\n  %s: %s\n\n",
                    compileErr.Pos.Filename(),
                    compileErr.Pos.Line(),
                    compileErr.Pos.Column(),
                    getErrorCode(compileErr),
                    compileErr.Message)
            } else {
                fmt.Printf("%s: %s\n\n",
                    getErrorCode(compileErr),
                    compileErr.Message)
            }
        } else {
            fmt.Printf("ERROR: %v\n\n", err)
        }
    }

    return fmt.Errorf("compilation failed with %d error(s)", len(errors))
}

func outputJSONError(errors []error) error {
    cliErrors := make([]CLIError, len(errors))
    for i, err := range errors {
        cliErrors[i] = CLIError{
            Code:    getErrorCodeFromError(err),
            Message: err.Error(),
        }
    }

    response := CLIResponse{
        Status: "error",
        Error:  &cliErrors[0], // Primary error
    }

    encoder := json.NewEncoder(os.Stdout)
    encoder.SetIndent("", "  ")
    if err := encoder.Encode(response); err != nil {
        return err
    }

    return fmt.Errorf("compilation failed")
}

func writeIRToFile(result *CompilationResult, filename string) error {
    // Use canonical JSON marshaling
    data, err := ir.MarshalCanonical(result)
    if err != nil {
        return fmt.Errorf("marshaling IR: %w", err)
    }

    if err := os.WriteFile(filename, data, 0644); err != nil {
        return fmt.Errorf("writing file: %w", err)
    }

    return nil
}

func formatActionRef(ref ir.ActionRef) string {
    // ActionRef format: "Concept.action"
    return fmt.Sprintf("%s.%s", ref.Concept, ref.Action)
}

func getErrorCode(err *compiler.CompileError) string {
    // Map error types to codes
    // E001-E099: Compilation errors
    // E100-E199: Validation errors

    switch err.Field {
    case "purpose":
        return "E101"
    case "action", "sync":
        return "E102"
    case "type":
        return "E103"
    default:
        return "E001"
    }
}

func getErrorCodeFromError(err error) string {
    if compileErr, ok := err.(*compiler.CompileError); ok {
        return getErrorCode(compileErr)
    }
    return "E001"
}

func formatCUEError(err error) error {
    // Format CUE SDK errors for CLI display
    return fmt.Errorf("CUE error: %w", err)
}

// CLIResponse represents structured CLI output for --format json
type CLIResponse struct {
    Status string    `json:"status"` // "ok" or "error"
    Data   any       `json:"data,omitempty"`
    Error  *CLIError `json:"error,omitempty"`
}

// CLIError represents a structured error for JSON output
type CLIError struct {
    Code    string `json:"code"`
    Message string `json:"message"`
    Details any    `json:"details,omitempty"`
}
```

### Integration with Root Command

```go
// cmd/nysm/main.go
package main

import (
    "os"

    "github.com/spf13/cobra"
    "github.com/tyler/nysm/internal/cli"
)

var rootCmd = &cobra.Command{
    Use:   "nysm",
    Short: "NYSM - Now You See Me",
    Long: `A framework for building legible software with the WYSIWYG pattern.

NYSM compiles concept and sync specifications into a canonical IR,
executes them with deterministic replay, and validates operational principles.`,
}

func init() {
    // Register compile command
    rootCmd.AddCommand(cli.CompileCmd())
}

func main() {
    if err := rootCmd.Execute(); err != nil {
        os.Exit(1)
    }
}
```

### Test Examples

```go
// internal/cli/compile_test.go
package cli

import (
    "bytes"
    "encoding/json"
    "os"
    "path/filepath"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestCompileValidSpecs(t *testing.T) {
    // Create temp directory with valid specs
    tmpDir := t.TempDir()
    cartSpec := `
concept Cart {
    purpose: "Manages shopping cart"

    action addItem {
        args: {
            item_id: string
            quantity: int
        }
        outputs: [{
            case: "Success"
            fields: { item_id: string, new_quantity: int }
        }]
    }
}
`
    err := os.WriteFile(filepath.Join(tmpDir, "cart.cue"), []byte(cartSpec), 0644)
    require.NoError(t, err)

    // Capture stdout
    var buf bytes.Buffer
    cmd := compileCmd
    cmd.SetOut(&buf)
    cmd.SetArgs([]string{tmpDir})

    err = cmd.Execute()
    require.NoError(t, err)

    output := buf.String()
    assert.Contains(t, output, "✓ Compiled 1 concepts")
    assert.Contains(t, output, "Cart: 1 actions")
}

func TestCompileJSONOutput(t *testing.T) {
    tmpDir := t.TempDir()
    cartSpec := `
concept Cart {
    purpose: "Manages shopping cart"

    action addItem {
        outputs: [{ case: "Success", fields: {} }]
    }
}
`
    err := os.WriteFile(filepath.Join(tmpDir, "cart.cue"), []byte(cartSpec), 0644)
    require.NoError(t, err)

    var buf bytes.Buffer
    cmd := compileCmd
    cmd.SetOut(&buf)
    cmd.SetArgs([]string{tmpDir, "--format", "json"})

    err = cmd.Execute()
    require.NoError(t, err)

    var response CLIResponse
    err = json.Unmarshal(buf.Bytes(), &response)
    require.NoError(t, err)

    assert.Equal(t, "ok", response.Status)
    assert.NotNil(t, response.Data)
}

func TestCompileOutputToFile(t *testing.T) {
    tmpDir := t.TempDir()
    outputFile := filepath.Join(tmpDir, "compiled.json")

    cartSpec := `
concept Cart {
    purpose: "Manages shopping cart"

    action addItem {
        outputs: [{ case: "Success", fields: {} }]
    }
}
`
    err := os.WriteFile(filepath.Join(tmpDir, "cart.cue"), []byte(cartSpec), 0644)
    require.NoError(t, err)

    cmd := compileCmd
    cmd.SetArgs([]string{tmpDir, "--output", outputFile})

    err = cmd.Execute()
    require.NoError(t, err)

    // Verify file was written
    _, err = os.Stat(outputFile)
    require.NoError(t, err)

    // Verify content is valid JSON
    data, err := os.ReadFile(outputFile)
    require.NoError(t, err)

    var result CompilationResult
    err = json.Unmarshal(data, &result)
    require.NoError(t, err)
    assert.Len(t, result.Concepts, 1)
}

func TestCompileErrorReporting(t *testing.T) {
    tmpDir := t.TempDir()
    invalidSpec := `
concept Bad {
    // Missing purpose - should error
    action foo {
        outputs: [{ case: "Success", fields: {} }]
    }
}
`
    err := os.WriteFile(filepath.Join(tmpDir, "bad.cue"), []byte(invalidSpec), 0644)
    require.NoError(t, err)

    var buf bytes.Buffer
    cmd := compileCmd
    cmd.SetOut(&buf)
    cmd.SetArgs([]string{tmpDir})

    err = cmd.Execute()
    require.Error(t, err)

    output := buf.String()
    assert.Contains(t, output, "✗ Compilation failed")
    assert.Contains(t, output, "E101")
    assert.Contains(t, output, "purpose")
}

func TestCompileEmptyDirectory(t *testing.T) {
    tmpDir := t.TempDir()

    cmd := compileCmd
    cmd.SetArgs([]string{tmpDir})

    err := cmd.Execute()
    require.Error(t, err)
    assert.Contains(t, err.Error(), "no CUE files found")
}

func TestCompileMultipleFiles(t *testing.T) {
    tmpDir := t.TempDir()

    // Create multiple concept files
    concepts := map[string]string{
        "cart.cue": `
concept Cart {
    purpose: "Manages shopping cart"
    action addItem {
        outputs: [{ case: "Success", fields: {} }]
    }
}
`,
        "inventory.cue": `
concept Inventory {
    purpose: "Tracks stock"
    action reserve {
        outputs: [{ case: "Success", fields: {} }]
    }
}
`,
    }

    for filename, content := range concepts {
        err := os.WriteFile(filepath.Join(tmpDir, filename), []byte(content), 0644)
        require.NoError(t, err)
    }

    var buf bytes.Buffer
    cmd := compileCmd
    cmd.SetOut(&buf)
    cmd.SetArgs([]string{tmpDir})

    err := cmd.Execute()
    require.NoError(t, err)

    output := buf.String()
    assert.Contains(t, output, "✓ Compiled 2 concepts")
    assert.Contains(t, output, "Cart")
    assert.Contains(t, output, "Inventory")
}
```

### File List

Files to create/modify:

1. `internal/cli/compile.go` - Compile command implementation
2. `internal/cli/compile_test.go` - Comprehensive tests
3. `internal/cli/output.go` - CLIResponse and CLIError types (shared)
4. `cmd/nysm/main.go` - Register compile command with root

### Relationship to Other Stories

- **Story 1.6:** Uses `compiler.CompileConcept` to compile concept specs
- **Story 1.7:** Uses `compiler.CompileSync` to compile sync rules
- **Story 1.4:** Uses `ir.MarshalCanonical` for JSON output
- **Story 7.1:** Integrates with CLI framework setup
- **Story 7.3:** Validate command is similar but faster (skips IR generation)

### Story Completion Checklist

- [ ] compileCmd defined in internal/cli/compile.go
- [ ] --format flag (text|json) implemented
- [ ] --output flag implemented
- [ ] Spec discovery walks directory for .cue files
- [ ] Compilation orchestration compiles all concepts and syncs
- [ ] Human-readable output displays summary
- [ ] JSON output uses CLIResponse struct
- [ ] Error reporting includes file:line:col
- [ ] Error codes assigned (E001-E199)
- [ ] All errors collected before reporting (not fail-fast)
- [ ] File output writes canonical IR
- [ ] Exit code 0 on success, 1 on errors
- [ ] All tests pass
- [ ] `go vet ./internal/cli` passes

### References

- [Source: docs/epics.md#Story 7.2] - Story definition
- [Source: docs/architecture.md#CLI Commands] - CLI structure
- [Source: docs/prd.md#FR-1] - Concept specification requirements
- [Source: docs/architecture.md#Error Codes] - E001-E399 error code scheme

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Validation History

- Initial creation: 2025-12-12

### Completion Notes

- Command uses Cobra CLI framework (v1.10.2)
- Human output by default, JSON with --format json
- Validation errors include file:line references
- --output flag writes canonical IR to file
- Error codes follow E001-E199 scheme for compilation/validation
- Exit codes: 0 = success, 1 = errors
