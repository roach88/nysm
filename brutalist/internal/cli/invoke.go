package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// InvokeOptions holds flags for the invoke command.
type InvokeOptions struct {
	*RootOptions
	Args string
}

// NewInvokeCommand creates the invoke command.
func NewInvokeCommand(rootOpts *RootOptions) *cobra.Command {
	opts := &InvokeOptions{RootOptions: rootOpts}

	cmd := &cobra.Command{
		Use:   "invoke <action-uri>",
		Short: "Invoke an action on the running engine",
		Long: `Invoke an action on the running engine.

For MVP, this sends an invocation to the engine via a simple mechanism
(file-based or in-memory for same process). Future versions will use HTTP.

Example:
  nysm invoke Cart.addItem --args '{"item_id":"widget","quantity":3}'`,
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return invokeAction(opts, args[0], cmd)
		},
	}

	cmd.Flags().StringVar(&opts.Args, "args", "{}", "action arguments as JSON")

	return cmd
}

func invokeAction(opts *InvokeOptions, actionURI string, cmd *cobra.Command) error {
	// Validate args JSON
	var argsMap map[string]interface{}
	if err := json.Unmarshal([]byte(opts.Args), &argsMap); err != nil {
		return fmt.Errorf("invalid --args JSON: %w", err)
	}

	// TODO: For MVP, this is a stub that prints instructions
	// Future: Send invocation to running engine via IPC/HTTP

	fmt.Fprintf(cmd.OutOrStdout(), "Invocation request:\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  Action: %s\n", actionURI)
	fmt.Fprintf(cmd.OutOrStdout(), "  Args: %s\n", opts.Args)
	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintln(cmd.OutOrStdout(), "Note: For MVP, the engine must be run in the same process.")
	fmt.Fprintln(cmd.OutOrStdout(), "Use the test harness (nysm test) to execute flows.")

	return fmt.Errorf("invoke subcommand not yet implemented - use 'nysm test' for MVP")
}
