package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/roach88/nysm/internal/compiler"
	"github.com/roach88/nysm/internal/ir"
)

// CompileOptions holds flags for the compile command.
type CompileOptions struct {
	*RootOptions
	Output string // output file path
}

// CompilationResult holds the compiled concepts and sync rules.
type CompilationResult struct {
	Concepts []ir.ConceptSpec `json:"concepts"`
	Syncs    []ir.SyncRule    `json:"syncs"`
}

// CompilationStats holds summary statistics.
type CompilationStats struct {
	ConceptCount             int
	SyncCount                int
	TotalActions             int
	TotalOperationalPrinciples int
}

// NewCompileCommand creates the compile command.
func NewCompileCommand(rootOpts *RootOptions) *cobra.Command {
	opts := &CompileOptions{RootOptions: rootOpts}

	cmd := &cobra.Command{
		Use:   "compile <specs-dir>",
		Short: "Compile CUE specs to canonical IR",
		Long: `Compile CUE concept specs and sync rules to canonical IR format.

The compiler parses CUE files, validates them against the IR schema,
and outputs canonical JSON for use by the engine.`,
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true, // Don't print usage on errors - we handle our own error output
		SilenceErrors: true, // Don't print errors - we handle our own error output
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCompile(opts, args[0], cmd)
		},
	}

	cmd.Flags().StringVarP(&opts.Output, "output", "o", "", "output file path")

	return cmd
}

func runCompile(opts *CompileOptions, specsDir string, cmd *cobra.Command) error {
	formatter := &OutputFormatter{
		Format:    opts.Format,
		Writer:    cmd.OutOrStdout(),
		ErrWriter: cmd.ErrOrStderr(), // Verbose logs go to stderr to avoid corrupting JSON
		Verbose:   opts.Verbose,
	}

	// Use shared loader with collect-all mode
	loadResult, loadErrors := LoadSpecs(specsDir, LoadModeCollectAll)

	// Handle load errors (directory not found, no files, etc.)
	if loadResult == nil && len(loadErrors) > 0 {
		var loadErr *LoadError
		if errors.As(loadErrors[0], &loadErr) {
			return outputCompileError(formatter, loadErr.Code, loadErr.Message, nil)
		}
		return outputCompileError(formatter, ErrCodeGeneric, loadErrors[0].Error(), nil)
	}

	formatter.VerboseLog("Found %d CUE file(s) in %s", loadResult.FileCount, specsDir)

	// Log verbose output for each concept/sync
	for _, concept := range loadResult.Concepts {
		formatter.VerboseLog("Compiling concept: %s", concept.Name)
	}
	for _, sync := range loadResult.Syncs {
		formatter.VerboseLog("Compiling sync: %s", sync.ID)
	}

	// Handle compilation errors
	if len(loadErrors) > 0 {
		return outputCompileErrors(formatter, loadErrors)
	}

	// Build result
	result := &CompilationResult{
		Concepts: loadResult.Concepts,
		Syncs:    loadResult.Syncs,
	}

	// Calculate statistics
	stats := calculateStats(result)

	// Write to file if --output specified
	if opts.Output != "" {
		if err := writeIRToFile(result, opts.Output); err != nil {
			return outputCompileError(formatter, ErrCodeWriteFailed, fmt.Sprintf("writing output file: %v", err), nil)
		}
	}

	// Output success
	return outputCompileSuccess(formatter, result, stats, opts.Output)
}

// calculateStats computes summary statistics from compilation result.
func calculateStats(result *CompilationResult) CompilationStats {
	stats := CompilationStats{
		ConceptCount: len(result.Concepts),
		SyncCount:    len(result.Syncs),
	}

	for _, concept := range result.Concepts {
		stats.TotalActions += len(concept.Actions)
		stats.TotalOperationalPrinciples += len(concept.OperationalPrinciples)
	}

	return stats
}

// outputCompileSuccess outputs successful compilation results.
func outputCompileSuccess(formatter *OutputFormatter, result *CompilationResult, stats CompilationStats, outputFile string) error {
	if formatter.Format == "json" {
		return formatter.Success(result)
	}

	// Human-readable text output
	fmt.Fprintf(formatter.Writer, "✓ Compiled %d concept(s), %d sync(s)\n\n",
		stats.ConceptCount, stats.SyncCount)

	if len(result.Concepts) > 0 {
		fmt.Fprintln(formatter.Writer, "Concepts:")
		for _, concept := range result.Concepts {
			opCount := len(concept.OperationalPrinciples)
			opSuffix := "principles"
			if opCount == 1 {
				opSuffix = "principle"
			}
			fmt.Fprintf(formatter.Writer, "  %s: %d action(s), %d operational %s\n",
				concept.Name, len(concept.Actions), opCount, opSuffix)
		}
		fmt.Fprintln(formatter.Writer)
	}

	if len(result.Syncs) > 0 {
		fmt.Fprintln(formatter.Writer, "Syncs:")
		for _, sync := range result.Syncs {
			fmt.Fprintf(formatter.Writer, "  %s: %s → %s\n",
				sync.ID, sync.When.ActionRef, sync.Then.ActionRef)
		}
		fmt.Fprintln(formatter.Writer)
	}

	if outputFile != "" {
		fmt.Fprintf(formatter.Writer, "Wrote canonical IR to %s\n", outputFile)
	}

	return nil
}

// outputCompileError outputs a single compilation error.
func outputCompileError(formatter *OutputFormatter, code, message string, details interface{}) error {
	_ = formatter.Error(code, message, details)
	// Compilation errors are command-level errors (exit code 2)
	return WrapExitError(ExitCommandError, fmt.Sprintf("%s: %s", code, message), nil)
}

// outputCompileErrors outputs multiple compilation errors.
func outputCompileErrors(formatter *OutputFormatter, errs []error) error {
	if formatter.Format == "json" {
		// JSON format - use CLIResponse with first error
		cliErrors := make([]CLIError, len(errs))
		for i, err := range errs {
			code, message := parseCompileError(err)
			cliErrors[i] = CLIError{
				Code:    code,
				Message: message,
			}
		}

		response := CLIResponse{
			Status: "error",
			Error:  &cliErrors[0],
			Data:   cliErrors, // Include all errors in data
		}

		encoder := json.NewEncoder(formatter.Writer)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(response); err != nil {
			return err
		}

		// Compilation errors are command-level errors (exit code 2)
		return NewExitError(ExitCommandError, fmt.Sprintf("compilation failed with %d error(s)", len(errs)))
	}

	// Text format
	fmt.Fprintln(formatter.Writer, "✗ Compilation failed")
	fmt.Fprintln(formatter.Writer)

	for _, err := range errs {
		code, message := parseCompileError(err)
		var compileErr *compiler.CompileError
		if errors.As(err, &compileErr) && compileErr.Pos.IsValid() {
			fmt.Fprintf(formatter.Writer, "%s:%d:%d\n",
				compileErr.Pos.Filename(),
				compileErr.Pos.Line(),
				compileErr.Pos.Column())
		}
		var loadErr *LoadError
		if errors.As(err, &loadErr) && loadErr.Pos.IsValid() {
			fmt.Fprintf(formatter.Writer, "%s:%d:%d\n",
				loadErr.Pos.Filename(),
				loadErr.Pos.Line(),
				loadErr.Pos.Column())
		}
		fmt.Fprintf(formatter.Writer, "  %s: %s\n\n", code, message)
	}

	// Compilation errors are command-level errors (exit code 2)
	return NewExitError(ExitCommandError, fmt.Sprintf("compilation failed with %d error(s)", len(errs)))
}

// parseCompileError extracts error code and message from an error.
func parseCompileError(err error) (string, string) {
	var compileErr *compiler.CompileError
	if errors.As(err, &compileErr) {
		code := MapFieldToErrorCode(compileErr.Field)
		return code, compileErr.Message
	}
	var loadErr *LoadError
	if errors.As(err, &loadErr) {
		return loadErr.Code, loadErr.Message
	}
	return ErrCodeGeneric, err.Error()
}

// writeIRToFile writes the compilation result to a file in canonical JSON format.
func writeIRToFile(result *CompilationResult, filename string) error {
	// Use standard JSON with indentation for readability
	// (canonical JSON without indentation is used only for hashing)
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling IR: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("writing file: %w", err)
	}

	return nil
}
