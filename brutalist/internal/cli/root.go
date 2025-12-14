package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// RootOptions holds global flags for all commands.
type RootOptions struct {
	Verbose bool
	Format  string // "json" | "text"
}

// ValidFormats defines the allowed output formats.
var ValidFormats = []string{"text", "json"}

// NewRootCommand creates the root command for the NYSM CLI.
func NewRootCommand() *cobra.Command {
	opts := &RootOptions{}

	cmd := &cobra.Command{
		Use:   "nysm",
		Short: "NYSM - Now You See Me",
		Long:  "A framework for building legible software with the WYSIWYG pattern.",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Validate format flag
			if !isValidFormat(opts.Format) {
				return fmt.Errorf("invalid format %q: must be one of %v", opts.Format, ValidFormats)
			}
			return nil
		},
	}

	// Global flags
	cmd.PersistentFlags().BoolVarP(&opts.Verbose, "verbose", "v", false, "verbose output")
	cmd.PersistentFlags().StringVar(&opts.Format, "format", "text", "output format (json|text)")

	// Add subcommands
	cmd.AddCommand(NewCompileCommand(opts))
	cmd.AddCommand(NewValidateCommand(opts))
	cmd.AddCommand(NewRunCommand(opts))
	cmd.AddCommand(NewInvokeCommand(opts))
	cmd.AddCommand(NewReplayCommand(opts))
	cmd.AddCommand(NewTestCommand(opts))
	cmd.AddCommand(NewTraceCommand(opts))

	return cmd
}

// isValidFormat checks if the format is one of the allowed values.
func isValidFormat(format string) bool {
	for _, f := range ValidFormats {
		if f == format {
			return true
		}
	}
	return false
}
