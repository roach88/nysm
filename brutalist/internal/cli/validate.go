package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/token"
	"github.com/spf13/cobra"

	"github.com/roach88/nysm/internal/compiler"
)

// ValidationResult holds validation results.
type ValidationResult struct {
	Valid  bool                        `json:"valid"`
	Errors []compiler.ValidationError `json:"errors,omitempty"`
}

// NewValidateCommand creates the validate command.
func NewValidateCommand(rootOpts *RootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate <specs-dir>",
		Short: "Validate specs without full compilation",
		Long: `Validate CUE concept specs and sync rules without full compilation.

Performs syntax checking, schema validation, and consistency checks
without generating output files. Faster than compile for development feedback.`,
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true, // Don't print usage on errors
		SilenceErrors: true, // Don't print errors - we handle our own error output
		RunE: func(cmd *cobra.Command, args []string) error {
			return runValidate(rootOpts, args[0], cmd)
		},
	}

	return cmd
}

func runValidate(opts *RootOptions, specsDir string, cmd *cobra.Command) error {
	formatter := &OutputFormatter{
		Format:    opts.Format,
		Writer:    cmd.OutOrStdout(),
		ErrWriter: cmd.ErrOrStderr(), // Verbose logs go to stderr to avoid corrupting JSON
		Verbose:   opts.Verbose,
	}

	// Use shared loader with fail-fast mode for validation
	loadResult, loadErrors := LoadSpecs(specsDir, LoadModeFailFast)

	// Handle load errors (directory not found, no files, etc.)
	if loadResult == nil && len(loadErrors) > 0 {
		var loadErr *LoadError
		if errors.As(loadErrors[0], &loadErr) {
			return outputValidateError(formatter, loadErr.Code, loadErr.Message, nil)
		}
		return outputValidateError(formatter, ErrCodeGeneric, loadErrors[0].Error(), nil)
	}

	formatter.VerboseLog("Found %d CUE file(s) in %s", loadResult.FileCount, specsDir)

	// Validate all concepts and syncs using the loaded CUE value
	validationErrors := validateAll(loadResult.CUEValue, formatter)

	// Add any load errors as validation errors
	for _, err := range loadErrors {
		var loadErr *LoadError
		if errors.As(err, &loadErr) {
			validationErrors = append(validationErrors, compiler.ValidationError{
				Field:   "load",
				Message: loadErr.Message,
				Code:    loadErr.Code,
				Line:    getLineFromCuePos(loadErr.Pos),
			})
		}
	}

	if len(validationErrors) > 0 {
		return outputValidationErrors(formatter, validationErrors)
	}

	// Output success
	return outputValidateSuccess(formatter)
}

// validateAll validates all concepts and syncs in the CUE value.
// Uses fast validation path - parses and validates without generating IR.
func validateAll(value cue.Value, formatter *OutputFormatter) []compiler.ValidationError {
	var allErrors []compiler.ValidationError

	// Validate concepts
	conceptsVal := value.LookupPath(cue.ParsePath("concept"))
	if conceptsVal.Exists() {
		iter, err := conceptsVal.Fields()
		if err == nil {
			for iter.Next() {
				conceptName := iter.Label()
				formatter.VerboseLog("Validating concept: %s", conceptName)

				// Try to compile the concept to get validation
				spec, compileErr := compiler.CompileConcept(iter.Value())
				if compileErr != nil {
					// Convert compile error to validation error
					if cErr, ok := compileErr.(*compiler.CompileError); ok {
						allErrors = append(allErrors, compiler.ValidationError{
							Field:   cErr.Field,
							Message: cErr.Message,
							Code:    mapCompileErrorToCode(cErr.Field),
							Line:    getLineFromTokenPos(cErr.Pos),
						})
					} else {
						allErrors = append(allErrors, compiler.ValidationError{
							Field:   "concept." + conceptName,
							Message: compileErr.Error(),
							Code:    "E001",
						})
					}
					continue
				}

				// Run schema validation on compiled spec
				validationErrs := compiler.Validate(spec)
				allErrors = append(allErrors, validationErrs...)
			}
		}
	}

	// Validate syncs
	syncsVal := value.LookupPath(cue.ParsePath("sync"))
	if syncsVal.Exists() {
		iter, err := syncsVal.Fields()
		if err == nil {
			for iter.Next() {
				syncID := iter.Label()
				formatter.VerboseLog("Validating sync: %s", syncID)

				// Try to compile the sync to get validation
				rule, compileErr := compiler.CompileSync(iter.Value())
				if compileErr != nil {
					// Convert compile error to validation error
					if cErr, ok := compileErr.(*compiler.CompileError); ok {
						allErrors = append(allErrors, compiler.ValidationError{
							Field:   cErr.Field,
							Message: cErr.Message,
							Code:    mapCompileErrorToCode(cErr.Field),
							Line:    getLineFromTokenPos(cErr.Pos),
						})
					} else {
						allErrors = append(allErrors, compiler.ValidationError{
							Field:   "sync." + syncID,
							Message: compileErr.Error(),
							Code:    "E001",
						})
					}
					continue
				}

				// Run schema validation on compiled rule
				validationErrs := compiler.Validate(rule)
				allErrors = append(allErrors, validationErrs...)
			}
		}
	}

	// Check if we found anything
	conceptsVal = value.LookupPath(cue.ParsePath("concept"))
	syncsVal = value.LookupPath(cue.ParsePath("sync"))
	conceptCount := 0
	syncCount := 0

	if conceptsVal.Exists() {
		iter, _ := conceptsVal.Fields()
		for iter.Next() {
			conceptCount++
		}
	}
	if syncsVal.Exists() {
		iter, _ := syncsVal.Fields()
		for iter.Next() {
			syncCount++
		}
	}

	if conceptCount == 0 && syncCount == 0 && len(allErrors) == 0 {
		allErrors = append(allErrors, compiler.ValidationError{
			Field:   "specs",
			Message: "no concepts or syncs found in specs",
			Code:    "E001",
		})
	}

	return allErrors
}

// mapCompileErrorToCode maps a compile error field to a validation error code.
func mapCompileErrorToCode(field string) string {
	switch field {
	case "purpose":
		return compiler.ErrConceptPurposeEmpty
	case "action":
		return compiler.ErrConceptNoActions
	case "outputs":
		return compiler.ErrActionNoOutputs
	case "type":
		return compiler.ErrInvalidFieldType
	case "scope":
		return compiler.ErrInvalidScopeMode
	case "when", "when.action", "when.event":
		return compiler.ErrInvalidActionRef
	case "then", "then.action":
		return compiler.ErrInvalidThenClause
	case "where", "where.from":
		return compiler.ErrInvalidWhereClause
	default:
		return "E001"
	}
}

// getLineFromCuePos extracts line number from a token.Pos.
func getLineFromCuePos(pos token.Pos) int {
	if pos.IsValid() {
		return pos.Line()
	}
	return 0
}

// getLineFromTokenPos extracts line number from a cue token.Pos.
func getLineFromTokenPos(pos interface{ IsValid() bool; Line() int }) int {
	if pos != nil && pos.IsValid() {
		return pos.Line()
	}
	return 0
}

// outputValidateSuccess outputs successful validation results.
func outputValidateSuccess(formatter *OutputFormatter) error {
	if formatter.Format == "json" {
		result := ValidationResult{Valid: true}
		return formatter.Success(result)
	}

	fmt.Fprintln(formatter.Writer, "✓ All specs valid")
	return nil
}

// outputValidateError outputs a single validation error.
func outputValidateError(formatter *OutputFormatter, code, message string, details interface{}) error {
	_ = formatter.Error(code, message, details)
	// Validation errors are command-level errors (exit code 2)
	return NewExitError(ExitCommandError, fmt.Sprintf("%s: %s", code, message))
}

// outputValidationErrors outputs multiple validation errors.
func outputValidationErrors(formatter *OutputFormatter, errs []compiler.ValidationError) error {
	if formatter.Format == "json" {
		result := ValidationResult{
			Valid:  false,
			Errors: errs,
		}

		response := CLIResponse{
			Status: "error",
			Data:   result,
			Error: &CLIError{
				Code:    errs[0].Code,
				Message: errs[0].Message,
			},
		}

		encoder := json.NewEncoder(formatter.Writer)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(response); err != nil {
			return err
		}

		// Validation failures = exit code 1 (test/validation failure)
		return NewExitError(ExitFailure, fmt.Sprintf("validation failed with %d error(s)", len(errs)))
	}

	// Text format
	fmt.Fprintln(formatter.Writer, "✗ Validation failed")
	fmt.Fprintln(formatter.Writer)

	for _, err := range errs {
		if err.Line > 0 {
			fmt.Fprintf(formatter.Writer, "line %d\n", err.Line)
		}
		fmt.Fprintf(formatter.Writer, "  %s: %s\n\n", err.Code, err.Message)
	}

	// Validation failures = exit code 1 (test/validation failure)
	return NewExitError(ExitFailure, fmt.Sprintf("validation failed with %d error(s)", len(errs)))
}

// ValidateSpecsDir validates all specs in a directory.
// This is a helper function for external callers.
func ValidateSpecsDir(specsDir string) ([]compiler.ValidationError, error) {
	// Use shared loader
	loadResult, loadErrors := LoadSpecs(specsDir, LoadModeFailFast)
	if loadResult == nil && len(loadErrors) > 0 {
		return nil, loadErrors[0]
	}

	// Create a silent formatter for validateAll
	silentFormatter := &OutputFormatter{Format: "text", Verbose: false, Writer: io.Discard}
	validationErrs := validateAll(loadResult.CUEValue, silentFormatter)

	return validationErrs, nil
}
