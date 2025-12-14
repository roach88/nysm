package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// Exit codes for CLI commands.
const (
	ExitSuccess        = 0 // Successful execution
	ExitFailure        = 1 // Test/validation failure (scenarios failed, non-deterministic replay, etc.)
	ExitCommandError   = 2 // Command error (invalid paths, database not found, etc.)
)

// ExitError represents an error with a specific exit code.
// Use this to return errors with meaningful exit codes from CLI commands.
type ExitError struct {
	Code    int    // Exit code (use ExitFailure or ExitCommandError)
	Message string // Error message
	Err     error  // Underlying error (optional)
}

func (e *ExitError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *ExitError) Unwrap() error {
	return e.Err
}

// NewExitError creates a new ExitError with the given code and message.
func NewExitError(code int, message string) *ExitError {
	return &ExitError{Code: code, Message: message}
}

// WrapExitError wraps an existing error with an exit code.
func WrapExitError(code int, message string, err error) *ExitError {
	return &ExitError{Code: code, Message: message, Err: err}
}

// GetExitCode extracts the exit code from an error.
// Returns ExitFailure (1) if the error is not an ExitError.
func GetExitCode(err error) int {
	var exitErr *ExitError
	if errors.As(err, &exitErr) {
		return exitErr.Code
	}
	return ExitFailure
}

// OutputFormatter handles JSON vs text output for CLI commands.
type OutputFormatter struct {
	Format    string
	Writer    io.Writer
	ErrWriter io.Writer // Separate writer for verbose/diagnostic output (defaults to Writer)
	Verbose   bool
}

// CLIResponse is the standard JSON response format for CLI output.
type CLIResponse struct {
	Status  string      `json:"status"`            // "ok" or "error"
	Data    interface{} `json:"data,omitempty"`    // success payload
	Error   *CLIError   `json:"error,omitempty"`   // error details
	TraceID string      `json:"trace_id,omitempty"` // optional trace correlation
}

// CLIError is the error structure for CLI responses.
type CLIError struct {
	Code    string      `json:"code"`              // "E001", "E002", etc.
	Message string      `json:"message"`           // human-readable message
	Details interface{} `json:"details,omitempty"` // additional context
}

// Success outputs a successful result in the configured format.
func (f *OutputFormatter) Success(data interface{}) error {
	if f.Format == "json" {
		return json.NewEncoder(f.Writer).Encode(CLIResponse{
			Status: "ok",
			Data:   data,
		})
	}

	// Human-readable text output
	fmt.Fprintln(f.Writer, data)
	return nil
}

// Error outputs an error in the configured format.
func (f *OutputFormatter) Error(code, message string, details interface{}) error {
	if f.Format == "json" {
		return json.NewEncoder(f.Writer).Encode(CLIResponse{
			Status: "error",
			Error: &CLIError{
				Code:    code,
				Message: message,
				Details: details,
			},
		})
	}

	// Human-readable error
	fmt.Fprintf(f.Writer, "Error [%s]: %s\n", code, message)
	if f.Verbose && details != nil {
		fmt.Fprintf(f.Writer, "Details: %v\n", details)
	}
	return nil
}

// VerboseLog outputs a message only if verbose mode is enabled.
// Uses ErrWriter if set, otherwise falls back to Writer.
// When format is JSON, verbose logs go to ErrWriter to avoid corrupting JSON output.
func (f *OutputFormatter) VerboseLog(format string, args ...interface{}) {
	if !f.Verbose {
		return
	}
	w := f.ErrWriter
	if w == nil {
		w = f.Writer
	}
	fmt.Fprintf(w, format+"\n", args...)
}

// GetErrWriter returns the appropriate writer for diagnostic output.
// Returns ErrWriter if set, otherwise Writer.
func (f *OutputFormatter) GetErrWriter() io.Writer {
	if f.ErrWriter != nil {
		return f.ErrWriter
	}
	return f.Writer
}
